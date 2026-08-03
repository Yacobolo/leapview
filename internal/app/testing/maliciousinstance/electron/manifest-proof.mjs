import assert from "node:assert/strict";

export const MANIFEST_VERSION = "leapview.desktop.security/v2";
export const MAX_MANIFEST_BYTES = 64 * 1024;

const attackIDPattern = /^[a-z][a-z0-9-]*(?:\.[a-z][a-z0-9-]*)+$/;
const triggers = new Set(["automatic", "user-gesture", "navigation"]);
const outcomes = new Set([
  "denied",
  "isolated",
  "responsive",
  "exposed",
  "unsupported",
  "error",
]);
const maxAttacks = 128;
const maxTextLength = 256;

export function validateManifest(value) {
  assertPlainObject(value, "manifest");
  assert.equal(value.version, MANIFEST_VERSION, "unsupported manifest version");
  assert.ok(Array.isArray(value.attacks), "manifest attacks must be an array");
  assert.ok(value.attacks.length > 0, "manifest must include attacks");
  assert.ok(value.attacks.length <= maxAttacks, "manifest includes too many attacks");

  const seen = new Set();
  for (const [index, attack] of value.attacks.entries()) {
    assertPlainObject(attack, `attack ${index}`);
    assert.match(attack.id, attackIDPattern, `attack ${index} has an invalid ID`);
    assert.ok(!seen.has(attack.id), `attack ID ${attack.id} is duplicated`);
    seen.add(attack.id);
    assertBoundedText(attack.title, `attack ${attack.id} title`);
    assertBoundedText(attack.category, `attack ${attack.id} category`);
    assertRelativeAttackPath(attack.path, attack.id);
    assert.ok(triggers.has(attack.trigger), `attack ${attack.id} has an invalid trigger`);
    assert.ok(outcomes.has(attack.expected), `attack ${attack.id} has an invalid expected outcome`);
  }
  return value;
}

export function createObservationRecorder(manifestValue) {
  const manifest = validateManifest(manifestValue);
  const attacks = new Map(manifest.attacks.map((attack) => [attack.id, attack]));
  const recorded = new Map();

  return Object.freeze({
    record(attackID, outcome) {
      const attack = attacks.get(attackID);
      assert.ok(attack, `unknown attack ID ${attackID}`);
      assert.ok(!recorded.has(attackID), `attack ${attackID} was recorded twice`);
      assert.equal(
        outcome,
        attack.expected,
        `attack ${attackID} produced ${outcome}, expected ${attack.expected}`,
      );
      recorded.set(attackID, Object.freeze({ attackId: attackID, outcome }));
    },
    finalize() {
      const missing = manifest.attacks
        .map((attack) => attack.id)
        .filter((attackID) => !recorded.has(attackID));
      assert.deepEqual(missing, [], `manifest attacks were not observed: ${missing.join(", ")}`);
      return manifest.attacks.map((attack) => recorded.get(attack.id));
    },
  });
}

export async function fetchBoundedJSON(url, options = {}) {
  const maxBytes = options.maxBytes ?? MAX_MANIFEST_BYTES;
  assert.ok(Number.isSafeInteger(maxBytes) && maxBytes > 0, "maxBytes must be a positive integer");

  const response = await fetch(url, {
    cache: "no-store",
    redirect: "error",
    signal: options.signal,
  });
  assert.equal(response.status, 200, `unexpected response status for ${new URL(url).pathname}`);

  const declaredLength = response.headers.get("content-length");
  if (declaredLength !== null) {
    const parsedLength = Number(declaredLength);
    assert.ok(Number.isSafeInteger(parsedLength), "invalid Content-Length");
    assert.ok(parsedLength <= maxBytes, `response exceeds ${maxBytes} bytes`);
  }
  assert.ok(response.body, "response body is unavailable");

  const reader = response.body.getReader();
  const chunks = [];
  let received = 0;
  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }
      received += value.byteLength;
      assert.ok(received <= maxBytes, `response exceeds ${maxBytes} bytes`);
      chunks.push(value);
    }
  } finally {
    if (received > maxBytes) {
      await reader.cancel("bounded response limit exceeded").catch(() => {});
    }
    reader.releaseLock();
  }

  const bytes = new Uint8Array(received);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
}

function assertPlainObject(value, label) {
  assert.ok(value && typeof value === "object" && !Array.isArray(value), `${label} must be an object`);
}

function assertBoundedText(value, label) {
  assert.equal(typeof value, "string", `${label} must be a string`);
  assert.ok(value.trim().length > 0, `${label} must not be empty`);
  assert.ok(value.length <= maxTextLength, `${label} is too long`);
}

function assertRelativeAttackPath(value, attackID) {
  assert.equal(typeof value, "string", `attack ${attackID} path must be a string`);
  const parsed = new URL(value, "https://manifest.invalid");
  assert.equal(parsed.origin, "https://manifest.invalid", `attack ${attackID} path must be relative`);
  assert.equal(parsed.pathname, value, `attack ${attackID} path must not contain a query or fragment`);
  assert.equal(parsed.pathname, `/attack/${attackID}`, `attack ${attackID} path is not canonical`);
}
