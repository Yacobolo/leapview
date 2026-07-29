import { spawnSync } from "node:child_process";
import { copyFile, mkdir, readFile, rm } from "node:fs/promises";
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
await mkdir(outputRoot, { recursive: true });

const themeSource = await readFile(
  join(repositoryRoot, "static", "app.input.css"),
  "utf8",
);
runNode(
  join(
    desktopRoot,
    "node_modules",
    "@tailwindcss",
    "cli",
    "dist",
    "index.mjs",
  ),
  [
    "--input",
    "-",
    "--output",
    join(outputRoot, "app.css"),
    "--cwd",
    desktopRoot,
    "--silent",
  ],
  desktopRoot,
  themeSource,
);
runNode(
  join(desktopRoot, "node_modules", "typescript", "bin", "tsc"),
  ["-p", join(desktopRoot, "tsconfig.json")],
  desktopRoot,
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

if (process.platform === "win32") {
  const nativeOutput = join(outputRoot, "native");
  await mkdir(nativeOutput, { recursive: true });
  run(
    "go",
    [
      "build",
      "-trimpath",
      "-ldflags=-s -w",
      "-o",
      join(nativeOutput, "leapview-windows-policy.exe"),
      "./desktop/native/windowspolicy",
    ],
    repositoryRoot,
  );
}

function runNode(entrypoint, arguments_, cwd, input) {
  run(process.execPath, [entrypoint, ...arguments_], cwd, input);
}

function run(command, arguments_, cwd, input) {
  const result = spawnSync(command, arguments_, {
    cwd,
    input,
    stdio:
      input === undefined ? "inherit" : ["pipe", "inherit", "inherit"],
  });
  if (result.error !== undefined) {
    throw result.error;
  }
  if (result.status !== 0) {
    process.exit(result.status ?? 1);
  }
}
