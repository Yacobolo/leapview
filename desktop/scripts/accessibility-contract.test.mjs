import assert from "node:assert/strict";
import test from "node:test";

import { verifyTrustedShellAccessibility } from "./accessibility-contract.mjs";

const node = (role, name, properties = []) => ({
  role: { value: role },
  name: { value: name },
  properties,
});

const treeNode = (
  id,
  role,
  name,
  properties = [],
  childIds = [],
) => ({
  ...node(role, name, properties),
  nodeId: id,
  childIds,
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
    announcement: "none",
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
      [{ name: "live", value: { value: "assertive" } }],
    ),
  ]);

  assert.deepEqual(report, {
    mode: "locked",
    announcement: "assertive",
    controls: 0,
    focusedControl: "Application document",
    regions: [],
  });
});

test("accepts Windows AX trees that expose alert text as a child", () => {
  const report = verifyTrustedShellAccessibility([
    treeNode("root", "RootWebArea", "LeapView", [
      { name: "focused", value: { value: true } },
    ], ["main"]),
    treeNode("main", "main", "", [], ["heading", "alert"]),
    treeNode("heading", "heading", "Connect to LeapView"),
    treeNode("alert", "alert", "", [
      { name: "live", value: { value: "assertive" } },
    ], ["alert-text"]),
    treeNode(
      "alert-text",
      "StaticText",
      "The managed desktop configuration is invalid; contact your administrator.",
    ),
  ]);

  assert.deepEqual(report, {
    mode: "locked",
    announcement: "assertive",
    controls: 0,
    focusedControl: "Application document",
    regions: [],
  });
});

test("does not mistake unrelated accessible text for the policy alert", () => {
  assert.throws(
    () =>
      verifyTrustedShellAccessibility([
        treeNode("root", "RootWebArea", "LeapView", [
          { name: "focused", value: { value: true } },
        ], ["main"]),
        treeNode("main", "main", "", [], ["heading", "alert", "text"]),
        treeNode("heading", "heading", "Connect to LeapView"),
        treeNode("alert", "alert", "", [
          { name: "live", value: { value: "assertive" } },
        ]),
        treeNode(
          "text",
          "StaticText",
          "The managed desktop configuration is invalid; contact your administrator.",
        ),
      ]),
    /managed configuration alert/u,
  );
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
