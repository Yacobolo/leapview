import { createHash } from "node:crypto";
import { execFile } from "node:child_process";
import { lstat, readFile } from "node:fs/promises";
import { promisify } from "node:util";
import {
  isAbsolute,
  dirname,
  join,
  posix,
  win32,
} from "node:path";

import { parseConfiguredOrigin } from "./security/remote-policy.mjs";

const POLICY_SCHEMA_VERSION = 1;
const MAXIMUM_POLICY_BYTES = 64 * 1024;
const MAXIMUM_PRECONFIGURED_ORIGINS = 20;
const DEVELOPMENT_POLICY_ENVIRONMENT_KEY =
  "LEAPVIEW_DESKTOP_POLICY_PATH";
const WINDOWS_POLICY_HELPER = "leapview-windows-policy.exe";
const execFileAsync = promisify(execFile);

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
  integrity?: "unchecked" | "verified" | "invalid";
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
  windowsProbe?: WindowsPolicyProbe;
}

export interface LoadDesktopPolicyOptions {
  allowLoopbackHTTP: boolean;
}

export interface WindowsPolicyProbe {
  policyPath: string;
  security: "missing" | "secure" | "insecure";
}

export async function probeWindowsDesktopPolicy(
  resourcesPath: string,
  runHelper: (
    executable: string,
  ) => Promise<{ stdout: string }> = runWindowsPolicyHelper,
): Promise<WindowsPolicyProbe | null> {
  try {
    const { stdout } = await runHelper(
      join(resourcesPath, WINDOWS_POLICY_HELPER),
    );
    if (Buffer.byteLength(stdout, "utf8") > 4 * 1024) {
      return null;
    }
    const input = JSON.parse(stdout) as unknown;
    if (
      typeof input !== "object" ||
      input === null ||
      Array.isArray(input) ||
      Object.keys(input).sort().join(",") !==
        "policyPath,schemaVersion,security"
    ) {
      return null;
    }
    const document = input as Record<string, unknown>;
    if (
      document.schemaVersion !== 1 ||
      typeof document.policyPath !== "string" ||
      !win32.isAbsolute(document.policyPath) ||
      win32.normalize(document.policyPath) !== document.policyPath ||
      win32.basename(document.policyPath) !== "desktop-policy.json" ||
      win32.basename(win32.dirname(document.policyPath)) !== "LeapView" ||
      !["missing", "secure", "insecure"].includes(
        String(document.security),
      )
    ) {
      return null;
    }
    return {
      policyPath: document.policyPath,
      security: document.security as WindowsPolicyProbe["security"],
    };
  } catch {
    return null;
  }
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
      integrity: "unchecked",
    };
  }
  switch (options.platform) {
    case "darwin":
      return {
        path:
          "/Library/Application Support/LeapView/desktop-policy.json",
        requireAdministratorOwner: true,
        integrity: "unchecked",
      };
    case "linux":
      return {
        path: "/etc/leapview/desktop-policy.json",
        requireAdministratorOwner: true,
        integrity: "unchecked",
      };
    case "win32": {
      if (options.windowsProbe === undefined) {
        return {
          path: null,
          requireAdministratorOwner: true,
          integrity: "invalid",
        };
      }
      return {
        path: options.windowsProbe.policyPath,
        requireAdministratorOwner: true,
        integrity:
          options.windowsProbe.security === "insecure"
            ? "invalid"
            : "verified",
      };
    }
    default:
      return {
        path: null,
        requireAdministratorOwner: true,
        integrity: "invalid",
      };
  }
}

export async function loadDesktopPolicy(
  source: DesktopPolicySource,
  options: LoadDesktopPolicyOptions,
): Promise<DesktopPolicy> {
  if (source.integrity === "invalid") {
    return lockedPolicy();
  }
  if (source.path === null) {
    return openPolicy();
  }
  try {
    if (
      source.requireAdministratorOwner &&
      process.platform !== "win32"
    ) {
      try {
        const directory = await lstat(dirname(source.path));
        if (
          directory.isSymbolicLink() ||
          !directory.isDirectory() ||
          (directory.mode & 0o022) !== 0 ||
          directory.uid !== 0
        ) {
          return lockedPolicy();
        }
      } catch (error) {
        if (!isMissingFile(error)) {
          return lockedPolicy();
        }
      }
    }
    const information = await lstat(source.path);
    if (
      information.isSymbolicLink() ||
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

async function runWindowsPolicyHelper(
  executable: string,
): Promise<{ stdout: string }> {
  const { stdout } = await execFileAsync(executable, [], {
    encoding: "utf8",
    maxBuffer: 4 * 1024,
    timeout: 5_000,
    windowsHide: true,
  });
  return { stdout };
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
