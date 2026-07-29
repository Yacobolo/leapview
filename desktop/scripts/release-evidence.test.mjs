import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join } from "node:path";
import test from "node:test";

import {
  buildSpdxDocument,
  createReleaseManifest,
  validateReleasePolicy,
} from "./release-evidence.mjs";
import { verifyReleaseEvidence } from "./verify-release-evidence.mjs";

const packageDocument = {
  name: "@leapview/desktop",
  productName: "LeapView",
  version: "0.1.0",
  devDependencies: {
    electron: "43.2.0",
    node: "24.14.0",
    "@electron-forge/cli": "7.11.2",
  },
};

const policy = {
  schemaVersion: 1,
  applicationVersion: "0.1.0",
  packageFormats: {
    darwin: "pkg",
    linux: "deb",
    win32: "msi",
  },
  installationScope: "per-machine",
  runtime: {
    electron: "43.2.0",
    electronMajor: 43,
    chromium: "150.0.7871.129",
    node: "24.14.0",
    forge: "7.11.2",
    bun: "1.3.7",
  },
  supportMatrix: [
    {
      platform: "darwin",
      architectures: ["arm64", "x64"],
      minimumVersion: "macOS 13 Ventura",
    },
    {
      platform: "linux",
      architectures: ["x64"],
      minimumVersion: "Ubuntu 22.04 LTS",
    },
    {
      platform: "win32",
      architectures: ["x64"],
      minimumVersion: "Windows 10",
    },
  ],
  hardening: {
    asarOnly: true,
    fuses: {
      RunAsNode: "disabled",
      EnableCookieEncryption: "enabled",
      EnableNodeOptionsEnvironmentVariable: "disabled",
      EnableNodeCliInspectArguments: "disabled",
      EnableEmbeddedAsarIntegrityValidation: {
        darwin: "enabled",
        linux: "disabled",
        win32: "enabled",
      },
      OnlyLoadAppFromAsar: "enabled",
      LoadBrowserProcessSpecificV8Snapshot: "disabled",
      GrantFileProtocolExtraPrivileges: "disabled",
      WasmTrapHandlers: "enabled",
    },
  },
  publication: {
    codeSigningRequired: true,
    githubAttestationsRequired: true,
    immutableArtifactsRequired: true,
  },
  privacy: {
    evidenceContainsCustomerData: false,
    evidenceContainsCredentials: false,
    evidenceContainsDiagnostics: false,
  },
};

const packageVerification = {
  schemaVersion: 1,
  platform: "darwin",
  architecture: "arm64",
  packageFormat: "pkg",
  asarOnly: true,
  runtime: {
    electron: "43.2.0",
    chromium: "150.0.7871.129",
    node: "24.14.0",
  },
  fuses: {
    RunAsNode: "disabled",
    EnableCookieEncryption: "enabled",
    EnableNodeOptionsEnvironmentVariable: "disabled",
    EnableNodeCliInspectArguments: "disabled",
    EnableEmbeddedAsarIntegrityValidation: "enabled",
    OnlyLoadAppFromAsar: "enabled",
    LoadBrowserProcessSpecificV8Snapshot: "disabled",
    GrantFileProtocolExtraPrivileges: "disabled",
    WasmTrapHandlers: "enabled",
  },
  asarFiles: 27,
  installer: {
    format: "pkg",
    scope: "per-machine",
    policyIntegration: "administrator-owned-retained",
    protocolIntegration: "installer-owned-quoted-single-url",
  },
  startup: "trusted-shell-ready",
};

test("release policy pins the supported Electron line and packaging contract", () => {
  assert.doesNotThrow(() => validateReleasePolicy(policy, packageDocument));

  const unsupported = structuredClone(policy);
  unsupported.runtime.electronMajor = 42;
  assert.throws(
    () => validateReleasePolicy(unsupported, packageDocument),
    /Electron major/,
  );

  const mutable = structuredClone(packageDocument);
  mutable.devDependencies.electron = "^43.2.0";
  assert.throws(
    () => validateReleasePolicy(policy, mutable),
    /exact Electron version/,
  );

  assert.throws(
    () =>
      buildSpdxDocument({
        artifactSha256: "a".repeat(64),
        createdAt: "2026-07-29T12:00:00.000Z",
        files: [],
        lock: {
          workspaces: { "": { devDependencies: { electron: "^43.0.0" } } },
          packages: {
            electron: ["electron@^43.0.0", "", {}, "sha512-ZWx1Y3Ryb24="],
          },
        },
        packageDocument,
        packageVerification,
        sourceSha: "d".repeat(40),
      }),
    /mutable resolution/,
  );
});

