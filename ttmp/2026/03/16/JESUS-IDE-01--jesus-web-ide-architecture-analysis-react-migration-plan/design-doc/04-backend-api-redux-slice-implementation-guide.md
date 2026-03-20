---
Title: Backend API & Redux Slice Implementation Guide
Ticket: JESUS-IDE-01
Status: active
Topics:
    - javascript
    - architecture
    - review
    - refactor
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: jesus/pkg/api/execute.go
      Note: Current /v1/execute contract and request parsing to replace with /api/v1 JSON
    - Path: jesus/pkg/engine/dispatcher.go
      Note: Execution pipeline and persistence handoff that the new execute endpoint continues to use
    - Path: jesus/pkg/repository/sqlite.go
      Note: Execution storage schema and filtering behavior for execution list/detail endpoints
    - Path: jesus/pkg/web/admin/globalstate.go
      Note: Existing globalState HTML/JSON mixed handler that the new state API replaces
    - Path: jesus/pkg/web/admin/logs.go
      Note: Current admin logs API surface to normalize under /api/v1/logs
    - Path: jesus/pkg/web/admin/sse.go
      Note: Current SSE transport used as the phase-one event channel baseline
    - Path: jesus/pkg/web/static/js/app.js
      Note: Current monolithic frontend state ownership used to derive the Redux slice split
ExternalSources: []
Summary: Backend-focused implementation guide for the Jesus IDE rewrite, covering /api/v1 contracts, transport choices, and frontend store boundaries.
LastUpdated: 2026-03-16T09:40:20.463555591-04:00
WhatFor: ""
WhenToUse: ""
---


# Backend API & Redux Slice Implementation Guide

This document is the backend-oriented companion to the existing JESUS-IDE-01
frontend planning docs. It translates the current `jesus` codebase into a
concrete implementation plan for two things:

1. A normalized JSON API under `/api/v1/` that the React frontend can target.
2. Redux slice boundaries that keep client-owned UI workflow state separate
   from server-owned data.

The intent is not to redesign the runtime. The Goja engine, dispatcher, request
logger, and execution repository are already good enough to support the next
frontend. The work is mostly contract cleanup, transport cleanup, and state
ownership cleanup.

## 1. Scope

This guide covers:

- HTTP and event contracts for the React rewrite.
- How existing handlers map into a cleaner backend package layout.
- Which data belongs in RTK Query cache versus classic Redux slices.
- A phased implementation order that preserves backward compatibility while the
  current templ/vanilla frontend still exists.

This guide does not cover:

- The System 7 visual design.
- Detailed component styling.
- Storybook organization.

For those topics, see the other JESUS-IDE-01 design docs.

## 2. Current Backend Surface and Why It Must Change

The current backend works, but it exposes UI-era decisions directly to the
client:

| Current surface | Current behavior | Problem for the React rewrite |
| --- | --- | --- |
| `POST /v1/execute` | Accepts raw `text/plain` JavaScript and returns ad-hoc JSON | Hard to evolve; no typed options; inconsistent error envelope |
| `POST /api/repl/execute` | Delegates to the same execute handler | Duplicated surface without distinct semantics |
| `POST /api/reset-vm` | Returns success with `"not implemented"` | UI cannot rely on reset behavior |
| `GET /api/docs?action=*` | Most actions return `501 Not Implemented` | Presets/docs widgets cannot use stable data |
| `GET/POST /admin/globalstate` | Content-negotiated HTML or JSON; updates use form posts | State editor cannot use a simple JSON mutation contract |
| `/admin/logs/api/*` | Separate admin namespace and shapes | Makes the admin dashboard a special-case frontend |
| `/admin/logs/events` | SSE endpoint tied to the admin page | Useful transport, but not exposed as a general app event channel |

The core internals are already usable:

- The dispatcher stores code executions in SQLite via
  `repository.CreateExecutionRequest`.
- Request logs exist in memory and are queryable.
- `GetGlobalState()` and `SetGlobalState()` already provide a backend state
  seam.
