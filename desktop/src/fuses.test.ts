import { expect, test } from "bun:test";
import { FuseV1Options, FuseVersion } from "@electron/fuses";

import { createFuseConfig, fuseConfig } from "../forge.config.js";

test("packaged Electron binaries use the complete fail-closed fuse policy", () => {
  expect(fuseConfig.version).toBe(FuseVersion.V1);
  expect(fuseConfig.strictlyRequireAllFuses).toBe(true);
  expect(fuseConfig[FuseV1Options.RunAsNode]).toBe(false);
  expect(fuseConfig[FuseV1Options.EnableCookieEncryption]).toBe(true);
  expect(
    fuseConfig[FuseV1Options.EnableNodeOptionsEnvironmentVariable],
  ).toBe(false);
  expect(fuseConfig[FuseV1Options.EnableNodeCliInspectArguments]).toBe(false);
  expect(
    fuseConfig[FuseV1Options.EnableEmbeddedAsarIntegrityValidation],
  ).toBe(true);
  expect(fuseConfig[FuseV1Options.OnlyLoadAppFromAsar]).toBe(true);
  expect(
    fuseConfig[FuseV1Options.LoadBrowserProcessSpecificV8Snapshot],
  ).toBe(false);
  expect(fuseConfig[FuseV1Options.GrantFileProtocolExtraPrivileges]).toBe(
    false,
  );
  expect(fuseConfig[FuseV1Options.WasmTrapHandlers]).toBe(true);
});

test("ASAR integrity follows Electron's supported desktop platforms", () => {
  expect(
    createFuseConfig("darwin")[
      FuseV1Options.EnableEmbeddedAsarIntegrityValidation
    ],
  ).toBe(true);
  expect(
    createFuseConfig("win32")[
      FuseV1Options.EnableEmbeddedAsarIntegrityValidation
    ],
  ).toBe(true);
  expect(
    createFuseConfig("linux")[
      FuseV1Options.EnableEmbeddedAsarIntegrityValidation
    ],
  ).toBe(false);
});
