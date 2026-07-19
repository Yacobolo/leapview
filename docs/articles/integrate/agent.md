# Agent integrations

LibreDash conversations are global and owned by the authenticated principal. Workspaces are asset containers: a workspace-aware tool requires an explicit `workspace` argument, then enforces the bearer credential's workspace restriction, the principal's privileges, data policies, and the governed query layer.

## Configure the built-in model provider

The built-in chat surface uses an OpenAI-compatible provider configuration:

```sh
LIBREDASH_AGENT_BASE_URL=https://api.openai.com/v1
LIBREDASH_AGENT_MODEL=<model-id>
LIBREDASH_AGENT_API_KEY=<secret>
```

Store the API key in the deployment secret manager. The global administrator-controlled system prompt is configured in the agent administration page. Provider prompts and responses may contain business context; review the provider's data handling, retention, regional, and contractual requirements before enabling it.

The MCP endpoint does not depend on this provider configuration. External MCP hosts can use LibreDash tools when the built-in model is disabled.

## Ask through the CLI

```sh
libredash agent ask \
  --target "$LIBREDASH_TARGET" \
  --token "$LIBREDASH_API_TOKEN" \
  "Which categories contributed most to revenue in the sales workspace?"
```

Use `--conversation <id>` to continue an existing principal-owned conversation and `--json` for machine processing. List conversations with bounded pagination through `libredash agent conversations`. The CLI follows the asynchronous run to a terminal state.

## Integrate through REST

The generated [Agent API](/docs/api/agent) is rooted at `/api/v1/agent` and exposes global conversation creation, update, archive, messages, runs, and run events. The removed `/api/v1/workspaces/{workspace}/agent` routes have no compatibility aliases.

A typical client creates or selects a conversation, starts a run, records its identity, follows the run/event surface to a documented terminal state, renders the assistant message and tool evidence, and archives conversations according to retention policy. List endpoints use opaque pagination tokens.

## Integrate through MCP

Connect a Streamable HTTP MCP client to `/mcp` and send an existing LibreDash API token as a bearer credential. LibreDash implements the 2025-11-25 protocol with stateless JSON responses and exposes tools only—no resources, prompts, nested conversation tools, or stdio transport.

MCP and built-in chat consume the same catalog, schemas, handlers, authorization, projections, audit path, and execution errors. Successful tool calls return both `structuredContent` and equivalent JSON text. MCP access requires `USE_AGENT`; each tool additionally requires its generated resource privilege. Workspace-bound tokens may call tools only for their bound workspace.

Browser sessions are not accepted at `/mcp`. Cross-origin requests are rejected. OAuth-based MCP authorization is outside the current surface.

## Validate answers and operate safely

Natural-language output is not a replacement for governed results. Present tool evidence, resource identity, filters, and relevant time or deployment context so a user can validate claims. Use deterministic semantic or dashboard queries for automated decisions that cannot tolerate interpretive variation.

Test empty results, authorization failures, workspace-bound credentials, ambiguous questions, provider timeouts, cancelled runs, and active deployment changes. Audit conversation and tool activity, apply bounded retention with `libredash admin maintenance`, and never log provider API keys or raw sensitive prompts into general diagnostics.

See [Service principals and API tokens](/docs/security/tokens) and the generated [`agent` CLI reference](/docs/cli/agent).