- The engine already knows the registered handlers map.

The rewrite should preserve those internals and replace the externally visible
contract.

## 3. Design Rules

The new API should follow these rules consistently:

1. All non-streaming endpoints return JSON.
2. All JSON endpoints use the same response envelope.
3. Server state is queried via RTK Query, not copied into ad-hoc component
   state.
4. Redux slices only own client workflow state.
5. Legacy endpoints stay alive as compatibility adapters until the React app is
   the default UI.
6. Event transport is abstracted so SSE can ship first and WebSocket can follow
   later without changing the store contract.

## 4. Target Backend Layout

The backend should be reorganized around API responsibilities instead of page
types.

```text
jesus/
├── pkg/
│   ├── api/
│   │   ├── v1/
│   │   │   ├── router.go
│   │   │   ├── response.go
│   │   │   ├── execute.go
│   │   │   ├── executions.go
│   │   │   ├── logs.go
│   │   │   ├── state.go
│   │   │   ├── runtime.go
│   │   │   ├── presets.go
│   │   │   ├── docs.go
│   │   │   └── events.go
│   │   └── compat/
│   │       ├── execute_legacy.go
│   │       ├── docs_legacy.go
│   │       └── admin_legacy.go
│   ├── services/
│   │   ├── executions.go
│   │   ├── logs.go
│   │   ├── runtime.go
│   │   └── docs.go
│   ├── engine/
│   ├── repository/
│   └── web/
│       ├── spa.go
│       └── static.go
```

Responsibilities:

- `pkg/api/v1`: request parsing, response envelopes, HTTP status codes.
- `pkg/services`: use-case logic composed from `engine` and `repository`.
- `pkg/engine`: unchanged execution core and runtime bindings.
- `pkg/repository`: persistence and query layer.
- `pkg/web`: page/static serving only.

This keeps the React app from depending on `pkg/web` page-era handler structure.

## 5. Common Response Contracts

Every JSON endpoint should return this envelope:

```json
{
  "ok": true,
  "data": {},
  "error": null,
  "meta": {
    "requestId": "6e807dc6-0f70-4021-bf61-68d6b0d65832",
    "timestamp": "2026-03-16T14:11:52Z",
    "durationMs": 12
  }
}
```

Error shape:

```json
{
  "ok": false,
  "data": null,
  "error": {
    "code": "INVALID_REQUEST",
    "message": "code must not be empty",
    "details": {
      "field": "code"
    }
  },
  "meta": {
    "requestId": "6e807dc6-0f70-4021-bf61-68d6b0d65832",
    "timestamp": "2026-03-16T14:11:52Z",
    "durationMs": 1
  }
}
```

### 5.1 Backend helper types

```go
type ResponseMeta struct {
    RequestID  string `json:"requestId"`
    Timestamp  string `json:"timestamp"`
    DurationMs int64  `json:"durationMs"`
}

type APIError struct {
    Code    string      `json:"code"`
    Message string      `json:"message"`
    Details interface{} `json:"details,omitempty"`
}

type APIResponse[T any] struct {
    OK    bool         `json:"ok"`
    Data  *T           `json:"data"`
    Error *APIError    `json:"error"`
    Meta  ResponseMeta `json:"meta"`
}
```

### 5.2 Conventions

- `timestamp` is always RFC3339.
- `requestId` is generated per HTTP request, not reused from execution session.
- Domain objects keep their own identifiers such as `sessionId` or execution
  `id`.
- Pagination metadata lives inside `data`, not `meta`, because it belongs to
  the resource collection.

## 6. Resource Model

The frontend rewrite needs a stable resource vocabulary. These are the objects
the backend should expose directly.

### 6.1 Execution

Represents one persisted code execution record already stored in
`script_executions`.

```json
{
  "id": 41,
  "sessionId": "7f50ed37-bcc3-4e3a-bde5-6cb75f45b72b",
  "code": "console.log('hi')",
  "result": {
    "raw": 42
  },
  "consoleLog": ["hi"],
  "error": null,
  "source": "api",
  "createdAt": "2026-03-16T14:11:52Z"
}
```

