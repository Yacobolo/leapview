import { createHash } from "node:crypto";
import { get } from "node:http";

import { describe, expect, test } from "bun:test";

import {
  authenticateDesktopProfile,
  disconnectDesktopProfile,
  desktopSessionAvailable,
  DesktopAuthenticationError,
  type DesktopAuthProfile,
} from "./auth.js";

const profile: DesktopAuthProfile = {
  id: "profile_0123456789abcdef0123456789abcdef",
  canonicalOrigin: "https://analytics.company.com",
  instanceId: "instance_0123456789abcdef0123456789abcdef",
  lastSafePath: "/workspaces",
};

describe("desktop system-browser authentication", () => {
  test("checks session state without following redirects", async () => {
    let observed: { input: string; init: RequestInit } | undefined;
    const available = await desktopSessionAvailable(
      profile,
      async (input, init) => {
        observed = { input, init };
        return new Response(null, { status: 204 });
      },
    );
    expect(available).toBe(true);
    expect(observed?.input).toBe(
      "https://analytics.company.com/auth/desktop/session",
    );
    expect(observed?.init.credentials).toBe("include");
    expect(observed?.init.redirect).toBe("error");
    expect(
      await desktopSessionAvailable(
        profile,
        async () => new Response(null, { status: 401 }),
      ),
    ).toBe(false);
  });

  test("disconnects only the bound desktop profile session", async () => {
    let observed: { input: string; init: RequestInit } | undefined;
    await disconnectDesktopProfile(profile, async (input, init) => {
      observed = { input, init };
      return new Response(null, { status: 204 });
    });
    expect(observed?.input).toBe(
      "https://analytics.company.com/auth/desktop/disconnect",
    );
    expect(observed?.init.credentials).toBe("include");
    expect(observed?.init.redirect).toBe("error");
    expect(new URLSearchParams(String(observed?.init.body)).get("profile_id"))
      .toBe(profile.id);
  });

  test("uses loopback S256 PKCE and redeems only through the profile session", async () => {
    let redemption: { input: string; init: RequestInit } | undefined;
    let authorizationChallenge = "";
    const openExternal = async (rawURL: string): Promise<void> => {
      const authorization = new URL(rawURL);
      expect(authorization.origin).toBe(profile.canonicalOrigin);
      expect(authorization.pathname).toBe("/auth/desktop/authorize");
      expect(authorization.searchParams.get("client_id")).toBe(
        "leapview-desktop",
      );
      expect(authorization.searchParams.get("response_type")).toBe("code");
      expect(authorization.searchParams.get("code_challenge_method")).toBe(
        "S256",
      );
      authorizationChallenge =
        authorization.searchParams.get("code_challenge") ?? "";
      expect(authorization.searchParams.get("instance_id")).toBe(
        profile.instanceId,
      );
      expect(authorization.searchParams.get("profile_id")).toBe(profile.id);
      expect(authorization.searchParams.get("return_path")).toBe(
        profile.lastSafePath,
      );
      const redirectURI = authorization.searchParams.get("redirect_uri");
      const state = authorization.searchParams.get("state");
      expect(redirectURI).toMatch(/^http:\/\/127\.0\.0\.1:\d+\/callback$/u);
      expect(state).toMatch(/^[A-Za-z0-9_-]{43}$/u);
      await requestLoopback(
        `${redirectURI}?${new URLSearchParams({
          code: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
          state: state!,
        })}`,
      );
    };
    const fetcher = async (
      input: string,
      init: RequestInit,
    ): Promise<Response> => {
      redemption = { input, init };
      return new Response(null, { status: 204 });
    };

    await authenticateDesktopProfile(profile, fetcher, openExternal);

    expect(redemption?.input).toBe(
      "https://analytics.company.com/auth/desktop/redeem",
    );
    expect(redemption?.init.method).toBe("POST");
    expect(redemption?.init.credentials).toBe("include");
    expect(redemption?.init.redirect).toBe("error");
    const form = new URLSearchParams(String(redemption?.init.body));
    expect(form.get("client_id")).toBe("leapview-desktop");
    expect(form.get("code")).toBe(
      "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
    );
    expect(form.get("instance_id")).toBe(profile.instanceId);
    expect(form.get("profile_id")).toBe(profile.id);
    expect(form.get("redirect_uri")).toMatch(
      /^http:\/\/127\.0\.0\.1:\d+\/callback$/u,
    );
    const verifier = form.get("code_verifier")!;
    expect(verifier).toMatch(/^[A-Za-z0-9_-]{43}$/u);
    const challenge = createHash("sha256")
      .update(verifier)
      .digest("base64url");
    expect(challenge).toBe(authorizationChallenge);
  });

  test("rejects a callback with the wrong state and closes the listener", async () => {
    const openExternal = async (rawURL: string): Promise<void> => {
      const authorization = new URL(rawURL);
      const redirectURI = authorization.searchParams.get("redirect_uri")!;
      const response = await requestLoopback(
        `${redirectURI}?${new URLSearchParams({
          code: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
          state: "wrong-state",
        })}`,
      );
      expect(response.status).toBe(400);
      expect(response.body).not.toContain("wrong-state");
    };

    await expect(
      authenticateDesktopProfile(profile, failIfFetched, openExternal),
    ).rejects.toBeInstanceOf(DesktopAuthenticationError);
  });

  test("times out and never redeems when the browser does not return", async () => {
    await expect(
      authenticateDesktopProfile(
        profile,
        failIfFetched,
        async () => undefined,
        { callbackTimeoutMs: 25 },
      ),
    ).rejects.toThrow("timed out");
  });

  test("fails closed when redemption is rejected", async () => {
    const openExternal = callbackWithValidCode;
    await expect(
      authenticateDesktopProfile(
        profile,
        async () => new Response(null, { status: 401 }),
        openExternal,
      ),
    ).rejects.toThrow("could not establish");
  });
});

async function callbackWithValidCode(rawURL: string): Promise<void> {
  const authorization = new URL(rawURL);
  await requestLoopback(
    `${authorization.searchParams.get("redirect_uri")}?${new URLSearchParams({
      code: "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopq",
      state: authorization.searchParams.get("state")!,
    })}`,
  );
}

async function failIfFetched(): Promise<Response> {
  throw new Error("profile fetch must not run");
}

async function requestLoopback(
  rawURL: string,
): Promise<{ status: number; body: string }> {
  return new Promise((resolve, reject) => {
    get(rawURL, (response) => {
      const chunks: Buffer[] = [];
      response.on("data", (chunk: Buffer) => chunks.push(chunk));
      response.on("end", () =>
        resolve({
          status: response.statusCode ?? 0,
          body: Buffer.concat(chunks).toString("utf8"),
        }),
      );
    }).on("error", reject);
  });
}
