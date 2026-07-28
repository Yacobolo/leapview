import { describe, expect, test } from "bun:test";

import { TrustedUI } from "./trusted-ui.js";

describe("TrustedUI", () => {
  test("serves a no-script connection screen with restrictive headers", async () => {
    const ui = new TrustedUI({
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

  test("connect form invokes only the origin action", async () => {
    const origins: string[] = [];
    const profiles: string[] = [];
    const ui = new TrustedUI({
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
    const ui = new TrustedUI({
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
    const ui = new TrustedUI({
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
    const ui = new TrustedUI({
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