Notes:

- `id` can remain integer-backed because the repository already uses SQLite
  auto-increment IDs.
- `sessionId` remains the public correlation ID for notebook cells and event
  streams.
- `result` should be emitted as decoded JSON, not a JSON-encoded string.
- `consoleLog` should be emitted as `[]string`, not a newline-joined blob.

### 6.2 Request Log

Represents one in-memory HTTP request log captured by `RequestLogger`.

```json
{
  "id": "req_abc123",
  "method": "GET",
  "path": "/hello",
  "url": "/hello?name=manuel",
  "status": 200,
  "startTime": "2026-03-16T14:11:52Z",
  "endTime": "2026-03-16T14:11:52Z",
  "durationMs": 4,
  "headers": {},
  "query": {
    "name": "manuel"
  },
  "body": "",
  "response": "{\"ok\":true}",
  "logs": [],
  "databaseOps": [],
  "error": "",
  "remoteIp": "127.0.0.1:53012"
}
```

### 6.3 Global State

Represents the runtime `globalState` JS object.

```json
{
  "value": {
    "counter": 3,
    "currentUser": "manuel"
  }
}
```

### 6.4 Runtime Handler

Represents one registered JS HTTP handler.

```json
{
  "path": "/hello",
  "method": "GET",
  "contentType": "application/json",
  "options": {}
}
```

### 6.5 Preset / Doc

These should become first-class resources instead of `action=` query branches.

```json
{
  "id": "route-hello-world",
  "title": "Hello World Route",
  "summary": "Registers a GET handler that returns JSON",
  "category": "routing",
  "code": "app.get('/hello', (req, res) => res.json({ ok: true }))"
}
```

## 7. Endpoint-by-Endpoint API Contract

### 7.1 Execute Code

`POST /api/v1/execute`

Purpose:

- Execute code immediately.
- Optionally persist the execution.
- Return the immediate result, console output, and session correlation ID.

Request:

```json
{
  "code": "app.get('/hello', (req, res) => res.json({ ok: true }));",
  "options": {
    "store": true,
    "timeoutMs": 30000,
    "source": "playground"
  }
}
```

Response `data`:

```json
{
  "sessionId": "f31cb55f-4c77-47dd-a347-197bb24292f5",
  "stored": true,
  "result": null,
  "consoleLog": [],
  "error": null,
  "handlerDelta": {
    "registered": [
      { "method": "GET", "path": "/hello" }
    ]
  }
}
```

Implementation notes:

- Reuse the existing dispatcher path.
- Add `Store bool`, `Timeout time.Duration`, and `Source string` to `EvalJob`.
- Only persist into `script_executions` when `options.store` is true.
- Decode the persisted `result` and `console_log` columns on read so the API
  shape stays typed.
- `handlerDelta` is optional in phase 1. It is useful for the notebook because
  route registration is a primary user action.

Compatibility:

- Keep `POST /v1/execute` alive as a shim that translates raw text into the new
  request body and unwraps the old response shape until the legacy frontend is
  removed.

### 7.2 List Executions

`GET /api/v1/executions?search=&sessionId=&source=&limit=20&offset=0`

Response `data`:

```json
{
  "items": [],
  "total": 0,
  "limit": 20,
  "offset": 0,
  "nextOffset": null
}
```

Implementation notes:

- Reuse `repository.ExecutionFilter` and `PaginationOptions`.
- Convert DB rows into typed API DTOs.
- Default `limit` to 20 and cap it at 100.

### 7.3 Get One Execution

`GET /api/v1/executions/:id`

Returns one execution resource.

### 7.4 Delete One Execution

`DELETE /api/v1/executions/:id`

Response `data`:

```json
{
  "deleted": true,
  "id": 41
}
```

### 7.5 Request Logs

`GET /api/v1/logs?limit=50`

`GET /api/v1/logs/:id`

`GET /api/v1/logs/stats`

