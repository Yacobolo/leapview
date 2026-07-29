import { randomBytes } from "node:crypto";
import {
  mkdir,
  open,
  readFile,
  rename as renameFile,
  stat,
  unlink,
} from "node:fs/promises";
import { dirname } from "node:path";

import {
  DesktopDiscoveryError,
  type DiscoveryDocument,
} from "./discovery.js";

const PROFILE_SCHEMA_VERSION = 2;
const LEGACY_PROFILE_SCHEMA_VERSION = 1;
const PROFILE_PARTITION_VERSION = 1;
const profileIDPattern = /^profile_[0-9a-f]{32}$/;
const instanceIDPattern = /^instance_[0-9a-f]{32}$/;

export interface Profile {
  id: string;
  canonicalOrigin: string;
  instanceId: string;
  displayName: string;
  lastSafePath: string;
  partitionVersion: 1;
  label?: string;
}

interface ProfileDocument {
  schemaVersion: 2;
  profiles: Profile[];
}

interface ProfileReadResult {
  document: ProfileDocument;
  migrated: boolean;
}

export class DesktopProfileReplacementCancelledError extends Error {
  constructor() {
    super("Saved instance replacement was canceled.");
    this.name = "DesktopProfileReplacementCancelledError";
  }
}

export function profilePartitionName(profile: Profile): string {
  if (
    !profileIDPattern.test(profile.id) ||
    profile.partitionVersion !== PROFILE_PARTITION_VERSION
  ) {
    throw new Error("desktop profile partition identity is invalid");
  }
  return `persist:leapview-profile-${profile.id.slice("profile_".length)}`;
}

export function profileDisplayName(profile: Profile): string {
  return profile.label ?? profile.displayName;
}

export class ProfileStore {
  readonly #path: string;
  #mutation = Promise.resolve();

  constructor(path: string) {
    this.#path = path;
  }

