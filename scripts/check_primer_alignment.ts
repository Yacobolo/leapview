import {readdir, readFile} from "node:fs/promises";
import path from "node:path";

export type PrimerAlignmentViolation = {
  file: string;
  line: number;
  kind: "raw-color" | "raw-var-fallback" | "undefined-token" | "local-primer-token";
  message: string;
};

type CheckPrimerAlignmentOptions = {
  root?: string;
  referenceRoot?: string;
  sourceFiles?: string[];
};

const cssSourceExtensions = new Set([".css", ".ts"]);
const productSourceRoots = [path.join("web", "components")];
const staticSourceFiles = [path.join("static", "app.input.css")];
const excludedPathParts = [
  `${path.sep}web${path.sep}components${path.sep}inspector${path.sep}datastar-inspector.ts`,
  `${path.sep}web${path.sep}vendor${path.sep}`,
];
const runtimeTokenNames = new Set([
  "--ld-cell-bar-color",
  "--ld-cell-bar-width",
  "--ld-cell-bg-color",
  "--ld-cell-bg-fade",
  "--ld-group-head-height",
  "--ld-head-top",
  "--ld-pin-left",
  "--ld-resize-guide-x",
  "--ld-row-height",
  "--ld-table-columns",
  "--ld-table-width",
  "--report-canvas-height",
  "--report-canvas-scale",
  "--report-canvas-width",
]);
const checkedTokenPattern =
  /^--(?:ld|base|motion|control|controlStack|border|borderColor|zIndex|shadow|fgColor|bgColor|data|button|overlay|text|fontStack|stack|selection|card|dashboard|report|color|spacing|container|radius|duration|ease|breakpoint|outline|focus)-/;

async function listFiles(root: string, relativeDir: string): Promise<string[]> {
  const absoluteDir = path.join(root, relativeDir);
  const entries = await readdir(absoluteDir, {withFileTypes: true});
  const files = await Promise.all(
    entries.map(async entry => {
      const relativePath = path.join(relativeDir, entry.name);
      const absolutePath = path.join(root, relativePath);

      if (entry.isDirectory()) {
        return listFiles(root, relativePath);
      }

      if (!entry.isFile() || !cssSourceExtensions.has(path.extname(entry.name))) {
        return [];
      }

      if (entry.name.endsWith(".test.ts") || entry.name.endsWith(".dom.test.ts")) {
        return [];
      }

      if (excludedPathParts.some(excluded => absolutePath.includes(excluded))) {
        return [];
      }

      return [relativePath];
    }),
  );

  return files.flat();
}

async function defaultSourceFiles(root: string): Promise<string[]> {
  const files = await Promise.all(productSourceRoots.map(sourceRoot => listFiles(root, sourceRoot)));

  return [...staticSourceFiles, ...files.flat()].sort((left, right) => left.localeCompare(right));
}

