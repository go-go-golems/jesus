---
title: "Jesus Web IDE — Review, Improvements & React Migration Plan"
doc-type: design-doc
ticket: JESUS-IDE-01
topics:
  - javascript
  - architecture
  - refactor
  - review
status: active
created: 2026-03-16
---

# Jesus Web IDE — Review, Improvements & React Migration Plan

## 1. Current-State Review

This section is an honest assessment of the existing IDE for someone new to the
codebase.  It identifies what works well, what is fragile, and what will cause the
most pain as the system grows.

### 1.1 What Works Well

- **Single-binary deployment.** The entire IDE, both servers, the JS runtime, and
  SQLite ship as one Go binary.  There is no Node, no npm, no webpack.  This is
  unusually easy to deploy.
- **Dual-server model is sound.** Separating "admin / code-editing" traffic from
  "JS-registered route" traffic means user-facing endpoints can't accidentally serve
  the playground page.
- **Single-threaded dispatcher is safe.** Because all Goja access goes through one
  goroutine, there are no data races.
- **Embedded static assets.** `go:embed` eliminates the "did you copy the static
  files?" class of deploy bugs.
- **Good preset/example system.** The docs API + presets dropdown lets new users start
  immediately.

### 1.2 Architectural Weaknesses

#### 1.2.1 Monolithic Frontend — One Class, One File

All client-side logic lives in a single `JSPlaygroundApp` class in `app.js` (615
lines).  This class handles playground initialization, REPL logic, API calls, toast
notifications, CodeMirror setup, localStorage management, and preset loading.  There
is no separation of concerns.

**Impact:**
- Cannot test playground and REPL logic independently.
- Adding a new feature (e.g., a file manager) means editing the same 615-line file.
- No module system — everything is global, leading to `window.loadToPlayground()`,
  `window.loadDocsExample()`, etc.

#### 1.2.2 Split Frontend Technologies

The main pages use Templ (server-side rendered HTML) while the admin pages are
standalone HTML+JS SPAs.  This means:

- Two different rendering strategies in the same project.
- No shared component model between main and admin.
- Styling is duplicated: `app.css` for main, `logs.css` + `globalstate.css` for
  admin, with overlapping dark-theme styles.

#### 1.2.3 No State Management

State is scattered across:
- CodeMirror instance (`this.editor`)
- DOM elements (`document.getElementById(...)` everywhere)
- `localStorage` (cross-page code passing, preferences)
- In-memory arrays (`replHistory`)
- Server-side (`globalState`)

There is no unified state model.  When you navigate from History → Playground, code
is passed via `localStorage` and a full page reload:

```javascript
window.loadToPlayground = function(code) {
    localStorage.setItem('playgroundCode', code);
    window.location.href = '/playground';  // ← full page reload!
};
```

#### 1.2.4 Styling Is Not Themeable

The CSS uses hard-coded values (`#1e1e1e`, `#0d1117`) rather than CSS custom
properties.  There is no token system.  Changing the theme requires editing many
selectors across three separate CSS files.

#### 1.2.5 No Client-Side Routing

Each view (playground, REPL, history, docs, admin) is a full page load from the
server.  This means:
- Editor state is lost when navigating away.
- REPL history is lost on page change.
- No SPA-style transitions.

#### 1.2.6 API Surface Is Inconsistent

| Endpoint | Content-Type In | Content-Type Out | Notes |
|----------|-----------------|------------------|-------|
| `POST /v1/execute` | `text/plain` | `application/json` | Code as raw body |
| `POST /api/repl/execute` | (delegates to above) | (same) | Redundant endpoint |
| `GET /api/preset?id=X` | — | `application/json` | Single preset |
| `GET /api/docs?action=examples` | — | `application/json` | All presets |
| `GET /admin/globalstate` | — | `text/html` or `application/json` | Content-negotiated |
| `POST /admin/globalstate` | `application/json` | Redirect or JSON | Mixed |

The execute endpoint uses `text/plain` for code submission rather than a JSON envelope.
There is no versioned API prefix for admin endpoints.  Error responses vary in shape.

#### 1.2.7 No WebSocket Support

Real-time updates use SSE (one-directional) with a polling fallback.  There is no
bidirectional communication channel for features like:
- Live code collaboration
- Streaming console output during long-running scripts
- Push-based VM state updates

#### 1.2.8 Console Capture Is Snapshot-Based

Console output is captured all at once after execution completes:

```go
// Current: override console, run code, collect logs, restore console
func executeCodeWithResult(code string) *EvalResult {
    var logs []string
    // override console.log to append to logs
    rt.RunString(code)
    // return logs
}
```

For long-running scripts, the user sees nothing until execution finishes.

---

## 2. Suggested Improvements (Independent of React Migration)

These improvements apply regardless of whether you adopt React:

### 2.1 Backend API Redesign

Normalize all API endpoints under `/api/v1/` with consistent JSON envelopes:

```
POST   /api/v1/execute          — Execute JavaScript code
GET    /api/v1/executions       — List past executions (paginated)
GET    /api/v1/executions/:id   — Get execution by session ID
DELETE /api/v1/executions/:id   — Delete execution

GET    /api/v1/presets          — List all presets
GET    /api/v1/presets/:id      — Get single preset

GET    /api/v1/state            — Get globalState
PUT    /api/v1/state            — Replace globalState
PATCH  /api/v1/state            — Merge into globalState

GET    /api/v1/handlers         — List registered JS handlers
POST   /api/v1/vm/reset        — Reset the JS VM

GET    /api/v1/logs             — Request logs (paginated)
GET    /api/v1/logs/:id         — Single request log detail
GET    /api/v1/logs/stats       — Aggregate statistics
DELETE /api/v1/logs             — Clear logs

GET    /api/v1/docs             — List documentation topics
GET    /api/v1/docs/:slug       — Get single doc (rendered HTML)

WS     /api/v1/ws               — WebSocket for real-time events
```

**Request envelope for execute:**

```json
POST /api/v1/execute
Content-Type: application/json

{
    "code": "app.get('/hello', (req, res) => res.json({ok: true}));",
    "options": {
        "store": true,
        "timeout": 10000
    }
}
```

**Response envelope (all endpoints):**

