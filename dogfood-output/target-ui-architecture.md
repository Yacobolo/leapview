# LibreDash Target UI Architecture

## Decision

Adopt the stream-first MPA defined by [`ui-spec.md`](../ui-spec.md). This document is subordinate to that north star: Go and gomponents render only the structural document and mount the Lit route root selected by the real server route. `/updates` hydrates generated signal contracts into Datastar, `DatastarLit` bridges those signals into the route root, and Lit owns the complete product UI composition beneath that root.

The release pass shows that the current problem is not the number of components. It is that effective user access, route identity, shell geometry, responsive behavior, and component state are not governed by clear ownership boundaries. The target architecture keeps route identity and authorization on the server, server/read-model state in Datastar signals, and layout plus local UI state in Lit.

## 1. Canonical route tree

```text
/
└── redirect -> /dashboards

/dashboards                                  global dashboard catalog
/chats                                       global conversation history
/chats/new                                   global new conversation
/chats/{conversation}                        user-scoped global conversation
/workspaces                                  global workspace catalog
/workspaces/{workspace}
├── assets                                   workspace asset catalog
│   └── {assetType}:{assetKey}/{section}     details | versions | lineage
├── dashboards/{dashboard}
│   └── pages/{page}                         canonical report route
└── data/{objectKind}/{objectKey}             workspace-scoped data preview

/connections                                 global connections and sources
/connections/{connection}/{section}
/admin/{section}                             general | principals | groups | agent | storage | queries
```

Rules:

- Resource routes that operate on one workspace contain `{workspace}` in the path. Scope is never inferred from a previous page or browser-only state.
- Chat is intentionally global and scoped to the authenticated user. Conversations, runs, and messages have no workspace ownership boundary. On every turn and tool call, the server derives effective access from the principal and current grants. A tool argument may name a workspace as a resource selector, but it is never trusted as authorization scope: the server re-authorizes that workspace before reading or executing anything.
- After the first accepted turn, the server returns the canonical `/chats/{conversation}` location and the browser transitions immediately.
- Collection and detail routes use stable nouns. Legacy singular routes such as `/chat` become redirects, not parallel implementations.
- `/workspaces/{workspace}/dashboards/{dashboard}` redirects to its configured first page. Page keys, not ordinal positions, are canonical.
- Query parameters represent view state only: search, sort, filter, zoom mode. They never supply missing authorization or domain scope.

## 2. Document, hydration, and access contracts

Every page handler resolves a small `DocumentRouteConfig`, renders the structural document, and mounts exactly one Lit route root. It must not serialize the page read model into HTML.

```text
DocumentRouteConfig (primitive static HTML configuration only)
├── routeId and canonical URL
├── route-root tag and route bundle
├── primitive ids, active section, labels, and booleans
├── literal /updates init metadata
└── CSRF and other document security metadata

/updates hydration (generated signal contracts)
├── $chrome
├── $page
├── route-owned domain roots
└── $status

Agent access context (server-only, recomputed for every tool call)
├── principal
├── effective grants and data policies
├── authorized selected resource/workspace
└── audit context
```

Go signal structs remain the source of truth; JSON Schema and TypeScript types are generated for Lit. The `/updates` URL contains enough primitive route metadata to reconstruct the first hydration patch without pre-existing signals. It sends signal patches only—never element morphs, scripts, or mixed transports.

Chat handlers inject the authenticated principal into the run context. Global discovery tools return only resources visible to that principal. Workspace-specific tools accept an explicit resource selector, resolve it server-side, and authorize it against the principal before dispatch. This keeps the conversation global without making tool execution implicit or unauditable.

## 3. Lit route roots

Each real server route mounts one Lit composition boundary. Route roots read their generated signal roots through `DatastarLit`, derive properties for child components, and translate child domain events into Datastar command signals.