function stripCssComments(css: string): string {
  return css.replace(/\/\*[\s\S]*?\*\//g, match => " ".repeat(match.length));
}

function cssBlocksForFile(file: string, content: string): string[] {
  if (file.endsWith(".css")) {
    return [content];
  }

  return Array.from(content.matchAll(/css`([\s\S]*?)`/g), match => match[1] ?? []);
}

function lineNumber(content: string, index: number): number {
  return content.slice(0, index).split("\n").length;
}

function tokenDefinitions(content: string): Set<string> {
  return new Set(Array.from(content.matchAll(/(--[A-Za-z0-9_-]+)\s*:/g), match => match[1]));
}

function tokenReferences(content: string): Set<string> {
  return new Set(Array.from(content.matchAll(/var\(\s*(--[A-Za-z0-9_-]+)/g), match => match[1]));
}

async function referenceTokenDefinitions(referenceRoot: string): Promise<Set<string>> {
  const entries = await readdir(referenceRoot, {withFileTypes: true});
  const definitions = new Set<string>();

  for (const entry of entries) {
    if (!entry.isFile() || !entry.name.endsWith(".css")) continue;
    const content = await readFile(path.join(referenceRoot, entry.name), "utf8");
    for (const token of tokenDefinitions(content)) {
      definitions.add(token);
    }
  }

  return definitions;
}

function addViolation(
  violations: PrimerAlignmentViolation[],
  file: string,
  css: string,
  index: number,
  kind: PrimerAlignmentViolation["kind"],
  message: string,
): void {
  violations.push({
    file,
    line: lineNumber(css, index),
    kind,
    message,
  });
}

function scanCssForValueViolations(file: string, css: string, violations: PrimerAlignmentViolation[]): void {
  const uncommented = stripCssComments(css);

  for (const match of uncommented.matchAll(/#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(/g)) {
    addViolation(violations, file, uncommented, match.index ?? 0, "raw-color", "Use a Primer or LibreDash semantic token instead of a raw color.");
  }

  for (const match of uncommented.matchAll(/var\(\s*(--[A-Za-z0-9_-]+)\s*,\s*(#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?)\(|[0-9.]+(?:px|rem|em|ms|s)\b|white\b|black\b|transparent\b)/g)) {
    const tokenName = match[1];
    if (runtimeTokenNames.has(tokenName)) continue;
    addViolation(violations, file, uncommented, match.index ?? 0, "raw-var-fallback", `Use a central token fallback for ${tokenName}, not a raw design value.`);
  }
}

function scanCssForTokenViolations(
  file: string,
  css: string,
  allDefinitions: Set<string>,
  primerDefinitions: Set<string>,
  violations: PrimerAlignmentViolation[],
): void {
  const uncommented = stripCssComments(css);

  for (const match of uncommented.matchAll(/(--base-size-[A-Za-z0-9_-]+)\s*:/g)) {
    const tokenName = match[1];
    if (primerDefinitions.has(tokenName)) continue;
    addViolation(violations, file, uncommented, match.index ?? 0, "local-primer-token", `${tokenName} extends the Primer base-size namespace locally; use an --ld-* alias.`);
  }

  for (const tokenName of tokenReferences(uncommented)) {
    if (!checkedTokenPattern.test(tokenName) || runtimeTokenNames.has(tokenName)) continue;
    if (allDefinitions.has(tokenName)) continue;
    const index = uncommented.indexOf(tokenName);
    addViolation(violations, file, uncommented, index, "undefined-token", `${tokenName} is referenced but is not defined by Primer or the LibreDash token layer.`);
  }
}

export async function checkPrimerAlignment(options: CheckPrimerAlignmentOptions = {}): Promise<PrimerAlignmentViolation[]> {
  const root = options.root ?? process.cwd();
  const referenceRoot = options.referenceRoot ?? path.join(root, "docs", "reference", "primer-primitives-css");
  const sourceFiles = options.sourceFiles ?? (await defaultSourceFiles(root));
  const primerDefinitions = await referenceTokenDefinitions(referenceRoot);
  const cssByFile = new Map<string, string[]>();
  const allDefinitions = new Set<string>([...primerDefinitions, ...runtimeTokenNames]);

  for (const file of sourceFiles) {
    const content = await readFile(path.join(root, file), "utf8");
    const blocks = cssBlocksForFile(file, content);
    cssByFile.set(file, blocks);
    for (const block of blocks) {
      for (const tokenName of tokenDefinitions(stripCssComments(block))) {
        allDefinitions.add(tokenName);
      }
    }
  }

  const violations: PrimerAlignmentViolation[] = [];

  for (const [file, blocks] of cssByFile) {
    for (const block of blocks) {
      scanCssForValueViolations(file, block, violations);
      scanCssForTokenViolations(file, block, allDefinitions, primerDefinitions, violations);
    }
  }

  return violations.sort((left, right) => left.file.localeCompare(right.file) || left.line - right.line || left.kind.localeCompare(right.kind));
}

if (import.meta.main) {
  const violations = await checkPrimerAlignment();

  if (violations.length > 0) {
    for (const violation of violations) {
      console.error(`${violation.file}:${violation.line}: ${violation.kind}: ${violation.message}`);
    }
    process.exit(1);
  }
}