```json
{
    "ok": true,
    "data": { ... },
    "error": null,
    "meta": {
        "requestId": "uuid",
        "timestamp": "ISO-8601",
        "duration_ms": 42
    }
}
```

### 2.2 WebSocket Channel for Real-Time Updates

Replace SSE with a WebSocket at `/api/v1/ws` that carries:

```
← (server → client)
{ "type": "console",    "sessionId": "...", "level": "log", "message": "hello" }
{ "type": "status",     "sessionId": "...", "state": "running" | "done" | "error" }
{ "type": "request",    "data": { method, path, status, duration } }
{ "type": "stateChange","path": "counter", "value": 42 }

→ (client → server)
{ "type": "execute",    "code": "...", "options": { ... } }
{ "type": "cancel",     "sessionId": "..." }
{ "type": "subscribe",  "channels": ["console", "requests", "state"] }
```

This enables streaming console output for long-running scripts and push-based
globalState updates.

### 2.3 Persistent Request Logs

Move from the in-memory circular buffer to a SQLite table:

```sql
CREATE TABLE request_logs (
    id TEXT PRIMARY KEY,
    method TEXT NOT NULL,
    path TEXT NOT NULL,
    status INTEGER,
    duration_ms INTEGER,
    remote_ip TEXT,
    query_params TEXT,     -- JSON
    headers TEXT,          -- JSON
    body TEXT,
    response TEXT,
    error TEXT,
    console_logs TEXT,     -- JSON array
    database_ops TEXT,     -- JSON array
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_request_logs_created_at ON request_logs(created_at);
CREATE INDEX idx_request_logs_path ON request_logs(path);
```

### 2.4 Execution Cancellation

Add a timeout and cancellation mechanism:

```go
type EvalJob struct {
    // ... existing fields ...
    Timeout time.Duration
    Cancel  chan struct{}   // Close to cancel execution
}
```

And in the dispatcher:

```go
func (e *Engine) processJob(job EvalJob) {
    ctx, cancel := context.WithTimeout(context.Background(), job.Timeout)
    defer cancel()

    // Goja supports runtime interruption:
    go func() {
        <-ctx.Done()
        e.rt.Interrupt("execution cancelled")
    }()
    // ...
}
```

---

## 3. React Migration Plan

### 3.1 Why React + RTK Query

| Concern | Current | After Migration |
|---------|---------|-----------------|
| State management | Scattered (DOM, localStorage, class fields) | Redux Toolkit store |
| API calls | Raw `fetch()` in class methods | RTK Query with caching/invalidation |
| Components | Monolithic HTML strings | Composable React components |
| Routing | Full page reloads | React Router (client-side) |
| Theming | Hard-coded CSS values | CSS custom properties + `data-part` selectors |
| Testing | Untestable DOM manipulation | Component unit tests + store tests |
| Real-time | SSE + polling | RTK Query streaming + WebSocket middleware |

### 3.2 Module Structure

> **See also:** `design/03-notebook-ui-system7-theme.md` for the updated module
> structure, component tree, and CSS token system based on the notebook paradigm and
> System 7 aesthetic drawn from the CozoDB Editor reference implementation.

Following the modular-themable-storybook pattern, the new frontend would live alongside
the Go backend:

```
jesus/
├── cmd/jesus/cmd/serve.go       # Serves SPA in production
├── pkg/
│   ├── api/                     # Go API handlers (JSON only, no HTML)
│   ├── engine/                  # Unchanged
│   └── web/                     # Simplified: serves SPA + /api/* routes
├── frontend/                    # New: React SPA
│   ├── package.json
│   ├── vite.config.ts
│   ├── tsconfig.json
│   ├── index.html
│   ├── src/
│   │   ├── main.tsx
│   │   ├── App.tsx
│   │   ├── store/
│   │   │   ├── index.ts            # configureStore
│   │   │   ├── api.ts              # RTK Query API definition
│   │   │   └── slices/
│   │   │       ├── editorSlice.ts   # Editor state (code, vim mode, font)
│   │   │       ├── replSlice.ts     # REPL history, input
│   │   │       └── uiSlice.ts       # Toasts, active view, preferences
│   │   ├── widgets/
│   │   │   ├── playground/
│   │   │   │   ├── index.ts
│   │   │   │   ├── Playground.tsx
│   │   │   │   ├── types.ts
│   │   │   │   ├── parts.ts
│   │   │   │   ├── components/
│   │   │   │   │   ├── EditorPanel.tsx
│   │   │   │   │   ├── OutputPanel.tsx
│   │   │   │   │   ├── StatusBar.tsx
│   │   │   │   │   ├── QuickReference.tsx
│   │   │   │   │   └── PresetsDropdown.tsx
│   │   │   │   └── styles/
│   │   │   │       ├── playground.css
│   │   │   │       └── theme-default.css
│   │   │   ├── repl/
│   │   │   │   ├── index.ts
│   │   │   │   ├── Repl.tsx
│   │   │   │   ├── types.ts
│   │   │   │   ├── parts.ts
│   │   │   │   ├── components/
│   │   │   │   │   ├── ReplConsole.tsx
│   │   │   │   │   ├── ReplInput.tsx
│   │   │   │   │   └── ExampleButtons.tsx
│   │   │   │   └── styles/
│   │   │   │       ├── repl.css
│   │   │   │       └── theme-default.css
│   │   │   ├── history/
│   │   │   │   ├── index.ts
│   │   │   │   ├── History.tsx
│   │   │   │   ├── types.ts
│   │   │   │   ├── parts.ts
│   │   │   │   ├── components/
│   │   │   │   │   ├── ExecutionCard.tsx
│   │   │   │   │   ├── FilterBar.tsx
│   │   │   │   │   └── Pagination.tsx
│   │   │   │   └── styles/
│   │   │   │       ├── history.css
│   │   │   │       └── theme-default.css
│   │   │   ├── admin/
│   │   │   │   ├── index.ts
│   │   │   │   ├── AdminDashboard.tsx
│   │   │   │   ├── components/
│   │   │   │   │   ├── RequestLogList.tsx
│   │   │   │   │   ├── RequestDetail.tsx
│   │   │   │   │   ├── ExecutionList.tsx
│   │   │   │   │   ├── StatsCards.tsx
│   │   │   │   │   └── GlobalStateEditor.tsx
│   │   │   │   └── styles/
│   │   │   │       ├── admin.css
│   │   │   │       └── theme-default.css
│   │   │   └── shared/
│   │   │       ├── CodeEditor.tsx      # CodeMirror 6 wrapper
│   │   │       ├── JsonEditor.tsx      # JSON editing component
│   │   │       ├── Toast.tsx
│   │   │       ├── Badge.tsx
│   │   │       └── styles/
│   │   │           ├── shared.css
│   │   │           └── theme-default.css
│   │   ├── hooks/
│   │   │   ├── useWebSocket.ts
│   │   │   ├── useLocalStorage.ts
│   │   │   └── useKeyboardShortcuts.ts
│   │   └── styles/
│   │       ├── tokens.css             # Global design tokens
│   │       └── reset.css              # Base resets
│   ├── stories/                        # Storybook stories
│   │   ├── Playground.stories.tsx
│   │   ├── Repl.stories.tsx
│   │   ├── History.stories.tsx
│   │   └── Admin.stories.tsx
│   └── .storybook/
│       └── main.ts
└── Makefile                            # Build targets
```

