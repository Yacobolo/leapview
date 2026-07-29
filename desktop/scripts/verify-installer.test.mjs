import assert from "node:assert/strict";
import test from "node:test";

import { validateInstallerContract } from "./verify-installer.mjs";

test("installer verification accepts only the selected machine-managed formats", () => {
  for (const [platform, format] of [
    ["darwin", "pkg"],
    ["linux", "deb"],
    ["win32", "msi"],
  ]) {
    assert.deepEqual(
      validateInstallerContract({
        platform,
        format,
        scope: "per-machine",
        policyIntegration: "administrator-owned-retained",
        protocolIntegration: "installer-owned-quoted-single-url",
      }),
      {
        format,
        scope: "per-machine",
        policyIntegration: "administrator-owned-retained",
        protocolIntegration: "installer-owned-quoted-single-url",
      },
    );
  }
  assert.throws(
    () =>
      validateInstallerContract({
        platform: "win32",
        format: "zip",
        scope: "per-user",
        policyIntegration: "unchecked",
        protocolIntegration: "runtime-owned",
      }),
    /incomplete/,
  );
});
