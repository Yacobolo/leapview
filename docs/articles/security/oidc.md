# OIDC

OpenID Connect provides interactive browser identity. LibreDash identifies a person by the stable pair of issuer and subject. Email and display name are profile metadata and may change without creating a new identity.

## Register the client

Create a confidential web application in the identity provider. Register the exact public callback URL:

```text
https://dash.example.com/auth/{provider_id}/callback
```

The provider ID must contain only route-safe letters, numbers, dots, underscores, or dashes. Use one stable value; changing it changes the callback route and requires a coordinated provider update.

Configure LibreDash:

```sh
LIBREDASH_OIDC_PROVIDER_ID=entra
LIBREDASH_OIDC_ISSUER_URL=https://login.microsoftonline.com/<tenant>/v2.0
LIBREDASH_OIDC_CLIENT_ID=<client-id>
LIBREDASH_OIDC_CLIENT_SECRET=<client-secret>
LIBREDASH_OIDC_CALLBACK_URL=https://dash.example.com/auth/entra/callback
LIBREDASH_OIDC_SCOPES="openid profile email"
```

Production validation requires the issuer and callback to use HTTPS and treats issuer, client ID, client secret, and callback as an all-or-none set. Store the client secret in the deployment secret manager.

## Configure the reverse proxy

Terminate TLS at a maintained trusted proxy and ensure the application sees the correct public scheme and host used by the callback. Set exact allowed hosts. Enable proxy-header trust only when that proxy overwrites client-supplied forwarding headers.

Secure cookies must remain enabled for browser auth. Clock synchronization matters for token and state validation on both LibreDash and the identity provider.

## Understand identity mapping

The issuer URL and the token's subject claim form identity. Do not map identity by email alone: email addresses can be renamed or reassigned. Profile changes may update display metadata while privileges remain attached to the same principal.

LibreDash intentionally does not treat OIDC group claims as the enterprise group source of truth. Use SCIM for directory users, groups, and membership, then use LibreDash grants and role bindings for product authorization.

## Assign access

A successful login can still result in no visible workspace. OIDC proves who the user is; it does not grant product access. Bind a provisioned or known principal/group to an appropriate workspace role or explicit grant.

Test with a non-administrator user. An owner account can hide missing group provisioning or role binding because it already has broad access.

## Harden the provider

Use provider MFA and conditional access. Restrict who may use the client, protect client-secret rotation, and monitor provider-side sign-in events. Keep redirect URI lists narrow and remove retired callbacks.

When rotating the client secret, install the new value through the secret manager and coordinate restart without exposing either value. Verify login before revoking the old provider credential where the provider supports overlap.

## Troubleshoot login

If the provider rejects the request, compare the callback URL character for character. If callback reaches LibreDash but state or cookie validation fails, check HTTPS, cookie security, allowed hosts, CSRF key consistency, proxy headers, and clock skew. If login succeeds but access is empty, inspect principal identity and grants rather than the OIDC handshake.

Preserve correlation timestamps and inspect both provider logs and LibreDash audit/application logs without recording raw tokens.

See [SCIM provisioning](/docs/security/scim), [Roles, grants, and policies](/docs/security/authorization), and [Authentication and authorization](/docs/enterprise-auth).