### 3.3 Theming Architecture — Parts and Tokens

Following the parts-and-tokens pattern, each widget exposes stable `data-*` attribute
hooks for external styling.

#### Global Design Tokens (`frontend/src/styles/tokens.css`)

```css
:root {
    /* Color tokens */
    --color-bg:        #0d1117;
    --color-surface:   #161b22;
    --color-surface-2: #1e1e1e;
    --color-border:    #30363d;
    --color-text:      #f0f6fc;
    --color-text-muted:#8b949e;
    --color-accent:    #58a6ff;
    --color-success:   #3fb950;
    --color-warning:   #d29922;
    --color-danger:    #f85149;
    --color-info:      #58a6ff;

    /* Typography tokens */
    --font-sans:  -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif;
    --font-mono:  'JetBrains Mono', 'Fira Code', 'Monaco', 'Menlo', monospace;
    --font-size:  14px;
    --line-height: 1.5;

    /* Spacing tokens */
    --space-1: 4px;
    --space-2: 8px;
    --space-3: 12px;
    --space-4: 16px;
    --space-5: 24px;
    --space-6: 32px;

    /* Radius tokens */
    --radius-1: 4px;
    --radius-2: 8px;
    --radius-3: 12px;

    /* Shadow tokens */
    --shadow-1: 0 1px 3px rgba(0,0,0,0.3);
    --shadow-2: 0 4px 12px rgba(0,0,0,0.4);
}
```

#### Playground Widget Parts (`frontend/src/widgets/playground/parts.ts`)

```typescript
// Single source of truth for data-part attribute values
export const PARTS = {
    root:           'playground',
    editorPanel:    'editor-panel',
    editorHeader:   'editor-header',
    editorBody:     'editor-body',
    toolbar:        'toolbar',
    runButton:      'run-button',
    storeButton:    'store-button',
    clearButton:    'clear-button',
    presetsMenu:    'presets-menu',
    settingsMenu:   'settings-menu',
    outputPanel:    'output-panel',
    statusBar:      'status-bar',
    consoleOutput:  'console-output',
    resultOutput:   'result-output',
    sessionInfo:    'session-info',
    quickReference: 'quick-reference',
} as const;
```

#### Example Themed Selector

```css
/* Base layout — widget.css */
:where([data-widget="playground"]) [data-part="editor-panel"] {
    display: flex;
    flex-direction: column;
    height: 100%;
}

:where([data-widget="playground"]) [data-part="console-output"] {
    font-family: var(--font-mono);
    font-size: var(--font-size);
    background: var(--color-surface-2);
    color: var(--color-text);
    padding: var(--space-3);
    border-radius: var(--radius-1);
    overflow-y: auto;
}

/* Theme override — a consumer can do this: */
[data-widget="playground"] {
    --color-surface-2: #2d2d2d;
    --color-accent: #ff6b6b;
}
```

#### Unstyled Mode

Each widget accepts an `unstyled` prop:

```tsx
<Playground unstyled />
```

When `unstyled` is true, the default theme CSS is not applied, but `data-part`
attributes are still rendered, allowing the consumer to provide all styling.

### 3.4 RTK Query — API Layer

#### API Slice Definition (`frontend/src/store/api.ts`)

