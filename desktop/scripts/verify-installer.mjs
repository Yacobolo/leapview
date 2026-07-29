import { execFile } from "node:child_process";
import {
  mkdtemp,
  readFile,
  readdir,
  rm,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join, resolve } from "node:path";
import { promisify } from "node:util";
import { pathToFileURL } from "node:url";

const execFileAsync = promisify(execFile);
const formatByPlatform = {
  darwin: "pkg",
  linux: "deb",
  win32: "msi",
};

export function validateInstallerContract({
  format,
  platform,
  policyIntegration,
  protocolIntegration,
  scope,
}) {
  if (
    format !== formatByPlatform[platform] ||
    scope !== "per-machine" ||
    policyIntegration !== "administrator-owned-retained" ||
    protocolIntegration !== "installer-owned-quoted-single-url"
  ) {
    throw new Error("production installer contract is incomplete");
  }
  return {
    format,
    scope,
    policyIntegration,
    protocolIntegration,
  };
}

async function main() {
  const desktopRoot = resolve(import.meta.dirname, "..");
  const out = join(desktopRoot, "out");
  const verificationPath = join(out, "package-verification.json");
  const verification = JSON.parse(
    await readFile(verificationPath, "utf8"),
  );
  const format = formatByPlatform[process.platform];
  if (
    format === undefined ||
    verification.platform !== process.platform ||
    verification.packageFormat !== format
  ) {
    throw new Error("installer target does not match package verification");
  }
  const artifacts = await findFiles(
    join(out, "make"),
    (path) => path.toLowerCase().endsWith(`.${format}`),
  );
  if (artifacts.length !== 1) {
    throw new Error(
      `expected exactly one ${format} installer, found ${artifacts.length}`,
    );
  }
  const inspection =
    process.platform === "darwin"
      ? await inspectMacOSInstaller(artifacts[0])
      : process.platform === "linux"
        ? await inspectDebianInstaller(artifacts[0])
        : await inspectWindowsInstaller(artifacts[0]);
  verification.installer = validateInstallerContract({
    format,
    platform: process.platform,
    ...inspection,
  });
  await writeFile(
    verificationPath,
    `${JSON.stringify(verification, null, 2)}\n`,
  );
  process.stdout.write(
    `${JSON.stringify({
      artifact: basename(artifacts[0]),
      ...verification.installer,
    })}\n`,
  );
}

async function inspectMacOSInstaller(artifact) {
  const temporary = await mkdtemp(
    join(tmpdir(), "leapview-pkg-inspection-"),
  );
  const expanded = join(temporary, "expanded");
  try {
    await runFile("pkgutil", ["--expand-full", artifact, expanded]);
    const files = await findFiles(expanded, () => true);
    const preinstall = files.find((path) => path.endsWith("/preinstall"));
    const postinstall = files.find((path) => path.endsWith("/postinstall"));
    const plist = files.find((path) =>
      path.endsWith("/LeapView.app/Contents/Info.plist"),
    );
    if (
      preinstall === undefined ||
      postinstall === undefined ||
      plist === undefined
    ) {
      throw new Error("macOS installer payload or scripts are incomplete");
    }
    const [preinstallBody, postinstallBody, plistBody] = await Promise.all([
      readFile(preinstall, "utf8"),
      readFile(postinstall, "utf8"),
      readFile(plist, "utf8"),
    ]);
    assertPolicyScripts(preinstallBody, postinstallBody, "root:wheel");
    if (
      !plistBody.includes("<string>leapview-desktop</string>") ||
      !plistBody.includes("<string>dev.leapview.desktop</string>")
    ) {
      throw new Error("macOS installer has no exact desktop protocol");
    }
    return {
      scope: "per-machine",
      policyIntegration: "administrator-owned-retained",
      protocolIntegration: "installer-owned-quoted-single-url",
    };
  } finally {
    await rm(temporary, { force: true, recursive: true });
  }
}

async function inspectDebianInstaller(artifact) {
  const temporary = await mkdtemp(
    join(tmpdir(), "leapview-deb-inspection-"),
  );
  const control = join(temporary, "control");
  const payload = join(temporary, "payload");
  try {
    await runFile("dpkg-deb", ["--control", artifact, control]);
    await runFile("dpkg-deb", ["--extract", artifact, payload]);
    const [preinstallBody, postinstallBody, desktopEntry] =
      await Promise.all([
        readFile(join(control, "preinst"), "utf8"),
        readFile(join(control, "postinst"), "utf8"),
        readFile(
          join(
            payload,
            "usr/share/applications/leapview-desktop.desktop",
          ),
          "utf8",
        ),
      ]);
    assertPolicyScripts(preinstallBody, postinstallBody, "root:root");
    if (
      !desktopEntry.includes("Exec=leapview-desktop %U\n") ||
      !desktopEntry.includes(
        "MimeType=x-scheme-handler/leapview-desktop;\n",
      ) ||
      desktopEntry.includes("sh -c")
    ) {
      throw new Error("Debian installer has an unsafe desktop protocol");
    }
    return {
      scope: "per-machine",
      policyIntegration: "administrator-owned-retained",
      protocolIntegration: "installer-owned-quoted-single-url",
    };
  } finally {
    await rm(temporary, { force: true, recursive: true });
  }
}

async function inspectWindowsInstaller(artifact) {
  const temporary = await mkdtemp(
    join(tmpdir(), "leapview-msi-inspection-"),
  );
  const source = join(temporary, "LeapView.wxs");
  try {
    await runFile("dark.exe", [
      "-nologo",
      "-x",
      join(temporary, "payload"),
      "-o",
      source,
      artifact,
    ]);
    const wix = await readFile(source, "utf8");
    for (const expected of [
      'Id="CommonAppDataFolder"',
      'Id="LeapViewPolicyDirectory"',
      'Root="HKLM"',
      "Software\\Classes\\leapview-desktop",
      "&quot;[APPLICATIONROOTDIRECTORY]LeapView.exe&quot; &quot;%1&quot;",
      'ForceDeleteOnUninstall="yes"',
    ]) {
      if (!wix.includes(expected)) {
        throw new Error(
          `Windows installer is missing ${JSON.stringify(expected)}`,
        );
      }
    }
    return {
      scope: "per-machine",
      policyIntegration: "administrator-owned-retained",
      protocolIntegration: "installer-owned-quoted-single-url",
    };
  } finally {
    await rm(temporary, { force: true, recursive: true });
  }
}

function assertPolicyScripts(preinstall, postinstall, owner) {
  for (const script of [preinstall, postinstall]) {
    if (
      !script.includes("desktop-policy.json") ||
      !script.includes("policy location must not be a symlink") ||
      script.includes("rm ")
    ) {
      throw new Error("installer policy script is unsafe");
    }
  }
  if (
    !postinstall.includes(`chown ${owner}`) ||
    !postinstall.includes("chmod 0644")
  ) {
    throw new Error("installer does not repair managed policy ownership");
  }
}

async function findFiles(root, predicate) {
  const matches = [];
  async function visit(directory) {
    for (const entry of await readdir(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        await visit(path);
      } else if (entry.isFile() && predicate(path)) {
        matches.push(path);
      }
    }
  }
  await visit(root);
  return matches.sort();
}

async function runFile(executable, arguments_) {
  await execFileAsync(executable, arguments_, {
    encoding: "utf8",
    maxBuffer: 4 * 1024 * 1024,
    windowsHide: true,
  });
}

if (
  process.argv[1] !== undefined &&
  import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  await main();
}
