import { readFileSync } from "node:fs";
import { join } from "node:path";

export const PREVIEW_DISTRIBUTION_MARKER = "preview-distribution.json";
const previewMarker = '{"schemaVersion":1,"channel":"preview","updates":false}\n';

export type DesktopDistribution =
  | "development"
  | "preview"
  | "stable"
  | "invalid";

export interface DesktopDistributionOptions {
  packaged: boolean;
  resourcesPath: string;
  readFile?: (path: string) => string;
}

export function resolveDesktopDistribution(
  options: DesktopDistributionOptions,
): DesktopDistribution {
  if (!options.packaged) {
    return "development";
  }
  const readFile =
    options.readFile ?? ((path: string) => readFileSync(path, "utf8"));
  try {
    return readFile(join(options.resourcesPath, PREVIEW_DISTRIBUTION_MARKER)) ===
        previewMarker
      ? "preview"
      : "invalid";
  } catch (error) {
    return error instanceof Error &&
        "code" in error &&
        error.code === "ENOENT"
      ? "stable"
      : "invalid";
  }
}