```typescript
import { createApi, fetchBaseQuery } from '@reduxjs/toolkit/query/react';

interface ExecuteRequest {
    code: string;
    options?: {
        store?: boolean;
        timeout?: number;
    };
}

interface ExecuteResponse {
    ok: boolean;
    data: {
        result: unknown;
        consoleLog: string[];
        sessionId: string;
    };
    error: string | null;
    meta: {
        requestId: string;
        timestamp: string;
        duration_ms: number;
    };
}

interface Execution {
    sessionId: string;
    code: string;
    result: string;
    consoleLog: string[];
    error: string;
    source: string;
    createdAt: string;
}

interface PaginatedResponse<T> {
    ok: boolean;
    data: {
        items: T[];
        total: number;
        offset: number;
        limit: number;
    };
}

interface Preset {
    id: string;
    name: string;
    description: string;
    code: string;
    category: string;
}

interface RequestLog {
    id: string;
    method: string;
    path: string;
    status: number;
    durationMs: number;
    remoteIp: string;
    createdAt: string;
    consoleLogs: string[];
    databaseOps: Array<{ sql: string; params: unknown[]; durationMs: number }>;
}

interface LogStats {
    totalRequests: number;
    successRate: number;
    avgResponseTimeMs: number;
    errorCount: number;
}

export const api = createApi({
    reducerPath: 'api',
    baseQuery: fetchBaseQuery({ baseUrl: '/api/v1' }),
    tagTypes: ['Execution', 'Preset', 'Log', 'State'],
    endpoints: (builder) => ({

        // === Execute ===
        executeCode: builder.mutation<ExecuteResponse, ExecuteRequest>({
            query: (body) => ({
                url: '/execute',
                method: 'POST',
                body,
            }),
            invalidatesTags: ['Execution'],
        }),

        // === Executions ===
        listExecutions: builder.query<PaginatedResponse<Execution>, {
            search?: string;
            source?: string;
            limit?: number;
            offset?: number;
        }>({
            query: (params) => ({
                url: '/executions',
                params,
            }),
            providesTags: ['Execution'],
        }),

        getExecution: builder.query<Execution, string>({
            query: (id) => `/executions/${id}`,
        }),

        // === Presets ===
        listPresets: builder.query<Preset[], void>({
            query: () => '/presets',
            providesTags: ['Preset'],
        }),

        getPreset: builder.query<Preset, string>({
            query: (id) => `/presets/${id}`,
        }),

        // === Global State ===
        getState: builder.query<Record<string, unknown>, void>({
            query: () => '/state',
            providesTags: ['State'],
        }),

        updateState: builder.mutation<void, Record<string, unknown>>({
            query: (body) => ({
                url: '/state',
                method: 'PUT',
                body,
            }),
            invalidatesTags: ['State'],
        }),

        // === Logs ===
        listLogs: builder.query<PaginatedResponse<RequestLog>, {
            limit?: number;
            offset?: number;
        }>({
            query: (params) => ({
                url: '/logs',
                params,
            }),
            providesTags: ['Log'],
        }),

        getLogDetail: builder.query<RequestLog, string>({
            query: (id) => `/logs/${id}`,
        }),

        getLogStats: builder.query<LogStats, void>({
            query: () => '/logs/stats',
            providesTags: ['Log'],
        }),

        clearLogs: builder.mutation<void, void>({
            query: () => ({
                url: '/logs',
                method: 'DELETE',
            }),
            invalidatesTags: ['Log'],
        }),

        // === VM ===
        resetVM: builder.mutation<void, void>({
            query: () => ({
                url: '/vm/reset',
                method: 'POST',
            }),
            invalidatesTags: ['State', 'Execution'],
        }),

        // === Docs ===
        listDocs: builder.query<Array<{ slug: string; title: string }>, void>({
            query: () => '/docs',
        }),

        getDoc: builder.query<{ slug: string; title: string; html: string }, string>({
            query: (slug) => `/docs/${slug}`,
        }),
    }),
});

export const {
    useExecuteCodeMutation,
    useListExecutionsQuery,
    useGetExecutionQuery,
    useListPresetsQuery,
    useGetPresetQuery,
    useGetStateQuery,
    useUpdateStateMutation,
    useListLogsQuery,
    useGetLogDetailQuery,
    useGetLogStatsQuery,
    useClearLogsMutation,
    useResetVMMutation,
    useListDocsQuery,
    useGetDocQuery,
} = api;
```

#### Store Configuration (`frontend/src/store/index.ts`)

```typescript
import { configureStore } from '@reduxjs/toolkit';
import { api } from './api';
import editorReducer from './slices/editorSlice';
import replReducer from './slices/replSlice';
import uiReducer from './slices/uiSlice';
import { websocketMiddleware } from './websocketMiddleware';

export const store = configureStore({
    reducer: {
        [api.reducerPath]: api.reducer,
        editor: editorReducer,
        repl: replReducer,
        ui: uiReducer,
    },
    middleware: (getDefaultMiddleware) =>
        getDefaultMiddleware()
            .concat(api.middleware)
            .concat(websocketMiddleware),
});

export type RootState = ReturnType<typeof store.getState>;
export type AppDispatch = typeof store.dispatch;
```

#### Editor Slice (`frontend/src/store/slices/editorSlice.ts`)

```typescript
import { createSlice, PayloadAction } from '@reduxjs/toolkit';

interface EditorState {
    code: string;
    vimMode: boolean;
    fontSize: number;
    lastResult: {
        value: unknown;
        consoleLog: string[];
        error: string | null;
        sessionId: string | null;
        durationMs: number;
    } | null;
    status: 'ready' | 'running' | 'success' | 'error';
}

const initialState: EditorState = {
    code: '// Welcome to the JavaScript Playground!\n',
    vimMode: true,
    fontSize: 14,
    lastResult: null,
    status: 'ready',
};

export const editorSlice = createSlice({
    name: 'editor',
    initialState,
    reducers: {
        setCode: (state, action: PayloadAction<string>) => {
            state.code = action.payload;
        },
        setVimMode: (state, action: PayloadAction<boolean>) => {
            state.vimMode = action.payload;
        },
        setFontSize: (state, action: PayloadAction<number>) => {
            state.fontSize = action.payload;
        },
        setStatus: (state, action: PayloadAction<EditorState['status']>) => {
            state.status = action.payload;
        },
        setLastResult: (state, action: PayloadAction<EditorState['lastResult']>) => {
            state.lastResult = action.payload;
        },
        clearResult: (state) => {
            state.lastResult = null;
            state.status = 'ready';
        },
    },
});
```

#### REPL Slice (`frontend/src/store/slices/replSlice.ts`)

```typescript
import { createSlice, PayloadAction } from '@reduxjs/toolkit';

interface ReplEntry {
    type: 'input' | 'result' | 'error' | 'log';
    content: string;
    timestamp: number;
}

interface ReplState {
    entries: ReplEntry[];
    history: string[];        // command history
    historyIndex: number;     // -1 = not navigating
    currentInput: string;
}

const initialState: ReplState = {
    entries: [
        { type: 'log', content: 'JavaScript REPL — Type expressions and press Enter', timestamp: Date.now() },
    ],
    history: [],
    historyIndex: -1,
    currentInput: '',
};

export const replSlice = createSlice({
    name: 'repl',
    initialState,
    reducers: {
        addEntry: (state, action: PayloadAction<Omit<ReplEntry, 'timestamp'>>) => {
            state.entries.push({ ...action.payload, timestamp: Date.now() });
        },
        clearEntries: (state) => {
            state.entries = [initialState.entries[0]];
            state.history = [];
            state.historyIndex = -1;
        },
        pushHistory: (state, action: PayloadAction<string>) => {
            state.history.push(action.payload);
            state.historyIndex = state.history.length;
        },
        navigateHistory: (state, action: PayloadAction<'up' | 'down'>) => {
            if (action.payload === 'up' && state.historyIndex > 0) {
                state.historyIndex--;
                state.currentInput = state.history[state.historyIndex];
            } else if (action.payload === 'down') {
                if (state.historyIndex < state.history.length - 1) {
                    state.historyIndex++;
                    state.currentInput = state.history[state.historyIndex];
                } else {
                    state.historyIndex = state.history.length;
                    state.currentInput = '';
                }
            }
        },
        setCurrentInput: (state, action: PayloadAction<string>) => {
            state.currentInput = action.payload;
        },
    },
});
```

