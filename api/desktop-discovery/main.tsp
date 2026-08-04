import "@yacobolo/apigen";

@apigen.`package`(#{
  title: "LeapView Desktop Discovery Contracts",
  version: "1.0.0",
  description: "Public compatibility document and stable client-side failure taxonomy used before a desktop client trusts a LeapView instance.",
})
namespace LeapViewDesktopDiscovery;

@minLength(1)
@maxLength(2048)
scalar DesktopCanonicalOrigin extends string;

@pattern("^instance_[0-9a-f]{32}$")
scalar DesktopInstanceId extends string;

@minLength(1)
@maxLength(120)
scalar DesktopDisplayName extends string;

@minLength(1)
@maxLength(64)
scalar DesktopServerVersion extends string;

@minValue(1)
scalar DesktopProtocolVersion extends int32;

/** Authentication handshakes a compatible desktop client may use. */
enum DesktopAuthenticationMode {
  browserSession: "browser-session",
  systemBrowserPKCE: "system-browser-pkce",
}

/** Server capabilities required by the native desktop shell. */
enum DesktopCapability {
  remoteWeb: "remote-web",
}

/**
 * Stable, non-secret failure categories. These values are safe to use for
 * trusted-shell UX and local diagnostics; raw network errors are never part of
 * this contract.
 */
enum DesktopDiscoveryFailureKind {
  invalidOrigin: "invalid_origin",
  tls: "tls",
  proxy: "proxy",
  dns: "dns",
  network: "network",
  timeout: "timeout",
  redirect: "redirect",
  http: "http",
  malformedResponse: "malformed_response",
  canonicalOriginMismatch: "canonical_origin_mismatch",
  instanceIdentityMismatch: "instance_identity_mismatch",
  schemaIncompatible: "schema_incompatible",
  protocolIncompatible: "protocol_incompatible",
  authenticationIncompatible: "authentication_incompatible",
  capabilityIncompatible: "capability_incompatible",
}

/**
 * Bounded public document served without credentials from
 * `/.well-known/leapview`.
 */
@apigen.contract(#{ kind: "desktop-discovery", tags: #["desktop", "public"] })
@apigen.`metadata`(#{ `x-leapview-contract-role`: "desktop-discovery" })
model DesktopDiscoveryDocument {
  `schemaVersion`: 1;
  `canonicalOrigin`: DesktopCanonicalOrigin;
  `instanceId`: DesktopInstanceId;
  `displayName`: DesktopDisplayName;
  `serverVersion`: DesktopServerVersion;
  `desktopProtocolMin`: DesktopProtocolVersion;
  `desktopProtocolMax`: DesktopProtocolVersion;
  @minItems(1)
  @maxItems(16)
  `authenticationModes`: DesktopAuthenticationMode[];
  @minItems(1)
  @maxItems(16)
  `capabilities`: DesktopCapability[];
}

/**
 * Serializable representation of a desktop discovery failure. The desktop
 * Error class carries the same generated kind while keeping its cause local.
 */
@apigen.contract(#{ kind: "desktop-discovery-failure", tags: #["desktop", "public"] })
@apigen.`metadata`(#{ `x-leapview-contract-role`: "desktop-discovery-failure" })
model DesktopDiscoveryFailure {
  `schemaVersion`: 1;
  `kind`: DesktopDiscoveryFailureKind;
}
