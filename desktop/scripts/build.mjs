import { spawnSync } from "node:child_process";
import { copyFile, mkdir, rm } from "node:fs/promises";
import { join, resolve } from "node:path";

const desktopRoot = resolve(import.meta.dirname, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const outputRoot = join(desktopRoot, "dist");
const fontFiles = [
  "inter-cyrillic-ext-wght-normal.woff2",
  "inter-cyrillic-wght-normal.woff2",
  "inter-greek-ext-wght-normal.woff2",
  "inter-greek-wght-normal.woff2",
  "inter-latin-ext-wght-normal.woff2",
  "inter-latin-wght-normal.woff2",
  "inter-vietnamese-wght-normal.woff2",
];

await rm(outputRoot, { force: true, recursive: true });
const compiler = spawnSync(
  process.execPath,
  [
    join(desktopRoot, "node_modules", "typescript", "bin", "tsc"),
    "-p",
    join(desktopRoot, "tsconfig.json"),
  ],
  {
    cwd: desktopRoot,
    stdio: "inherit",
  },
);
if (compiler.error !== undefined) {
  throw compiler.error;
}
if (compiler.status !== 0) {
  process.exit(compiler.status ?? 1);
}

await copyFile(
  join(repositoryRoot, "static", "app.css"),
  join(outputRoot, "app.css"),
);
const fontOutput = join(outputRoot, "files");
await mkdir(fontOutput, { recursive: true });
await Promise.all(
  fontFiles.map((name) =>
    copyFile(
      join(repositoryRoot, "static", "files", name),
      join(fontOutput, name),
    ),
  ),
);