`DELETE /api/v1/logs`

Implementation notes:

- Phase 1 can continue using the in-memory `RequestLogger`.
- The stats endpoint should expose a stable object:

```json
{
  "totalRequests": 12,
  "maxLogs": 100,
  "statusCounts": { "200": 10, "500": 2 },
  "methodCounts": { "GET": 9, "POST": 3 },
  "avgDurationMs": 7
}
```

- `DELETE /api/v1/logs` replaces `/admin/logs/api/clear`.

### 7.6 Global State

`GET /api/v1/state`

`PUT /api/v1/state`

`PATCH /api/v1/state`

Recommended semantics:

- `GET` returns the full parsed `globalState` object.
- `PUT` replaces the full object.
- `PATCH` applies JSON Merge Patch semantics.

Request examples:

```json
{
  "value": {
    "counter": 10
  }
}
```

```json
{
  "patch": {
    "counter": 11
  }
}
```

Implementation notes:

- `GET` wraps `GetGlobalState()`.
- `PUT` validates JSON and calls `SetGlobalState()`.
- `PATCH` should be implemented server-side by reading the current object,
  applying merge-patch, then calling `SetGlobalState()` with the merged JSON.
- Do not keep the current form-post contract in the new API.

### 7.7 Runtime Introspection

`GET /api/v1/runtime/handlers`

`POST /api/v1/runtime/reset`

Recommended reset semantics:

1. Recreate the engine runtime.
2. Clear registered handlers and files.
3. Reset `globalState`.
4. Optionally re-run bootstrap/startup scripts when the server was started with
   them.

The current `"not implemented"` response is not sufficient for the new UI.

### 7.8 Presets

`GET /api/v1/presets`

`GET /api/v1/presets/:id`

Implementation notes:

- Stop exposing presets via `/api/preset?id=...`.
- The current docs API is not implemented, so phase 1 should use an explicit
  preset registry instead of pretending the docs extraction exists.
- Presets should be lightweight read-only resources with stable IDs.

### 7.9 Docs

`GET /api/v1/docs`

`GET /api/v1/docs/:slug`

Implementation notes:

- Return structured docs metadata first.
- Rendered HTML or markdown body can be added once the doc package wiring is
  restored.
- The important change is that the route no longer depends on an `action`
  query parameter.

## 8. Event Transport Plan

The frontend store should not care whether the live transport is SSE or
WebSocket. It should care about typed runtime events.

### 8.1 Event schema

```json
{
  "type": "execution.completed",
  "payload": {
    "sessionId": "f31cb55f-4c77-47dd-a347-197bb24292f5",
    "status": "complete"
  }
}
```

Recommended event types:

- `execution.started`
- `execution.completed`
- `execution.failed`
- `request.logged`
- `logs.cleared`
- `state.updated`
- `runtime.reset`
- `handlers.updated`

### 8.2 Staged rollout

Phase 1:

- Ship `GET /api/v1/events` using SSE.
- Adapt the current `SSEHandler` into the new route.
- Broadcast typed events instead of the current count-only payloads.

Phase 2:

- Add `GET /api/v1/ws`.
- Reuse the same event payload shapes.
- Move client-originating commands like cancel/subscribe to WebSocket only if
  needed.

### 8.3 Backend abstraction

Create a transport-agnostic broadcaster interface:

```go
type RuntimeEvent struct {
    Type    string      `json:"type"`
    Payload interface{} `json:"payload"`
}

type EventBroadcaster interface {
    Publish(event RuntimeEvent)
}
```

Then let both SSE and future WebSocket transports subscribe to the same event
bus.

## 9. Redux Ownership Model

This is the most important frontend/backend contract decision.

### 9.1 Data that belongs in RTK Query

Anything fetched from the backend and invalidated by backend changes belongs in
RTK Query cache:

- executions list and execution detail
- request logs list and detail
- request log stats
- global state snapshot
- runtime handler registry
- presets list/detail
- docs list/detail

