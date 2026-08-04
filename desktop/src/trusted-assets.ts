import { readFile } from "node:fs/promises";

import type { TrustedUIAssets } from "./trusted-ui.js";

const FONT_FILES = [
  "inter-cyrillic-ext-wght-normal.woff2",
  "inter-cyrillic-wght-normal.woff2",
  "inter-greek-ext-wght-normal.woff2",
  "inter-greek-wght-normal.woff2",
  "inter-latin-ext-wght-normal.woff2",
  "inter-latin-wght-normal.woff2",
  "inter-vietnamese-wght-normal.woff2",
] as const;

export async function loadTrustedUIAssets(): Promise<TrustedUIAssets> {
  const stylesheet = await readFile(
    new URL("../app.css", import.meta.url),
    "utf8",
  );
  const fonts = new Map<string, ArrayBuffer>();
  await Promise.all(
    FONT_FILES.map(async (name) => {
      const file = await readFile(new URL(`../files/${name}`, import.meta.url));
      fonts.set(`/files/${name}`, new Uint8Array(file).buffer);
    }),
  );
  return { stylesheet, fonts };
}
