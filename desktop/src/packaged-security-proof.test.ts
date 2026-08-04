import { describe, expect, test } from "bun:test";

import { packagedProofOrigin } from "./packaged-security-proof-request.js";

describe("packagedProofOrigin", () => {
  test("accepts only an exact loopback HTTP origin", () => {
    expect(packagedProofOrigin("http://127.0.0.1:49152")).toBe(
      "http://127.0.0.1:49152",
    );
    for (const candidate of [
      "https://127.0.0.1:49152",
      "http://localhost:49152",
      "http://attacker.example:49152",
      "http://127.0.0.1:49152/path",
      "http://user:secret@127.0.0.1:49152",
    ]) {
      expect(() => packagedProofOrigin(candidate)).toThrow();
    }
  });
});
