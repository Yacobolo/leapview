import { execFile } from "node:child_process";
import { rm } from "node:fs/promises";
import { basename, join, resolve } from "node:path";
import { promisify } from "node:util";

import {
  MakerBase,
  type MakerOptions,
} from "@electron-forge/maker-base";
import type { ForgePlatform } from "@electron-forge/shared-types";

const execFileAsync = promisify(execFile);

export interface MakerPKGConfig {
  identity?: string;
  installLocation: "/Applications";
  keychain?: string;
  scripts: string;
}

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
    await this.ensureFile(output);
    await this.ensureFile(component);
    try {
      await runFile("pkgbuild", [
        "--install-location",
        this.config.installLocation,
        "--component",
        join(dir, `${appName}.app`),
        "--scripts",
        this.config.scripts,
        component,
      ]);
      const arguments_ = [
        "--package",
        component,
        this.config.installLocation,
      ];
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
      await rm(component, { force: true });
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
