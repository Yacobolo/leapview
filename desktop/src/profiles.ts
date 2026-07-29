import { randomBytes } from "node:crypto";
import {
  mkdir,
  open,
  readFile,
  rename,
  stat,
  unlink,
} from "node:fs/promises";
import { dirname } from "node:path";

import {
  DesktopDiscoveryError,
  type DiscoveryDocument,
} from "./discovery.js";

const PROFILE_SCHEMA_VERSION = 1;
const profileIDPattern = /^profile_[0-9a-f]{32}$/;
const instanceIDPattern = /^instance_[0-9a-f]{32}$/;

export interface Profile {
  id: string;
  canonicalOrigin: string;
  instanceId: string;
  displayName: string;
  lastSafePath: string;
}

interface ProfileDocument {
  schemaVersion: number;
  profiles: Profile[];
}

export class ProfileStore {
  readonly #path: string;
  #mutation = Promise.resolve();

  constructor(path: string) {
    this.#path = path;
  }

  async list(): Promise<Profile[]> {
    return structuredClone((await this.#read()).profiles);
  }

  async upsertFromDiscovery(
    discovery: DiscoveryDocument,
  ): Promise<Profile> {
    return this.#serialize(async () => {
      const document = await this.#read();
      const originProfile = document.profiles.find(
        (profile) => profile.canonicalOrigin === discovery.canonicalOrigin,
      );
      if (
        originProfile !== undefined &&
        originProfile.instanceId !== discovery.instanceId
      ) {
        throw new DesktopDiscoveryError(
          "instance_identity_mismatch",
          "the saved origin now reports a different LeapView instance identity",
        );
      }
      const instanceProfile = document.profiles.find(
        (profile) => profile.instanceId === discovery.instanceId,
      );
      if (
        instanceProfile !== undefined &&
        instanceProfile.canonicalOrigin !== discovery.canonicalOrigin
      ) {
        throw new DesktopDiscoveryError(
          "canonical_origin_mismatch",
          "the saved LeapView instance now reports a different canonical origin",
        );
      }
      const profile = originProfile ?? instanceProfile ?? {
        id: `profile_${randomBytes(16).toString("hex")}`,
        canonicalOrigin: discovery.canonicalOrigin,
        instanceId: discovery.instanceId,
        displayName: discovery.displayName,
        lastSafePath: "/",
      };
      profile.displayName = discovery.displayName;
      const index = document.profiles.findIndex(
        (candidate) => candidate.id === profile.id,
      );
      if (index === -1) {
        document.profiles.push(profile);
      } else {
        document.profiles[index] = profile;
      }
      validateProfileDocument(document);
      await this.#write(document);
      return structuredClone(profile);
    });
  }

  async remove(profileID: string): Promise<void> {
    if (!profileIDPattern.test(profileID)) {
      throw new Error("desktop profile id is invalid");
    }
    await this.#serialize(async () => {
      const document = await this.#read();
      const index = document.profiles.findIndex(
        (profile) => profile.id === profileID,
      );
      if (index === -1) {
        throw new Error("desktop profile was not found");
      }
      document.profiles.splice(index, 1);
      validateProfileDocument(document);
      await this.#write(document);
    });
  }

  async #read(): Promise<ProfileDocument> {
    try {
      const information = await stat(this.#path);
      if (
        process.platform !== "win32" &&
        (information.mode & 0o077) !== 0
      ) {
        throw new Error("desktop profile file permissions are not private");
      }
      const body = await readFile(this.#path, "utf8");
      return validateProfileDocument(JSON.parse(body) as unknown);
    } catch (error) {
      if (
        typeof error === "object" &&
        error !== null &&
        "code" in error &&
        error.code === "ENOENT"
      ) {
        return { schemaVersion: PROFILE_SCHEMA_VERSION, profiles: [] };
      }
      if (error instanceof SyntaxError) {
        throw new Error("desktop profile file contains invalid JSON", {
          cause: error,
        });
      }
      throw error;
    }
  }

  async #write(document: ProfileDocument): Promise<void> {
    await mkdir(dirname(this.#path), { mode: 0o700, recursive: true });
    const temporaryPath = `${this.#path}.${process.pid}.${randomBytes(8).toString("hex")}.tmp`;
    const handle = await open(temporaryPath, "wx", 0o600);
    try {
      await handle.writeFile(`${JSON.stringify(document, null, 2)}\n`, "utf8");
      await handle.sync();
    } finally {
      await handle.close();
    }
    try {
      await rename(temporaryPath, this.#path);
    } catch (error) {
      await unlink(temporaryPath).catch(() => undefined);
      throw error;
    }
  }

  async #serialize<T>(operation: () => Promise<T>): Promise<T> {
    const prior = this.#mutation;
    let release: () => void = () => undefined;
    this.#mutation = new Promise<void>((resolve) => {
      release = resolve;
    });
    await prior;
    try {
      return await operation();
    } finally {
      release();
    }
  }
}

