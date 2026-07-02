import {describe, expect, test} from "bun:test";
import {mkdir, mkdtemp, readFile, readdir, rm, writeFile} from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import {generatePrimerPrimitivesReference} from "./primer_primitives_reference";

describe("generatePrimerPrimitivesReference", () => {
  test("writes grouped CSS files by concatenating source CSS", async () => {
    const workspace = await mkdtemp(path.join(os.tmpdir(), "primer-primitives-reference-"));
    const sourceRoot = path.join(workspace, "source");
    const outputRoot = path.join(workspace, "output");

    try {
      await mkdir(path.join(sourceRoot, "base", "motion"), {recursive: true});
      await mkdir(path.join(sourceRoot, "base", "size"), {recursive: true});
      await mkdir(path.join(sourceRoot, "functional", "spacing"), {recursive: true});
      await mkdir(path.join(sourceRoot, "functional", "themes"), {recursive: true});
      await writeFile(path.join(sourceRoot, "base", "size", "size.css"), ":root { --base-size-4: 0.25rem; }\n");
      await writeFile(path.join(sourceRoot, "base", "motion", "motion.css"), ":root { --base-duration-100: 80ms; }\n");
      await writeFile(path.join(sourceRoot, "functional", "spacing", "space.css"), ":root { --stack-gap-normal: 1rem; }\n");
      await writeFile(path.join(sourceRoot, "functional", "themes", "dark.css"), ":root { --bgColor-default: #000; }\n");
      await writeFile(path.join(sourceRoot, "functional", "themes", "light.css"), ":root { --bgColor-default: #fff; }\n");
      await writeFile(
        path.join(sourceRoot, "functional", "themes", "light-colorblind.css"),
        ":root { --bgColor-default: #eee; }\n",
      );
      await writeFile(path.join(sourceRoot, "primitives.css"), "@import './base/size/size.css';\n");

      await generatePrimerPrimitivesReference({sourceRoot, outputRoot});

      const entries = await readdir(outputRoot);
      expect(entries.sort()).toEqual(["motion.css", "size.css", "theme-dark.css", "theme-light.css"]);
      expect(await readFile(path.join(outputRoot, "size.css"), "utf8")).toBe(
        [
          "/* Size */",
          "",
          "/* Source: @primer/primitives/dist/css/base/size/size.css */",
          "",
          ":root { --base-size-4: 0.25rem; }",
          "",
          "/* Source: @primer/primitives/dist/css/functional/spacing/space.css */",
          "",
          ":root { --stack-gap-normal: 1rem; }",
          "",
        ].join("\n"),
      );
      expect(await readFile(path.join(outputRoot, "theme-light.css"), "utf8")).toBe(
        [
          "/* Light Theme */",
          "",
          "/* Source: @primer/primitives/dist/css/functional/themes/light.css */",
          "",
          ":root { --bgColor-default: #fff; }",
          "",
        ].join("\n"),
      );
    } finally {
      await rm(workspace, {recursive: true, force: true});
    }
  });
});
