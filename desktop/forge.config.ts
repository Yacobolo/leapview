import type { ForgeConfig } from "@electron-forge/shared-types";
import { MakerZIP } from "@electron-forge/maker-zip";
import { FusesPlugin } from "@electron-forge/plugin-fuses";
import {
  type FuseConfig,
  FuseV1Options,
  FuseVersion,
} from "@electron/fuses";

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
    asar: true,
    executableName: "LeapView",
    protocols: [
      {
        name: "LeapView Desktop",
        schemes: ["leapview-desktop"],
      },
    ],
    ignore: [
      /^\/(?!dist(?:\/|$)|package\.json$).+/u,
    ],
  },
  rebuildConfig: {},
  makers: [new MakerZIP({}, ["darwin", "linux", "win32"])],
  plugins: [new FusesPlugin(fuseConfig)],
};

export default config;
