import type { ForgeConfig } from "@electron-forge/shared-types";
import { MakerDeb } from "@electron-forge/maker-deb";
import { MakerWix } from "@electron-forge/maker-wix";
import { MakerZIP } from "@electron-forge/maker-zip";
import { FusesPlugin } from "@electron-forge/plugin-fuses";
import { resolve } from "node:path";
import {
  type FuseConfig,
  FuseV1Options,
  FuseVersion,
} from "@electron/fuses";

import {
  addWindowsManagedDeployment,
  desktopInstallerContract,
} from "./installer-contract.js";
import { MakerPKG } from "./makers/maker-pkg.js";

const desktopRoot = import.meta.dirname;

export function createFuseConfig(platform: NodeJS.Platform): FuseConfig {
  return {
    version: FuseVersion.V1,
    strictlyRequireAllFuses: true,
    [FuseV1Options.RunAsNode]: false,
    [FuseV1Options.EnableCookieEncryption]: true,
    [FuseV1Options.EnableNodeOptionsEnvironmentVariable]: false,
    [FuseV1Options.EnableNodeCliInspectArguments]: false,
    [FuseV1Options.EnableEmbeddedAsarIntegrityValidation]:
      platform === "darwin" || platform === "win32",
    [FuseV1Options.OnlyLoadAppFromAsar]: true,
    [FuseV1Options.LoadBrowserProcessSpecificV8Snapshot]: false,
    [FuseV1Options.GrantFileProtocolExtraPrivileges]: false,
    [FuseV1Options.WasmTrapHandlers]: true,
  };
}

export const fuseConfig = createFuseConfig(process.platform);

const config: ForgeConfig = {
  packagerConfig: {
    appBundleId: "dev.leapview.desktop",
    asar: true,
    executableName: "LeapView",
    extraResource:
      process.platform === "win32"
        ? [
            resolve(
              desktopRoot,
              "dist/native/leapview-windows-policy.exe",
            ),
          ]
        : [],
    protocols: [
      {
        name: "LeapView Desktop",
        schemes: ["leapview-desktop"],
      },
    ],
    ignore: [
      /^\/(?!dist(?:\/|$)|package\.json$).+/u,
      /^\/dist\/(?:forge\.config\.js|installer-contract\.js|makers(?:\/|$))/u,
      /^\/dist\/native(?:\/|$)/u,
    ],
  },
  rebuildConfig: {},
  makers: [
    new MakerWix(
      {
        arch: "x64",
        defaultInstallMode: "perMachine",
        description:
          "End-user desktop client for deployed LeapView instances.",
        exe: "LeapView.exe",
        features: false,
        language: 1033,
        manufacturer: "LeapView",
        rebootMode: "ReallySuppress",
        upgradeCode: "6D09CFB5-BA75-4D55-A1D4-2494137EE78D",
        beforeCreate: (creator) => {
          creator.wixTemplate = addWindowsManagedDeployment(
            creator.wixTemplate,
          );
        },
      },
      ["win32"],
    ),
    new MakerPKG(
      {
        installLocation: "/Applications",
        scripts: resolve(desktopRoot, "installer/macos/scripts"),
        ...(process.env.LEAPVIEW_APPLE_INSTALLER_IDENTITY === undefined
          ? {}
          : {
              identity: process.env.LEAPVIEW_APPLE_INSTALLER_IDENTITY,
            }),
        ...(process.env.LEAPVIEW_APPLE_KEYCHAIN === undefined
          ? {}
          : { keychain: process.env.LEAPVIEW_APPLE_KEYCHAIN }),
      },
      ["darwin"],
    ),
    new MakerDeb(
      {
        options: {
          categories: ["Office"],
          description:
            "End-user desktop client for deployed LeapView instances.",
          homepage: "https://leapview.dev",
          maintainer: "LeapView",
          mimeType: [
            `x-scheme-handler/${desktopInstallerContract.protocol.scheme}`,
          ],
          name: "leapview-desktop",
          productDescription:
            "Connects to deployed LeapView instances while preserving server-side authentication, access, and dashboard authority.",
          productName: "LeapView",
          scripts: {
            preinst: resolve(desktopRoot, "installer/linux/preinst"),
            postinst: resolve(desktopRoot, "installer/linux/postinst"),
          },
          section: "utils",
        },
      },
      ["linux"],
    ),
    new MakerZIP({}, ["darwin", "linux", "win32"]),
  ],
  plugins: [new FusesPlugin(fuseConfig)],
};

export default config;