| Page family | Lit route root | Representative child components |
|-------------|----------------|---------------------------------|
| Catalog | `ld-dashboard-catalog-page` | page header, search, dashboard cards, empty/error state |
| Chat history | `ld-chat-history-page` | history table/list, search, conversation actions |
| Chat conversation | `ld-chat-page` | composer, messages, tool results, streaming/error status |
| Workspace/asset | `ld-workspace-page`, `ld-asset-page` | breadcrumbs, tabs, asset table, lineage view, access dialog |
| Data explorer | `ld-data-explorer-page` | object tree, preview toolbar, SQL panel, virtualized grid |
| Connections | `ld-connections-page` | filters, source table, connection detail |
| Admin | `ld-admin-page` | section navigation, forms, tables, schema/tool inspector |
| Report | `ld-dashboard-page` | report shell, page rail, filters, KPIs, charts, report tables |

Route roots remain intentionally coarse. They must not become client routers, fetch backend state, own authorization, or call `/updates` themselves. Child components receive ordinary Lit properties and emit typed domain events back to the owning route root.

## 4. Component ownership

```text
Gomponents (structural document only)
├── PageDocument
├── common and route-bundle asset tags
├── literal /updates data-init
├── CSRF/security metadata
├── declarative Datastar event-to-command attributes on the route-root host
└── <ld-app-shell><ld-route-root slot="page"></ld-route-root></ld-app-shell> mount

Lit application shell and shared UI
├── ld-app-shell
├── ld-page-header, ld-breadcrumbs, ld-tabs
├── ld-drawer, ld-dialog, ld-icon-button, ld-search-field
├── ld-data-table, ld-empty-state, ld-error-state, ld-loading-state
├── ld-report-shell, ld-page-rail, ld-filter-rail, ld-zoom-toolbar
├── ld-chat-message
├── ld-data-grid
├── ld-report-filter
├── ld-kpi
├── ld-chart
└── ld-report-table
```

Gomponents must not render product landmarks, cards, navigation internals, forms, dialogs, tables, empty states, or report controls. It may choose and mount the correct route-root tag, set primitive static host attributes, and attach Datastar expressions that translate route-root domain events into CQRS commands. Those expressions contain no visual markup and no read-model payload.

The current renderer audit confirms the desired structural shape in `internal/ui/*.go` and `internal/dashboard/ui/page.go`: the mounted custom elements are application shells, route roots, and the development inspector. Product internals such as record tables, chat transcript/composer, lineage graphs, filters, KPIs, charts, and report tables are Lit-owned. Migration work should simplify and share the document builders; it must not reintroduce those internals as Go components.

Each Lit component has one accessibility contract. In particular, `ld-drawer` owns focus trapping, Escape, backdrop behavior, one close control, and focus restoration. `ld-icon-button` requires a unique accessible name. Table and chart components receive typed values plus formatting metadata so visual clipping can never change the semantic value.

Lit components use Shadow DOM CSS consuming Primer and LibreDash variables. Tailwind is limited to the outer document shell; product UI must not depend on global selectors or Tailwind utilities injected into Shadow DOM.

## 5. Report layout contract

Replace uniform canvas scaling with explicit presentation modes:

- `responsive`: default below 768px. Filters become a sheet, KPIs and visuals stack in reading order, charts have a minimum readable height, and tables use a dedicated horizontally scrollable frame.
- `fit-width`: default desktop mode. The authored grid fits the available width and the report surface scrolls vertically.
- `fit-page`: optional overview mode. It fits both width and height and is only offered when the resulting body text remains at least 12 CSS pixels.
- `actual-size`: 100% authored size with two-dimensional pan/scroll when required.

One Lit `ReportViewportController`, owned by `ld-dashboard-page`/`ld-report-shell`, owns available shell dimensions, zoom mode, scale, minimum readable scale, and the scroll container. It passes final rectangles to visual children through Lit properties; children do not independently infer width from nested containers.

Required invariants:

- KPI values and units never clip; cards either grow, reflow, or use a documented compact formatter.
- Every authored visual is reachable without browser-level zoom.
- Page and primary navigation consume space exactly once in viewport calculations.
- A route resize changes presentation mode without losing filters, selection, or focus.

## 6. Datastar and Lit state boundary

