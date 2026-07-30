import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import test from "node:test";

test("desktop workflow builds and qualifies both native macOS architectures", async () => {
  const root = resolve(import.meta.dirname, "..", "..");
  const [workflow, readme] = await Promise.all([
    readFile(resolve(root, ".github/workflows/electron-security-proof.yml"), "utf8"),
    readFile(resolve(root, "desktop/README.md"), "utf8"),
  ]);

  for (const required of [
    "name: macOS Apple silicon",
    "os: macos-15",
    "artifact: macos-arm64",
    "name: macOS Intel",
    "os: macos-15-intel",
    "artifact: macos-x64",
    "name: Candidate proof (macOS ${{ matrix.architecture }})",
    "architecture: Apple silicon",
    "architecture: Intel",
  ]) {
    assert.ok(workflow.includes(required), `workflow is missing ${required}`);
  }
  assert.match(readme, /macOS Intel and Apple-silicon candidates are\s+built natively/);
  assert.doesNotMatch(readme, /Only the Intel macOS/);
});
