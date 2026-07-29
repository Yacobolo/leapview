import { expect, test } from "bun:test";
import { FuseV1Options, FuseVersion } from "@electron/fuses";

import forgeConfig, {
  createFuseConfig,
  fuseConfig,
} from "../forge.config.js";

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
  ).toBe(process.platform !== "linux");
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

test("packaged applications declare the isolated desktop deep-link scheme", () => {
  expect(forgeConfig.packagerConfig?.protocols).toEqual([
    {
      name: "LeapView Desktop",
      schemes: ["leapview-desktop"],
    },
  ]);
});

test("production makers match the consumer installer and updater matrix", () => {
  expect(
    forgeConfig.makers?.map((maker) => ({
      name: maker.name,
      platforms: maker.platforms,
    })),
  ).toEqual([
    {
      name: "squirrel",
      platforms: ["win32"],
    },
    {
      name: "dmg",
      platforms: ["darwin"],
    },
    {
      name: "deb",
      platforms: ["linux"],
    },
    {
      name: "zip",
      platforms: ["darwin"],
    },
  ]);
});

test("Debian packaging maps the packaged binary to the stable command name", async () => {
  const maker = forgeConfig.makers?.find(
    (candidate) => candidate.name === "deb",
  );
  expect(maker).toBeDefined();
  await maker?.prepareConfig("x64");
  expect(
    (
      maker as typeof maker & {
        config: { options?: { bin?: string; name?: string } };
      }
    ).config.options,
  ).toMatchObject({
    bin: "LeapView",
    name: "leapview-desktop",
  });
});
