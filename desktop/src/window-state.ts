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

const WINDOW_STATE_SCHEMA_VERSION = 1;
const MAXIMUM_DOCUMENT_BYTES = 64 * 1024;
const MAXIMUM_WINDOWS = 101;
const MAXIMUM_COORDINATE = 1_000_000;
const MAXIMUM_DIMENSION = 16_384;
const stateKeyPattern = /^(?:shell|profile_[0-9a-f]{32})$/u;

export interface WindowBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface PersistedWindowState {
  bounds: WindowBounds;
  maximized: boolean;
}

interface WindowStateDocument {
  schemaVersion: 1;
  windows: Record<string, PersistedWindowState>;
}

export class WindowStateStore {
  readonly #path: string;
  readonly #states: Map<string, PersistedWindowState>;
  #dirty = false;
  #flushPromise: Promise<void> | null = null;

  private constructor(
    path: string,
    states: Map<string, PersistedWindowState>,
  ) {
    this.#path = path;
    this.#states = states;
  }

  static async open(path: string): Promise<WindowStateStore> {
    try {
      const information = await stat(path);
      if (
        process.platform !== "win32" &&
        (information.mode & 0o077) !== 0
      ) {
        throw new Error("desktop window state file permissions are not private");
      }
      if (information.size > MAXIMUM_DOCUMENT_BYTES) {
        throw new Error("desktop window state document is too large");
      }
      const body = await readFile(path, "utf8");
      const document = validateDocument(JSON.parse(body) as unknown);
      return new WindowStateStore(
        path,
        new Map(Object.entries(document.windows)),
      );
    } catch {
      // Window placement is optional and never important enough to block startup.
      return new WindowStateStore(path, new Map());
    }
  }

  get(key: string): PersistedWindowState | undefined {
    const state = this.#states.get(key);
    return state === undefined ? undefined : structuredClone(state);
  }

  record(key: string, state: PersistedWindowState): void {
    validateStateKey(key);
    const validated = validateState(state);
    if (!this.#states.has(key) && this.#states.size >= MAXIMUM_WINDOWS) {
      throw new Error("desktop window state list is full");
    }
    this.#states.set(key, validated);
    this.#dirty = true;
  }

  remove(key: string): void {
    validateStateKey(key);
    if (this.#states.delete(key)) {
      this.#dirty = true;
    }
  }

  flush(): Promise<void> {
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
      const document: WindowStateDocument = {
        schemaVersion: WINDOW_STATE_SCHEMA_VERSION,
        windows: Object.fromEntries(
          [...this.#states].map(([key, state]) => [
            key,
            structuredClone(state),
          ]),
        ),
      };
      try {
        await this.#write(document);
      } catch (error) {
        this.#dirty = true;
        throw error;
      }
    }
  }

  async #write(document: WindowStateDocument): Promise<void> {
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
}

export function fitWindowStateToWorkArea(
  state: PersistedWindowState,
  workArea: WindowBounds,
  minimumSize: { width: number; height: number },
): PersistedWindowState {
  const validated = validateState(state);
  const area = validateBounds(workArea);
  const minimumWidth = validateDimension(minimumSize.width);
  const minimumHeight = validateDimension(minimumSize.height);
  const width = Math.min(
    area.width,
    Math.max(validated.bounds.width, minimumWidth),
  );
  const height = Math.min(
    area.height,
    Math.max(validated.bounds.height, minimumHeight),
  );
  return {
    bounds: {
      x: clamp(
        validated.bounds.x,
        area.x,
        area.x + area.width - width,
      ),
      y: clamp(
        validated.bounds.y,
        area.y,
        area.y + area.height - height,
      ),
      width,
      height,
    },
    maximized: validated.maximized,
  };
}

function validateDocument(input: unknown): WindowStateDocument {
  if (
    typeof input !== "object" ||
    input === null ||
    Array.isArray(input)
  ) {
    throw new Error("desktop window state document must be an object");
  }
  const document = input as Record<string, unknown>;
  if (
    !hasOnlyKeys(document, ["schemaVersion", "windows"]) ||
    document.schemaVersion !== WINDOW_STATE_SCHEMA_VERSION ||
    typeof document.windows !== "object" ||
    document.windows === null ||
    Array.isArray(document.windows)
  ) {
    throw new Error("desktop window state document is invalid");
  }
  const entries = Object.entries(document.windows);
  if (entries.length > MAXIMUM_WINDOWS) {
    throw new Error("desktop window state list is invalid");
  }
  const windows: Record<string, PersistedWindowState> = {};
  for (const [key, value] of entries) {
    validateStateKey(key);
    windows[key] = validateState(value);
  }
  return { schemaVersion: WINDOW_STATE_SCHEMA_VERSION, windows };
}

function validateStateKey(key: string): void {
  if (!stateKeyPattern.test(key)) {
    throw new Error("desktop window state key is invalid");
  }
}

function validateState(input: unknown): PersistedWindowState {
  if (
    typeof input !== "object" ||
    input === null ||
    Array.isArray(input)
  ) {
    throw new Error("desktop window state must be an object");
  }
  const state = input as Record<string, unknown>;
  if (
    !hasOnlyKeys(state, ["bounds", "maximized"]) ||
    typeof state.maximized !== "boolean"
  ) {
    throw new Error("desktop window state is invalid");
  }
  return {
    bounds: validateBounds(state.bounds),
    maximized: state.maximized,
  };
}

function validateBounds(input: unknown): WindowBounds {
  if (
    typeof input !== "object" ||
    input === null ||
    Array.isArray(input)
  ) {
    throw new Error("desktop window bounds must be an object");
  }
  const bounds = input as Record<string, unknown>;
  if (
    !hasOnlyKeys(bounds, ["x", "y", "width", "height"]) ||
    !isCoordinate(bounds.x) ||
    !isCoordinate(bounds.y) ||
    !isDimension(bounds.width) ||
    !isDimension(bounds.height)
  ) {
    throw new Error("desktop window bounds are invalid");
  }
  return {
    x: bounds.x,
    y: bounds.y,
    width: bounds.width,
    height: bounds.height,
  };
}

function hasOnlyKeys(
  input: Record<string, unknown>,
  expected: string[],
): boolean {
  const keys = Object.keys(input);
  return (
    keys.length === expected.length &&
    expected.every((key) => Object.hasOwn(input, key))
  );
}

function isCoordinate(input: unknown): input is number {
  return (
    typeof input === "number" &&
    Number.isSafeInteger(input) &&
    Math.abs(input) <= MAXIMUM_COORDINATE
  );
}

function isDimension(input: unknown): input is number {
  return (
    typeof input === "number" &&
    Number.isSafeInteger(input) &&
    input > 0 &&
    input <= MAXIMUM_DIMENSION
  );
}

function validateDimension(input: number): number {
  if (!isDimension(input)) {
    throw new Error("desktop window dimension is invalid");
  }
  return input;
}

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(Math.max(value, minimum), maximum);
}
