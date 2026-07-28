import assert from "node:assert/strict";
import { createServer } from "node:http";
import test from "node:test";

import {
  createObservationRecorder,
  fetchBoundedJSON,
  validateManifest,
} from "./manifest-proof.mjs";

const manifest = Object.freeze({
  version: "leapview.desktop.security/v1",
  attacks: Object.freeze([
    Object.freeze({
      id: "native.electron-global",
      title: "Electron globals",
      category: "native-bridge",
      path: "/attack/native.electron-global",
      trigger: "automatic",
      expected: "blocked",
    }),
    Object.freeze({
      id: "navigation.cross-origin",
      title: "Cross-origin navigation",
      category: "navigation",
      path: "/attack/navigation.cross-origin",
      trigger: "navigation",
      expected: "blocked",
    }),
  ]),
});

test("manifest validation accepts only the exact bounded contract", () => {
  assert.equal(validateManifest(manifest), manifest);

  for (const invalid of [
    { ...manifest, version: "leapview.desktop.security/v2" },
    { ...manifest, attacks: [] },
    { ...manifest, attacks: [manifest.attacks[0], manifest.attacks[0]] },
    {
      ...manifest,
      attacks: [{ ...manifest.attacks[0], path: "https://attacker.example" }],
    },
    {
      ...manifest,
      attacks: [{ ...manifest.attacks[0], trigger: "sometimes" }],
    },
    {
      ...manifest,
      attacks: [{ ...manifest.attacks[0], expected: "maybe" }],
    },
  ]) {
    assert.throws(() => validateManifest(invalid));
  }
});

test("observation recorder requires one expected result for every manifest attack", () => {
  const recorder = createObservationRecorder(manifest);
  recorder.record("navigation.cross-origin", "blocked");
  recorder.record("native.electron-global", "blocked");

  assert.deepEqual(recorder.finalize(), [
    { attackId: "native.electron-global", outcome: "blocked" },
    { attackId: "navigation.cross-origin", outcome: "blocked" },
  ]);
});

test("observation recorder rejects unknown, duplicate, incomplete, or exposed results", () => {
  const unknown = createObservationRecorder(manifest);
  assert.throws(() => unknown.record("native.unknown", "blocked"));

  const duplicate = createObservationRecorder(manifest);
  duplicate.record("native.electron-global", "blocked");
  assert.throws(() => duplicate.record("native.electron-global", "blocked"));

  const incomplete = createObservationRecorder(manifest);
  incomplete.record("native.electron-global", "blocked");
  assert.throws(() => incomplete.finalize());

  const exposed = createObservationRecorder(manifest);
  assert.throws(() => exposed.record("native.electron-global", "exposed"));
});

test("bounded JSON reader rejects malformed and oversized discovery responses", async (context) => {
  const server = createServer((request, response) => {
    if (request.url === "/valid") {
      response.setHeader("Content-Type", "application/json");
      response.end('{"version":1}');
      return;
    }
    if (request.url === "/malformed") {
      response.setHeader("Content-Type", "application/json");
      response.end('{"version":');
      return;
    }
    response.setHeader("Content-Type", "application/json");
    response.end("x".repeat(1024));
  });
  await new Promise((resolve) => server.listen(0, "127.0.0.1", resolve));
  context.after(() => new Promise((resolve) => server.close(resolve)));
  const address = server.address();
  assert.ok(address && typeof address === "object");
  const origin = `http://127.0.0.1:${address.port}`;

  assert.deepEqual(await fetchBoundedJSON(`${origin}/valid`, { maxBytes: 64 }), {
    version: 1,
  });
  await assert.rejects(fetchBoundedJSON(`${origin}/malformed`, { maxBytes: 64 }));
  await assert.rejects(
    fetchBoundedJSON(`${origin}/oversized`, { maxBytes: 64 }),
    /exceeds 64 bytes/,
  );
});
