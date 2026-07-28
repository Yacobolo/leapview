import { resolve } from "node:path";
import { rm } from "node:fs/promises";

import electronPath from "electron";

const command = process.argv[2];
if (!["start", "package", "make"].includes(command)) {
  throw new Error("usage: run-electron.mjs <start|package|make>");
}

const root = resolve(import.meta.dirname, "..");
const forgeCLI = resolve(
  root,
  "node_modules",
  "@electron-forge",
  "cli",
  "dist",
  "electron-forge.js",
);
const pinnedNode = resolve(
  root,
  "node_modules",
  "node",
  "bin",
  process.platform === "win32" ? "node.exe" : "node",
);
if (command === "package" || command === "make") {
  await rm(resolve(root, "out"), { force: true, recursive: true });
}
const child =
  command === "start"
    ? Bun.spawn({
        cmd: [electronPath, root],
        cwd: root,
        stdin: "inherit",
        stdout: "inherit",
        stderr: "inherit",
      })
    : Bun.spawn({
        cmd: [pinnedNode, forgeCLI, command],
        cwd: root,
        stdin: "inherit",
        stdout: "inherit",
        stderr: "inherit",
      });

process.exit(await child.exited);