- Datastar stores all server/read-model state: chrome, route page model, permissions/capabilities, visible workspace summaries, query lifecycle, filters, selected page, chat lifecycle, results, and errors.
- Lit route roots consume Datastar through `DatastarLit`. They never fetch backend state or mirror the read model into a hidden component store.
- Lit owns genuinely local presentation state: drawer visibility, focus, transient expanded panels, local table column widths, and viewport/zoom interaction unless a value is deliberately part of the shareable server read model.
- Child components emit typed domain events. The owning route root maps those events to the smallest command signal; Datastar posts it to the CQRS endpoint, and the command publishes signal patches to the existing stream.
- Initial route roots render stable loading/empty UI until the first `/updates` hydration patch. Initial HTML contains no duplicated read model, large JSON attributes, or MPA `data-signals` payloads.
- Loading, success, empty, and actionable error are explicit signal states. A completed chat command cannot silently return the composer to its initial state.
- Signal roots are stable and route-owned. Dashboard roots remain renderer-neutral: `$chrome`, `$page`, `$filters`, `$filterOptions`, `$visuals`, `$tables`, `$status`.

## 7. Release and migration sequence (TDD)

1. **Characterization tests (red):** encode the eight report findings as route, browser, accessibility, and screenshot tests at 390×844, 1280×720, and 1440×900.
2. **Route and access contract (green):** introduce named server routes, canonical redirects, global user-scoped chat, principal-filtered discovery, and per-tool authorization of explicit workspace resource selectors. Keep legacy routes as tested redirects.
3. **Structural document (green):** reduce gomponents to assets, security metadata, literal `/updates` init, `ld-app-shell`, and the server-selected route-root host.
4. **Lit shell and primitives (green):** consolidate desktop/mobile navigation, drawer, breadcrumbs, tabs, fields, dialogs, loading/error states, and shared table structure as Lit components with Shadow DOM accessibility tests.
5. **Route-root migration:** consolidate each page family behind its Lit route root and `DatastarLit` bridge; remove product UI internals from gomponents as each route moves.
6. **Report viewport (green):** implement `responsive`, `fit-width`, `fit-page`, and `actual-size` in the Lit report composition with one scroll owner and minimum readable scale.
7. **Refactor:** remove duplicate components, legacy signal fields, global product CSS selectors, and redirects only after route telemetry and tests show no remaining consumers.

## 8. Release gates

- All canonical and legacy redirect routes have handler tests and browser deep-link tests.
- Gomponents output contains only the structural document, primitive route configuration, assets, security metadata, and route-root hosts—no product UI internals or serialized read model.
- `/updates` emits signal patches only, and Lit has no backend fetch paths.
- Every route mounts exactly one `DatastarLit`-connected Lit route root beneath `ld-app-shell`.
- Zero unnamed inputs, duplicate modal close controls, nested `main` landmarks, or disabled navigation toggles without explanation.
- At 390×844, every report value and control is readable/tappable without browser zoom.
- At desktop widths, no value/unit clipping and every visual is reachable in default mode.
- No silent command completion: every command resolves to success, empty, or actionable error UI.
- Visual regression coverage for all page families plus representative chart, table, filter, drawer, and dialog states.
- `task ci` passes after migration slices; generated signal artifacts remain current.

## 9. Implementation status (2026-07-18)

The release-readiness slice is implemented:

- Canonical global chat routes are `/chats`, `/chats/new`, and `/chats/{conversation}`; legacy page routes redirect permanently while the obsolete `/chat/updates` transport remains unavailable.
- Conversation persistence is principal-scoped rather than workspace-scoped. The authenticated principal is propagated into background agent runs, global workspace discovery is filtered by effective access, and generated workspace-bound tools require an explicit workspace selector that is authorized again at dispatch.
- The first chat turn transitions to its canonical conversation route even if the navigation patch and active-conversation signal arrive in different orders.
- Dashboard presentation now has responsive, fit-width, fit-page, actual-size, and custom zoom behavior with one report scroll owner and derived authored content bounds.
- Asset detail scrolling, KPI sizing, mobile navigation semantics, and catalog pluralization are covered by regression tests.
- The gomponents audit found route hosts and structural document concerns only; the changed product UI remains implemented in Lit components under `web/components/`.

The broader component-consolidation sequence remains a target rather than a prerequisite for this release. Future work should continue enforcing the structural gomponents boundary with architecture tests while consolidating Lit primitives and route roots incrementally.
