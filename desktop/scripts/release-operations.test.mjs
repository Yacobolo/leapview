import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import test from "node:test";

test("release runbook defines every protected and externally owned gate", async () => {
  const body = await readFile(
    resolve(import.meta.dirname, "..", "RELEASING.md"),
    "utf8",
  );
  const normalized = body.toLowerCase();
  for (const required of [
    "No production artifact is built from a pull request",
    "Apple Developer ID Application",
    "notarization",
    "Windows signing",
    "APT repository",
    "independent security assessor",
    "VoiceOver",
    "NVDA",
    "Orca",
    "immutable version directory",
    "never moves backward",
    "withdrawal",
    "retained",
    "rotation drill",
    "byte-for-byte reproducibility",
    "source commit",
    "SBOM",
    "provenance",
    "privacy-safe",
  ]) {
    assert.ok(normalized.includes(required.toLowerCase()), `release runbook is missing ${required}`);
  }
  for (const forbidden of [
    "APPLE_ID_PASSWORD=",
    "CERTIFICATE_PASSWORD=",
    "BEGIN PRIVATE KEY",
  ]) {
    assert.ok(!body.includes(forbidden), `release runbook embeds ${forbidden}`);
  }
});
