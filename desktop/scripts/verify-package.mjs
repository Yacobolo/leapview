import {
  access,
  mkdtemp,
  readdir,
  rm,
} from "node:fs/promises";
import { constants } from "node:fs";
import { spawn } from "node:child_process";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

import { listPackage } from "@electron/asar";
import {
  FuseState,
  FuseV1Options,
  FuseVersion,
  getCurrentFuseWire,
} from "@electron/fuses";

const root = resolve(import.meta.dirname, "..");
const out = join(root, "out");
const platformName = {
  darwin: "darwin",
  linux: "linux",
  win32: "win32",
}[process.platform];
if (platformName === undefined) {
  throw new Error(`unsupported verification platform ${process.platform}`);
}
const candidates = (await readdir(out, { withFileTypes: true }))
  .filter(
    (entry) =>
      entry.isDirectory() &&
      entry.name.startsWith(`LeapView-${platformName}-`),
  )
  .map((entry) => join(out, entry.name));
if (candidates.length !== 1) {
  throw new Error(
    `expected one packaged LeapView application, found ${candidates.length}`,
  );
}
const packageRoot = candidates[0];
const appPath =
  process.platform === "darwin"
    ? join(packageRoot, "LeapView.app")
    : process.platform === "win32"
      ? join(packageRoot, "LeapView.exe")
      : join(packageRoot, "LeapView");
const executablePath =
  process.platform === "darwin"
    ? join(appPath, "Contents", "MacOS", "LeapView")
    : appPath;
const resources =
  process.platform === "darwin"
    ? join(packageRoot, "LeapView.app", "Contents", "Resources")
    : join(packageRoot, "resources");
const asarPath = join(resources, "app.asar");
await access(appPath, constants.R_OK);
await access(asarPath, constants.R_OK);
await expectMissing(join(resources, "app"));

const wire = await getCurrentFuseWire(appPath);
if (wire.version !== FuseVersion.V1) {
  throw new Error(`unexpected Electron fuse version ${wire.version}`);
}
const expectedFuses = new Map([
  [FuseV1Options.RunAsNode, FuseState.DISABLE],
  [FuseV1Options.EnableCookieEncryption, FuseState.ENABLE],
  [FuseV1Options.EnableNodeOptionsEnvironmentVariable, FuseState.DISABLE],
  [FuseV1Options.EnableNodeCliInspectArguments, FuseState.DISABLE],
  [
    FuseV1Options.EnableEmbeddedAsarIntegrityValidation,
    process.platform === "linux" ? FuseState.DISABLE : FuseState.ENABLE,
  ],
  [FuseV1Options.OnlyLoadAppFromAsar, FuseState.ENABLE],
  [FuseV1Options.LoadBrowserProcessSpecificV8Snapshot, FuseState.DISABLE],
  [FuseV1Options.GrantFileProtocolExtraPrivileges, FuseState.DISABLE],
  [FuseV1Options.WasmTrapHandlers, FuseState.ENABLE],
]);
for (const [fuse, expected] of expectedFuses) {
  if (wire[fuse] !== expected) {
    throw new Error(
      `Electron fuse ${FuseV1Options[fuse]} is ${wire[fuse]}, expected ${expected}`,
    );
  }
}

const archiveFiles = listPackage(asarPath).map((file) =>
  file.replaceAll("\\", "/"),
);
const unexpected = archiveFiles.filter(
  (file) =>
    file !== "/package.json" &&
    file !== "/dist" &&
    !file.startsWith("/dist/"),
);
if (unexpected.length > 0) {
  throw new Error(
    `packaged ASAR contains unexpected files: ${unexpected.join(", ")}`,
  );
}
for (const required of [
  "/package.json",
  "/dist/app.css",
  "/dist/files/inter-cyrillic-ext-wght-normal.woff2",
  "/dist/files/inter-cyrillic-wght-normal.woff2",
  "/dist/files/inter-greek-ext-wght-normal.woff2",
  "/dist/files/inter-greek-wght-normal.woff2",
  "/dist/files/inter-latin-ext-wght-normal.woff2",
  "/dist/files/inter-latin-wght-normal.woff2",
  "/dist/files/inter-vietnamese-wght-normal.woff2",
  "/dist/src/main.js",
  "/dist/src/auth.js",
  "/dist/src/security/remote-policy.mjs",
]) {
  if (!archiveFiles.includes(required)) {
    throw new Error(`packaged ASAR is missing ${required}`);
  }
}
if (
  archiveFiles.some(
    (file) =>
      file.endsWith(".ts") ||
      file.includes(".test.") ||
      file.includes("maliciousinstance"),
  )
) {
  throw new Error("packaged ASAR contains source or test-only content");
}

