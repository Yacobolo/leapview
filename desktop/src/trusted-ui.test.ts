import { describe, expect, test } from "bun:test";

import { TrustedUI } from "./trusted-ui.js";

const trustedAssets = {
  stylesheet: ":root { --lv-bg-app: canvas; }",
  fonts: new Map<string, ArrayBuffer>(),
};

function trustedUI(actions: ConstructorParameters<typeof TrustedUI>[0]) {
  return new TrustedUI(actions, trustedAssets);
}

describe("TrustedUI", () => {
  test("serves a no-script connection screen with restrictive headers", async () => {
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      connectOrigin: async () => undefined,
      connectProfile: async () => undefined,
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [],
    });
    const response = await ui.handle(new Request("leapview://app/"));
    const body = await response.text();

    expect(response.status).toBe(200);
    expect(response.headers.get("content-security-policy")).toContain(
      "default-src 'none'",
    );
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(body).not.toContain("<script");
    expect(body).toContain("https://analytics.company.com");
  });

  test("uses the canonical LeapView theme and semantic design tokens", async () => {
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      connectOrigin: async () => undefined,
      connectProfile: async () => undefined,
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [],
    });

    const body = await (
      await ui.handle(new Request("leapview://app/"))
    ).text();
    const stylesheet = await ui.handle(
      new Request("leapview://app/app.css"),
    );
    const componentCSS = body.match(/<style>([\s\S]*?)<\/style>/u)?.[1] ?? "";

    expect(body).toContain('data-color-mode="auto"');
    expect(body).toContain('data-light-theme="light"');
    expect(body).toContain('data-dark-theme="dark"');
    expect(body).toContain('href="leapview://app/app.css"');
    expect(body).toContain('aria-label="LeapView"');
    expect(componentCSS).toContain("var(--lv-bg-app)");
    expect(componentCSS).toContain("var(--lv-button-accent-bg-rest)");
    expect(componentCSS).toContain("var(--base-size-24)");
    expect(componentCSS).not.toMatch(/#[0-9a-f]{3,8}\b/iu);
    expect(stylesheet.headers.get("content-type")).toBe(
      "text/css; charset=utf-8",
    );
    expect(await stylesheet.text()).toBe(trustedAssets.stylesheet);
  });

  test("connect form invokes only the origin action", async () => {
    const origins: string[] = [];
    const profiles: string[] = [];
    const ui = trustedUI({
      allowLoopbackHTTP: true,
      connectOrigin: async (origin) => {
        origins.push(origin);
      },
      connectProfile: async (profileID) => {
        profiles.push(profileID);
      },
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [],
    });
    const response = await ui.handle(
      new Request("leapview://app/connect", {
        method: "POST",
        headers: { "content-type": "application/x-www-form-urlencoded" },
        body: "origin=http%3A%2F%2Flocalhost%3A8080",
      }),
    );

    expect(response.status).toBe(200);
    expect(await response.text()).toContain("Instance verified and opened.");
    expect(origins).toEqual(["http://localhost:8080"]);
    expect(profiles).toEqual([]);
  });

  test("escapes saved server-controlled display metadata", async () => {
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      connectOrigin: async () => undefined,
      connectProfile: async () => undefined,
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [
        {
          id: "profile_0123456789abcdef0123456789abcdef",
          canonicalOrigin: "https://analytics.company.com",
          instanceId: "instance_0123456789abcdef0123456789abcdef",
          displayName: `<img src=x onerror="alert(1)">`,
          lastSafePath: "/",
        },
      ],
    });
    const body = await (await ui.handle(new Request("leapview://app/"))).text();

    expect(body).not.toContain("<img");
    expect(body).toContain("&lt;img");
  });

  test("disconnect and remove are distinct trusted profile actions", async () => {
    const disconnected: string[] = [];
    const removed: string[] = [];
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      connectOrigin: async () => undefined,
      connectProfile: async () => undefined,
      disconnectProfile: async (profileID) => {
        disconnected.push(profileID);
      },
      removeProfile: async (profileID) => {
        removed.push(profileID);
      },
      listProfiles: async () => [],
    });
    const profileID = "profile_0123456789abcdef0123456789abcdef";
    for (const operation of ["disconnect", "remove"]) {
      const response = await ui.handle(
        new Request("leapview://app/connect", {
          method: "POST",
          headers: { "content-type": "application/x-www-form-urlencoded" },
          body: new URLSearchParams({ profileId: profileID, operation }),
        }),
      );
      expect(response.status).toBe(200);
    }
    expect(disconnected).toEqual([profileID]);
    expect(removed).toEqual([profileID]);
  });

  test("rejects oversized connection forms before invoking actions", async () => {
    let invoked = false;
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      connectOrigin: async () => {
        invoked = true;
      },
      connectProfile: async () => {
        invoked = true;
      },
      disconnectProfile: async () => {
        invoked = true;
      },
      removeProfile: async () => {
        invoked = true;
      },
      listProfiles: async () => [],
    });
    const response = await ui.handle(
      new Request("leapview://app/connect", {
        method: "POST",
        body: `origin=${"a".repeat(4_097)}`,
      }),
    );

    expect(await response.text()).toContain("too large");
    expect(invoked).toBeFalse();
  });
});
