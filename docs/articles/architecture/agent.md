# Agent architecture

LeapView agents are governed, read-only consumers of the same catalog,
semantic-model, visualization, authorization, and documentation boundaries
used by the product. The agent surface is an integration boundary, not a
second query engine or an unrestricted SQL interface.

## Ownership

- `internal/agent/` owns conversations, runs, transcripts, the curated tool
  catalog, model interaction ports, product-documentation access, and agent UI
  projections.
- `internal/agent/contracts/typespec/` is the source of truth for curated
  agent DTOs and closed tool schemas. Generated contracts are not independent
  authorities.
- `internal/app/` composes the agent module into built-in chat, the deployment
  MCP endpoint, and the CLI surface. It owns global routing and composition,
  not agent product state.
- `internal/platform/` supplies generic protocol, HTTP, security, and
  observability mechanisms without knowing agent or BI product language.

## Governed request boundary

Every catalog or query request carries an explicit workspace and resource
identity. The agent must discover refs through `catalog_search` or
`catalog_list`, inspect definitions with `catalog_get`, and then use the
capability returned by the catalog item. It must not infer identifiers, read
raw source data, construct arbitrary SQL, or bypass row and column policies.

Semantic queries and saved-visual queries converge on the same governed query
and authorization boundaries as dashboards and headless API clients. The
agent receives compact typed result envelopes with provenance, freshness,
completeness, and safe diagnostics rather than renderer internals or database
rows.

## Tool catalog

The runtime catalog exposes the same versioned tool set through built-in chat,
deployment MCP, and `leapview agent tools`:

- `catalog_search`, `catalog_list`, and `catalog_get` discover governed
  resources;
- `query_semantic_model`, `query_dashboard_visual`, and `query_visual` return
  bounded governed analytical results;
- `docs_search` and `docs_read` expose version-matched product documentation.

The catalog is read-only. It does not expose connections, raw sources, model
tables, refresh runs, raw SQL, mutation operations, or grant administration.
Use the generated [agent tool reference](/docs/agent-tools) for exact schemas,
privileges, defaults, and result contracts.

## Lifecycle and security

1. The caller authenticates to the target and receives an ordinary LeapView
   principal and agent scope.
2. The provider resolves the requested catalog or documentation operation and
   applies the caller's effective privileges.
3. Governed query tools resolve the active serving state, semantic model,
   filters, limits, and data policies before execution.
4. The provider returns a bounded typed result or a structured error without
   exposing inaccessible-resource distinctions.

Agent access requires `USE_AGENT`; catalog reads and governed queries continue
to require their own item and data privileges. Built-in chat and MCP do not
receive source credentials, deployment mutation authority, raw SQL access, or
permission to widen a principal's visibility.
