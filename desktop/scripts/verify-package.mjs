import {
  access,
  mkdtemp,
  readdir,
  readFile,
  rm,
  stat,
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
const processGoneReasons = new Set([
  "clean-exit",
  "abnormal-exit",
  "killed",
  "crashed",
  "oom",
  "launch-failed",
  "integrity-failure",
  "memory-eviction",
]);
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
if (process.platform === "darwin") {
  const information = await readFile(
    join(appPath, "Contents", "Info.plist"),
    "utf8",
  );
  if (
    !information.includes("<string>LeapView Desktop</string>") ||
    !information.includes("<string>leapview-desktop</string>")
  ) {
    throw new Error(
      "packaged macOS application is missing the desktop URL handler",
    );
  }
}

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
  "/dist/src/deep-link.js",
  "/dist/src/diagnostics.js",
  "/dist/src/native-menu.js",
  "/dist/src/remote-lifecycle.js",
  "/dist/src/security/remote-policy.mjs",
  "/dist/src/window-state.js",
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
      let shellReady = false;
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
        shellReady =
          Array.isArray(targets) &&
          targets.some(
            (target) =>
              target?.type === "page" &&
              target?.url === "leapview://app/",
          );
      } catch {}
      if (shellReady) {
        await verifyPackagedDiagnosticJournal(userData, deadline);
        return "trusted-shell-ready";
      }
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

async function verifyPackagedDiagnosticJournal(userData, deadline) {
  const path = join(userData, "diagnostics.json");
  while (Date.now() < deadline) {
    try {
      const body = await readFile(path, "utf8");
      if (Buffer.byteLength(body, "utf8") > 128 * 1024) {
        throw new Error("packaged diagnostic journal exceeds its size limit");
      }
      const document = JSON.parse(body);
      if (
        Object.keys(document).sort().join(",") !==
          "events,schemaVersion" ||
        document.schemaVersion !== 1 ||
        !Array.isArray(document.events) ||
        document.events.length === 0 ||
        document.events.length > 256
      ) {
        throw new Error(
          "packaged diagnostic journal has an unexpected manifest",
        );
      }
      if (
        !document.events.some(
          (event) =>
            event?.kind === "startup" && event.packaged === true,
        )
      ) {
        throw new Error(
          "packaged diagnostic journal is missing its startup event",
        );
      }
      for (const event of document.events) {
        verifyPackagedDiagnosticEvent(event);
      }
      if (
        /https?:|origin|cookie|token|authorization|console|filename/iu.test(
          body,
        )
      ) {
        throw new Error(
          "packaged diagnostic journal contains forbidden sensitive fields",
        );
      }
      if (
        process.platform !== "win32" &&
        ((await stat(path)).mode & 0o077) !== 0
      ) {
        throw new Error(
          "packaged diagnostic journal permissions are not private",
        );
      }
      return;
    } catch (error) {
      if (
        error instanceof SyntaxError ||
        (error instanceof Error &&
          !("code" in error && error.code === "ENOENT"))
      ) {
        throw error;
      }
    }
    await new Promise((resolveDelay) => setTimeout(resolveDelay, 50));
  }
  throw new Error("packaged application did not persist diagnostics");
}

function verifyPackagedDiagnosticEvent(event) {
  if (typeof event !== "object" || event === null || Array.isArray(event)) {
    throw new Error("packaged diagnostic journal contains an invalid event");
  }
  if (
    typeof event.at !== "string" ||
    new Date(event.at).toISOString() !== event.at
  ) {
    throw new Error(
      "packaged diagnostic journal contains an invalid timestamp",
    );
  }
  if (event.kind === "startup") {
    if (
      Object.keys(event).sort().join(",") !== "at,kind,packaged" ||
      event.packaged !== true
    ) {
      throw new Error(
        "packaged diagnostic journal contains invalid startup data",
      );
    }
    return;
  }
  if (event.kind === "render-process-gone") {
    if (
      Object.keys(event).sort().join(",") !==
        "at,kind,reason,surface" ||
      !["trusted-shell", "unknown"].includes(event.surface) ||
      !processGoneReasons.has(event.reason)
    ) {
      throw new Error(
        "packaged diagnostic journal contains invalid renderer data",
      );
    }
    return;
  }
  if (event.kind === "child-process-gone") {
    if (
      Object.keys(event).sort().join(",") !==
        "at,kind,processType,reason" ||
      ![
        "utility",
        "zygote",
        "sandbox-helper",
        "gpu",
        "pepper-plugin",
        "pepper-plugin-broker",
        "unknown",
      ].includes(event.processType) ||
      !processGoneReasons.has(event.reason)
    ) {
      throw new Error(
        "packaged diagnostic journal contains invalid child-process data",
      );
    }
    return;
  }
  throw new Error(
    "packaged diagnostic journal contains an unexpected startup event",
  );
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
