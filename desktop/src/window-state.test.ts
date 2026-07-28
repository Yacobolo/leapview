import { describe, expect, test } from "bun:test";
import { mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

import {
  WindowStateStore,
  fitWindowStateToWorkArea,
  type PersistedWindowState,
} from "./window-state.js";

const savedState: PersistedWindowState = {
  bounds: { x: 120, y: 80, width: 1440, height: 920 },
  maximized: false,
};

describe("WindowStateStore", () => {
  test("persists only validated geometry in a private atomic document", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-window-state-"));
    try {
      const path = join(directory, "window-state.json");
      const store = await WindowStateStore.open(path);
      store.record("shell", savedState);
      store.record("profile_0123456789abcdef0123456789abcdef", {
        bounds: { x: -800, y: 20, width: 1280, height: 800 },
        maximized: true,
      });
      await store.flush();

      expect((await WindowStateStore.open(path)).get("shell")).toEqual(
        savedState,
      );
      store.remove("profile_0123456789abcdef0123456789abcdef");
      await store.flush();
      expect(
        (await WindowStateStore.open(path)).get(
          "profile_0123456789abcdef0123456789abcdef",
        ),
      ).toBeUndefined();
      expect((await stat(path)).mode & 0o777).toBe(0o600);
      const body = await readFile(path, "utf8");
      expect(body).not.toContain("http");
      expect(body).not.toContain("title");
      expect(body).not.toContain("content");
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });

  test("ignores corrupt, oversized, and unsupported state instead of applying it", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-window-state-"));
    try {
      const path = join(directory, "window-state.json");
      const corruptInputs = [
        "{",
        JSON.stringify({
          schemaVersion: 2,
          windows: { shell: savedState },
        }),
        JSON.stringify({
          schemaVersion: 1,
          windows: {
            shell: {
              bounds: {
                x: 0,
                y: 0,
                width: Number.MAX_SAFE_INTEGER,
                height: 800,
              },
              maximized: false,
            },
          },
        }),
        " ".repeat(70_000),
      ];

      for (const input of corruptInputs) {
        await writeFile(path, input, { mode: 0o600 });
        const store = await WindowStateStore.open(path);
        expect(store.get("shell")).toBeUndefined();
      }
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });

  test("rejects arbitrary state keys and invalid runtime geometry", async () => {
    const directory = await mkdtemp(join(tmpdir(), "leapview-window-state-"));
    try {
      const store = await WindowStateStore.open(
        join(directory, "window-state.json"),
      );
      expect(() => store.record("https://analytics.company.com", savedState))
        .toThrow("window state key");
      expect(() =>
        store.record("shell", {
          bounds: { x: Number.NaN, y: 0, width: 800, height: 600 },
          maximized: false,
        }),
      ).toThrow("window bounds");
    } finally {
      await rm(directory, { force: true, recursive: true });
    }
  });
});

describe("fitWindowStateToWorkArea", () => {
  test("keeps valid visible geometry unchanged", () => {
    expect(
      fitWindowStateToWorkArea(
        savedState,
        { x: 0, y: 0, width: 1920, height: 1080 },
        { width: 800, height: 600 },
      ),
    ).toEqual(savedState);
  });

  test("clamps a restored window to the current monitor work area", () => {
    expect(
      fitWindowStateToWorkArea(
        {
          bounds: { x: 2600, y: -200, width: 1800, height: 1200 },
          maximized: true,
        },
        { x: 0, y: 24, width: 1512, height: 958 },
        { width: 800, height: 600 },
      ),
    ).toEqual({
      bounds: { x: 0, y: 24, width: 1512, height: 958 },
      maximized: true,
    });
  });

  test("uses the available work area when it is smaller than minimum size", () => {
    expect(
      fitWindowStateToWorkArea(
        {
          bounds: { x: -100, y: -100, width: 200, height: 100 },
          maximized: false,
        },
        { x: 10, y: 20, width: 640, height: 480 },
        { width: 800, height: 600 },
      ),
    ).toEqual({
      bounds: { x: 10, y: 20, width: 640, height: 480 },
      maximized: false,
    });
  });
});