### 3.5 WebSocket Middleware for Real-Time Updates

```typescript
// frontend/src/store/websocketMiddleware.ts

import { Middleware } from '@reduxjs/toolkit';
import { api } from './api';

interface WSMessage {
    type: 'console' | 'status' | 'request' | 'stateChange';
    sessionId?: string;
    data?: unknown;
}

export const websocketMiddleware: Middleware = (store) => {
    let ws: WebSocket | null = null;
    let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

    function connect() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        ws = new WebSocket(`${protocol}//${window.location.host}/api/v1/ws`);

        ws.onopen = () => {
            // Subscribe to all channels
            ws?.send(JSON.stringify({
                type: 'subscribe',
                channels: ['console', 'requests', 'state'],
            }));
        };

        ws.onmessage = (event) => {
            const msg: WSMessage = JSON.parse(event.data);

            switch (msg.type) {
                case 'request':
                    // Invalidate logs cache so RTK Query refetches
                    store.dispatch(api.util.invalidateTags(['Log']));
                    break;
                case 'stateChange':
                    store.dispatch(api.util.invalidateTags(['State']));
                    break;
                case 'console':
                    // Dispatch to REPL or playground depending on context
                    // (streaming console output for long-running scripts)
                    break;
            }
        };

        ws.onclose = () => {
            // Reconnect after 3 seconds
            reconnectTimer = setTimeout(connect, 3000);
        };
    }

    // Connect on middleware creation
    connect();

    return (next) => (action) => next(action);
};
```

### 3.6 Key React Components — Pseudocode

#### Playground Component

```tsx
// frontend/src/widgets/playground/Playground.tsx

function Playground({ unstyled = false }: PlaygroundProps) {
    const dispatch = useAppDispatch();
    const { code, vimMode, fontSize, lastResult, status } = useAppSelector(s => s.editor);
    const [executeCode] = useExecuteCodeMutation();
    const { data: presets } = useListPresetsQuery();

    // Import styles conditionally
    if (!unstyled) {
        import('./styles/playground.css');
        import('./styles/theme-default.css');
    }

    async function handleRun() {
        dispatch(setStatus('running'));
        const start = Date.now();
        try {
            const result = await executeCode({ code, options: { store: false } }).unwrap();
            dispatch(setLastResult({
                value: result.data.result,
                consoleLog: result.data.consoleLog,
                error: null,
                sessionId: result.data.sessionId,
                durationMs: Date.now() - start,
            }));
            dispatch(setStatus('success'));
        } catch (err) {
            dispatch(setLastResult({
                value: null, consoleLog: [], error: err.message,
                sessionId: null, durationMs: Date.now() - start,
            }));
            dispatch(setStatus('error'));
        }
    }

    return (
        <div data-widget="playground" data-state={status}>
            <div data-part="editor-panel">
                <div data-part="editor-header">
                    <h5>JavaScript Editor</h5>
                    <div data-part="toolbar">
                        <button data-part="run-button" onClick={handleRun}>Run</button>
                        <button data-part="store-button" onClick={handleExecuteAndStore}>
                            Execute & Store
                        </button>
                        <button data-part="clear-button" onClick={() => dispatch(setCode(''))}>
                            Clear
                        </button>
                        <PresetsDropdown presets={presets} />
                        <SettingsMenu vimMode={vimMode} fontSize={fontSize} />
                    </div>
                </div>
                <div data-part="editor-body">
                    <CodeEditor
                        value={code}
                        onChange={(val) => dispatch(setCode(val))}
                        vimMode={vimMode}
                        fontSize={fontSize}
                        onRun={handleRun}
                        onStore={handleExecuteAndStore}
                    />
                </div>
            </div>
            <OutputPanel result={lastResult} status={status} />
        </div>
    );
}
```

#### Shared CodeEditor Component

```tsx
// frontend/src/widgets/shared/CodeEditor.tsx
// Wraps CodeMirror 6 (not 5!) for the React ecosystem

import { useCodeMirror } from '@uiw/react-codemirror';
import { javascript } from '@codemirror/lang-javascript';
import { vim } from '@replit/codemirror-vim';
import { oneDark } from '@codemirror/theme-one-dark';

interface CodeEditorProps {
    value: string;
    onChange: (value: string) => void;
    vimMode?: boolean;
    fontSize?: number;
    onRun?: () => void;
    onStore?: () => void;
    readOnly?: boolean;
}

function CodeEditor({ value, onChange, vimMode, fontSize, onRun, onStore, readOnly }: CodeEditorProps) {
    const extensions = [
        javascript(),
        oneDark,
        // Conditionally add vim extension
        ...(vimMode ? [vim()] : []),
        // Keybindings
        keymap.of([
            { key: 'Ctrl-Enter', run: () => { onRun?.(); return true; } },
            { key: 'Cmd-Enter',  run: () => { onRun?.(); return true; } },
            { key: 'Ctrl-s',     run: () => { onStore?.(); return true; } },
            { key: 'Cmd-s',      run: () => { onStore?.(); return true; } },
        ]),
    ];

    return (
        <div data-part="code-editor" style={{ fontSize: `${fontSize}px` }}>
            <ReactCodeMirror
                value={value}
                onChange={onChange}
                extensions={extensions}
                readOnly={readOnly}
            />
        </div>
    );
}
```

#### REPL Component

```tsx
// frontend/src/widgets/repl/Repl.tsx

