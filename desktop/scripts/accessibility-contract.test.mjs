import assert from "node:assert/strict";
import test from "node:test";

import { verifyTrustedShellAccessibility } from "./accessibility-contract.mjs";

const node = (role, name, properties = []) => ({
  role: { value: role },
  name: { value: name },
  properties,
});

test("accepts the named, focused trusted-shell accessibility contract", () => {
  const report = verifyTrustedShellAccessibility([
    node("RootWebArea", "LeapView", [
      { name: "focused", value: { value: true } },
    ]),
    node("main", ""),
    node("heading", "Connect to LeapView"),
    node("region", "Connect an instance"),
    node("textbox", "LeapView URL", [
      { name: "focused", value: { value: true } },
      { name: "required", value: { value: true } },
    ]),
    node("button", "Verify & open"),
  ]);

  assert.deepEqual(report, {
    mode: "open",
    controls: 2,
    focusedControl: "LeapView URL",
    regions: ["Connect an instance"],
  });
});

test("accepts the focused fail-closed managed-policy state", () => {
  const report = verifyTrustedShellAccessibility([
    node("RootWebArea", "LeapView", [
      { name: "focused", value: { value: true } },
    ]),
    node("main", ""),
    node("heading", "Connect to LeapView"),
    node(
      "alert",
      "The managed desktop configuration is invalid; contact your administrator.",
      [{ name: "focused", value: { value: true } }],
    ),
  ]);

  assert.deepEqual(report, {
    mode: "locked",
    controls: 0,
    focusedControl: "Managed configuration error",
    regions: [],
  });
});

test("rejects missing landmarks, names, and deterministic initial focus", () => {
  for (const nodes of [
    [
      node("RootWebArea", "LeapView"),
      node("heading", "Connect to LeapView"),
      node("textbox", "LeapView URL"),
      node("button", "Verify & open"),
    ],
    [
      node("RootWebArea", "LeapView"),
      node("main", ""),
      node("heading", "Connect to LeapView"),
      node("region", "Connect an instance"),
      node("textbox", "", [
        { name: "focused", value: { value: true } },
      ]),
      node("button", "Verify & open"),
    ],
    [
      node("RootWebArea", "LeapView"),
      node("main", ""),
      node("heading", "Connect to LeapView"),
      node("region", "Connect an instance"),
      node("textbox", "LeapView URL"),
      node("button", "Verify & open"),
    ],
  ]) {
    assert.throws(
      () => verifyTrustedShellAccessibility(nodes),
      /accessib/u,
    );
  }
});

test("rejects duplicate or unnamed interactive controls", () => {
  assert.throws(
    () =>
      verifyTrustedShellAccessibility([
        node("RootWebArea", "LeapView"),
        node("main", ""),
        node("heading", "Connect to LeapView"),
        node("region", "Connect an instance"),
        node("textbox", "LeapView URL", [
          { name: "focused", value: { value: true } },
        ]),
        node("button", "Verify & open"),
        node("button", ""),
      ]),
    /accessible name/u,
  );
});
