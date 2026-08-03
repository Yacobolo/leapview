import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { mkdtemp, mkdir, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { basename, join } from "node:path";
import test from "node:test";

import {
  previewVersionFromTag,
  stagePreviewRelease,
} from "./preview-release.mjs";

test("preview tags are exact prerelease versions", () => {
  assert.equal(
    previewVersionFromTag("desktop-v0.1.0-alpha.7", "0.1.0"),
    "0.1.0-alpha.7",
  );
  for (const tag of [
    "v0.1.0-alpha.7",
    "desktop-v0.1.0",
    "desktop-v0.1.1-alpha.1",
    "desktop-v0.1.0-alpha.0",
    "desktop-v0.1.0-alpha.1/../latest",
  ]) {
    assert.throws(
      () => previewVersionFromTag(tag, "0.1.0"),
      /preview release tag/,
    );
  }
});

test("staging gives every installer and evidence file an immutable public name", async () => {
  const root = await mkdtemp(join(tmpdir(), "leapview-preview-"));
  const candidates = join(root, "candidates");
  const output = join(root, "release");
  const version = "0.1.0-alpha.3";
  const matrix = [
    ["macos-arm64", "dmg"],
    ["macos-x64", "dmg"],
    ["windows-x64", "exe"],
    ["linux-x64", "deb"],
  ];
  for (const [target, extension] of matrix) {
    const directory = join(candidates, `leapview-desktop-${target}`);
    await mkdir(join(directory, "out", "make"), { recursive: true });
    await mkdir(join(directory, "out", "evidence"), { recursive: true });
    await writeFile(
      join(directory, "out", "make", `candidate.${extension}`),
      `${target}-installer`,
    );
    await writeFile(
      join(directory, "out", "evidence", `${target}.release.json`),
      `${target}-manifest`,
    );
    await writeFile(
      join(directory, "out", "evidence", `${target}.spdx.json`),
      `${target}-sbom`,
    );
  }

  const staged = await stagePreviewRelease({ candidates, output, version });
  assert.deepEqual(staged, [
    `LeapView-Desktop-${version}-linux-x64.deb`,
    `LeapView-Desktop-${version}-linux-x64.release.json`,
    `LeapView-Desktop-${version}-linux-x64.spdx.json`,
    `LeapView-Desktop-${version}-macos-arm64.dmg`,
    `LeapView-Desktop-${version}-macos-arm64.release.json`,
    `LeapView-Desktop-${version}-macos-arm64.spdx.json`,
    `LeapView-Desktop-${version}-macos-x64.dmg`,
    `LeapView-Desktop-${version}-macos-x64.release.json`,
    `LeapView-Desktop-${version}-macos-x64.spdx.json`,
    `LeapView-Desktop-${version}-windows-x64.exe`,
    `LeapView-Desktop-${version}-windows-x64.release.json`,
    `LeapView-Desktop-${version}-windows-x64.spdx.json`,
    "SHA256SUMS",
  ]);
  const checksums = await readFile(join(output, "SHA256SUMS"), "utf8");
  for (const name of staged.slice(0, -1)) {
    const content = await readFile(join(output, name));
    const digest = createHash("sha256").update(content).digest("hex");
    assert.ok(
      checksums.includes(`${digest}  ${basename(name)}\n`),
      `missing checksum for ${name}`,
    );
  }
});

test("staging rejects incomplete candidate sets", async () => {
  const root = await mkdtemp(join(tmpdir(), "leapview-preview-missing-"));
  await assert.rejects(
    () =>
      stagePreviewRelease({
        candidates: root,
        output: join(root, "release"),
        version: "0.1.0-alpha.1",
      }),
    /expected exactly one/,
  );
});
