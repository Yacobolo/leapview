import { randomBytes } from "node:crypto";
import {
  mkdir,
  open,
  readFile,
  rename,
  stat,
  unlink,
} from "node:fs/promises";
import { dirname, isAbsolute } from "node:path";
import { isDeepStrictEqual } from "node:util";

const DIAGNOSTIC_SCHEMA_VERSION = 1;
const MAXIMUM_DOCUMENT_BYTES = 128 * 1024;
const MAXIMUM_EVENTS = 256;
const RETENTION_MS = 7 * 24 * 60 * 60 * 1_000;
const MAXIMUM_FUTURE_SKEW_MS = 5 * 60 * 1_000;
const COALESCE_WINDOW_MS = 30 * 1_000;

const processGoneReasons = new Set([
  "clean-exit",
  "abnormal-exit",
  "killed",
  "crashed",
  "oom",
  "launch-failed",
  "integrity-failure",
  "memory-eviction",
]);

const processTypes = new Set([
  "utility",
  "zygote",
  "sandbox-helper",
  "gpu",
  "pepper-plugin",
  "pepper-plugin-broker",
  "unknown",
]);

export type ProcessGoneReason =
  | "clean-exit"
  | "abnormal-exit"
  | "killed"
  | "crashed"
  | "oom"
  | "launch-failed"
  | "integrity-failure"
  | "memory-eviction";

export type DiagnosticEvent =
  | { kind: "startup"; packaged: boolean }
  | {
      kind: "policy";
      mode: "open" | "managed" | "locked";
      userInstances: "allowed" | "restricted";
      diagnostics: "enabled" | "disabled";
    }
  | {
      kind: "discovery";
      outcome: "success" | "rejected" | "unavailable";
    }
  | {
      kind: "authentication";
      phase:
        | "session-valid"
        | "required"
        | "started"
        | "completed"
        | "failed"
        | "disconnected";
    }
  | {
      kind: "profile";
      action:
        | "added"
        | "opened"
        | "disconnected"
        | "removed"
        | "renamed"
        | "replaced";
      outcome: "success" | "failed";
    }
  | {
      kind: "navigation";
      action:
        | "allowed-same-origin-window"
        | "requested-external"
        | "blocked-main-frame"
        | "blocked-popup"
        | "blocked-webview"
        | "blocked-native-transport"
        | "allowed-csv-export"
        | "blocked-download";
    }
  | {
      kind: "stream";
      phase: "opened" | "closed" | "failed";
    }
  | {
      kind: "remote-lifecycle";
      state: "offline" | "crashed";
    }
  | {
      kind: "render-process-gone";
      surface: "trusted-shell" | "remote" | "unknown";
      reason: ProcessGoneReason;
    }
  | {
      kind: "child-process-gone";
      processType:
        | "utility"
        | "zygote"
        | "sandbox-helper"
        | "gpu"
        | "pepper-plugin"
        | "pepper-plugin-broker"
        | "unknown";
      reason: ProcessGoneReason;
    }
  | {
      kind: "update";
      phase:
        | "checking"
        | "available"
        | "not-available"
        | "downloaded"
        | "failed";
    };

export type StoredDiagnosticEvent = DiagnosticEvent & { at: string };
type EventOf<Kind extends DiagnosticEvent["kind"]> = Extract<
  DiagnosticEvent,
  { kind: Kind }
>;

export interface DiagnosticEnvironment {
  applicationVersion: string;
  electronVersion: string;
  chromiumVersion: string;
  nodeVersion: string;
  platform: NodeJS.Platform;
  osRelease: string;
  architecture: string;
  packaged: boolean;
  policyRevision: string;
}

export interface DiagnosticReport {
  format: "leapview-desktop-diagnostic-report";
  schemaVersion: 1;
  generatedAt: string;
  manifest: {
    files: Array<{
      name: "leapview-diagnostic-report.json";
      sections: ["environment", "privacy", "events"];
    }>;
    topLevelFields: [
      "format",
      "schemaVersion",
      "generatedAt",
      "manifest",
      "environment",
      "privacy",
      "events",
    ];
    environmentFields: Array<keyof DiagnosticEnvironment>;
    privacyFields: Array<keyof DiagnosticReport["privacy"]>;
    eventFields: Record<DiagnosticEvent["kind"], string[]>;
  };
  environment: DiagnosticEnvironment;
  privacy: {
    crashCollection: "disabled";
    crashUpload: "disabled";
    rendererConsole: "not-collected";
    minidumps: "excluded";
    instanceOrigins: "excluded";
    instanceMetadata: "excluded";
    credentials: "excluded";
    retentionDays: 7;
  };
  events: StoredDiagnosticEvent[];
}

