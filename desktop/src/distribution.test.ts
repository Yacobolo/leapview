import { describe, expect, test } from "bun:test";

import {
  PREVIEW_DISTRIBUTION_MARKER,
  resolveDesktopDistribution,
} from "./distribution.js";

describe("resolveDesktopDistribution", () => {
  test("keeps unpackaged development builds off release channels", () => {
    expect(
      resolveDesktopDistribution({
        packaged: false,
        resourcesPath: "/unused",
        readFile: () => {
          throw new Error("development must not inspect packaged resources");
        },
      }),
    ).toBe("development");
  });

  test("recognizes only the exact packaged preview marker", () => {
    expect(
      resolveDesktopDistribution({
        packaged: true,
        resourcesPath: "/application/resources",
        readFile: (path) => {
          expect(path).toBe(
            `/application/resources/${PREVIEW_DISTRIBUTION_MARKER}`,
          );
          return '{"schemaVersion":1,"channel":"preview","updates":false}\n';
        },
      }),
    ).toBe("preview");
  });

  test("uses stable only when the preview marker is absent", () => {
    expect(
      resolveDesktopDistribution({
        packaged: true,
        resourcesPath: "/application/resources",
        readFile: () => {
          const error = new Error("missing") as NodeJS.ErrnoException;
          error.code = "ENOENT";
          throw error;
        },
      }),
    ).toBe("stable");
  });

  test("fails closed when a packaged marker is malformed", () => {
    expect(
      resolveDesktopDistribution({
        packaged: true,
        resourcesPath: "/application/resources",
        readFile: () => '{"schemaVersion":1,"channel":"stable"}',
      }),
    ).toBe("invalid");
  });
});