function validateProfileDocument(input: unknown): ProfileDocument {
  if (typeof input !== "object" || input === null || Array.isArray(input)) {
    throw new Error("desktop profile document must be an object");
  }
  const document = input as Record<string, unknown>;
  if (document.schemaVersion !== PROFILE_SCHEMA_VERSION) {
    throw new Error("desktop profile schema version is unsupported");
  }
  if (!Array.isArray(document.profiles) || document.profiles.length > 100) {
    throw new Error("desktop profile list is invalid");
  }
  const seenIDs = new Set<string>();
  const seenOrigins = new Set<string>();
  const seenInstances = new Set<string>();
  const profiles = document.profiles.map((value) => {
    const profile = validateProfile(value);
    if (
      seenIDs.has(profile.id) ||
      seenOrigins.has(profile.canonicalOrigin) ||
      seenInstances.has(profile.instanceId)
    ) {
      throw new Error("desktop profile identities must be unique");
    }
    seenIDs.add(profile.id);
    seenOrigins.add(profile.canonicalOrigin);
    seenInstances.add(profile.instanceId);
    return profile;
  });
  return { schemaVersion: PROFILE_SCHEMA_VERSION, profiles };
}

function validateProfile(input: unknown): Profile {
  if (typeof input !== "object" || input === null || Array.isArray(input)) {
    throw new Error("desktop profile must be an object");
  }
  const profile = input as Record<string, unknown>;
  const id = requireProfileString(profile.id, "profile id", 64);
  if (!profileIDPattern.test(id)) {
    throw new Error("desktop profile id is invalid");
  }
  const canonicalOrigin = requireProfileString(
    profile.canonicalOrigin,
    "profile canonical origin",
    2_048,
  );
  const parsed = new URL(canonicalOrigin);
  if (
    parsed.origin !== canonicalOrigin ||
    !["https:", "http:"].includes(parsed.protocol)
  ) {
    throw new Error("desktop profile canonical origin is invalid");
  }
  const instanceId = requireProfileString(
    profile.instanceId,
    "profile instance id",
    64,
  );
  if (!instanceIDPattern.test(instanceId)) {
    throw new Error("desktop profile instance id is invalid");
  }
  const displayName = requireProfileString(
    profile.displayName,
    "profile display name",
    120,
  );
  const lastSafePath = requireProfileString(
    profile.lastSafePath,
    "profile safe path",
    2_048,
  );
  if (!lastSafePath.startsWith("/") || lastSafePath.startsWith("//")) {
    throw new Error("desktop profile safe path is invalid");
  }
  return { id, canonicalOrigin, instanceId, displayName, lastSafePath };
}

function requireProfileString(
  input: unknown,
  name: string,
  maximumBytes: number,
): string {
  if (
    typeof input !== "string" ||
    input.length === 0 ||
    input.trim() !== input ||
    new TextEncoder().encode(input).byteLength > maximumBytes ||
    /[\u0000-\u001f\u007f]/u.test(input)
  ) {
    throw new Error(`${name} is invalid`);
  }
  return input;
}
