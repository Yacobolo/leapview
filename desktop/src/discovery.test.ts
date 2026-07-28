import { describe, expect, test } from "bun:test";

import {
  DesktopDiscoveryError,
  discoverInstance,
  validateDiscoveryDocument,
} from "./discovery.js";

const origin = "https://analytics.company.com";
const validDocument = {
  schemaVersion: 1,
  canonicalOrigin: origin,
  instanceId: "instance_0123456789abcdef0123456789abcdef",
  displayName: "Company Analytics",
  serverVersion: "v1.4.2",
  desktopProtocolMin: 1,
  desktopProtocolMax: 1,
  authenticationModes: ["browser-session"],
  capabilities: ["remote-web"],
};

describe("validateDiscoveryDocument", () => {
  test("accepts the supported bounded contract", () => {
    expect(validateDiscoveryDocument(validDocument, origin)).toEqual(validDocument);
  });

  test("rejects origin substitution, instance spoofing, and incompatible protocols", () => {
    expect(() =>
      validateDiscoveryDocument(
        { ...validDocument, canonicalOrigin: "https://attacker.example" },
        origin,
      ),
    ).toThrow("canonical origin");
    expect(() =>
      validateDiscoveryDocument({ ...validDocument, instanceId: origin }, origin),
    ).toThrow("instance id");
    expect(() =>
      validateDiscoveryDocument(
        { ...validDocument, desktopProtocolMin: 2, desktopProtocolMax: 4 },
        origin,
      ),
    ).toThrow("not compatible");
  });

  test("rejects excessive nesting, strings, and arrays", () => {
    expect(() =>
      validateDiscoveryDocument(
        { ...validDocument, displayName: "a".repeat(121) },
        origin,
      ),
    ).toThrow("display name");
    expect(() =>
      validateDiscoveryDocument(
        { ...validDocument, capabilities: Array.from({ length: 17 }, () => "remote-web") },
        origin,
      ),
    ).toThrow("array");
    expect(() =>
      validateDiscoveryDocument(
        { ...validDocument, extension: { a: { b: { c: { d: { e: { f: {} } } } } } } },
        origin,
      ),
    ).toThrow("nested");
  });
});

describe("discoverInstance", () => {
  test("fetches without credentials, redirects, or cache", async () => {
    let requestURL = "";
    let requestInit: RequestInit | undefined;
    const document = await discoverInstance(origin, async (url, init) => {
      requestURL = url;
      requestInit = init;
      return Response.json(validDocument, {
        headers: { "content-type": "application/json" },
      });
    });

    expect(document).toEqual(validDocument);
    expect(requestURL).toBe(`${origin}/.well-known/leapview`);
    expect(requestInit).toMatchObject({
      cache: "no-store",
      credentials: "omit",
      redirect: "error",
      referrerPolicy: "no-referrer",
    });
  });

  test("rejects non-JSON and oversized responses", async () => {
    await expect(
      discoverInstance(origin, async () =>
        new Response("not json", { headers: { "content-type": "text/plain" } }),
      ),
    ).rejects.toBeInstanceOf(DesktopDiscoveryError);
    await expect(
      discoverInstance(origin, async () =>
        new Response("x".repeat(65_537), {
          headers: { "content-type": "application/json" },
        }),
      ),
    ).rejects.toThrow("too large");
  });
});
