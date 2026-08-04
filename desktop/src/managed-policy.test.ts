import { describe, expect, test } from "bun:test";
import {
  chmod,
  mkdtemp,
  rm,
  symlink,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import {
  loadDesktopPolicy,
  policyAllowsOrigin,
  policyAllowsProfile,
  policyManagesOrigin,
  probeWindowsDesktopPolicy,
  resolveDesktopPolicySource,
} from "./managed-policy.js";

const managedDocument = {
  schemaVersion: 1,
  allowUserAddedInstances: false,
  diagnosticsEnabled: false,
  preconfiguredOrigins: [
    "https://analytics.company.com",
    "https://finance.company.com",
  ],
};

describe("resolveDesktopPolicySource", () => {
  test("uses fixed packaged system locations and ignores development overrides", () => {
    expect(
      resolveDesktopPolicySource({
        platform: "darwin",
        packaged: true,
        environment: {
          LEAPVIEW_DESKTOP_POLICY_PATH: "/tmp/attacker-policy.json",
        },
      }),
    ).toEqual({
      path: "/Library/Application Support/LeapView/desktop-policy.json",
      requireAdministratorOwner: true,
      integrity: "unchecked",
    });
    expect(
      resolveDesktopPolicySource({
        platform: "linux",
        packaged: true,
        environment: {
          LEAPVIEW_DESKTOP_POLICY_PATH: "/tmp/attacker-policy.json",
        },
      }),
    ).toEqual({
      path: "/etc/leapview/desktop-policy.json",
      requireAdministratorOwner: true,
      integrity: "unchecked",
    });
    expect(
      resolveDesktopPolicySource({
        platform: "win32",
        packaged: true,
        environment: {
          ProgramData: String.raw`D:\ProgramData`,
          LEAPVIEW_DESKTOP_POLICY_PATH: String.raw`C:\attacker.json`,
        },
        windowsProbe: {
          policyPath: String.raw`D:\ProgramData\LeapView\desktop-policy.json`,
          security: "secure",
        },
      }),
    ).toEqual({
      path: String.raw`D:\ProgramData\LeapView\desktop-policy.json`,
      requireAdministratorOwner: true,
      integrity: "verified",
    });
  });

  test("accepts only an absolute development override in unpackaged builds", () => {
    expect(
      resolveDesktopPolicySource({
        platform: "linux",
        packaged: false,
        environment: {
          LEAPVIEW_DESKTOP_POLICY_PATH: "/tmp/leapview-policy.json",
        },
      }),
    ).toEqual({
      path: "/tmp/leapview-policy.json",
      requireAdministratorOwner: false,
      integrity: "unchecked",
    });
    expect(
      resolveDesktopPolicySource({
        platform: "linux",
        packaged: false,
        environment: {
          LEAPVIEW_DESKTOP_POLICY_PATH: "./policy.json",
        },
      }),
    ).toEqual({
      path: null,
      requireAdministratorOwner: false,
      integrity: "unchecked",
    });
  });

  test("locks packaged Windows policy when the native probe is missing or insecure", async () => {
    const missingProbe = resolveDesktopPolicySource({
      platform: "win32",
      packaged: true,
    });
    expect(missingProbe).toEqual({
      path: null,
      requireAdministratorOwner: true,
      integrity: "invalid",
    });
    expect(
      (
        await loadDesktopPolicy(missingProbe, {
          allowLoopbackHTTP: false,
        })
      ).mode,
    ).toBe("locked");
    expect(
      resolveDesktopPolicySource({
        platform: "win32",
        packaged: true,
        windowsProbe: {
          policyPath: String.raw`C:\ProgramData\LeapView\desktop-policy.json`,
          security: "insecure",
        },
      }).integrity,
    ).toBe("invalid");
  });

  test("treats an absent managed policy as open mode for consumer Windows installs", async () => {
    const source = resolveDesktopPolicySource({
      platform: "win32",
      packaged: true,
      windowsProbe: {
        policyPath: String.raw`C:\ProgramData\LeapView\desktop-policy.json`,
        security: "missing",
      },
    });
    expect(source.integrity).toBe("verified");
    expect(
      await loadDesktopPolicy(source, {
        allowLoopbackHTTP: false,
      }),
    ).toEqual({
      mode: "open",
      allowUserAddedInstances: true,
      diagnosticsEnabled: true,
      preconfiguredOrigins: [],
      revision: "desktop-policy-v1",
    });
  });
});

describe("probeWindowsDesktopPolicy", () => {
  test("accepts only exact bounded native helper output", async () => {
    const policyPath =
      String.raw`D:\ProgramData\LeapView\desktop-policy.json`;
    expect(
      await probeWindowsDesktopPolicy("C:\\resources", async (executable) => {
        expect(executable).toBe(
          join("C:\\resources", "leapview-windows-policy.exe"),
        );
        return {
          stdout: `${JSON.stringify({
            schemaVersion: 1,
            policyPath,
            security: "missing",
          })}\n`,
        };
      }),
    ).toEqual({ policyPath, security: "missing" });
    for (const output of [
      "{}",
      JSON.stringify({
        schemaVersion: 1,
        policyPath: String.raw`C:\attacker.json`,
        security: "secure",
      }),
      JSON.stringify({
        schemaVersion: 1,
        policyPath,
        security: "secure",
        command: "attacker",
      }),
      "x".repeat(5 * 1024),
    ]) {
      expect(
        await probeWindowsDesktopPolicy(
          "C:\\resources",
          async () => ({ stdout: output }),
        ),
      ).toBeNull();
    }
  });
});

describe("loadDesktopPolicy", () => {
  test("defaults to deterministic open mode only when no policy exists", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-policy-"));
    try {
      const policy = await loadDesktopPolicy(
        {
          path: join(directory, "missing.json"),
          requireAdministratorOwner: false,
        },
        { allowLoopbackHTTP: false },
      );
      expect(policy).toEqual({
        mode: "open",
        allowUserAddedInstances: true,
        diagnosticsEnabled: true,
        preconfiguredOrigins: [],
        revision: "desktop-policy-v1",
      });
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });

  test("loads a strict bounded managed policy and derives a non-secret revision", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-policy-"));
    try {
      const path = join(directory, "desktop-policy.json");
      await writeFile(path, JSON.stringify(managedDocument), { mode: 0o600 });
      const policy = await loadDesktopPolicy(
        { path, requireAdministratorOwner: false },
        { allowLoopbackHTTP: false },
      );
      expect(policy).toEqual({
        mode: "managed",
        allowUserAddedInstances: false,
        diagnosticsEnabled: false,
        preconfiguredOrigins: managedDocument.preconfiguredOrigins,
        revision: expect.stringMatching(
          /^desktop-policy-v1-managed-[0-9a-f]{16}$/u,
        ),
      });
      expect(JSON.stringify(policy)).not.toContain(path);
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });

  test("locks instead of falling back to open mode for invalid configured policy", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-policy-"));
    try {
      const path = join(directory, "desktop-policy.json");
      const invalidDocuments = [
        { ...managedDocument, accessToken: "secret" },
        { ...managedDocument, schemaVersion: 2 },
        {
          ...managedDocument,
          preconfiguredOrigins: [
            "https://analytics.company.com/path",
          ],
        },
        {
          ...managedDocument,
          preconfiguredOrigins: [
            "https://analytics.company.com",
            "https://analytics.company.com",
          ],
        },
        {
          ...managedDocument,
          preconfiguredOrigins: ["http://analytics.company.com"],
        },
      ];
      for (const document of invalidDocuments) {
        await writeFile(path, JSON.stringify(document), { mode: 0o600 });
        expect(
          await loadDesktopPolicy(
            { path, requireAdministratorOwner: false },
            { allowLoopbackHTTP: false },
          ),
        ).toEqual({
          mode: "locked",
          allowUserAddedInstances: false,
          diagnosticsEnabled: false,
          preconfiguredOrigins: [],
          revision: "desktop-policy-v1-invalid",
        });
      }
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });

  test("locks oversized and writable policy files", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-policy-"));
    try {
      const path = join(directory, "desktop-policy.json");
      await writeFile(path, JSON.stringify(managedDocument), { mode: 0o600 });
      if (process.platform !== "win32") {
        await chmod(path, 0o666);
        expect(
          (
            await loadDesktopPolicy(
              { path, requireAdministratorOwner: false },
              { allowLoopbackHTTP: false },
            )
          ).mode,
        ).toBe("locked");
      }
      await writeFile(path, "x".repeat(65 * 1024), { mode: 0o600 });
      expect(
        (
          await loadDesktopPolicy(
            { path, requireAdministratorOwner: false },
            { allowLoopbackHTTP: false },
          )
        ).mode,
      ).toBe("locked");
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });

  test("locks a policy path that has drifted to a symbolic link", async () => {
    if (process.platform === "win32") {
      return;
    }
    const directory = await mkdtemp(join(tmpdir(), "leapview-policy-"));
    try {
      const target = join(directory, "target.json");
      const path = join(directory, "desktop-policy.json");
      await writeFile(target, JSON.stringify(managedDocument), {
        mode: 0o600,
      });
      await symlink(target, path);
      expect(
        (
          await loadDesktopPolicy(
            { path, requireAdministratorOwner: false },
            { allowLoopbackHTTP: false },
          )
        ).mode,
      ).toBe("locked");
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });

  test("requires administrator ownership for packaged POSIX policy", async () => {
    if (
      process.platform === "win32" ||
      process.getuid === undefined ||
      process.getuid() === 0
    ) {
      return;
    }
    const directory = await mkdtemp(join(tmpdir(), "leapview-policy-"));
    try {
      const path = join(directory, "desktop-policy.json");
      await writeFile(path, JSON.stringify(managedDocument), { mode: 0o600 });
      expect(
        (
          await loadDesktopPolicy(
            { path, requireAdministratorOwner: true },
            { allowLoopbackHTTP: false },
          )
        ).mode,
      ).toBe("locked");
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });

  test("allows loopback origins only in explicitly enabled development policy", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-policy-"));
    try {
      const path = join(directory, "desktop-policy.json");
      await writeFile(
        path,
        JSON.stringify({
          ...managedDocument,
          preconfiguredOrigins: ["http://localhost:8080"],
        }),
        { mode: 0o600 },
      );
      expect(
        (
          await loadDesktopPolicy(
            { path, requireAdministratorOwner: false },
            { allowLoopbackHTTP: true },
          )
        ).mode,
      ).toBe("managed");
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });
});

describe("managed policy precedence", () => {
  test("permits only managed origins and profiles when user additions are disabled", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-policy-"));
    try {
      const path = join(directory, "desktop-policy.json");
      await writeFile(path, JSON.stringify(managedDocument), { mode: 0o600 });
      const policy = await loadDesktopPolicy(
        { path, requireAdministratorOwner: false },
        { allowLoopbackHTTP: false },
      );
      expect(
        policyAllowsOrigin(policy, "https://analytics.company.com"),
      ).toBe(true);
      expect(policyAllowsOrigin(policy, "https://other.company.com")).toBe(
        false,
      );
      expect(
        policyAllowsProfile(policy, {
          canonicalOrigin: "https://finance.company.com",
        }),
      ).toBe(true);
      expect(
        policyAllowsProfile(policy, {
          canonicalOrigin: "https://other.company.com",
        }),
      ).toBe(false);
      expect(
        policyManagesOrigin(policy, "https://analytics.company.com"),
      ).toBe(true);
      expect(policyManagesOrigin(policy, "https://other.company.com")).toBe(
        false,
      );
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });
});
