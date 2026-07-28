import type { ForgeConfig } from "@electron-forge/shared-types";
import { MakerZIP } from "@electron-forge/maker-zip";
import { FusesPlugin } from "@electron-forge/plugin-fuses";
import {
  type FuseConfig,
  FuseV1Options,
  FuseVersion,
} from "@electron/fuses";

export const fuseConfig: FuseConfig = {
  version: FuseVersion.V1,
  strictlyRequireAllFuses: true,
  [FuseV1Options.RunAsNode]: false,
  [FuseV1Options.EnableCookieEncryption]: true,
  [FuseV1Options.EnableNodeOptionsEnvironmentVariable]: false,
  [FuseV1Options.EnableNodeCliInspectArguments]: false,
  [FuseV1Options.EnableEmbeddedAsarIntegrityValidation]: true,
  [FuseV1Options.OnlyLoadAppFromAsar]: true,
  [FuseV1Options.LoadBrowserProcessSpecificV8Snapshot]: false,
  [FuseV1Options.GrantFileProtocolExtraPrivileges]: false,
  [FuseV1Options.WasmTrapHandlers]: true,
};

const config: ForgeConfig = {
  packagerConfig: {
    asar: true,
    executableName: "LeapView",
    ignore: [
      /^\/(?!dist(?:\/|$)|package\.json$).+/u,
    ],
  },
  rebuildConfig: {},
  makers: [new MakerZIP({}, ["darwin", "linux", "win32"])],
  plugins: [new FusesPlugin(fuseConfig)],
};

export default config;
