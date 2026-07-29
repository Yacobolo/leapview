import { createHash } from "node:crypto";
import { readFile, stat } from "node:fs/promises";
import { basename, resolve } from "node:path";
import { pathToFileURL } from "node:url";

export async function verifyReleaseEvidence({
  artifactPath,
  checksumsPath,
  manifestPath,
  policy,
  policySha256,
  publication = false,
  sbomPath,
  updateArtifactPaths = [],
}) {
  assertConsumerReleasePolicy(policy);
  const [
    artifact,
    updateArtifacts,
    checksumsText,
    manifestText,
    sbomText,
  ] = await Promise.all([
    identity(artifactPath),
    Promise.all(
      updateArtifactPaths.map(async (path) => ({
        ...(await identity(path)),
        fileName: basename(path),
      })),
    ),
    readFile(checksumsPath, "utf8"),
    readFile(manifestPath, "utf8"),
    readFile(sbomPath, "utf8"),
  ]);
  const manifest = parseExactJson(manifestText, "release manifest");
  const sbom = parseExactJson(sbomText, "SPDX SBOM");
  const sbomSha256 = sha256(sbomText);
  const manifestSha256 = sha256(manifestText);
  assertExactKeys(
    manifest,
    [
      "application",
      "artifact",
      "attestations",
      "channel",
      "createdAt",
      "packageVerification",
      "privacy",
      "reproducibility",
      "sbom",
      "schemaVersion",
      "signing",
      "source",
      "support",
      "toolchain",
      "updateArtifacts",
    ],
    "release manifest",
  );
  assertExactKeys(
    manifest.source,
    [
      "commit",
      "dirty",
      "repository",
      "runAttempt",
      "runId",
      "workflowRef",
      "workflowRevision",
    ],
    "release source",
  );
  assertExactKeys(
    manifest.packageVerification,
    [
      "architecture",
      "accessibility",
      "asarFiles",
      "asarOnly",
      "fuses",
      "installer",
      "packageFormat",
      "platform",
      "runtime",
      "schemaVersion",
      "startup",
    ],
    "package verification",
  );
  for (const [value, expected, label] of [
    [manifest.application, ["name", "packageName", "version"], "application"],
    [
      manifest.toolchain,
      ["bun", "chromium", "electron", "electronMajor", "forge", "node"],
      "toolchain",
    ],
    [
      manifest.artifact,
      ["architecture", "bytes", "fileName", "format", "platform", "sha256"],
      "artifact",
    ],
    [manifest.sbom, ["fileName", "format", "sha256"], "SBOM identity"],
    [
      manifest.reproducibility,
      [
        "lockfileSha256",
        "packageDocumentSha256",
        "policySha256",
        "sourceDateEpoch",
      ],
      "reproducibility",
    ],
    [manifest.support, ["minimumVersion", "qualification"], "support"],
    [manifest.signing, ["identity", "productionEligible", "state"], "signing"],
    [
      manifest.attestations,
      [
        "generatedForMainCandidates",
        "provenanceRequiredForPublication",
        "provider",
        "sbomRequiredForPublication",
      ],
      "attestations",
    ],
    [
      manifest.privacy,
      [
        "evidenceContainsCredentials",
        "evidenceContainsCustomerData",
        "evidenceContainsDiagnostics",
      ],
      "privacy",
    ],
    [
      manifest.packageVerification.accessibility,
      [
        "announcement",
        "controls",
        "focusedControl",
        "mode",
        "regions",
      ],
      "accessibility verification",
    ],
    [
      manifest.packageVerification.runtime,
      ["chromium", "electron", "node"],
      "package runtime",
    ],
    [
      manifest.packageVerification.installer,
      [
        "format",
        "policyIntegration",
        "protocolIntegration",
        "scope",
        "updateArtifacts",
        "updateMechanism",
      ],
      "installer verification",
    ],
  ]) {
    assertExactKeys(value, expected, label);
  }
  if (
    manifest?.schemaVersion !== 2 ||
    manifest.artifact?.fileName !== basename(artifactPath) ||
    manifest.artifact?.bytes !== artifact.bytes ||
    manifest.artifact?.sha256 !== artifact.sha256
  ) {
    throw new Error(
      "artifact checksum or metadata does not match release manifest",
    );
  }
  const expectedUpdateFormats =
    policy.distribution?.[manifest.artifact?.platform]?.updateArtifacts;
  if (
    !Array.isArray(manifest.updateArtifacts) ||
    !Array.isArray(expectedUpdateFormats) ||
    manifest.updateArtifacts.length !== expectedUpdateFormats.length ||
    updateArtifacts.length !== expectedUpdateFormats.length
  ) {
    throw new Error("release update artifact set is incomplete");
  }
  for (let index = 0; index < expectedUpdateFormats.length; index += 1) {
    const declared = manifest.updateArtifacts[index];
    const actual = updateArtifacts[index];
    assertExactKeys(
      declared,
      ["bytes", "fileName", "format", "sha256"],
      "release update artifact",
    );
    if (
      declared.format !== expectedUpdateFormats[index] ||
      declared.fileName !== actual.fileName ||
      declared.bytes !== actual.bytes ||
      declared.sha256 !== actual.sha256 ||
      !matchesUpdateArtifactFormat(declared.fileName, declared.format)
    ) {
      throw new Error(
        "update artifact checksum or metadata does not match release manifest",
      );
    }
  }
  if (manifest.artifact.platform === "win32") {
    validateSquirrelReleaseIndex(
      await readFile(updateArtifactPaths[1], "utf8"),
      updateArtifacts[0],
    );
  }
  if (
    !/^[0-9a-f]{40}$/.test(manifest.source.commit ?? "") ||
    !/^[0-9a-f]{40}$/.test(manifest.source.workflowRevision ?? "") ||
    typeof manifest.source.dirty !== "boolean" ||
    Number.isNaN(Date.parse(manifest.createdAt)) ||
    new Date(manifest.createdAt).toISOString() !== manifest.createdAt
  ) {
    throw new Error("release source identity is incomplete");
  }
  if (
    manifest.sbom?.fileName !== basename(sbomPath) ||
    manifest.sbom?.sha256 !== sbomSha256 ||
    manifest.sbom?.format !== "SPDX-2.3-json"
  ) {
    throw new Error(
      "SBOM checksum or metadata does not match release manifest",
    );
  }
  const expectedChecksums = [
    { ...artifact, fileName: basename(artifactPath) },
    ...updateArtifacts,
    { sha256: sbomSha256, fileName: basename(sbomPath) },
    { sha256: manifestSha256, fileName: basename(manifestPath) },
  ]
    .map((entry) => `${entry.sha256} *${entry.fileName}\n`)
    .join("");
  if (checksumsText !== expectedChecksums) {
    throw new Error("checksum file is incomplete or does not match evidence");
  }
  if (
    sbom?.spdxVersion !== "SPDX-2.3" ||
    sbom.dataLicense !== "CC0-1.0" ||
    sbom.SPDXID !== "SPDXRef-DOCUMENT" ||
    !Array.isArray(sbom.packages) ||
    !Array.isArray(sbom.files) ||
    !Array.isArray(sbom.relationships)
  ) {
    throw new Error("SBOM is not a complete SPDX 2.3 JSON document");
  }
  const rootPackage = sbom.packages.find(
    (candidate) => candidate.SPDXID === "SPDXRef-Package-LeapView-Desktop",
  );
  if (
    rootPackage?.versionInfo !== manifest.application?.version ||
    !rootPackage.checksums?.some(
      (checksum) =>
        checksum.algorithm === "SHA256" &&
        checksum.checksumValue.toLowerCase() === artifact.sha256,
    )
  ) {
    throw new Error("SBOM is not bound to the release artifact");
  }
  if (
    manifest.toolchain?.electron !== policy.runtime?.electron ||
    Number.parseInt(manifest.toolchain.electron, 10) !==
      policy.runtime.electronMajor ||
    manifest.artifact?.format !==
      policy.distribution?.[manifest.artifact?.platform]?.installer ||
    manifest.packageVerification?.installer?.format !==
      manifest.artifact?.format ||
    manifest.packageVerification?.installer?.scope !==
      policy.distribution?.[manifest.artifact?.platform]?.scope ||
    manifest.packageVerification?.installer?.updateMechanism !==
      policy.distribution?.[manifest.artifact?.platform]?.updateMechanism ||
    JSON.stringify(
      manifest.packageVerification?.installer?.updateArtifacts,
    ) !==
      JSON.stringify(
        policy.distribution?.[manifest.artifact?.platform]
          ?.updateArtifacts,
      ) ||
    manifest.packageVerification?.schemaVersion !== 2 ||
    manifest.packageVerification?.asarOnly !== policy.hardening?.asarOnly ||
    !validAccessibilityVerification(
      manifest.packageVerification?.accessibility,
    )
  ) {
    throw new Error(
      "release metadata does not match the immutable release policy",
    );
  }
  if (
    policySha256 !== undefined &&
    manifest.reproducibility?.policySha256 !== policySha256
  ) {
    throw new Error("release policy checksum does not match release metadata");
  }
  for (const [runtime, version] of Object.entries(policy.runtime ?? {})) {
    if (
      runtime !== "electronMajor" &&
      manifest.toolchain?.[runtime] !== version
    ) {
      throw new Error(`release metadata has an unsupported ${runtime} version`);
    }
  }
  const support = policy.supportMatrix?.find(
    (candidate) => candidate.platform === manifest.artifact.platform,
  );
  if (
    support === undefined ||
    !support.architectures.includes(manifest.artifact.architecture) ||
    support.minimumVersion !== manifest.support?.minimumVersion
  ) {
    throw new Error("release target is outside the support matrix");
  }
  if (
    Object.values(manifest.privacy ?? {}).some((value) => value !== false) ||
    JSON.stringify(manifest.privacy) !== JSON.stringify(policy.privacy)
  ) {
    throw new Error("release evidence privacy declaration is invalid");
  }
  if (
    publication &&
    (manifest.signing?.state !== "signed" ||
      manifest.signing?.productionEligible !== true ||
      typeof manifest.signing?.identity !== "string" ||
      manifest.signing.identity.length === 0 ||
      manifest.source.dirty !== false)
  ) {
    throw new Error("publication requires a verified signed release");
  }
  return {
    artifactSha256: artifact.sha256,
    platform: manifest.artifact.platform,
    architecture: manifest.artifact.architecture,
    status: publication
      ? "verified-publishable-release"
      : "verified-unsigned-candidate",
  };
}

