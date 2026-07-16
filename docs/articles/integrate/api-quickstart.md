# API quickstart

The headless API is served beneath `/api/v1`. This guide verifies authentication, discovers a workspace, and shows how to move from raw HTTP to generated operation metadata.

## Create a scoped credential

Use a dedicated service principal or a user token issued for this integration. Grant only the workspace and privileges needed for the first call. Store the values in the current shell from a secret manager:

```sh
export LIBREDASH_TARGET=https://dash.example.com
export LIBREDASH_API_TOKEN=<secret>
```

Do not include bearer tokens in URLs. Avoid shell tracing while secrets are present.

## Verify the principal

```sh
curl --fail-with-body \
  --silent --show-error \
  --header "Authorization: Bearer $LIBREDASH_API_TOKEN" \
  --header "Accept: application/json" \
  "$LIBREDASH_TARGET/api/v1/me"
```

A `200` response identifies the authenticated principal. `401` means the credential is absent, invalid, expired, or revoked. `403` on a later operation means authentication succeeded but effective privilege is insufficient.

## List workspaces

Request a bounded page:

```sh
curl --fail-with-body \
  --silent --show-error \
  --header "Authorization: Bearer $LIBREDASH_API_TOKEN" \
  --header "Accept: application/json" \
  "$LIBREDASH_TARGET/api/v1/workspaces?limit=50"
```

Use stable workspace IDs from the response in path parameters. Titles are display metadata and are not safe identifiers. If the response provides a next-page token, pass it back as `pageToken` without inspecting or modifying it.

## Discover a dashboard

With a workspace ID such as `sales`:

```sh
curl --fail-with-body \
  --silent --show-error \
  --header "Authorization: Bearer $LIBREDASH_API_TOKEN" \
  --header "Accept: application/json" \
  "$LIBREDASH_TARGET/api/v1/workspaces/sales/dashboards?limit=50"
```

The BI API then provides dashboard description, page-component discovery, filter options, coordinated page queries, visual data, and table windows. Request and response bodies are defined by OpenAPI; do not infer them from browser network traffic.

## Use the generated CLI API client

The CLI can discover and call operations from the generated API registry:

```sh
libredash api list
libredash api describe <operation>
libredash api call <operation> \
  --target "$LIBREDASH_TARGET" \
  --token "$LIBREDASH_API_TOKEN"
```

Use repeatable `--path key=value` and `--query key=value` arguments for parameters. Supply JSON through `--body-json` for small controlled values or `--body-file` to keep larger payloads out of shell quoting and history.

## Handle responses safely

Set client timeouts, check status before decoding success payloads, cap response sizes appropriate to the operation, and avoid automatic retries for state-changing requests unless the operation documents safe idempotency. Honor `429` backoff and preserve request/operation IDs from responses or logs for support.

Use the generated [API reference](/docs/api), [API conventions](/docs/guides/integrate/api-conventions), and downloadable [OpenAPI document](/docs/openapi.yaml) for exact contracts.
