import assert from "node:assert/strict";
import test from "node:test";

import { requirePackagedDistribution } from "./distribution-packaging.mjs";

test("packaging accepts only explicit preview or stable identity", () => {
  assert.equal(requirePackagedDistribution("preview"), "preview");
  assert.equal(requirePackagedDistribution("stable"), "stable");
  for (const value of [undefined, "", "development", "production", "Preview"]) {
    assert.throws(
      () => requirePackagedDistribution(value),
      /must be explicitly set to preview or stable/,
    );
  }
});
