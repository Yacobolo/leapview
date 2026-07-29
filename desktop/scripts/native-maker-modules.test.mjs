import assert from "node:assert/strict";
import test from "node:test";

import { nativeMakerPreparation } from "./native-maker-modules.mjs";

test("prepares only the macOS DMG native helper with pinned build tools", () => {
  assert.deepEqual(nativeMakerPreparation("darwin", "/desktop"), {
    addon: "/desktop/node_modules/macos-alias/build/Release/volume.node",
    packageDirectory: "/desktop/node_modules/macos-alias",
    pinnedNode: "/desktop/node_modules/node/bin/node",
    nodeGyp: "/desktop/node_modules/@electron/node-gyp/bin/node-gyp.js",
  });
  assert.equal(nativeMakerPreparation("linux", "/desktop"), null);
  assert.equal(nativeMakerPreparation("win32", "/desktop"), null);
});
