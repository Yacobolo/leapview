import {describe, expect, test} from "bun:test";
import {mkdir, mkdtemp, rm, writeFile} from "node:fs/promises";
import os from "node:os";
import path from "node:path";

import {checkPrimerAlignment} from "./check_primer_alignment";

async function withWorkspace(run: (workspace: string) => Promise<void>): Promise<void> {
  const workspace = await mkdtemp(path.join(os.tmpdir(), "primer-alignment-"));

  try {
    await mkdir(path.join(workspace, "docs", "reference", "primer-primitives-css"), {recursive: true});
    await mkdir(path.join(workspace, "static"), {recursive: true});
    await mkdir(path.join(workspace, "web", "components", "example"), {recursive: true});
    await writeFile(
      path.join(workspace, "docs", "reference", "primer-primitives-css", "size.css"),
      ":root { --base-size-4: 0.25rem; --base-size-8: 0.5rem; --motion-duration-short: 200ms; }\n",
    );
    await writeFile(
      path.join(workspace, "docs", "reference", "primer-primitives-css", "theme-light.css"),
      ":root { --fgColor-accent: #0969da; --bgColor-default: #fff; }\n",
    );
    await run(workspace);
  } finally {
    await rm(workspace, {recursive: true, force: true});
  }
}

describe("checkPrimerAlignment", () => {
  test("accepts Primer-backed LibreDash aliases", async () => {
    await withWorkspace(async workspace => {
      await writeFile(
        path.join(workspace, "static", "app.input.css"),
        ":root { --ld-accent: var(--fgColor-accent); --ld-space-control: calc(var(--base-size-8) + var(--base-size-4)); }\n",
      );
      await writeFile(
        path.join(workspace, "web", "components", "example", "good.ts"),
        "import {css} from 'lit';\nexport const styles = css`:host { color: var(--ld-accent); padding: var(--ld-space-control); }`;\n",
      );

      await expect(checkPrimerAlignment({root: workspace})).resolves.toEqual([]);
    });
  });

  test("rejects raw color values and raw design fallbacks in product CSS", async () => {
    await withWorkspace(async workspace => {
      await writeFile(path.join(workspace, "static", "app.input.css"), ":root { --ld-accent: var(--fgColor-accent); }\n");
      await writeFile(
        path.join(workspace, "web", "components", "example", "bad.ts"),
        "import {css} from 'lit';\nexport const styles = css`.button { color: #0969da; background: var(--ld-accent, #0969da); }`;\n",
      );

      const violations = await checkPrimerAlignment({root: workspace});
      expect(violations.map(violation => violation.kind)).toEqual(["raw-color", "raw-color", "raw-var-fallback"]);
    });
  });

  test("rejects undefined design tokens and local Primer namespace extensions", async () => {
    await withWorkspace(async workspace => {
      await writeFile(
        path.join(workspace, "static", "app.input.css"),
        ":root { --base-size-10: 0.625rem; --ld-accent: var(--fgColor-accent); }\n",
      );
      await writeFile(
        path.join(workspace, "web", "components", "example", "bad.ts"),
        "import {css} from 'lit';\nexport const styles = css`:host { transition-duration: var(--motion-duration-fast); color: var(--ld-missing); }`;\n",
      );

      const violations = await checkPrimerAlignment({root: workspace});
      expect(violations.map(violation => violation.kind)).toEqual([
        "local-primer-token",
        "undefined-token",
        "undefined-token",
      ]);
    });
  });

  test("excludes the Datastar inspector from product alignment checks", async () => {
    await withWorkspace(async workspace => {
      await mkdir(path.join(workspace, "web", "components", "inspector"), {recursive: true});
      await writeFile(path.join(workspace, "static", "app.input.css"), ":root { --ld-accent: var(--fgColor-accent); }\n");
      await writeFile(
        path.join(workspace, "web", "components", "inspector", "datastar-inspector.ts"),
        "import {css} from 'lit';\nexport const styles = css`:host { color: #fff; }`;\n",
      );

      await expect(checkPrimerAlignment({root: workspace})).resolves.toEqual([]);
    });
  });
});
