import { describe, expect, test } from "bun:test";
import { chmod, mkdtemp, readFile, rm, stat, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";

import { DesktopDiscoveryError } from "./discovery.js";
import { ProfileStore } from "./profiles.js";

const discovery = {
  schemaVersion: 1,
  canonicalOrigin: "https://analytics.company.com",
  instanceId: "instance_0123456789abcdef0123456789abcdef",
  displayName: "Company Analytics",
  serverVersion: "v1.4.2",
  desktopProtocolMin: 1,
  desktopProtocolMax: 1,
  authenticationModes: ["browser-session", "system-browser-pkce"],
  capabilities: ["remote-web"],
};

describe("ProfileStore", () => {
  test("persists only non-secret connection metadata in a private file", async () => {
    const directoryPath = await mkdtemp(join(tmpdir(), "leapview-profiles-"));
    try {
      const path = join(directoryPath, "profiles.json");
      const store = new ProfileStore(path);
      const profile = await store.upsertFromDiscovery(discovery);

      expect(profile.id).toMatch(/^profile_[0-9a-f]{32}$/);
      expect(await new ProfileStore(path).list()).toEqual([profile]);
      expect((await stat(path)).mode & 0o777).toBe(0o600);
      const persisted = await readFile(path, "utf8");
      expect(persisted).not.toContain("cookie");
      expect(persisted).not.toContain("token");
      expect(persisted).not.toContain("password");
    } finally {
      await rm(directoryPath, { force: true, recursive: true });
    }
  });

  test("updates an existing profile by immutable instance id", async () => {
    const directoryPath = await mkdtemp(join(tmpdir(), "leapview-profiles-"));
    try {
      const store = new ProfileStore(join(directoryPath, "profiles.json"));
      const first = await store.upsertFromDiscovery(discovery);
      const updated = await store.upsertFromDiscovery({
        ...discovery,
        displayName: "Renamed Analytics",
      });
      expect(updated.id).toBe(first.id);
      expect(await store.list()).toHaveLength(1);
      expect(updated.displayName).toBe("Renamed Analytics");
    } finally {
      await rm(directoryPath, { force: true, recursive: true });
    }
  });

  test("removes only the selected profile and cannot reopen a stale mapping", async () => {
    const directoryPath = await mkdtemp(join(tmpdir(), "leapview-profiles-"));
    try {
      const store = new ProfileStore(join(directoryPath, "profiles.json"));
      const first = await store.upsertFromDiscovery(discovery);
      const second = await store.upsertFromDiscovery({
        ...discovery,
        canonicalOrigin: "https://finance.company.com",
        instanceId: "instance_abcdef0123456789abcdef0123456789",
        displayName: "Finance",
      });
      await store.remove(first.id);
      expect(await store.list()).toEqual([second]);
      await expect(store.remove(first.id)).rejects.toThrow("not found");
    } finally {
      await rm(directoryPath, { force: true, recursive: true });
    }
  });

  test("detects origin replacement and instance migration instead of silently trusting them", async () => {
    const directoryPath = await mkdtemp(join(tmpdir(), "leapview-profiles-"));
    try {
      const store = new ProfileStore(join(directoryPath, "profiles.json"));
      await store.upsertFromDiscovery(discovery);
      try {
        await store.upsertFromDiscovery({
          ...discovery,
          instanceId: "instance_abcdef0123456789abcdef0123456789",
        });
        throw new Error("expected instance identity mismatch");
      } catch (error) {
        expect(error).toBeInstanceOf(DesktopDiscoveryError);
        expect((error as DesktopDiscoveryError).kind).toBe(
          "instance_identity_mismatch",
        );
      }
      try {
        await store.upsertFromDiscovery({
          ...discovery,
          canonicalOrigin: "https://new.company.com",
        });
        throw new Error("expected canonical origin mismatch");
      } catch (error) {
        expect(error).toBeInstanceOf(DesktopDiscoveryError);
        expect((error as DesktopDiscoveryError).kind).toBe(
          "canonical_origin_mismatch",
        );
      }
    } finally {
      await rm(directoryPath, { force: true, recursive: true });
    }
  });

  test("fails closed on corrupt or overly permissive storage", async () => {
    const directoryPath = await mkdtemp(join(tmpdir(), "leapview-profiles-"));
    try {
      const path = join(directoryPath, "profiles.json");
      await writeFile(path, `{"schemaVersion":1,"profiles":[{"id":"bad"}]}`, {
        mode: 0o600,
      });
      await expect(new ProfileStore(path).list()).rejects.toThrow("profile");
      await writeFile(path, `{"schemaVersion":1,"profiles":[]}`);
      await chmod(path, 0o644);
      if (process.platform !== "win32") {
        await expect(new ProfileStore(path).list()).rejects.toThrow("permissions");
      }
    } finally {
      await rm(directoryPath, { force: true, recursive: true });
    }
  });
});