function validAccessibilityVerification(accessibility) {
  if (
    accessibility === null ||
    typeof accessibility !== "object" ||
    !Array.isArray(accessibility.regions) ||
    !Number.isSafeInteger(accessibility.controls) ||
    accessibility.controls < 0
  ) {
    return false;
  }
  if (accessibility.mode === "open") {
    return (
      accessibility.announcement === "none" &&
      accessibility.controls >= 2 &&
      accessibility.focusedControl === "LeapView URL" &&
      accessibility.regions.includes("Connect an instance")
    );
  }
  return (
    accessibility.mode === "locked" &&
    accessibility.announcement === "assertive" &&
    accessibility.controls === 0 &&
    [
      "Application document",
      "Managed configuration error",
    ].includes(accessibility.focusedControl)
  );
}

async function main() {
  const argumentsByName = parseArguments(process.argv.slice(2));
  for (const required of [
    "artifact",
    "checksums",
    "manifest",
    "policy",
    "sbom",
  ]) {
    if (argumentsByName[required] === undefined) {
      throw new Error(
        "usage: verify-release-evidence.mjs --artifact <installer> [--update-artifact <companion>]... --checksums <txt> --manifest <json> --policy <json> --sbom <json> [--publication]",
      );
    }
  }
  const policyText = await readFile(argumentsByName.policy, "utf8");
  const policy = parseExactJson(policyText, "release policy");
  const result = await verifyReleaseEvidence({
    artifactPath: argumentsByName.artifact,
    checksumsPath: argumentsByName.checksums,
    manifestPath: argumentsByName.manifest,
    policy,
    policySha256: sha256(policyText),
    publication: argumentsByName.publication === true,
    sbomPath: argumentsByName.sbom,
    updateArtifactPaths: argumentsByName.updateArtifact ?? [],
  });
  process.stdout.write(`${JSON.stringify(result)}\n`);
}