test("SPDX document covers every locked dependency and packaged runtime file", () => {
  const lock = {
    workspaces: {
      "": {
        devDependencies: {
          electron: "43.2.0",
          rxjs: "7.8.2",
        },
      },
    },
    packages: {
      electron: [
        "electron@43.2.0",
        "",
        { dependencies: { "@electron/get": "^3.0.0" } },
        "sha512-ZWx1Y3Ryb24=",
      ],
      rxjs: ["rxjs@7.8.2", "", {}, "sha512-cnhqcw=="],
      "@electron/get": ["@electron/get@3.1.0", "", {}, "sha512-Z2V0"],
    },
  };
  const files = [
    {
      path: "LeapView.app/Contents/MacOS/LeapView",
      sha1: "1".repeat(40),
      sha256: "a".repeat(64),
      type: "BINARY",
    },
    {
      path: "LeapView.app/Contents/Resources/app.asar",
      sha1: "2".repeat(40),
      sha256: "b".repeat(64),
      type: "ARCHIVE",
    },
  ];
  const document = buildSpdxDocument({
    artifactSha256: "c".repeat(64),
    createdAt: "2026-07-29T12:00:00.000Z",
    files,
    lock,
    packageDocument,
    packageVerification,
    sourceSha: "d".repeat(40),
  });

  assert.equal(document.spdxVersion, "SPDX-2.3");
  assert.equal(
    document.packages.filter((entry) =>
      entry.SPDXID.startsWith("SPDXRef-Dependency-"),
    ).length,
    Object.keys(lock.packages).length,
  );
  assert.equal(document.files.length, files.length);
  assert.ok(
    document.relationships.some(
      (entry) =>
        entry.relationshipType === "DEPENDS_ON" &&
        document.packages.some(
          (candidate) =>
            candidate.SPDXID === entry.relatedSpdxElement &&
            candidate.name === "@electron/get",
        ),
    ),
  );
});

test("release evidence verification detects artifact, SBOM, and publication tampering", async () => {
  const directory = await mkdtemp(join(tmpdir(), "leapview-evidence-test-"));
  const artifactPath = join(directory, "LeapView-darwin-arm64-0.1.0.pkg");
  const sbomPath = join(directory, "release.spdx.json");
  const manifestPath = join(directory, "release.json");
  const checksumsPath = join(directory, "checksums.txt");
  await writeFile(artifactPath, "candidate");

  const files = [
    {
      path: "LeapView.app/Contents/Resources/app.asar",
      sha1: "2".repeat(40),
      sha256: "b".repeat(64),
      type: "ARCHIVE",
    },
  ];
  const sbom = buildSpdxDocument({
    artifactSha256:
      "dda18a0e21ae47c53b4309434cbc02ae8bf764fa83a6defbb719431242722aa7",
    createdAt: "2026-07-29T12:00:00.000Z",
    files,
    lock: { workspaces: { "": { devDependencies: {} } }, packages: {} },
    packageDocument,
    packageVerification,
    sourceSha: "d".repeat(40),
  });
  await writeFile(sbomPath, `${JSON.stringify(sbom, null, 2)}\n`);

  const manifest = await createReleaseManifest({
    artifactPath,
    createdAt: "2026-07-29T12:00:00.000Z",
    lockfileSha256: "e".repeat(64),
    packageDocument,
    packageDocumentSha256: "f".repeat(64),
    packageVerification,
    policy,
    policySha256: "0".repeat(64),
    sbomPath,
    source: {
      commit: "d".repeat(40),
      repository: "flidai/leapview",
      workflowRef:
        "flidai/leapview/.github/workflows/electron-security-proof.yml@refs/heads/main",
      workflowRevision: "d".repeat(40),
      runId: "123",
      runAttempt: "1",
      dirty: false,
    },
    channel: "pull-request",
  });
  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
  const manifestSha256 = createHash("sha256")
    .update(await readFile(manifestPath))
    .digest("hex");
  await writeFile(
    checksumsPath,
    `${manifest.artifact.sha256} *${manifest.artifact.fileName}\n${manifest.sbom.sha256} *${manifest.sbom.fileName}\n${manifestSha256} *${basename(manifestPath)}\n`,
  );

  await assert.doesNotReject(() =>
    verifyReleaseEvidence({
      artifactPath,
      checksumsPath,
      manifestPath,
      policy,
      sbomPath,
    }),
  );
  await assert.rejects(
    () =>
      verifyReleaseEvidence({
        artifactPath,
        checksumsPath,
        manifestPath,
        policy,
        publication: true,
        sbomPath,
      }),
    /signed release/,
  );

  const injected = { ...manifest, instanceOrigin: "https://tenant.invalid" };
  await writeFile(manifestPath, `${JSON.stringify(injected, null, 2)}\n`);
  const injectedManifestSha256 = createHash("sha256")
    .update(await readFile(manifestPath))
    .digest("hex");
  await writeFile(
    checksumsPath,
    `${manifest.artifact.sha256} *${manifest.artifact.fileName}\n${manifest.sbom.sha256} *${manifest.sbom.fileName}\n${injectedManifestSha256} *${basename(manifestPath)}\n`,
  );
  await assert.rejects(
    () =>
      verifyReleaseEvidence({
        artifactPath,
        checksumsPath,
        manifestPath,
        policy,
        sbomPath,
      }),
    /unexpected fields/,
  );

  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
  await writeFile(
    checksumsPath,
    `${manifest.artifact.sha256} *${manifest.artifact.fileName}\n${manifest.sbom.sha256} *${manifest.sbom.fileName}\n${manifestSha256} *${basename(manifestPath)}\n`,
  );
  await writeFile(artifactPath, "tampered");
  await assert.rejects(
    () =>
      verifyReleaseEvidence({
        artifactPath,
        checksumsPath,
        manifestPath,
        policy,
        sbomPath,
      }),
    /artifact checksum/,
  );
});
