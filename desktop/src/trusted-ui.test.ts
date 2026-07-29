import { describe, expect, test } from "bun:test";

import { DesktopDiscoveryError } from "./discovery.js";
import { TrustedUI } from "./trusted-ui.js";

const trustedAssets = {
  stylesheet: ":root { --lv-bg-app: canvas; }",
  fonts: new Map<string, ArrayBuffer>(),
};

type TrustedUIActions = ConstructorParameters<typeof TrustedUI>[0];
type TrustedUITestActions = Omit<TrustedUIActions, "policy"> & {
  policy?: TrustedUIActions["policy"];
};

const openPolicy: TrustedUIActions["policy"] = {
  mode: "open",
  allowUserAddedInstances: true,
  diagnosticsEnabled: true,
  preconfiguredOrigins: [],
  revision: "desktop-policy-v1",
};

function trustedUI(actions: TrustedUITestActions) {
  const { policy = openPolicy, ...testActions } = actions;
  return new TrustedUI({ ...testActions, policy }, trustedAssets);
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

    expect(response.status).toBe(303);
    const operationURL = response.headers.get("location") ?? "";
    expect(operationURL).toMatch(
      /^leapview:\/\/app\/operations\/[0-9a-f]{32}$/u,
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
    const completed = await ui.handle(new Request(operationURL));
    expect(await completed.text()).toContain("Instance verified and opened.");
    expect(origins).toEqual(["http://localhost:8080"]);
    expect(profiles).toEqual([]);
  });

  test("shows a safe no-script connecting state while an operation is pending", async () => {
    let finish: () => void = () => undefined;
    const pending = new Promise<void>((resolve) => {
      finish = resolve;
    });
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      connectOrigin: async () => pending,
      connectProfile: async () => undefined,
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [],
    });
    const response = await Promise.race([
      ui.handle(
        new Request("leapview://app/connect", {
          method: "POST",
          body: "origin=https%3A%2F%2Fanalytics.company.com",
        }),
      ),
      new Promise<null>((resolve) => setTimeout(() => resolve(null), 50)),
    ]);

    expect(response).not.toBeNull();
    expect(response?.status).toBe(303);
    const operationURL = response?.headers.get("location") ?? "";
    const status = await ui.handle(new Request(operationURL));
    const body = await status.text();
    expect(body).toContain('data-state="connecting"');
    expect(body).toContain("Verifying the instance");
    expect(body).toContain('http-equiv="refresh"');
    expect(body).not.toContain("<script");

    finish();
    await pending;
    await new Promise((resolve) => setTimeout(resolve, 0));
    const completed = await ui.handle(new Request(operationURL));
    expect(await completed.text()).toContain("Instance verified and opened.");
  });

  test("classifies compatibility and offline failures without exposing causes", async () => {
    for (const [error, state, safeMessage] of [
      [
        new DesktopDiscoveryError(
          "protocol_incompatible",
          "the server desktop protocol is not compatible with this client",
        ),
        "incompatible",
        "not compatible",
      ],
      [
        new DesktopDiscoveryError("network", "instance discovery failed"),
        "offline",
        "could not be reached",
      ],
      [
        new DesktopDiscoveryError("tls", "instance discovery failed"),
        "tls-error",
        "certificate",
      ],
      [
        new DesktopDiscoveryError("proxy", "instance discovery failed"),
        "proxy-error",
        "proxy",
      ],
      [
        new DesktopDiscoveryError("dns", "instance discovery failed"),
        "dns-error",
        "could not be resolved",
      ],
      [
        new DesktopDiscoveryError(
          "malformed_response",
          "instance discovery returned invalid JSON",
        ),
        "invalid-instance",
        "invalid discovery",
      ],
      [
        new DesktopDiscoveryError(
          "invalid_origin",
          "instance URL must use HTTPS",
        ),
        "invalid-instance",
        "HTTPS",
      ],
    ] as const) {
      const ui = trustedUI({
        allowLoopbackHTTP: false,
        connectOrigin: async () => {
          throw error;
        },
        connectProfile: async () => undefined,
        disconnectProfile: async () => undefined,
        removeProfile: async () => undefined,
        listProfiles: async () => [],
      });
      const response = await ui.handle(
        new Request("leapview://app/connect", {
          method: "POST",
          body: "origin=https%3A%2F%2Fanalytics.company.com",
        }),
      );
      await new Promise((resolve) => setTimeout(resolve, 0));
      const operationURL = response.headers.get("location") ?? "";
      const body = await (await ui.handle(new Request(operationURL))).text();

      expect(body).toContain(`data-state="${state}"`);
      expect(body).toContain(safeMessage);
      expect(body).not.toContain("secret.internal");
    }
  });

  test("renders trusted lifecycle notices reported by the main process", async () => {
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      connectOrigin: async () => undefined,
      connectProfile: async () => undefined,
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [],
    });
    ui.reportNotice({
      kind: "error",
      state: "crashed",
      message: "Company Analytics stopped unexpectedly. Reopen it to continue.",
    });

    const body = await (await ui.handle(new Request("leapview://app/"))).text();
    expect(body).toContain('data-state="crashed"');
    expect(body).toContain("stopped unexpectedly");
    expect(body).toContain('role="alert"');
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
      expect(response.status).toBe(303);
      await new Promise((resolve) => setTimeout(resolve, 0));
      await ui.handle(
        new Request(response.headers.get("location") ?? ""),
      );
    }
    expect(disconnected).toEqual([profileID]);
    expect(removed).toEqual([profileID]);
  });

  test("completed operations do not exhaust the active operation limit", async () => {
    let connections = 0;
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      connectOrigin: async () => {
        connections += 1;
      },
      connectProfile: async () => undefined,
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [],
    });

    for (let attempt = 0; attempt < 17; attempt += 1) {
      const response = await ui.handle(
        new Request("leapview://app/connect", {
          method: "POST",
          body: "origin=https%3A%2F%2Fanalytics.company.com",
        }),
      );
      expect(response.status).toBe(303);
      await new Promise((resolve) => setTimeout(resolve, 0));
    }

    expect(connections).toBe(17);
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

  test("renders only policy-managed profiles and origins when user additions are disabled", async () => {
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      policy: {
        mode: "managed",
        allowUserAddedInstances: false,
        diagnosticsEnabled: true,
        preconfiguredOrigins: [
          "https://analytics.company.com",
          "https://finance.company.com",
        ],
        revision: "desktop-policy-v1-managed-0123456789abcdef",
      },
      connectOrigin: async () => undefined,
      connectProfile: async () => undefined,
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [
        {
          id: "profile_0123456789abcdef0123456789abcdef",
          canonicalOrigin: "https://analytics.company.com",
          instanceId: "instance_0123456789abcdef0123456789abcdef",
          displayName: "Managed Analytics",
          lastSafePath: "/",
        },
        {
          id: "profile_abcdef0123456789abcdef0123456789",
          canonicalOrigin: "https://personal.example.com",
          instanceId: "instance_abcdef0123456789abcdef0123456789",
          displayName: "Personal Analytics",
          lastSafePath: "/",
        },
      ],
    });

    const body = await (await ui.handle(new Request("leapview://app/"))).text();
    expect(body).toContain("Managed by your organization");
    expect(body).toContain("Managed Analytics");
    expect(body).toContain("https://finance.company.com");
    expect(body).not.toContain("Personal Analytics");
    expect(body).not.toContain('id="origin"');
    expect(body).not.toContain('value="remove"');
  });

  test("keeps user-added instances available when managed policy permits them", async () => {
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      policy: {
        mode: "managed",
        allowUserAddedInstances: true,
        diagnosticsEnabled: true,
        preconfiguredOrigins: ["https://analytics.company.com"],
        revision: "desktop-policy-v1-managed-0123456789abcdef",
      },
      connectOrigin: async () => undefined,
      connectProfile: async () => undefined,
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [
        {
          id: "profile_0123456789abcdef0123456789abcdef",
          canonicalOrigin: "https://personal.example.com",
          instanceId: "instance_0123456789abcdef0123456789abcdef",
          displayName: "Personal instance",
          lastSafePath: "/",
        },
      ],
    });

    const body = await (
      await ui.handle(new Request("leapview://app/"))
    ).text();

    expect(body).toContain("Connect an instance");
    expect(body).toContain("Saved instances");
    expect(body).toContain("Personal instance");
    expect(body).toContain("Managed instance");
    expect(body).toContain("approved instances are preconfigured");
    expect(body).not.toContain("Only approved instances are available");
  });

  test("rejects forged user-added origins before invoking managed actions", async () => {
    let invoked = false;
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      policy: {
        mode: "managed",
        allowUserAddedInstances: false,
        diagnosticsEnabled: true,
        preconfiguredOrigins: ["https://analytics.company.com"],
        revision: "desktop-policy-v1-managed-0123456789abcdef",
      },
      connectOrigin: async () => {
        invoked = true;
      },
      connectProfile: async () => {
        invoked = true;
      },
      disconnectProfile: async () => undefined,
      removeProfile: async () => undefined,
      listProfiles: async () => [],
    });

    const response = await ui.handle(
      new Request("leapview://app/connect", {
        method: "POST",
        body: "origin=https%3A%2F%2Fpersonal.example.com",
      }),
    );
    const body = await response.text();
    expect(response.status).toBe(200);
    expect(body).toContain("managed by your organization");
    expect(invoked).toBeFalse();
  });

  test("locks every connection action when managed configuration is invalid", async () => {
    let invoked = false;
    const ui = trustedUI({
      allowLoopbackHTTP: false,
      policy: {
        mode: "locked",
        allowUserAddedInstances: false,
        diagnosticsEnabled: false,
        preconfiguredOrigins: [],
        revision: "desktop-policy-v1-invalid",
      },
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

    const page = await ui.handle(new Request("leapview://app/"));
    const body = await page.text();
    expect(body).toContain("configuration is invalid");
    expect(body).toContain("contact your administrator");
    expect(body).not.toContain("<form");

    const action = await ui.handle(
      new Request("leapview://app/connect", {
        method: "POST",
        body: "origin=https%3A%2F%2Fanalytics.company.com",
      }),
    );
    expect(action.status).toBe(403);
    expect(invoked).toBeFalse();
  });
});