function Repl({ unstyled = false }: ReplProps) {
    const dispatch = useAppDispatch();
    const { entries, currentInput, historyIndex } = useAppSelector(s => s.repl);
    const [executeCode] = useExecuteCodeMutation();
    const consoleRef = useRef<HTMLDivElement>(null);

    async function handleExecute() {
        if (!currentInput.trim()) return;

        dispatch(addEntry({ type: 'input', content: currentInput }));
        dispatch(pushHistory(currentInput));

        try {
            const result = await executeCode({
                code: currentInput,
                options: { store: false },
            }).unwrap();

            if (result.data.consoleLog?.length) {
                result.data.consoleLog.forEach(log =>
                    dispatch(addEntry({ type: 'log', content: log }))
                );
            }
            if (result.data.result !== undefined) {
                dispatch(addEntry({
                    type: 'result',
                    content: typeof result.data.result === 'object'
                        ? JSON.stringify(result.data.result, null, 2)
                        : String(result.data.result),
                }));
            }
        } catch (err) {
            dispatch(addEntry({ type: 'error', content: err.message }));
        }

        dispatch(setCurrentInput(''));
    }

    function handleKeyDown(e: React.KeyboardEvent) {
        if (e.key === 'Enter' && !e.shiftKey) {
            e.preventDefault();
            handleExecute();
        } else if (e.key === 'ArrowUp') {
            e.preventDefault();
            dispatch(navigateHistory('up'));
        } else if (e.key === 'ArrowDown') {
            e.preventDefault();
            dispatch(navigateHistory('down'));
        }
    }

    // Auto-scroll to bottom
    useEffect(() => {
        consoleRef.current?.scrollTo(0, consoleRef.current.scrollHeight);
    }, [entries]);

    return (
        <div data-widget="repl">
            <div data-part="console" ref={consoleRef}>
                {entries.map((entry, i) => (
                    <ReplEntry key={i} entry={entry} />
                ))}
            </div>
            <div data-part="input-row">
                <span data-part="prompt">&gt;</span>
                <textarea
                    data-part="input"
                    value={currentInput}
                    onChange={e => dispatch(setCurrentInput(e.target.value))}
                    onKeyDown={handleKeyDown}
                />
            </div>
            <div data-part="actions">
                <button onClick={handleExecute}>Execute</button>
                <button onClick={() => dispatch(clearEntries())}>Clear</button>
                <ResetVMButton />
            </div>
        </div>
    );
}
```

### 3.7 Go Backend Changes

The Go backend needs to change from "serve HTML pages" to "serve a SPA + JSON API":

#### New Router Setup (pseudocode)

```go
// pkg/web/routes.go — after migration

func SetupAdminServerRoutes(jsEngine *engine.Engine) *mux.Router {
    r := mux.NewRouter()

    // API routes — all JSON
    apiRouter := r.PathPrefix("/api/v1").Subrouter()
    apiRouter.HandleFunc("/execute", api.ExecuteHandler(jsEngine)).Methods("POST")
    apiRouter.HandleFunc("/executions", api.ListExecutionsHandler(jsEngine)).Methods("GET")
    apiRouter.HandleFunc("/executions/{id}", api.GetExecutionHandler(jsEngine)).Methods("GET")
    apiRouter.HandleFunc("/presets", api.ListPresetsHandler()).Methods("GET")
    apiRouter.HandleFunc("/presets/{id}", api.GetPresetHandler()).Methods("GET")
    apiRouter.HandleFunc("/state", api.GetStateHandler(jsEngine)).Methods("GET")
    apiRouter.HandleFunc("/state", api.UpdateStateHandler(jsEngine)).Methods("PUT")
    apiRouter.HandleFunc("/logs", api.ListLogsHandler(jsEngine)).Methods("GET")
    apiRouter.HandleFunc("/logs", api.ClearLogsHandler(jsEngine)).Methods("DELETE")
    apiRouter.HandleFunc("/logs/stats", api.LogStatsHandler(jsEngine)).Methods("GET")
    apiRouter.HandleFunc("/logs/{id}", api.GetLogHandler(jsEngine)).Methods("GET")
    apiRouter.HandleFunc("/vm/reset", api.ResetVMHandler(jsEngine)).Methods("POST")
    apiRouter.HandleFunc("/docs", api.ListDocsHandler()).Methods("GET")
    apiRouter.HandleFunc("/docs/{slug}", api.GetDocHandler()).Methods("GET")
    apiRouter.Handle("/ws", api.WebSocketHandler(jsEngine))

    // SPA fallback — serve index.html for all non-API routes
    r.PathPrefix("/assets/").Handler(spaFileServer)  // Vite build output
    r.PathPrefix("/").HandlerFunc(spaFallback)        // → index.html

    return r
}
```

#### New Execute Endpoint (JSON envelope)

```go
// pkg/api/execute.go — after migration

type ExecuteRequest struct {
    Code    string         `json:"code"`
    Options ExecuteOptions `json:"options"`
}

type ExecuteOptions struct {
    Store   bool `json:"store"`
    Timeout int  `json:"timeout"` // milliseconds, 0 = default 30s
}

type APIResponse struct {
    OK    bool        `json:"ok"`
    Data  interface{} `json:"data,omitempty"`
    Error string      `json:"error,omitempty"`
    Meta  APIMeta     `json:"meta"`
}

type APIMeta struct {
    RequestID  string `json:"requestId"`
    Timestamp  string `json:"timestamp"`
    DurationMs int64  `json:"duration_ms"`
}

func ExecuteHandler(jsEngine *engine.Engine) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        var req ExecuteRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            writeJSON(w, http.StatusBadRequest, APIResponse{
                OK:    false,
                Error: "Invalid JSON request body",
                Meta:  newMeta(),
            })
            return
        }

        // ... submit job, wait for result ...

        writeJSON(w, http.StatusOK, APIResponse{
            OK: true,
            Data: map[string]interface{}{
                "result":     result.Value,
                "consoleLog": result.ConsoleLog,
                "sessionId":  sessionID,
            },
            Meta: newMetaWithDuration(start),
        })
    }
}
```

### 3.8 Development Workflow — Two-Process Dev Loop

During development, you run two processes:

```
Terminal 1 (Go backend):
$ go run ./cmd/jesus serve -p 9922 --admin-port 9090