These are server resources. They should not be duplicated into normal slices
unless there is a specific offline or optimistic-edit reason.

### 9.2 Data that belongs in classic Redux slices

Anything owned by the client workflow belongs in slices:

- notebook cells and their local ordering
- active cell / selection / focus
- dirty flags
- editor preferences like Vim mode and font size
- REPL local history
- open panels, modals, toasts, and transient filters
- transport connection status

### 9.3 Anti-pattern to avoid

Do not do this:

- fetch executions into RTK Query
- copy them into `historySlice`
- mutate the copied data separately

That creates two sources of truth. The React rewrite should use:

- RTK Query for server entities
- slices for view state around those entities

## 10. Recommended Redux Slice Set

The frontend docs already sketch multiple slices. From the backend contract
perspective, this is the clean split.

### 10.1 `apiSlice`

This is the RTK Query root slice, not a classic reducer.

Tag types:

- `Execution`
- `ExecutionList`
- `RequestLog`
- `RequestLogList`
- `RequestStats`
- `GlobalState`
- `RuntimeHandlers`
- `Preset`
- `PresetList`
- `Doc`
- `DocList`

Mutation invalidation rules:

- `execute` invalidates `ExecutionList`, `RuntimeHandlers`, `RequestLogList`,
  and `RequestStats`
- `deleteExecution` invalidates `Execution` and `ExecutionList`
- `clearLogs` invalidates `RequestLogList` and `RequestStats`
- `putState` and `patchState` invalidate `GlobalState`
- `resetRuntime` invalidates `GlobalState`, `RuntimeHandlers`, `RequestLogList`,
  and `RequestStats`

### 10.2 `notebookSlice`

Owns notebook workflow state for the new primary UI.

Recommended state:

```ts
type CellKind = 'code' | 'markdown';
type CellStatus = 'idle' | 'dirty' | 'running' | 'complete' | 'error';

interface NotebookCell {
  id: string;
  kind: CellKind;
  source: string;
}

interface CellRuntime {
  status: CellStatus;
  lastSessionId?: string;
  lastExecutionId?: number;
  lastResultPreview?: unknown;
  lastConsoleLog?: string[];
  lastError?: string;
  durationMs?: number;
  executionCount?: number;
}

interface NotebookState {
  title: string;
  cells: NotebookCell[];
  runtimes: Record<string, CellRuntime>;
  activeCellId: string | null;
}
```

Why this belongs in a slice:

- Cells exist before they are persisted.
- Ordering and focus are purely local UI concerns.
- Runtime previews are attached to cell UX, even when the authoritative
  execution record lives in RTK Query.

Backend contract dependency:

- `lastExecutionId` and `lastSessionId` let the cell link to persisted
  `executions/:id` data without owning the full resource copy.

### 10.3 `replSlice`

Owns ephemeral REPL workflow state.

Recommended state:

```ts
interface ReplEntry {
  id: string;
  kind: 'input' | 'log' | 'result' | 'error';
  text: string;
  sessionId?: string;
}

interface ReplState {
  draft: string;
  entries: ReplEntry[];
  history: string[];
  historyIndex: number;
}
```

Reasoning:

- REPL input history is not a server resource.
- The current `JSPlaygroundApp` already manages this entirely client-side.
- The slice replaces `replHistory` and `replHistoryIndex` from `app.js`.

### 10.4 `uiSlice`

Owns application chrome and transient feedback.

Recommended state:

- current route or active workspace tab if React Router state needs store
  integration
- toasts
- modal visibility
- selected admin panels
- filter drafts not yet committed to the URL

### 10.5 `settingsSlice`

Owns durable client preferences.

Recommended state:

- `vimMode`
- `fontSize`
- `theme`
- optional notebook behavior such as `runOnShiftEnter`

Reasoning:

- These values map directly from today’s `localStorage` usage in `app.js`.
- They should persist locally without being server resources.

### 10.6 `connectionSlice`

Owns transport status only.

Recommended state:

