import { createHash } from "node:crypto";
import { readFile, stat } from "node:fs/promises";
import {
  isAbsolute,
  posix,
  win32,
} from "node:path";

import { parseConfiguredOrigin } from "./security/remote-policy.mjs";

const POLICY_SCHEMA_VERSION = 1;
const MAXIMUM_POLICY_BYTES = 64 * 1024;
const MAXIMUM_PRECONFIGURED_ORIGINS = 20;
const DEVELOPMENT_POLICY_ENVIRONMENT_KEY =
  "LEAPVIEW_DESKTOP_POLICY_PATH";

export interface DesktopPolicy {
  mode: "open" | "managed" | "locked";
  allowUserAddedInstances: boolean;
  diagnosticsEnabled: boolean;
  preconfiguredOrigins: string[];
  revision: string;
}

export interface DesktopPolicySource {
  path: string | null;
  requireAdministratorOwner: boolean;
}

interface DesktopPolicyDocument {
  schemaVersion: 1;
  allowUserAddedInstances: boolean;
  diagnosticsEnabled: boolean;
  preconfiguredOrigins: string[];
}

export interface DesktopPolicySourceOptions {
  platform: NodeJS.Platform;
  packaged: boolean;
  environment?: Readonly<Record<string, string | undefined>>;
}

export interface LoadDesktopPolicyOptions {
  allowLoopbackHTTP: boolean;
}

export function resolveDesktopPolicySource(
  options: DesktopPolicySourceOptions,
): DesktopPolicySource {
  const environment = options.environment ?? process.env;
  if (!options.packaged) {
    const override = environment[DEVELOPMENT_POLICY_ENVIRONMENT_KEY];
    return {
      path:
        typeof override === "string" &&
        platformIsAbsolute(options.platform, override)
          ? override
          : null,
      requireAdministratorOwner: false,
    };
  }
  switch (options.platform) {
    case "darwin":
      return {
        path:
          "/Library/Application Support/LeapView/desktop-policy.json",
        requireAdministratorOwner: true,
      };
    case "linux":
      return {
        path: "/etc/leapview/desktop-policy.json",
        requireAdministratorOwner: true,
      };
    case "win32": {
      return {
        path: win32.join(
          String.raw`C:\ProgramData`,
          "LeapView",
          "desktop-policy.json",
        ),
        requireAdministratorOwner: true,
      };
    }
    default:
      return { path: null, requireAdministratorOwner: true };
  }
}

export async function loadDesktopPolicy(
  source: DesktopPolicySource,
  options: LoadDesktopPolicyOptions,
): Promise<DesktopPolicy> {
  if (source.path === null) {
    return openPolicy();
  }
  try {
    const information = await stat(source.path);
    if (
      !information.isFile() ||
      information.size === 0 ||
      information.size > MAXIMUM_POLICY_BYTES ||
      (
        process.platform !== "win32" &&
        (information.mode & 0o022) !== 0
      ) ||
      (
        source.requireAdministratorOwner &&
        process.platform !== "win32" &&
        information.uid !== 0
      )
    ) {
      return lockedPolicy();
    }
    const body = await readFile(source.path, "utf8");
    const document = validatePolicyDocument(
      JSON.parse(body) as unknown,
      options,
    );
    const digest = createHash("sha256")
      .update(JSON.stringify(document))
      .digest("hex")
      .slice(0, 16);
    return {
      mode: "managed",
      allowUserAddedInstances: document.allowUserAddedInstances,
      diagnosticsEnabled: document.diagnosticsEnabled,
      preconfiguredOrigins: [...document.preconfiguredOrigins],
      revision: `desktop-policy-v1-managed-${digest}`,
    };
  } catch (error) {
    if (isMissingFile(error)) {
      return openPolicy();
    }
    return lockedPolicy();
  }
}

export function policyAllowsOrigin(
  policy: DesktopPolicy,
  canonicalOrigin: string,
): boolean {
  return (
    policy.mode !== "locked" &&
    (
      policy.allowUserAddedInstances ||
      policy.preconfiguredOrigins.includes(canonicalOrigin)
    )
  );
}

export function policyAllowsProfile(
  policy: DesktopPolicy,
  profile: { canonicalOrigin: string },
): boolean {
  return policyAllowsOrigin(policy, profile.canonicalOrigin);
}

export function policyManagesOrigin(
  policy: DesktopPolicy,
  canonicalOrigin: string,
): boolean {
  return (
    policy.mode === "managed" &&
    policy.preconfiguredOrigins.includes(canonicalOrigin)
  );
}

function validatePolicyDocument(
  input: unknown,
  options: LoadDesktopPolicyOptions,
): DesktopPolicyDocument {
  if (
    typeof input !== "object" ||
    input === null ||
    Array.isArray(input)
  ) {
    throw new Error("desktop policy must be an object");
  }
  const document = input as Record<string, unknown>;
  const expectedKeys = [
    "schemaVersion",
    "allowUserAddedInstances",
    "diagnosticsEnabled",
    "preconfiguredOrigins",
  ];
  if (
    Object.keys(document).length !== expectedKeys.length ||
    !expectedKeys.every((key) => Object.hasOwn(document, key)) ||
    document.schemaVersion !== POLICY_SCHEMA_VERSION ||
    typeof document.allowUserAddedInstances !== "boolean" ||
    typeof document.diagnosticsEnabled !== "boolean" ||
    !Array.isArray(document.preconfiguredOrigins) ||
    document.preconfiguredOrigins.length >
      MAXIMUM_PRECONFIGURED_ORIGINS
  ) {
    throw new Error("desktop policy fields are invalid");
  }
  const origins = document.preconfiguredOrigins.map((inputOrigin) => {
    if (typeof inputOrigin !== "string") {
      throw new Error("desktop policy origin is invalid");
    }
    const canonicalOrigin = parseConfiguredOrigin(inputOrigin, {
      allowLoopbackHTTP: options.allowLoopbackHTTP,
    });
    if (canonicalOrigin !== inputOrigin) {
      throw new Error("desktop policy origin is not canonical");
    }
    return canonicalOrigin;
  });
  if (new Set(origins).size !== origins.length) {
    throw new Error("desktop policy origins must be unique");
  }
  return {
    schemaVersion: POLICY_SCHEMA_VERSION,
    allowUserAddedInstances: document.allowUserAddedInstances,
    diagnosticsEnabled: document.diagnosticsEnabled,
    preconfiguredOrigins: origins,
  };
}

function openPolicy(): DesktopPolicy {
  return {
    mode: "open",
    allowUserAddedInstances: true,
    diagnosticsEnabled: true,
    preconfiguredOrigins: [],
    revision: "desktop-policy-v1",
  };
}

function lockedPolicy(): DesktopPolicy {
  return {
    mode: "locked",
    allowUserAddedInstances: false,
    diagnosticsEnabled: false,
    preconfiguredOrigins: [],
    revision: "desktop-policy-v1-invalid",
  };
}

function platformIsAbsolute(
  platform: NodeJS.Platform,
  candidate: string,
): boolean {
  return platform === "win32"
    ? win32.isAbsolute(candidate)
    : isAbsolute(candidate);
}

function isMissingFile(error: unknown): boolean {
  return (
    typeof error === "object" &&
    error !== null &&
    "code" in error &&
    error.code === "ENOENT"
  );
}