Terminal 2 (Vite dev server):
$ cd frontend && npm run dev
  → Vite on http://localhost:5173
  → Proxies /api/* to http://localhost:9090
```

**Vite config** (`frontend/vite.config.ts`):

```typescript
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
    plugins: [react()],
    server: {
        port: 5173,
        proxy: {
            '/api': {
                target: 'http://localhost:9090',
                ws: true,  // proxy WebSocket too
            },
        },
    },
    build: {
        outDir: '../pkg/web/dist',  // build output goes into Go embed directory
        emptyOutDir: true,
    },
});
```

**Production build:**

```bash
cd frontend && npm run build
# Output in pkg/web/dist/
# Go embeds it via //go:embed dist/*
# Single binary serves everything
```

### 3.9 Storybook Stories

Each widget gets stories for default, themed, unstyled, and interactive variants:

```tsx
// frontend/stories/Playground.stories.tsx

import type { Meta, StoryObj } from '@storybook/react';
import { Provider } from 'react-redux';
import { Playground } from '../src/widgets/playground';
import { store } from '../src/store';

const meta: Meta<typeof Playground> = {
    title: 'Widgets/Playground',
    component: Playground,
    decorators: [(Story) => <Provider store={store}><Story /></Provider>],
    argTypes: {
        unstyled: { control: 'boolean' },
    },
};

export default meta;
type Story = StoryObj<typeof Playground>;

export const Default: Story = {};

export const Unstyled: Story = {
    args: { unstyled: true },
};

export const CustomTheme: Story = {
    decorators: [
        (Story) => (
            <div style={{
                '--color-bg': '#1a1a2e',
                '--color-accent': '#e94560',
                '--color-surface': '#16213e',
            } as React.CSSProperties}>
                <Story />
            </div>
        ),
    ],
};

export const WithResult: Story = {
    play: async ({ canvasElement }) => {
        // Use Storybook interactions to simulate execution
    },
};
```

### 3.10 Migration Phases

The migration should be incremental.  Here is a phased plan:

#### Phase 1: Backend API Normalization (1 week)

- Add new `/api/v1/*` JSON endpoints alongside existing endpoints.
- Keep existing Templ pages working (no frontend changes).
- Add WebSocket endpoint.
- Write Go tests for all new endpoints.

**Files changed:**
- `pkg/api/execute.go` — add JSON envelope
- `pkg/api/executions.go` — new: list/get executions
- `pkg/api/presets.go` — new: list/get presets
- `pkg/api/state.go` — new: get/put state
- `pkg/api/logs.go` — new: logs endpoints
- `pkg/api/ws.go` — new: WebSocket handler
- `pkg/api/response.go` — new: shared response helpers
- `pkg/web/routes.go` — register new API routes

#### Phase 2: React Scaffold + Shared Components (1 week)

- Initialize Vite + React + TypeScript + Redux Toolkit project in `frontend/`.
- Create store with RTK Query API slice.
- Build shared `CodeEditor` component (CodeMirror 6).
- Build `Toast` and `Badge` shared components.
- Set up Storybook.
- Set up Vite proxy for development.

**New files:** everything in `frontend/src/store/`, `frontend/src/widgets/shared/`

#### Phase 3: Playground Widget (1 week)

- Port `PlaygroundPage` from Templ + vanilla JS to React.
- Implement `EditorPanel`, `OutputPanel`, `StatusBar`, `QuickReference`, `PresetsDropdown`.
- Define `parts.ts` and theme tokens.
- Wire `useExecuteCodeMutation` for Run and Execute & Store.
- Add keyboard shortcuts via `useKeyboardShortcuts` hook.
- Write Storybook stories.

**New files:** everything in `frontend/src/widgets/playground/`

#### Phase 4: REPL Widget (3 days)

- Port REPL from Templ + vanilla JS to React.
- Implement `ReplConsole`, `ReplInput`, `ExampleButtons`.
- REPL state fully in Redux (entries, history, input).
- Write Storybook stories.

**New files:** everything in `frontend/src/widgets/repl/`

#### Phase 5: History + Docs Widgets (3 days)

- Port History page to React with RTK Query pagination.
- Port Docs page to React with RTK Query doc fetching.
- Cross-widget navigation (History → Playground) via Redux actions instead of
  localStorage + page reload.

**New files:** `frontend/src/widgets/history/`, `frontend/src/widgets/docs/`

#### Phase 6: Admin Dashboard Widget (1 week)

- Port request logs and global state editor to React.
- Replace SSE with WebSocket via `websocketMiddleware`.
- Implement `StatsCards`, `RequestLogList`, `RequestDetail`, `GlobalStateEditor`.
- Write Storybook stories.

**New files:** `frontend/src/widgets/admin/`

#### Phase 7: Go Embed + Production Build (2 days)

- Add `go:generate` for `cd frontend && npm run build`.
- Embed `frontend/dist/` into Go binary.
- Add SPA fallback handler.
- Remove old Templ templates and vanilla JS/CSS.
- Update `Makefile`.

**Files removed:** All `pkg/web/templates/*.templ`, `pkg/web/static/`

#### Phase 8: Polish + QA (3 days)

- Accessibility audit (keyboard navigation, ARIA attributes, focus management).
- Responsive design testing.
- Performance profiling (code splitting, lazy loading).
- Update documentation.

---

## 4. Backend API Reference (Complete)

This section documents every endpoint in the proposed new API, with request/response
examples.  This is the contract that RTK Query will consume.

### 4.1 POST /api/v1/execute

Execute JavaScript code in the Goja runtime.

**Request:**
```json
{
    "code": "console.log('hello'); 2 + 2;",
    "options": {
        "store": true,
        "timeout": 10000
    }
}
```

**Response (200):**
```json
{
    "ok": true,
    "data": {
        "result": 4,
        "consoleLog": ["hello"],
        "sessionId": "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
    },
    "meta": {
        "requestId": "req_abc123",
        "timestamp": "2026-03-16T14:30:00Z",
        "duration_ms": 12
    }
}
```

**Response (400):**
```json
{
    "ok": false,
    "error": "Invalid JSON request body",
    "meta": { "requestId": "req_abc124", "timestamp": "...", "duration_ms": 0 }
}
```

**Response (408):**
```json
{
    "ok": false,
    "error": "Execution timed out after 10000ms",
    "meta": { "requestId": "req_abc125", "timestamp": "...", "duration_ms": 10001 }
}
```

### 4.2 GET /api/v1/executions

List past executions with pagination and filtering.

**Query Parameters:**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `limit` | int | 20 | Page size |
| `offset` | int | 0 | Page offset |
| `search` | string | — | Search in code text |
| `source` | string | — | Filter by source: "api", "mcp", "file" |
| `session_id` | string | — | Filter by exact session ID |

**Response:**
```json
{
    "ok": true,
    "data": {
        "items": [
            {
                "sessionId": "a1b2c3d4-...",
                "code": "2 + 2",
                "result": "4",
                "consoleLog": [],
                "error": "",
                "source": "api",
                "createdAt": "2026-03-16T14:30:00Z"
            }
        ],
        "total": 142,
        "offset": 0,
        "limit": 20
    }
}
```

### 4.3 GET /api/v1/executions/:id

Get a single execution by session ID.

### 4.4 GET /api/v1/presets

List all code presets/examples.

**Response:**
```json
{
    "ok": true,
    "data": [
        {
            "id": "hello-world",
            "name": "Hello World",
            "description": "Basic hello world endpoint",
            "code": "app.get('/hello', ...);\n",
            "category": "basics"
        }
    ]
}
```

### 4.5 GET /api/v1/state

Get the current globalState object.

**Response:**
```json
{
    "ok": true,
    "data": {
        "counter": 42,
        "lastUser": "alice"
    }
}
```

### 4.6 PUT /api/v1/state

Replace the entire globalState object.

**Request:**
```json
{ "counter": 0 }
```

### 4.7 PATCH /api/v1/state

Merge fields into globalState (shallow merge).

### 4.8 GET /api/v1/logs

List request logs with pagination.

**Response:**
```json
{
    "ok": true,
    "data": {
        "items": [
            {
                "id": "req_abc123",
                "method": "GET",
                "path": "/hello",
                "status": 200,
                "durationMs": 12,
                "remoteIp": "127.0.0.1",
                "createdAt": "2026-03-16T14:30:00Z",
                "consoleLogs": ["handled /hello"],
                "databaseOps": [
                    { "sql": "SELECT * FROM users", "params": [], "durationMs": 2 }
                ]
            }
        ],
        "total": 87,
        "offset": 0,
        "limit": 20
    }
}
```

### 4.9 GET /api/v1/logs/stats

Aggregate statistics.

**Response:**
```json
{
    "ok": true,
    "data": {
        "totalRequests": 87,
        "successRate": 94.2,
        "avgResponseTimeMs": 23,
        "errorCount": 5,
        "methodCounts": { "GET": 60, "POST": 25, "PUT": 2 },
        "statusCounts": { "200": 72, "404": 10, "500": 5 }
    }
}
```

### 4.10 DELETE /api/v1/logs

Clear all request logs.

### 4.11 POST /api/v1/vm/reset

Reset the JavaScript VM (reinitialize Goja runtime).

### 4.12 WS /api/v1/ws

WebSocket for real-time events.

**Client → Server:**
```json
{ "type": "subscribe", "channels": ["console", "requests", "state"] }
{ "type": "execute", "code": "2+2", "options": { "store": false } }
{ "type": "cancel", "sessionId": "..." }
```

**Server → Client:**
```json
{ "type": "console", "sessionId": "...", "level": "log", "message": "hello" }
{ "type": "status", "sessionId": "...", "state": "done", "durationMs": 12 }
{ "type": "request", "data": { "id": "...", "method": "GET", "path": "/hello", "status": 200 } }
{ "type": "stateChange", "data": { "counter": 43 } }
```

---

## 5. Mapping Current → New (Reference Table)

This table maps every current feature to its new location after migration:

| Current Location | Current Technology | New Location | New Technology |
|-----------------|-------------------|--------------|----------------|
| `templates/playground.templ` | Templ SSR | `widgets/playground/Playground.tsx` | React component |
| `templates/repl.templ` | Templ SSR | `widgets/repl/Repl.tsx` | React component |
| `templates/history.templ` | Templ SSR | `widgets/history/History.tsx` | React + RTK Query |
| `templates/docs.templ` | Templ SSR | `widgets/docs/Docs.tsx` | React + RTK Query |
| `templates/admin.templ` | Templ SSR | `widgets/admin/AdminDashboard.tsx` | React component |
| `static/js/app.js` (JSPlaygroundApp) | Vanilla JS class | Split across widgets + store | React + Redux |
| `static/css/app.css` | Monolithic CSS | `styles/tokens.css` + per-widget CSS | CSS custom properties |
| `static/admin/logs.*` | Standalone SPA | `widgets/admin/components/*` | React components |
| `static/admin/globalstate.*` | Standalone SPA | `widgets/admin/GlobalStateEditor.tsx` | React + RTK Query |
| `handlers.templ.go` (page handlers) | Go HTTP handlers | SPA fallback handler | Single handler |
| `handlers.templ.go` (static serving) | Go embed FS | Vite build output + Go embed | Same pattern |
| `admin/sse.go` | Server-Sent Events | `api/ws.go` | WebSocket |
| `localStorage` code passing | Cross-page hack | Redux store dispatch | In-memory |
| CDN CodeMirror 5 | Script tag | npm CodeMirror 6 | Import |
| CDN Bootstrap 5 | Script tag | Removed (custom tokens) | CSS custom properties |

---

## 6. Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| CodeMirror 6 migration breaks Vim mode | Medium | High | Test early; `@replit/codemirror-vim` is mature |
| RTK Query cache invalidation is tricky with WebSocket | Low | Medium | Use explicit `invalidateTags` on WS messages |
| Losing single-binary simplicity | Low | High | `go:generate` + `go:embed` preserves it |
| Bootstrap removal breaks responsive layout | Medium | Medium | Implement responsive tokens from the start |
| Phase 1 API changes break existing clients | Low | Low | Keep old endpoints during migration |
| Storybook stories become stale | Medium | Low | CI job that builds Storybook on PR |

---

## 7. Summary of Recommendations

1. **Phase the migration** — don't rewrite everything at once.  Keep old Templ pages
   working while building React widgets one at a time.

2. **Start with the API** — normalize the backend first.  Everything else depends on
   clean JSON endpoints.

3. **Use RTK Query aggressively** — it handles caching, loading states, refetching,
   and WebSocket invalidation.  Don't write manual `fetch()` calls.

4. **Adopt CSS custom properties from day one** — the current hard-coded dark theme
   is the single biggest obstacle to theming.  Tokens + `data-part` selectors make
   every widget independently themeable.

5. **Upgrade to CodeMirror 6** — the current CDN-hosted CodeMirror 5 is legacy.
   CM6 is modular, tree-shakeable, and has better React integration.

6. **Replace SSE with WebSocket** — bidirectional communication enables streaming
   console output, execution cancellation, and push-based state updates.

7. **Preserve the single-binary deployment** — use `go:generate` to build the
   frontend and `go:embed` to bundle it.  The result should still be one `jesus`
   binary.
