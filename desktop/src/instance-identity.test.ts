import { describe, expect, test } from "bun:test";

import { isValidInstanceID } from "./instance-identity.js";

describe("isValidInstanceID", () => {
  test("accepts durable server and desktop instance ids", () => {
    expect(isValidInstanceID("lvinst_0123456789abcdefghijklmnopqrstuv")).toBe(true);
    expect(isValidInstanceID("lvinst_0123456789ABCDEFGHIJKLMNOPQRSTUV")).toBe(true);
    expect(
      isValidInstanceID("instance_0123456789abcdef0123456789abcdef"),
    ).toBe(true);
  });

  test("rejects malformed instance ids", () => {
    expect(isValidInstanceID("")).toBe(false);
    expect(isValidInstanceID("lvinst_short")).toBe(false);
    expect(
      isValidInstanceID("instance_0123456789ABCDEF0123456789ABCDEF"),
    ).toBe(false);
  });
});