```ts
interface ConnectionState {
  status: 'disconnected' | 'connecting' | 'connected' | 'error';
  transport: 'sse' | 'ws' | null;
  lastEventAt?: string;
  reconnectAttempt: number;
}
```

Reasoning:

- Connection health is UI state.
- Incoming events should trigger RTK Query invalidation or notebook/repl
  actions, but the connection object itself is not a server resource.

## 11. Mapping Current Vanilla State to the New Store

The current `JSPlaygroundApp` tells us what state already exists, even if it is
scattered.

| Current source | Current field or behavior | New owner |
| --- | --- | --- |
| `app.js` | `editor` content | `notebookSlice` or `replSlice` depending on screen |
| `app.js` | `replHistory`, `replHistoryIndex` | `replSlice` |
| `app.js` | `vimMode` | `settingsSlice` |
| `app.js` | `fontSize` | `settingsSlice` |
| `app.js` | result / console / session info area | `notebookSlice.runtimes` plus `apiSlice.execute` mutation result |
| `app.js` | toasts | `uiSlice` |
| `localStorage` | cross-page handoff to `/playground` and `/repl` | removed; replaced by slice actions and router navigation |
| `/admin/logs` polling/SSE page state | ad-hoc admin JS | `apiSlice` + `connectionSlice` + `uiSlice` |

This table is the migration bridge. If a piece of current state does not appear
in the new store design, it will get reintroduced later as component-local
state and the architecture will drift again.

## 12. RTK Query Endpoint Plan

The following API definition is the right shape for the first React pass.

```ts
export const jesusApi = createApi({
  reducerPath: 'jesusApi',
  baseQuery: fetchBaseQuery({ baseUrl: '/api/v1' }),
  tagTypes: [
    'Execution',
    'ExecutionList',
    'RequestLog',
    'RequestLogList',
    'RequestStats',
    'GlobalState',
    'RuntimeHandlers',
    'Preset',
    'PresetList',
    'Doc',
    'DocList',
  ],
  endpoints: (builder) => ({
    execute: builder.mutation<ExecuteResponse, ExecuteRequest>({
      query: (body) => ({
        url: '/execute',
        method: 'POST',
        body,
      }),
      invalidatesTags: [
        'ExecutionList',
        'RuntimeHandlers',
        'RequestLogList',
        'RequestStats',
      ],
    }),

    listExecutions: builder.query<ExecutionListResponse, ListExecutionsParams>({
      query: (params) => ({ url: '/executions', params }),
      providesTags: ['ExecutionList'],
    }),

    getExecution: builder.query<ExecutionResponse, number>({
      query: (id) => `/executions/${id}`,
      providesTags: (_result, _error, id) => [{ type: 'Execution', id }],
    }),

    listLogs: builder.query<RequestLogListResponse, { limit?: number }>({
      query: (params) => ({ url: '/logs', params }),
      providesTags: ['RequestLogList'],
    }),

    getLogStats: builder.query<RequestStatsResponse, void>({
      query: () => '/logs/stats',
      providesTags: ['RequestStats'],
    }),

    getState: builder.query<GlobalStateResponse, void>({
      query: () => '/state',
      providesTags: ['GlobalState'],
    }),

    patchState: builder.mutation<GlobalStateResponse, PatchStateRequest>({
      query: (body) => ({
        url: '/state',
        method: 'PATCH',
        body,
      }),
      invalidatesTags: ['GlobalState'],
    }),

    listHandlers: builder.query<RuntimeHandlersResponse, void>({
      query: () => '/runtime/handlers',
      providesTags: ['RuntimeHandlers'],
    }),
  }),
});
```

Important rule:

- The notebook screen should call `execute` and store only the returned
  references/previews in `notebookSlice`.
- The history/admin views should query authoritative records from RTK Query.

## 13. Backend Implementation Sequence

This order minimizes breakage and lets the old UI continue working.

### Phase 1: Shared response and request helpers

Deliverables:

- `pkg/api/v1/response.go`
- request ID middleware
- JSON helper functions

Exit criteria:

