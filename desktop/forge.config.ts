import type { ForgeConfig } from "@electron-forge/shared-types";
import { MakerZIP } from "@electron-forge/maker-zip";

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
};

export default config;