const startup = await verifyPackagedStartup(executablePath);
process.stdout.write(
  `${JSON.stringify({
    application: appPath,
    asarFiles: archiveFiles.length,
    fuseVersion: wire.version,
    startup,
    verifiedFuses: expectedFuses.size,
  })}\n`,
);

async function expectMissing(path) {
  try {
    await access(path, constants.F_OK);
  } catch {
    return;
  }
  throw new Error(`mutable unpackaged application directory exists: ${path}`);
}

async function verifyPackagedStartup(executable) {
  const userData = await mkdtemp(join(tmpdir(), "leapview-package-smoke-"));
  const devtoolsPort = await reserveLoopbackPort();
  const child = spawn(
    executable,
    [
      "--headless",
      "--disable-gpu",
      `--remote-debugging-port=${devtoolsPort}`,
      `--user-data-dir=${userData}`,
    ],
    {
      stdio: ["ignore", "pipe", "pipe"],
    },
  );
  let diagnostic = "";
  const appendDiagnostic = (chunk) => {
    diagnostic = `${diagnostic}${String(chunk)}`.slice(-16_384);
  };
  child.stdout.on("data", appendDiagnostic);
  child.stderr.on("data", appendDiagnostic);
  try {
    const deadline = Date.now() + 15_000;
    while (Date.now() < deadline) {
      if (child.exitCode !== null || child.signalCode !== null) {
        throw startupFailure(
          "packaged application exited during startup",
          child,
          diagnostic,
        );
      }
      try {
        const response = await fetch(
          `http://127.0.0.1:${devtoolsPort}/json/list`,
          {
          signal: AbortSignal.timeout(1_000),
          },
        );
        if (!response.ok) {
          throw new Error("packaged application debug target was unavailable");
        }
        const targets = await response.json();
        if (
          Array.isArray(targets) &&
          targets.some(
            (target) =>
              target?.type === "page" &&
              target?.url === "leapview://app/",
          )
        ) {
          return "trusted-shell-ready";
        }
      } catch {}
      await new Promise((resolveDelay) => setTimeout(resolveDelay, 50));
    }
    throw startupFailure(
      "packaged application did not open its trusted shell",
      child,
      diagnostic,
    );
  } finally {
    if (child.exitCode === null && child.signalCode === null) {
      child.kill();
      await Promise.race([
        new Promise((resolveExit) => child.once("exit", resolveExit)),
        new Promise((resolveDelay) => setTimeout(resolveDelay, 2_000)),
      ]);
      if (child.exitCode === null && child.signalCode === null) {
        child.kill("SIGKILL");
      }
    }
    await rm(userData, {
      force: true,
      maxRetries: 5,
      recursive: true,
      retryDelay: 100,
    });
  }
}

async function reserveLoopbackPort() {
  const server = createServer();
  await new Promise((resolveListen, rejectListen) => {
    server.once("error", rejectListen);
    server.listen(0, "127.0.0.1", resolveListen);
  });
  const address = server.address();
  await new Promise((resolveClose, rejectClose) => {
    server.close((error) => {
      if (error) {
        rejectClose(error);
        return;
      }
      resolveClose();
    });
  });
  if (address === null || typeof address === "string") {
    throw new Error("failed to reserve a loopback debug port");
  }
  return address.port;
}

function startupFailure(message, child, diagnostic) {
  return new Error(
    [
      message,
      `exit=${child.exitCode ?? "none"}`,
      `signal=${child.signalCode ?? "none"}`,
      diagnostic.trim(),
    ]
      .filter(Boolean)
      .join("\n"),
  );
}
