import { createHash } from "node:crypto";
import {
  constants,
  copyFile,
  mkdir,
  readFile,
  readdir,
  writeFile,
} from "node:fs/promises";
import { basename, join, resolve } from "node:path";
import { pathToFileURL } from "node:url";

const targets = [
  { id: "linux-x64", extension: "deb" },
  { id: "macos-arm64", extension: "dmg" },
  { id: "macos-x64", extension: "dmg" },
  { id: "windows-x64", extension: "exe" },
];

export function previewVersionFromTag(tag, baseVersion) {
  const escapedBase = String(baseVersion).replaceAll(
    /[.*+?^${}()|[\]\\]/gu,
    "\\$&",
  );
  const match = new RegExp(
    `^desktop-v(${escapedBase}-alpha\\.([1-9][0-9]*))$`,
    "u",
  ).exec(tag);
  if (match === null) {
    throw new Error(
      `preview release tag must match desktop-v${baseVersion}-alpha.<positive integer>`,
    );
  }
  return match[1];
}

export async function stagePreviewRelease({ candidates, output, version }) {
  previewVersionFromTag(`desktop-v${version}`, version.split("-alpha.")[0]);
  await mkdir(output, { recursive: true });
  const staged = [];
  for (const target of targets) {
    const candidate = join(candidates, `leapview-desktop-${target.id}`);
    for (const file of [
      {
        extension: target.extension,
        source: await findExactlyOne(
          candidate,
          (path) => path.toLowerCase().endsWith(`.${target.extension}`),
          `${target.id} installer`,
        ),
        suffix: target.extension,
      },
      {
        source: await findExactlyOne(
          candidate,
          (path) => path.endsWith(".release.json"),
          `${target.id} release manifest`,
        ),
        suffix: "release.json",
      },
      {
        source: await findExactlyOne(
          candidate,
          (path) => path.endsWith(".spdx.json"),
          `${target.id} SBOM`,
        ),
        suffix: "spdx.json",
      },
    ]) {
      const name = `LeapView-Desktop-${version}-${target.id}.${file.suffix}`;
      await copyFile(file.source, join(output, name), constants.COPYFILE_EXCL);
      staged.push(name);
    }
  }
  staged.sort();
  const checksums = [];
  for (const name of staged) {
    const content = await readFile(join(output, name));
    checksums.push(
      `${createHash("sha256").update(content).digest("hex")}  ${basename(name)}\n`,
    );
  }
  await writeFile(join(output, "SHA256SUMS"), checksums.join(""), {
    flag: "wx",
  });
  return [...staged, "SHA256SUMS"];
}

async function findExactlyOne(root, predicate, label) {
  const matches = [];
  async function visit(directory) {
    let entries;
    try {
      entries = await readdir(directory, { withFileTypes: true });
    } catch (error) {
      if (
        error instanceof Error &&
        "code" in error &&
        error.code === "ENOENT"
      ) {
        return;
      }
      throw error;
    }
    for (const entry of entries.sort((left, right) =>
      left.name.localeCompare(right.name),
    )) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) {
        await visit(path);
      } else if (entry.isFile() && predicate(path)) {
        matches.push(path);
      }
    }
  }
  await visit(root);
  if (matches.length !== 1) {
    throw new Error(`expected exactly one ${label}, found ${matches.length}`);
  }
  return matches[0];
}

async function main(arguments_) {
  const [command, ...rest] = arguments_;
  const options = parseOptions(rest);
  if (command === "version") {
    process.stdout.write(
      `${previewVersionFromTag(options.tag, options["base-version"])}\n`,
    );
    return;
  }
  if (command === "stage") {
    const staged = await stagePreviewRelease({
      candidates: resolve(options.candidates),
      output: resolve(options.output),
      version: options.version,
    });
    process.stdout.write(`${JSON.stringify(staged)}\n`);
    return;
  }
  throw new Error("usage: preview-release.mjs <version|stage> [options]");
}

function parseOptions(arguments_) {
  const options = {};
  for (let index = 0; index < arguments_.length; index += 2) {
    const name = arguments_[index];
    const value = arguments_[index + 1];
    if (!name?.startsWith("--") || value === undefined) {
      throw new Error("preview release options must be name/value pairs");
    }
    options[name.slice(2)] = value;
  }
  return options;
}

if (import.meta.url === pathToFileURL(process.argv[1] ?? "").href) {
  await main(process.argv.slice(2));
}