  async list(): Promise<Profile[]> {
    return this.#serialize(async () => {
      const result = await this.#read();
      if (result.migrated) {
        await this.#write(result.document);
      }
      return structuredClone(result.document.profiles);
    });
  }

  async upsertFromDiscovery(
    discovery: DiscoveryDocument,
  ): Promise<Profile> {
    return this.#serialize(async () => {
      const { document } = await this.#read();
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
        partitionVersion: PROFILE_PARTITION_VERSION,
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

  async setLabel(profileID: string, label: string | null): Promise<Profile> {
    if (!profileIDPattern.test(profileID)) {
      throw new Error("desktop profile id is invalid");
    }
    const normalizedLabel = label === null
      ? undefined
      : requireProfileString(label, "profile label", 120);
    return this.#serialize(async () => {
      const { document } = await this.#read();
      const profile = document.profiles.find(
        (candidate) => candidate.id === profileID,
      );
      if (profile === undefined) {
        throw new Error("desktop profile was not found");
      }
      if (normalizedLabel === undefined) {
        delete profile.label;
      } else {
        profile.label = normalizedLabel;
      }
      validateProfileDocument(document);
      await this.#write(document);
      return structuredClone(profile);
    });
  }

  async replaceFromDiscovery(
    profileID: string,
    discovery: DiscoveryDocument,
  ): Promise<Profile> {
    if (!profileIDPattern.test(profileID)) {
      throw new Error("desktop profile id is invalid");
    }
    return this.#serialize(async () => {
      const { document } = await this.#read();
      const index = document.profiles.findIndex(
        (candidate) => candidate.id === profileID,
      );
      if (index === -1) {
        throw new Error("desktop profile was not found");
      }
      const current = document.profiles[index]!;
      const sameOrigin =
        current.canonicalOrigin === discovery.canonicalOrigin;
      const sameInstance = current.instanceId === discovery.instanceId;
      if (!sameOrigin && !sameInstance) {
        throw new Error(
          "replacement discovery is not related to the saved profile",
        );
      }
      if (sameOrigin && sameInstance) {
        throw new Error("replacement does not change profile identity");
      }
      const replacement: Profile = {
        id: `profile_${randomBytes(16).toString("hex")}`,
        canonicalOrigin: discovery.canonicalOrigin,
        instanceId: discovery.instanceId,
        displayName: discovery.displayName,
        lastSafePath: "/",
        partitionVersion: PROFILE_PARTITION_VERSION,
        ...(current.label === undefined ? {} : { label: current.label }),
      };
      document.profiles[index] = replacement;
      validateProfileDocument(document);
      await this.#write(document);
      return structuredClone(replacement);
    });
  }

  async remove(profileID: string): Promise<void> {
    if (!profileIDPattern.test(profileID)) {
      throw new Error("desktop profile id is invalid");
    }
    await this.#serialize(async () => {
      const { document } = await this.#read();
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

  async #read(): Promise<ProfileReadResult> {
    try {
      const information = await stat(this.#path);
      if (
        process.platform !== "win32" &&
        (information.mode & 0o077) !== 0
      ) {
        throw new Error("desktop profile file permissions are not private");
      }
      const body = await readFile(this.#path, "utf8");
      const parsed = JSON.parse(body) as unknown;
      if (
        typeof parsed === "object" &&
        parsed !== null &&
        !Array.isArray(parsed) &&
        (parsed as Record<string, unknown>).schemaVersion ===
          LEGACY_PROFILE_SCHEMA_VERSION
      ) {
        return {
          document: migrateLegacyProfileDocument(parsed),
          migrated: true,
        };
      }
      return {
        document: validateProfileDocument(parsed),
        migrated: false,
      };
    } catch (error) {
      if (
        typeof error === "object" &&
        error !== null &&
        "code" in error &&
        error.code === "ENOENT"
      ) {
        return {
          document: {
            schemaVersion: PROFILE_SCHEMA_VERSION,
            profiles: [],
          },
          migrated: false,
        };
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
      await renameFile(temporaryPath, this.#path);
    } catch (error) {
      await unlink(temporaryPath).catch(() => undefined);
      throw error;
    }
    if (process.platform !== "win32") {
      const directory = await open(dirname(this.#path), "r");
      try {
        await directory.sync();
      } finally {
        await directory.close();
      }
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

function migrateLegacyProfileDocument(input: unknown): ProfileDocument {
  if (typeof input !== "object" || input === null || Array.isArray(input)) {
    throw new Error("desktop profile document must be an object");
  }
  const document = input as Record<string, unknown>;
  requireExactKeys(
    document,
    ["schemaVersion", "profiles"],
    "desktop profile document",
  );
  if (
    document.schemaVersion !== LEGACY_PROFILE_SCHEMA_VERSION ||
    !Array.isArray(document.profiles) ||
    document.profiles.length > 100
  ) {
    throw new Error("desktop profile schema version is unsupported");
  }
  return validateProfileDocument({
    schemaVersion: PROFILE_SCHEMA_VERSION,
    profiles: document.profiles.map((profile) => {
      if (
        typeof profile !== "object" ||
        profile === null ||
        Array.isArray(profile)
      ) {
        throw new Error("desktop profile must be an object");
      }
      requireExactKeys(
        profile as Record<string, unknown>,
        [
          "id",
          "canonicalOrigin",
          "instanceId",
          "displayName",
          "lastSafePath",
        ],
        "legacy desktop profile",
      );
      return {
        ...profile,
        partitionVersion: PROFILE_PARTITION_VERSION,
      };
    }),
  });
}

function validateProfileDocument(input: unknown): ProfileDocument {
  if (typeof input !== "object" || input === null || Array.isArray(input)) {
    throw new Error("desktop profile document must be an object");
  }
  const document = input as Record<string, unknown>;
  requireExactKeys(
    document,
    ["schemaVersion", "profiles"],
    "desktop profile document",
  );
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
  requireExactKeys(
    profile,
    [
      "id",
      "canonicalOrigin",
      "instanceId",
      "displayName",
      "lastSafePath",
      "partitionVersion",
      "label",
    ],
    "desktop profile",
  );
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
  if (profile.partitionVersion !== PROFILE_PARTITION_VERSION) {
    throw new Error("desktop profile partition version is unsupported");
  }
  const label = profile.label === undefined
    ? undefined
    : requireProfileString(profile.label, "profile label", 120);
  return {
    id,
    canonicalOrigin,
    instanceId,
    displayName,
    lastSafePath,
    partitionVersion: PROFILE_PARTITION_VERSION,
    ...(label === undefined ? {} : { label }),
  };
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

function requireExactKeys(
  input: Record<string, unknown>,
  allowedKeys: readonly string[],
  name: string,
): void {
  const allowed = new Set(allowedKeys);
  if (Object.keys(input).some((key) => !allowed.has(key))) {
    throw new Error(`${name} contains unknown fields`);
  }
}