function parseArguments(args) {
  const parsed = {};
  for (let index = 0; index < args.length; index += 1) {
    const argument = args[index];
    if (argument === "--publication") {
      parsed.publication = true;
      continue;
    }
    if (argument === "--update-artifact") {
      if (args[index + 1] === undefined) {
        throw new Error("invalid release evidence update artifact");
      }
      parsed.updateArtifact ??= [];
      parsed.updateArtifact.push(args[index + 1]);
      index += 1;
      continue;
    }
    if (!argument.startsWith("--") || args[index + 1] === undefined) {
      throw new Error(`invalid release evidence argument ${argument}`);
    }
    parsed[argument.slice(2)] = args[index + 1];
    index += 1;
  }
  return parsed;
}

function matchesUpdateArtifactFormat(fileName, format) {
  if (format === "RELEASES") {
    return fileName === "RELEASES";
  }
  return fileName.toLowerCase().endsWith(`.${format}`);
}

export function validateSquirrelReleaseIndex(indexText, nupkg) {
  const line = indexText.replace(/^\uFEFF/, "").trim();
  const match =
    /^([0-9a-f]{40}) ([^/\\\s]+\.nupkg) ([1-9][0-9]*)$/i.exec(line);
  if (
    match === null ||
    match[1].toLowerCase() !== nupkg.sha1 ||
    match[2] !== nupkg.fileName ||
    Number.parseInt(match[3], 10) !== nupkg.bytes
  ) {
    throw new Error("Squirrel RELEASES does not bind the exact nupkg");
  }
}

