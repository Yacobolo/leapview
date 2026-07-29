import { execFile } from "node:child_process";
import {
  mkdir,
  rm,
  writeFile,
} from "node:fs/promises";
import { basename, dirname, join, resolve } from "node:path";
import { promisify } from "node:util";

import {
  MakerBase,
  type MakerOptions,
} from "@electron-forge/maker-base";
import type { ForgePlatform } from "@electron-forge/shared-types";

const execFileAsync = promisify(execFile);

export interface MakerPKGConfig {
  identifier: "dev.leapview.desktop";
  identity?: string;
  installLocation: "/Applications";
  keychain?: string;
  scripts: string;
}

export const macOSComponentPropertyList = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<array>
  <dict>
    <key>BundleHasStrictIdentifier</key>
    <true/>
    <key>BundleIsRelocatable</key>
    <false/>
    <key>BundleIsVersionChecked</key>
    <true/>
    <key>BundleOverwriteAction</key>
    <string>upgrade</string>
    <key>RootRelativeBundlePath</key>
    <string>Applications/LeapView.app</string>
  </dict>
</array>
</plist>
`;

export class MakerPKG extends MakerBase<MakerPKGConfig> {
  name = "pkg";

  defaultPlatforms: ForgePlatform[] = ["darwin"];

  isSupportedOnCurrentPlatform(): boolean {
    return process.platform === "darwin";
  }

  async make({
    dir,
    makeDir,
    appName,
    packageJSON,
    targetPlatform,
    targetArch,
  }: MakerOptions): Promise<string[]> {
    if (targetPlatform !== "darwin") {
      throw new Error(`pkg maker does not support ${targetPlatform}`);
    }
    const output = resolve(
      makeDir,
      `${appName}-${packageJSON.version}-${targetArch}.pkg`,
    );
    const component = resolve(
      makeDir,
      `${appName}-${packageJSON.version}-${targetArch}-component.pkg`,
    );
    const stagingRoot = resolve(
      makeDir,
      `${appName}-${packageJSON.version}-${targetArch}-root`,
    );
    const componentPropertyList = resolve(
      makeDir,
      `${appName}-${packageJSON.version}-${targetArch}-components.plist`,
    );
    await this.ensureFile(output);
    await this.ensureFile(component);
    await rm(stagingRoot, { force: true, recursive: true });
    try {
      const stagedApplication = join(
        stagingRoot,
        this.config.installLocation.slice(1),
        `${appName}.app`,
      );
      await mkdir(dirname(stagedApplication), {
        recursive: true,
      });
      await runFile("ditto", [
        join(dir, `${appName}.app`),
        stagedApplication,
      ]);
      await writeFile(
        componentPropertyList,
        macOSComponentPropertyList,
        "utf8",
      );
      await runFile("pkgbuild", [
        "--root",
        stagingRoot,
        "--component-plist",
        componentPropertyList,
        "--identifier",
        this.config.identifier,
        "--version",
        String(packageJSON.version),
        "--install-location",
        "/",
        "--scripts",
        this.config.scripts,
        component,
      ]);
      const arguments_ = ["--package", component];
      if (this.config.identity !== undefined) {
        arguments_.push("--sign", this.config.identity);
        if (this.config.keychain !== undefined) {
          arguments_.push("--keychain", this.config.keychain);
        }
      }
      arguments_.push(output);
      await runFile("productbuild", arguments_);
      return [output];
    } finally {
      await Promise.all([
        rm(component, { force: true }),
        rm(componentPropertyList, { force: true }),
        rm(stagingRoot, { force: true, recursive: true }),
      ]);
    }
  }
}

async function runFile(
  executable: string,
  arguments_: string[],
): Promise<void> {
  try {
    await execFileAsync(executable, arguments_, {
      encoding: "utf8",
      maxBuffer: 1024 * 1024,
    });
  } catch (error) {
    throw new Error(
      `${basename(executable)} failed while building the macOS installer`,
      { cause: error },
    );
  }
}