interface DiagnosticDocument {
  schemaVersion: 1;
  events: StoredDiagnosticEvent[];
}

export interface DiagnosticJournalOptions {
  enabled?: boolean;
  now?: () => Date;
}

export class DiagnosticJournal {
  readonly #path: string;
  readonly #enabled: boolean;
  readonly #now: () => Date;
  readonly #events: StoredDiagnosticEvent[];
  readonly #lastFingerprintAt = new Map<string, number>();
  #dirty = false;
  #flushPromise: Promise<void> | null = null;

  private constructor(
    path: string,
    enabled: boolean,
    now: () => Date,
    events: StoredDiagnosticEvent[],
  ) {
    this.#path = path;
    this.#enabled = enabled;
    this.#now = now;
    this.#events = events;
    for (const event of events) {
      this.#lastFingerprintAt.set(
        eventFingerprint(event),
        Date.parse(event.at),
      );
    }
  }

  static async open(
    path: string,
    options: DiagnosticJournalOptions = {},
  ): Promise<DiagnosticJournal> {
    const enabled = options.enabled ?? true;
    const now = options.now ?? (() => new Date());
    if (!enabled) {
      return new DiagnosticJournal(path, false, now, []);
    }
    try {
      const currentTime = validCurrentTime(now);
      const information = await stat(path);
      if (
        process.platform !== "win32" &&
        (information.mode & 0o077) !== 0
      ) {
        throw new Error("desktop diagnostic file permissions are not private");
      }
      if (information.size > MAXIMUM_DOCUMENT_BYTES) {
        throw new Error("desktop diagnostic document is too large");
      }
      const body = await readFile(path, "utf8");
      const document = validateDocument(JSON.parse(body) as unknown);
      const events = document.events.filter((event) => {
        const timestamp = Date.parse(event.at);
        return (
          timestamp >= currentTime - RETENTION_MS &&
          timestamp <= currentTime + MAXIMUM_FUTURE_SKEW_MS
        );
      });
      return new DiagnosticJournal(path, true, now, events);
    } catch {
      return new DiagnosticJournal(path, true, now, []);
    }
  }

  record(input: DiagnosticEvent): void {
    if (!this.#enabled) {
      return;
    }
    const event = validateEvent(input);
    const currentTime = validCurrentTime(this.#now);
    const fingerprint = eventFingerprint(event);
    const lastRecordedAt = this.#lastFingerprintAt.get(fingerprint);
    if (
      lastRecordedAt !== undefined &&
      currentTime - lastRecordedAt < COALESCE_WINDOW_MS
    ) {
      return;
    }
    const stored = { ...event, at: new Date(currentTime).toISOString() };
    this.#events.push(stored);
    if (this.#events.length > MAXIMUM_EVENTS) {
      this.#events.splice(0, this.#events.length - MAXIMUM_EVENTS);
    }
    this.#lastFingerprintAt.set(fingerprint, currentTime);
    this.#dirty = true;
  }

  events(): StoredDiagnosticEvent[] {
    return structuredClone(this.#events);
  }

  report(environment: DiagnosticEnvironment): DiagnosticReport {
    return {
      format: "leapview-desktop-diagnostic-report",
      schemaVersion: DIAGNOSTIC_SCHEMA_VERSION,
      generatedAt: new Date(validCurrentTime(this.#now)).toISOString(),
      manifest: diagnosticManifest(),
      environment: validateEnvironment(environment),
      privacy: diagnosticPrivacy(),
      events: this.events(),
    };
  }

  flush(): Promise<void> {
    if (!this.#enabled || !this.#dirty) {
      return Promise.resolve();
    }
    if (this.#flushPromise === null) {
      this.#flushPromise = this.#flushLoop().finally(() => {
        this.#flushPromise = null;
      });
    }
    return this.#flushPromise;
  }

  async #flushLoop(): Promise<void> {
    while (this.#dirty) {
      this.#dirty = false;
      const document: DiagnosticDocument = {
        schemaVersion: DIAGNOSTIC_SCHEMA_VERSION,
        events: this.events(),
      };
      try {
        await writePrivateJSON(this.#path, document);
      } catch (error) {
        this.#dirty = true;
        throw error;
      }
    }
  }
}

export async function writeDiagnosticReport(
  path: string,
  report: DiagnosticReport,
): Promise<void> {
  validateReport(report);
  await writePrivateJSON(path, report);
}

export function normalizeProcessGoneReason(
  input: unknown,
): ProcessGoneReason {
  return typeof input === "string" && processGoneReasons.has(input)
    ? (input as ProcessGoneReason)
    : "abnormal-exit";
}

export function normalizeChildProcessType(
  input: unknown,
): EventOf<"child-process-gone">["processType"] {
  const normalized = String(input ?? "")
    .trim()
    .toLowerCase()
    .replaceAll(" ", "-");
  return processTypes.has(normalized)
    ? (normalized as EventOf<"child-process-gone">["processType"])
    : "unknown";
}

function validateDocument(input: unknown): DiagnosticDocument {
  const document = requireExactRecord(
    input,
    ["schemaVersion", "events"],
    "diagnostic document",
  );
  if (
    document.schemaVersion !== DIAGNOSTIC_SCHEMA_VERSION ||
    !Array.isArray(document.events) ||
    document.events.length > MAXIMUM_EVENTS
  ) {
    throw new Error("desktop diagnostic document is invalid");
  }
  return {
    schemaVersion: DIAGNOSTIC_SCHEMA_VERSION,
    events: document.events.map((event) => validateStoredEvent(event)),
  };
}

function validateStoredEvent(input: unknown): StoredDiagnosticEvent {
  if (typeof input !== "object" || input === null || Array.isArray(input)) {
    throw new Error("desktop diagnostic event is invalid");
  }
  const record = input as Record<string, unknown>;
  const at = record.at;
  if (
    typeof at !== "string" ||
    at.length !== 24 ||
    new Date(at).toISOString() !== at
  ) {
    throw new Error("desktop diagnostic event timestamp is invalid");
  }
  const { at: _at, ...event } = record;
  return { ...validateEvent(event), at };
}

function validateEvent(input: unknown): DiagnosticEvent {
  if (typeof input !== "object" || input === null || Array.isArray(input)) {
    throw new Error("desktop diagnostic event is invalid");
  }
  const event = input as Record<string, unknown>;
  switch (event.kind) {
    case "startup":
      requireEvent(event, ["kind", "packaged"], {
        packaged: [true, false],
      });
      return { kind: "startup", packaged: event.packaged as boolean };
    case "policy":
      requireEvent(
        event,
        ["kind", "mode", "userInstances", "diagnostics"],
        {
          mode: ["open", "managed", "locked"],
          userInstances: ["allowed", "restricted"],
          diagnostics: ["enabled", "disabled"],
        },
      );
      return {
        kind: "policy",
        mode: event.mode as EventOf<"policy">["mode"],
        userInstances:
          event.userInstances as EventOf<"policy">["userInstances"],
        diagnostics:
          event.diagnostics as EventOf<"policy">["diagnostics"],
      };
    case "discovery":
      requireEvent(event, ["kind", "outcome"], {
        outcome: ["success", "rejected", "unavailable"],
      });
      return {
        kind: "discovery",
        outcome: event.outcome as EventOf<"discovery">["outcome"],
      };
    case "authentication":
      requireEvent(event, ["kind", "phase"], {
        phase: [
          "session-valid",
          "required",
          "started",
          "completed",
          "failed",
          "disconnected",
        ],
      });
      return {
        kind: "authentication",
        phase: event.phase as EventOf<"authentication">["phase"],
      };
    case "profile":
      requireEvent(event, ["kind", "action", "outcome"], {
        action: [
          "added",
          "opened",
          "disconnected",
          "removed",
          "renamed",
          "replaced",
        ],
        outcome: ["success", "failed"],
      });
      return {
        kind: "profile",
        action: event.action as EventOf<"profile">["action"],
        outcome: event.outcome as EventOf<"profile">["outcome"],
      };
    case "navigation":
      requireEvent(event, ["kind", "action"], {
        action: [
          "allowed-same-origin-window",
          "requested-external",
          "blocked-main-frame",
          "blocked-popup",
          "blocked-webview",
          "blocked-native-transport",
          "allowed-csv-export",
          "blocked-download",
        ],
      });
      return {
        kind: "navigation",
        action: event.action as EventOf<"navigation">["action"],
      };
    case "stream":
      requireEvent(event, ["kind", "phase"], {
        phase: ["opened", "closed", "failed"],
      });
      return {
        kind: "stream",
        phase: event.phase as EventOf<"stream">["phase"],
      };
    case "remote-lifecycle":
      requireEvent(event, ["kind", "state"], {
        state: ["offline", "crashed"],
      });
      return {
        kind: "remote-lifecycle",
        state: event.state as EventOf<"remote-lifecycle">["state"],
      };
    case "render-process-gone":
      requireEvent(event, ["kind", "surface", "reason"], {
        surface: ["trusted-shell", "remote", "unknown"],
        reason: [...processGoneReasons],
      });
      return {
        kind: "render-process-gone",
        surface:
          event.surface as EventOf<"render-process-gone">["surface"],
        reason: event.reason as ProcessGoneReason,
      };
    case "child-process-gone":
      requireEvent(event, ["kind", "processType", "reason"], {
        processType: [...processTypes],
        reason: [...processGoneReasons],
      });
      return {
        kind: "child-process-gone",
        processType:
          event.processType as EventOf<"child-process-gone">["processType"],
        reason: event.reason as ProcessGoneReason,
      };
    case "update":
      requireEvent(event, ["kind", "phase"], {
        phase: [
          "checking",
          "available",
          "not-available",
          "downloaded",
          "failed",
        ],
      });
      return {
        kind: "update",
        phase: event.phase as EventOf<"update">["phase"],
      };
    default:
      throw new Error("desktop diagnostic event kind is invalid");
  }
}

function requireEvent(
  event: Record<string, unknown>,
  keys: string[],
  allowed: Record<string, unknown[]>,
): void {
  requireExactRecord(event, keys, "diagnostic event");
  for (const [key, values] of Object.entries(allowed)) {
    if (!values.includes(event[key])) {
      throw new Error("desktop diagnostic event value is invalid");
    }
  }
}

function validateEnvironment(
  input: DiagnosticEnvironment,
): DiagnosticEnvironment {
  const environment = requireExactRecord(
    input,
    [
      "applicationVersion",
      "electronVersion",
      "chromiumVersion",
      "nodeVersion",
      "platform",
      "osRelease",
      "architecture",
      "packaged",
      "policyRevision",
    ],
    "diagnostic environment",
  );
  for (const key of [
    "applicationVersion",
    "electronVersion",
    "chromiumVersion",
    "nodeVersion",
    "osRelease",
  ]) {
    if (
      typeof environment[key] !== "string" ||
      !/^[A-Za-z0-9][A-Za-z0-9._+-]{0,63}$/u.test(environment[key])
    ) {
      throw new Error("desktop diagnostic environment value is invalid");
    }
  }
  if (
    typeof environment.platform !== "string" ||
    !/^[a-z0-9]{2,16}$/u.test(environment.platform) ||
    typeof environment.architecture !== "string" ||
    !/^[a-z0-9_]{2,16}$/u.test(environment.architecture) ||
    typeof environment.packaged !== "boolean" ||
    typeof environment.policyRevision !== "string" ||
    !/^desktop-policy-v1(?:-managed-[0-9a-f]{16}|-invalid)?$/u.test(
      environment.policyRevision,
    )
  ) {
    throw new Error("desktop diagnostic environment value is invalid");
  }
  return structuredClone(input);
}

function validateReport(report: DiagnosticReport): void {
  requireExactRecord(
    report,
    [
      "format",
      "schemaVersion",
      "generatedAt",
      "manifest",
      "environment",
      "privacy",
      "events",
    ],
    "diagnostic report",
  );
  if (
    report.format !== "leapview-desktop-diagnostic-report" ||
    report.schemaVersion !== DIAGNOSTIC_SCHEMA_VERSION ||
    typeof report.generatedAt !== "string" ||
    report.generatedAt.length !== 24 ||
    new Date(report.generatedAt).toISOString() !== report.generatedAt ||
    !isDeepStrictEqual(report.manifest, diagnosticManifest()) ||
    !isDeepStrictEqual(report.privacy, diagnosticPrivacy()) ||
    !Array.isArray(report.events) ||
    report.events.length > MAXIMUM_EVENTS
  ) {
    throw new Error("desktop diagnostic report is invalid");
  }
  validateEnvironment(report.environment);
  for (const event of report.events) {
    validateStoredEvent(event);
  }
  const encoded = new TextEncoder().encode(JSON.stringify(report));
  if (encoded.byteLength > MAXIMUM_DOCUMENT_BYTES) {
    throw new Error("desktop diagnostic report is too large");
  }
}

function diagnosticManifest(): DiagnosticReport["manifest"] {
  return {
    files: [
      {
        name: "leapview-diagnostic-report.json",
        sections: ["environment", "privacy", "events"],
      },
    ],
    topLevelFields: [
      "format",
      "schemaVersion",
      "generatedAt",
      "manifest",
      "environment",
      "privacy",
      "events",
    ],
    environmentFields: [
      "applicationVersion",
      "electronVersion",
      "chromiumVersion",
      "nodeVersion",
      "platform",
      "osRelease",
      "architecture",
      "packaged",
      "policyRevision",
    ],
    privacyFields: [
      "crashCollection",
      "crashUpload",
      "rendererConsole",
      "minidumps",
      "instanceOrigins",
      "instanceMetadata",
      "credentials",
      "retentionDays",
    ],
    eventFields: {
      startup: ["at", "kind", "packaged"],
      policy: [
        "at",
        "kind",
        "mode",
        "userInstances",
        "diagnostics",
      ],
      discovery: ["at", "kind", "outcome"],
      authentication: ["at", "kind", "phase"],
      profile: ["at", "kind", "action", "outcome"],
      navigation: ["at", "kind", "action"],
      stream: ["at", "kind", "phase"],
      "remote-lifecycle": ["at", "kind", "state"],
      "render-process-gone": ["at", "kind", "surface", "reason"],
      "child-process-gone": ["at", "kind", "processType", "reason"],
      update: ["at", "kind", "phase"],
    },
  };
}

function diagnosticPrivacy(): DiagnosticReport["privacy"] {
  return {
    crashCollection: "disabled",
    crashUpload: "disabled",
    rendererConsole: "not-collected",
    minidumps: "excluded",
    instanceOrigins: "excluded",
    instanceMetadata: "excluded",
    credentials: "excluded",
    retentionDays: 7,
  };
}

function requireExactRecord(
  input: unknown,
  keys: string[],
  name: string,
): Record<string, unknown> {
  if (typeof input !== "object" || input === null || Array.isArray(input)) {
    throw new Error(`desktop ${name} must be an object`);
  }
  const record = input as Record<string, unknown>;
  const actual = Object.keys(record);
  if (
    actual.length !== keys.length ||
    !keys.every((key) => Object.hasOwn(record, key))
  ) {
    throw new Error(`desktop ${name} fields are invalid`);
  }
  return record;
}

function validCurrentTime(now: () => Date): number {
  const current = now().getTime();
  if (!Number.isFinite(current)) {
    throw new Error("desktop diagnostic clock is invalid");
  }
  return current;
}

function eventFingerprint(
  input: DiagnosticEvent | StoredDiagnosticEvent,
): string {
  const { at: _at, ...event } = input as StoredDiagnosticEvent;
  return JSON.stringify(event);
}

async function writePrivateJSON(path: string, input: unknown): Promise<void> {
  if (!isAbsolute(path)) {
    throw new Error("desktop diagnostic path must be absolute");
  }
  const body = `${JSON.stringify(input, null, 2)}\n`;
  if (new TextEncoder().encode(body).byteLength > MAXIMUM_DOCUMENT_BYTES) {
    throw new Error("desktop diagnostic document is too large");
  }
  await mkdir(dirname(path), { mode: 0o700, recursive: true });
  const temporaryPath = `${path}.${process.pid}.${randomBytes(8).toString("hex")}.tmp`;
  const handle = await open(temporaryPath, "wx", 0o600);
  try {
    await handle.writeFile(body, "utf8");
    await handle.sync();
  } finally {
    await handle.close();
  }
  try {
    await rename(temporaryPath, path);
  } catch (error) {
    await unlink(temporaryPath).catch(() => undefined);
    throw error;
  }
}