- one test proving success and error envelopes are stable

### Phase 2: Execution endpoints and compatibility shim

Deliverables:

- `POST /api/v1/execute`
- `GET /api/v1/executions`
- `GET /api/v1/executions/:id`
- `DELETE /api/v1/executions/:id`
- `POST /v1/execute` shim

Exit criteria:

- legacy playground still runs
- React notebook can execute code using JSON request bodies

### Phase 3: Logs and global state

Deliverables:

- `/api/v1/logs`
- `/api/v1/logs/:id`
- `/api/v1/logs/stats`
- `/api/v1/logs` `DELETE`
- `/api/v1/state` `GET/PUT/PATCH`

Exit criteria:

- no React admin screen needs `/admin/logs/api/*` or `/admin/globalstate`

### Phase 4: Runtime introspection and reset

Deliverables:

- `/api/v1/runtime/handlers`
- `/api/v1/runtime/reset`

Exit criteria:

- runtime reset has real semantics, not placeholder success

### Phase 5: Presets, docs, and events

Deliverables:

- `/api/v1/presets`
- `/api/v1/presets/:id`
- `/api/v1/docs`
- `/api/v1/docs/:slug`
- `/api/v1/events`

Exit criteria:

- notebook quick reference and admin dashboard can run entirely from the new
  API

### Phase 6: Remove legacy page-oriented endpoints

Deliverables:

- delete or hard-deprecate:
  - `/api/repl/execute`
  - `/api/reset-vm`
  - `/api/preset`
  - `/api/docs?action=*`
  - `/admin/logs/api/*`
  - `/admin/logs/events`

Exit criteria:

- `pkg/web` no longer contains API business logic

## 14. Testing Strategy

### 14.1 Backend tests

Add `httptest` coverage for:

- execute request validation
- execute success and error envelopes
- execution listing filters and pagination
- log clearing and stats
- global state `PUT` and `PATCH`
- runtime reset behavior

### 14.2 Contract tests

Maintain example responses for:

- `POST /api/v1/execute`
- `GET /api/v1/executions`
- `GET /api/v1/logs/:id`
- `GET /api/v1/state`

These can be snapshot-like JSON fixtures checked in alongside handler tests.

### 14.3 Frontend store tests

The React app should test:

- `notebookSlice` reducers
- `replSlice` history navigation
- `settingsSlice` hydration/persistence
- RTK Query invalidation behavior after execute, clear logs, patch state, and
  reset runtime

The reason to mention this in a backend guide is simple: if the response shapes
move casually, the store tests become the first place the breakage shows up.

## 15. Defaults and Open Decisions

These should be decided early and then treated as contract decisions.

### 15.1 Should execute store by default?

Recommendation:

- notebook UI default: `store = true`
- REPL default: `store = false`

Reason:

- notebook cells are history-bearing artifacts
- REPL input is often disposable

### 15.2 Should runtime reset delete execution history?

Recommendation:

- no

Reason:

- reset should clear in-memory runtime state, not destroy historical records in
  SQLite

### 15.3 Should logs remain in memory?

Recommendation:

- yes for phase 1
- revisit after the React admin dashboard lands

Reason:

- request logs already support the new UI contract well enough
- moving them to SQLite is useful, but not required to unblock the rewrite

### 15.4 Which transport ships first?

Recommendation:

- SSE first, WebSocket later

Reason:

- the codebase already has an SSE path
- store invalidation does not require bidirectional messaging

## 16. Summary Recommendation

The cleanest path is:

1. Keep the engine, dispatcher, and repository layer.
2. Add a versioned `/api/v1/` facade with stable JSON contracts.
3. Keep server resources in RTK Query.
4. Keep notebook/REPL/preferences/connection workflow state in classic slices.
5. Treat SSE and WebSocket as interchangeable transports behind one event
   schema.

If the team holds that line, the React rewrite stays modular. If it does not,
the new frontend will inherit the same coupling that currently lives in
`pkg/web/static/js/app.js`, only with more files.