async function identity(path) {
  const [content, metadata] = await Promise.all([readFile(path), stat(path)]);
  return {
    bytes: metadata.size,
    sha1: createHash("sha1").update(content).digest("hex"),
    sha256: sha256(content),
  };
}

function assertConsumerReleasePolicy(policy) {
  const expectedDistribution = {
    darwin: {
      installer: "dmg",
      updateArtifacts: ["zip"],
      updateMechanism: "squirrel-mac",
      scope: "user-installed",
    },
    linux: {
      installer: "deb",
      updateArtifacts: [],
      updateMechanism: "apt",
      scope: "system-package-manager",
    },
    win32: {
      installer: "exe",
      updateArtifacts: ["nupkg", "RELEASES"],
      updateMechanism: "squirrel-windows",
      scope: "per-user",
    },
  };
  if (
    policy?.schemaVersion !== 2 ||
    policy.channel !== "consumer-v1" ||
    JSON.stringify(policy.distribution) !==
      JSON.stringify(expectedDistribution)
  ) {
    throw new Error("release evidence requires the consumer release policy");
  }
}

function parseExactJson(text, label) {
  try {
    const parsed = JSON.parse(text);
    if (
      parsed === null ||
      typeof parsed !== "object" ||
      Array.isArray(parsed)
    ) {
      throw new Error(`${label} must be a JSON object`);
    }
    return parsed;
  } catch (error) {
    throw new Error(`${label} is not valid JSON: ${error.message}`);
  }
}

function assertExactKeys(value, expected, label) {
  if (
    value === null ||
    typeof value !== "object" ||
    Array.isArray(value) ||
    Object.keys(value).sort().join(",") !== [...expected].sort().join(",")
  ) {
    throw new Error(`${label} contains missing or unexpected fields`);
  }
}

function sha256(value) {
  return createHash("sha256").update(value).digest("hex");
}

if (
  process.argv[1] !== undefined &&
  import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  await main();
}
