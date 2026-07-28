import { describe, expect, test } from "bun:test";

import { buildNativeMenuTemplate } from "./native-menu.js";

describe("buildNativeMenuTemplate", () => {
  test("provides native lifecycle, editing, view, and window actions", () => {
    const saveDiagnosticReport = () => undefined;
    const template = buildNativeMenuTemplate("darwin", "LeapView", {
      showInstances: () => undefined,
      saveDiagnosticReport,
    });
    const serialized = JSON.stringify(template);
    const help = template.find((item) => item.label === "Help");
    const diagnosticItem = Array.isArray(help?.submenu)
      ? help.submenu.find(
          (item) => item.label === "Save Diagnostic Report…",
        )
      : undefined;

    expect(serialized).toContain("Manage Instances");
    expect(serialized).toContain("CmdOrCtrl+Shift+L");
    expect(serialized).toContain("Save Diagnostic Report");
    expect(diagnosticItem?.click).toBe(saveDiagnosticReport);
    expect(serialized).toContain('"role":"about"');
    expect(serialized).toContain('"role":"quit"');
    expect(serialized).toContain('"role":"copy"');
    expect(serialized).toContain('"role":"paste"');
    expect(serialized).toContain('"role":"reload"');
    expect(serialized).toContain('"role":"resetZoom"');
    expect(serialized).toContain('"role":"togglefullscreen"');
    expect(serialized).toContain('"role":"minimize"');
  });

  test("never exposes development or destructive reload actions", () => {
    for (const platform of ["darwin", "linux", "win32"] as const) {
      const serialized = JSON.stringify(
        buildNativeMenuTemplate(platform, "LeapView", {
          showInstances: () => undefined,
          saveDiagnosticReport: () => undefined,
        }),
      );
      expect(serialized).not.toContain("toggleDevTools");
      expect(serialized).not.toContain("forceReload");
      expect(serialized).not.toContain("developer");
    }
  });

  test("uses platform-native close and quit placement", () => {
    const mac = JSON.stringify(
      buildNativeMenuTemplate("darwin", "LeapView", {
        showInstances: () => undefined,
        saveDiagnosticReport: () => undefined,
      }),
    );
    const linux = JSON.stringify(
      buildNativeMenuTemplate("linux", "LeapView", {
        showInstances: () => undefined,
        saveDiagnosticReport: () => undefined,
      }),
    );

    expect(mac).toContain('"role":"close"');
    expect(linux).not.toContain('"role":"close"');
    expect(linux).toContain('"role":"quit"');
  });
});
