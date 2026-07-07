# Enterprise Auth

LibreDash uses the standard enterprise split:

```text
OIDC = interactive human login
SCIM = enterprise user and group provisioning
Grant engine = LibreDash authorization
Service principals = non-human workload identity
API tokens = scoped credentials, not identities
```

OIDC proves who a user is. SCIM syncs users, groups, and memberships.
LibreDash grants remain the only source of product authorization.

## Local Auth

Enable local browser login for self-hosted deployments or break-glass access:

```sh
LIBREDASH_LOCAL_AUTH=1
LIBREDASH_CSRF_KEY=<32+ byte secret>
```

Local users are admin-created only. A grant manager creates the user from
Admin / Principals or `POST /api/v1/principals`, receives a one-time temporary
password, and shares it out of band. Password resets use
`POST /api/v1/principals/{principal}/password-reset` and force the user to
change the temporary password on next sign-in.

Local users map to ordinary `principals.kind = user`. Local groups remain
workspace groups with `provider = local`. Both use the same roles, grants,
sessions, API tokens, and audit events as OIDC and SCIM identities.

## OIDC

Configure one browser identity provider with:

```sh
LIBREDASH_OIDC_PROVIDER_ID=entra
LIBREDASH_OIDC_ISSUER_URL=https://login.microsoftonline.com/<tenant-id>/v2.0
LIBREDASH_OIDC_CLIENT_ID=<client-id>
LIBREDASH_OIDC_CLIENT_SECRET=<client-secret>
LIBREDASH_OIDC_CALLBACK_URL=https://<host>/auth/entra/callback
LIBREDASH_OIDC_SCOPES="openid profile email"
```

Provider examples:

- Entra ID issuer: `https://login.microsoftonline.com/<tenant-id>/v2.0`
- Okta issuer: `https://<org>.okta.com/oauth2/default`
- Auth0 issuer: `https://<tenant>.<region>.auth0.com/`
- Keycloak issuer: `https://<host>/realms/<realm>`

Register the callback URL as:

```text
https://<host>/auth/{provider_id}/callback
```

LibreDash maps identity by OIDC issuer plus subject. Email and display name are
metadata and may change.

## SCIM

Enable SCIM by setting a dedicated provisioning bearer token:

```sh
LIBREDASH_SCIM_BEARER_TOKEN=<long-random-secret>
```

When the token is set, LibreDash mounts:

```text
https://<host>/scim/v2
```

Supported SCIM resources:

- `GET /scim/v2/ServiceProviderConfig`
- `GET /scim/v2/Schemas`
- `GET /scim/v2/ResourceTypes`
- `GET|POST|PATCH|DELETE /scim/v2/Users`
- `GET|POST|PATCH|DELETE /scim/v2/Groups`

SCIM users map to `principals.kind = user`. SCIM groups are global directory
groups and can be granted access to any workspace or securable object. SCIM
group membership changes affect effective access immediately.

`active=false` and `DELETE /Users/{id}` disable the principal, remove SCIM group
memberships, and revoke sessions and API tokens. Deletes are soft disables to
preserve audit history.

OIDC group claims are intentionally ignored. SCIM is the enterprise group source
of truth.

## Authorization

After OIDC or SCIM establishes identities, grant access in LibreDash:

```text
principal/group/service_principal -> privilege -> securable_object
```

API tokens are credentials for an existing principal. They can only reduce
access:

```text
principal effective privileges ∩ token workspace scope ∩ token privilege allowlist
```
