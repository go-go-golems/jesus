---
title: "Jesus Web IDE — Current Architecture & Functionality"
doc-type: design-doc
ticket: JESUS-IDE-01
topics:
  - javascript
  - architecture
status: active
created: 2026-03-16
---

# Jesus Web IDE — Current Architecture & Functionality

## 1. Introduction and Purpose

The Jesus project ships a **web-based JavaScript IDE** — a browser-hosted environment
for writing, executing, and debugging JavaScript code against a live Go backend powered
by the [Goja](https://github.com/nicholasgasior/goja) ECMAScript runtime.  The IDE is
not a static playground: the JavaScript code you write can register Express.js-style
HTTP routes and query a real SQLite database.  Think of it as a tiny Heroku-for-one
where you prototype server-side JS logic in a browser, instantly deploy it behind a
running HTTP server, and watch every request flow through an admin dashboard — all in a
single binary.

### Who Should Read This

This document is written for a new team member who has basic familiarity with Go, HTTP,
and front-end concepts but has never seen the Jesus codebase.  After reading it you
should be able to:

- Start the server and open every view in your browser.
- Trace a piece of JavaScript from the browser editor through the Go backend to the
  Goja runtime and back.
- Locate every relevant source file and understand its role.
- Explain the dual-server architecture, the job dispatcher, and the admin monitoring
  tools to someone else.

---

## 2. High-Level Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                         Browser (User)                               │
│                                                                      │
│   ┌───────────┐  ┌──────┐  ┌─────────┐  ┌──────┐  ┌──────────────┐ │
│   │Playground │  │ REPL │  │ History │  │ Docs │  │ Admin Logs   │ │
│   │(CodeMirror│  │      │  │         │  │      │  │ GlobalState  │ │
│   │ Editor)   │  │      │  │         │  │      │  │ SSE Monitor  │ │
│   └─────┬─────┘  └──┬───┘  └────┬────┘  └──┬───┘  └──────┬───────┘ │
│         │           │           │           │              │         │
└─────────┼───────────┼───────────┼───────────┼──────────────┼─────────┘
          │           │           │           │              │
          │  POST /v1/execute     │  GET /api/*              │
          │  POST /api/repl/exec  │                          │
          ▼           ▼           ▼           ▼              ▼
┌─────────────────────────────────────────────────────────────────────┐
│                   Admin Server  (default :9090)                      │
│                                                                      │
│   gorilla/mux router                                                │
│   ├─ /static/*           → embedded static file server              │
│   ├─ /v1/execute         → api.ExecuteHandler                       │
│   ├─ /api/repl/execute   → ExecuteREPLHandler                       │
│   ├─ /api/reset-vm       → ResetVMHandler                           │
│   ├─ /api/preset         → PresetHandler                            │
│   ├─ /api/docs           → DocsAPIHandler                           │
│   ├─ /playground         → PlaygroundHandler (Templ SSR)            │
│   ├─ /repl               → REPLHandler (Templ SSR)                  │
│   ├─ /history            → HistoryHandler (Templ SSR)               │
│   ├─ /docs               → DocsHandler (Templ SSR)                  │
│   ├─ /admin/logs         → LogsHandler (HTML + REST API)            │
│   ├─ /admin/globalstate  → GlobalStateHandler                       │
│   └─ /scripts            → ScriptsHandler                           │
│                                                                      │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           │ SubmitJob(EvalJob)
                           ▼
┌──────────────────────────────────────────────────────────────────────┐
│                       Engine (pkg/engine)                             │
│                                                                      │
│   ┌──────────────┐   ┌──────────────┐   ┌───────────────────────┐   │
│   │  Goja Runtime│   │  Event Loop  │   │  Module Registry      │   │
│   │  (ES5.1+)   │   │  (async ops) │   │  (require() support)  │   │
│   └──────┬───────┘   └──────────────┘   └───────────────────────┘   │
│          │                                                           │
│   ┌──────┴───────┐   ┌──────────────┐   ┌───────────────────────┐   │
│   │  Dispatcher  │   │ RequestLogger│   │  Handler Registry     │   │
│   │  (goroutine) │   │ (in-memory)  │   │  [path][method]→fn   │   │
│   └──────────────┘   └──────────────┘   └───────────────────────┘   │
│                                                                      │
└──────────────────────────┬──────────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────────┐
│                   JS Web Server  (default :9922)                     │
│                                                                      │
│   gorilla/mux router                                                │
│   └─ /*  → DynamicRouteHandler                                      │
│           (looks up engine.handlers[method][path],                   │
│            dispatches EvalJob for matched handler)                   │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
                           │
                           ▼
┌──────────────────────────────────────────────────────────────────────┐
│                     SQLite Databases                                 │
│                                                                      │
│   data.sqlite  — application database (user JS code uses `db.*`)    │
│   system.sqlite — system database (execution logs, request logs)    │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

### The Two Servers

Jesus runs **two HTTP servers in the same process**:

| Server | Default Port | Purpose |
|--------|-------------|---------|
| **Admin Server** | 9090 | IDE UI (playground, REPL, history, docs), admin dashboard (logs, globalState), and the `/v1/execute` API endpoint. |
| **JS Web Server** | 9922 | User-facing server that hosts routes **registered by JavaScript code** via `app.get()`, `app.post()`, etc. |

The Admin Server is where you write and execute code.  The JS Web Server is where that
code *runs* when real traffic arrives.

---

## 3. Starting the Server

```bash
# From the jesus project root:
go run ./cmd/jesus serve -p 9922 --admin-port 9090

# With scripts loaded on startup:
go run ./cmd/jesus serve -p 9922 --admin-port 9090 --scripts ./scripts

# With custom databases:
go run ./cmd/jesus serve --app-db myapp.sqlite --system-db mysystem.sqlite
```

**Startup Sequence** (file: `cmd/jesus/cmd/serve.go`):

1. Parse CLI flags via Glazed command framework.
2. Find free ports starting from requested values (tries up to +99).
3. Create `scripts/` directory if absent.
4. Initialize the Goja JavaScript engine with both SQLite databases.
5. Optionally execute `bootstrap.js`.
6. Start the **Dispatcher** goroutine (a single worker consuming a 1024-slot buffered
   channel of `EvalJob` structs).
7. Walk the `--scripts` directory and submit each `.js` file as an `EvalJob`
   with 10-second timeout.
8. Build both `gorilla/mux` routers.
9. Launch the JS server in a background goroutine; run the Admin server on the main
   goroutine (blocking).

---

## 4. The JavaScript Engine (`pkg/engine/`)

### 4.1 Engine Structure

The engine is the heart of Jesus.  It wraps a single Goja runtime with an event loop,
a job dispatcher, and the bindings that make JavaScript code useful.

```
File: pkg/engine/engine.go (~450 lines)

type Engine struct {
    rt              *goja.Runtime           // The JavaScript VM
    loop            *eventloop.EventLoop    // Async support (setTimeout, etc.)
    repos           RepositoryManager       // Database access
    jobs            chan EvalJob             // Buffered job queue (cap 1024)
    handlers        map[string]map[string]*HandlerInfo  // [path][method] → handler
    files           map[string]goja.Callable            // [path] → file handler
    mu              sync.RWMutex            // Protects handlers/files maps
    reqLogger       *RequestLogger          // In-memory request log
    currentReqID    string                  // Active request correlation
    moduleRegistry  *Registry               // require() module system
}
```

**Key point:** There is a **single Goja runtime** — JavaScript execution is
inherently **single-threaded and sequential**.  The dispatcher goroutine pops one job at
a time from the channel and runs it to completion before starting the next one.  This
is safe because Goja is not goroutine-safe; all access must happen on one goroutine.

### 4.2 EvalJob — the Unit of Work

Every piece of JavaScript that gets executed is wrapped in an `EvalJob`:

```go
// File: pkg/engine/engine.go

type EvalJob struct {
    Handler   *HandlerInfo        // Non-nil → run a pre-registered HTTP handler
    Code      string              // Non-empty → execute raw JS code
    W         http.ResponseWriter // For handler jobs: write the HTTP response
    R         *http.Request       // For handler jobs: read the HTTP request
    Done      chan error           // Signalled when execution finishes
    Result    chan *EvalResult     // Carries the result for code-execution jobs
    SessionID string              // UUID for tracking this execution
    Source    string              // "api", "mcp", or "file"
}
```

There are **two execution modes**:

- **Raw code execution** (`Handler == nil`): the playground and REPL send code as a
  string; the engine runs `rt.RunString(code)` and captures the result + console output.
- **Handler invocation** (`Handler != nil`): the JS web server dispatches an incoming
  HTTP request to the JavaScript function that was previously registered via
  `app.get("/path", fn)`.

### 4.3 EvalResult — What Comes Back

```go
type EvalResult struct {
    Value      interface{} // The JavaScript return value, JSON-compatible
    ConsoleLog []string    // All console.log/error/warn/info output
    Error      error       // Non-nil if execution failed
}
```

### 4.4 The Dispatcher

```
File: pkg/engine/dispatcher.go (~205 lines)

StartDispatcher() → spawns dispatcher() goroutine
    │
    └─ for job := range e.jobs {
           processJob(job)
       }

processJob(job):
    1. defer panic recovery → send error on Done channel
    2. if job has HTTP request → start request logging
    3. if job.Handler != nil → executeHandler(job)
    4. else                  → executeDirectCode(job)
    5. finish request logging
    6. signal Done channel
```

The dispatcher is **the only goroutine that touches the Goja runtime**, which prevents
data races.  All other goroutines (HTTP handlers, API endpoints) only send jobs into
the channel and wait on the `Done`/`Result` channels.

### 4.5 JavaScript Bindings

The engine exposes several global objects to JavaScript code:

| Global | Purpose | Source File |
|--------|---------|-------------|
| `app` | Express.js-compatible routing (`app.get()`, `app.post()`, `app.put()`, `app.delete()`, `app.use()`) | `pkg/engine/http_bindings.go` |
| `db` | SQLite database access (`db.query()`, `db.execute()`, `db.all()`, `db.get()`) | `pkg/engine/bindings.go` |
| `console` | Output capture (`console.log()`, `.error()`, `.warn()`, `.info()`, `.debug()`) | `pkg/engine/bindings.go` |
| `globalState` | Persistent key-value object shared across all executions | `pkg/engine/bindings.go` |
| `JSON` | `JSON.stringify()` and `JSON.parse()` utilities | `pkg/engine/bindings.go` |
| `req` / `res` | Express-style Request and Response objects (only inside handler callbacks) | `pkg/engine/http_bindings.go` |

#### Express.js Compatibility (http_bindings.go, ~288 lines)

When JavaScript code calls `app.get("/users", handler)`, the engine:

1. Creates a `HandlerInfo` struct storing the path, method, and Goja callable.
2. Registers it in `engine.handlers["/users"]["GET"]`.
3. When the JS web server receives `GET /users`, it looks up the handler, wraps the
   Go `http.Request` and `http.ResponseWriter` into Express-like `Request` and
   `Response` JavaScript objects, and calls the handler function.

The `Response` object supports:
- `res.json(data)` — serialize to JSON and send
- `res.send(text)` — send text/html
- `res.status(code)` — set HTTP status
- `res.redirect(url)` — HTTP redirect
- `res.header(name, value)` — set header

The `Request` object exposes:
- `req.params` — path parameters (e.g., `/users/:id`)
- `req.query` — query string parameters
- `req.body` — parsed request body
- `req.method`, `req.path`, `req.headers`

---

## 5. The Web Layer (`pkg/web/`)

### 5.1 Directory Map

```
pkg/web/
├── routes.go                   # Router setup for both servers
├── handlers.templ.go           # Page handlers + static file serving
├── admin.go                    # AdminHandler struct & routing
├── admin/
│   ├── logs.go                 # Logs REST API
│   ├── sse.go                  # Server-Sent Events for real-time updates
│   └── globalstate.go          # Global state inspection/editing
├── templates/
│   ├── base.templ              # HTML shell: navbar, CDN resources
│   ├── playground.templ        # Code editor + output panel
│   ├── repl.templ              # Interactive REPL console
│   ├── history.templ           # Execution history with filtering
│   ├── docs.templ              # Documentation browser
│   └── admin.templ             # Request log dashboard components
├── static/
│   ├── css/
│   │   └── app.css             # Global styles (dark theme, CodeMirror, REPL)
│   ├── js/
│   │   └── app.js              # JSPlaygroundApp class — all client-side logic
│   └── admin/
│       ├── logs.html           # Admin logs standalone page
│       ├── logs.js             # Admin logs client logic + SSE
│       ├── logs.css            # Admin logs styling
│       ├── globalstate.html    # Global state editor page
│       ├── globalstate.js      # Global state client logic
│       └── globalstate.css     # Global state styling
├── docs.go                     # Preset examples & docs data
└── docsapi.go                  # Docs API endpoint
```

### 5.2 Template System — Templ

The IDE uses [Templ](https://templ.guide/), a typed Go template language that compiles
`.templ` files into Go code.  Each page is a **server-side rendered component**.

```
templ PlaygroundPage() {
    @BaseLayout("Playground") {
        <div class="row h-100">
            <!-- editor panel -->
            <!-- output panel -->
        </div>
    }
}
```

**BaseLayout** (`base.templ`) provides:
- Dark-theme Bootstrap 5.3.0 (`data-bs-theme="dark"`)
- Navbar: Playground | REPL | History | Docs | Admin
- CDN links: Bootstrap CSS/JS, Bootstrap Icons, CodeMirror 6.65.7
- The `{ children... }` slot for page content
- The `/static/js/app.js` and `/static/css/app.css` scripts

### 5.3 Static File Serving

Static files are **embedded** into the Go binary using `go:embed` on the `static/`
directory.  The `StaticHandler()` function in `handlers.templ.go`:

- Reads files from the embedded FS
- Detects MIME types by extension (`.js`, `.css`, `.json`, `.svg`, etc.)
- Sets `Cache-Control: public, max-age=3600`
- Protects against path traversal

---

## 6. IDE Views In Detail

### 6.1 Playground (`/playground`)

**File:** `pkg/web/templates/playground.templ` (223 lines)

The main development interface.  A two-column layout:

```
┌──────────────────────────────────────────────────────────────┐
│  [Playground]  [REPL]  [History]  [Docs]  [Admin]           │
├──────────────────────────────┬───────────────────────────────┤
│                              │  [Output]  [Quick Reference]  │
│  JavaScript Editor           │                               │
│  ┌────────────────────────┐  │  Status: ● Ready     120ms   │
│  │ // Welcome to the JS   │  │                               │
│  │ // Playground!          │  │  Console Output               │
│  │                         │  │  ┌───────────────────────┐   │
│  │ app.get("/hello", ...); │  │  │ console output here   │   │
│  │                         │  │  └───────────────────────┘   │
│  │                         │  │                               │
│  │                         │  │  Result                       │
│  │                         │  │  ┌───────────────────────┐   │
│  │                         │  │  │ { "message": "Hello" }│   │
│  │                         │  │  └───────────────────────┘   │
│  └────────────────────────┘  │                               │
│                              │  Session: abc123...           │
│  [▶ Run] [⬆ Execute&Store]  │                               │
│  [🗑 Clear] [📋 Examples ▾]  │                               │
│  [⚙ Settings ▾]             │                               │
├──────────────────────────────┴───────────────────────────────┤
```

**Editor** — CodeMirror 6.65.7 with:
- JavaScript syntax highlighting
- Darcula dark theme
- Vim keybindings (toggleable)
- Bracket matching + auto-closing
- Configurable font size (10–20px range slider)

**Action Buttons:**

| Button | Keyboard Shortcut | What It Does |
|--------|-------------------|--------------|
| Run | Ctrl/Cmd + Enter | Sends code to `POST /v1/execute`, shows result. Code is NOT persisted. |
| Execute & Store | Ctrl/Cmd + S | Same as Run, but the execution is stored in the system database with a session ID. |
| Clear | — | Empties the editor. |
| Examples | — | Dropdown populated via `GET /api/docs?action=examples`. Loads preset code into editor. |
| Settings | — | Toggle Vim mode; adjust font size. Preferences saved to `localStorage`. |

**Output Panel** has two tabs:

1. **Output** — status bar, console output pane (200px scrollable), result pane
   (150px scrollable), session ID.
2. **Quick Reference** — accordion with API functions, Database functions, and Console
   & Utilities reference snippets.

### 6.2 REPL (`/repl`)

**File:** `pkg/web/templates/repl.templ` (107 lines)

An interactive Read-Eval-Print Loop:

```
┌────────────────────────────────────────────────┐
│  JavaScript REPL                               │
│  ────────────────────────────────              │
│  > 2 + 2                                       │
│  ← 4                                           │
│  > globalState.counter = (globalState.counter   │
│      || 0) + 1                                 │
│  ← 1                                           │
│  > console.log("hello")                        │
│    hello                                       │
│  ← undefined                                   │
│                                                │
│  > _                                           │
│  [Execute]  [Clear History]  [Reset VM]        │
│                                                │
│  Quick Examples:                               │
│  [2 + 2] [JSON.stringify] [db.query] [app.get] │
└────────────────────────────────────────────────┘
```

**Key features:**
- **Enter** submits; **Shift+Enter** adds a newline (multi-line input).
- **Arrow Up/Down** navigates REPL history (in-memory array).
- **Reset VM** calls `POST /api/reset-vm` to re-initialize the runtime.
- Console output, results, and errors are visually distinguished with prefixes
  (`>`, `←`, `✗`) and CSS classes.
- Example buttons pre-fill the input field.

### 6.3 History (`/history`)

**File:** `pkg/web/templates/history.templ` (244 lines)

A searchable, paginated list of past executions:

- **Filters**: search code text, session ID, source (API/REPL/File).
- **Each execution card** shows: status icon (✓/✗), session ID (first 8 chars),
  source badge, timestamp, code preview (max 100px), error/result/console sections.
- **Actions**: "Load in Playground", "Load in REPL" (via `localStorage` +
  redirect), copy session ID.
- **Pagination**: Previous / Next / "Showing X-Y of Z".

Data is fetched server-side from the repository and rendered with Templ.

### 6.4 Docs (`/docs`)

**File:** `pkg/web/templates/docs.templ` (111 lines)

A two-column documentation browser:

- **Sidebar** (3 cols): list of documentation topics + code example buttons.
- **Main** (9 cols): rendered Markdown content using
  [Goldmark](https://github.com/yuin/goldmark) with GFM, typographer,
  definition list, and footnote extensions.

### 6.5 Admin Dashboard

The admin section is a **standalone SPA** (separate from the Templ pages) with its own
HTML/JS/CSS files served from `/static/admin/`.

#### 6.5.1 Request Logs (`/admin/logs`)

**Files:** `static/admin/logs.html` (75 lines), `logs.js` (493 lines), `logs.css` (441 lines)

A two-tab monitoring dashboard:

**Tab 1 — HTTP Requests:**
```
┌──────────────┬─────────────────────────────────────────┐
│  Statistics  │  Request List → Detail Panel            │
│              │                                         │
│  Total: 42   │  GET /hello  200  12ms                  │
│  Success: 95%│  POST /users 201  45ms                  │
│  Avg: 23ms   │  GET /bad    404   5ms                  │
│  Errors: 2   │                                         │
│              │  ── Detail ──                           │
│              │  Method: GET                            │
│              │  Path: /hello                           │
│              │  Status: 200                            │
│              │  Duration: 12ms                         │
│              │  Headers: { ... }                       │
│              │  Console Logs: [...]                    │
│              │  Database Ops: [...]                    │
└──────────────┴─────────────────────────────────────────┘
```

**Tab 2 — Script Executions:**
Similar layout but for `EvalJob` executions: code, result, console output, errors.

**Real-time updates** via Server-Sent Events (SSE):
- `GET /admin/logs/api/sse` opens a persistent connection.
- Server sends `{"type":"newRequest","count":N}` or `{"type":"newExecution","count":N}`.
- Client refreshes the relevant list.
- Fallback: 5-second polling if SSE connection fails.

**SSE Implementation** (`pkg/web/admin/sse.go`, 128 lines):
- Maintains a `map[string]chan string` of connected clients.
- Broadcasts new events to all clients.
- Cleans up on client disconnect.

#### 6.5.2 Global State Inspector (`/admin/globalstate`)

**Files:** `static/admin/globalstate.html` (74 lines), `globalstate.js` (121 lines), `globalstate.css` (231 lines)

A JSON editor for the `globalState` object shared across all JS executions:

- **View**: pretty-printed JSON in a textarea.
- **Edit**: type JSON, real-time validation (border turns red on invalid JSON).
- **Save**: `POST /admin/globalstate` with JSON body.
- **Reset**: set to `{}` with confirmation dialog.
- **Auto-refresh**: toggle to poll every 5 seconds.
- **Unsaved changes warning**: `beforeunload` handler.

---

## 7. The Frontend Application (`static/js/app.js`)

**615 lines**, organized as a single `JSPlaygroundApp` class instantiated on
`DOMContentLoaded`:

```javascript
class JSPlaygroundApp {
    constructor() {
        this.editor = null;          // CodeMirror instance
        this.replHistory = [];       // REPL command history
        this.replHistoryIndex = -1;  // Current position in history
        this.vimMode = true;         // Vim keybindings on by default
        this.init();
    }
    // ...
}

document.addEventListener('DOMContentLoaded', () => {
    window.jsPlayground = new JSPlaygroundApp();
});
```

### Initialization Flow

```
init()
  ├─ detect page by DOM element presence:
  │    #editor exists?    → initPlayground()
  │    #replConsole exists? → initREPL()
  ├─ initToasts()  — create toast container
  └─ loadFromLocalStorage()  — restore vim mode + font size
```

### API Communication

All execution flows through a single endpoint:

```
POST /v1/execute
Content-Type: text/plain
Body: <raw JavaScript code>

Response 200:
{
    "success": true,
    "result": <any>,
    "consoleLog": ["line1", "line2"],
    "sessionID": "uuid-string",
    "message": "JavaScript code executed and stored in database"
}

Response 4xx/5xx:
{
    "success": false,
    "error": "description",
    "sessionID": "uuid-string"
}
```

### Global Window Functions

These functions are used by server-rendered HTML (onclick handlers in Templ templates):

| Function | Purpose |
|----------|---------|
| `window.loadToPlayground(code)` | Store code in `localStorage`, redirect to `/playground` |
| `window.loadToRepl(code)` | Store code in `localStorage`, redirect to `/repl` |
| `window.copyToClipboard(text)` | Copy + show toast |
| `window.copySessionId(id)` | Copy session ID + toast |
| `window.loadPresetExample(id)` | Fetch preset via `/api/preset?id=...`, load into editor |
| `window.loadDocsExample(id)` | Fetch all examples via `/api/docs?action=examples`, find by ID |

### LocalStorage Keys

| Key | Value | Used By |
|-----|-------|---------|
| `playgroundCode` | Temporary code string | Cross-page code loading |
| `replCode` | Temporary code string | Cross-page code loading |
| `vimMode` | `"true"` / `"false"` | Editor preference |
| `fontSize` | `"10"` – `"20"` | Editor preference |

---

## 8. Execution Flow — End to End

Here is the complete journey of a code snippet from the editor to the result panel:

```
Step 1: User clicks "Run" (or Ctrl+Enter)
        ↓
Step 2: app.js → runCode()
        - Reads code from CodeMirror editor
        - Sets status to "Running..." with spinner
        - Records startTime = Date.now()
        ↓
Step 3: fetch('/v1/execute', { method: 'POST', body: code })
        ↓
Step 4: Go: api.ExecuteHandler (pkg/api/execute.go)
        - Reads body from HTTP request
        - Generates UUID sessionID
        - Creates EvalJob { Code: code, SessionID: id, Source: "api" }
        - Creates Done channel and Result channel
        - Calls jsEngine.SubmitJob(job)
        ↓
Step 5: Engine: job enters the buffered channel (cap 1024)
        ↓
Step 6: Dispatcher goroutine (pkg/engine/dispatcher.go)
        - Picks up job from channel
        - processJob() → executeDirectCode()
        ↓
Step 7: Engine: executeCodeWithResult()
        - Temporarily overrides console.log/error/warn/info to capture output
        - Calls rt.RunString(code)  (Goja executes the JavaScript)
        - Captures return value
        - Restores original console functions
        - Returns EvalResult { Value, ConsoleLog, Error }
        ↓
Step 8: Dispatcher: stores execution in repository
        - repos.Executions().CreateExecution(sessionID, code, result, logs, ...)
        ↓
Step 9: Dispatcher: sends result on Result channel, signals Done
        ↓
Step 10: api.ExecuteHandler receives on Result channel (or 30s timeout)
         - Marshals JSON response
         - Writes to http.ResponseWriter
        ↓
Step 11: Browser receives JSON response
         - app.js parses response
         - Calls showResult(result, consoleLog, error, duration, sessionId)
         - Updates status bar, console pane, result pane
         - Shows execution time
```

---

## 9. Request Logging

**File:** `pkg/engine/request_logger.go` (365 lines)

The `RequestLogger` is an **in-memory circular buffer** that stores the last 100
requests.  It is used by the admin dashboard for monitoring.

```go
type RequestLog struct {
    ID           string
    Method       string
    Path         string
    Status       int
    RemoteIP     string
    StartTime    time.Time
    EndTime      time.Time
    Duration     time.Duration
    Query        map[string][]string
    Headers      map[string][]string
    Body         string
    Response     string
    Error        string
    Logs         []*LogEntry          // Console logs during this request
    DatabaseOps  []*DatabaseOperation // SQL queries during this request
}
```

The logger is wired into the dispatcher: when a handler job starts, `StartRequest()` is
called; when it finishes, `FinishRequest()` records status and duration.  During
execution, `AddLog()` and `AddDatabaseOp()` attach console output and SQL operations
to the active request.

---

## 10. Styling Architecture

**File:** `pkg/web/static/css/app.css` (403 lines)

The IDE uses a **dark theme** built on Bootstrap 5.3's dark mode:

- **Color scheme:** `--editor-bg: #1e1e1e`, `--console-bg: #0d1117`
- **Fonts:** JetBrains Mono, Fira Code, Monaco, Menlo (monospace stack)
- **Custom scrollbars** styled for dark theme
- **CodeMirror overrides:** cursor color, selection, syntax token colors
- **REPL classes:** `.repl-input`, `.repl-error`, `.repl-result`, `.repl-log`
- **Status indicator:** pulse animation for running state
- **Responsive:** `@media (max-width: 768px)` collapses to single column

The admin pages (`logs.css`, `globalstate.css`) have their own standalone stylesheets
that do not share tokens or variables with the main app CSS.

---

## 11. Data Persistence

### Execution Repository

All code executions from the playground/REPL are stored in SQLite:

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | TEXT | UUID identifying the execution |
| `code` | TEXT | The JavaScript source code |
| `result` | TEXT | JSON-serialized return value |
| `console_log` | TEXT | JSON array of console output lines |
| `error` | TEXT | Error message (empty if successful) |
| `source` | TEXT | "api", "mcp", or "file" |
| `created_at` | TIMESTAMP | When the execution occurred |

### Global State

The `globalState` JavaScript object is persistent **in-memory** across executions within
a single server session.  It can be inspected and modified via the admin UI.  It does
**not** survive server restarts.

### Request Logs

Request logs are stored **in-memory only** in the `RequestLogger` circular buffer
(capacity 100).  They are not persisted to disk.

---

## 12. File Reference Table

| File | Lines | Role |
|------|-------|------|
| `cmd/jesus/cmd/serve.go` | 276 | CLI serve command — startup, port discovery, server launch |
| `pkg/api/execute.go` | 107 | `/v1/execute` endpoint — bridge between HTTP and engine |
| `pkg/engine/engine.go` | ~450 | Core engine — Goja runtime, job queue, handler registry |
| `pkg/engine/dispatcher.go` | 205 | Single-threaded job processor |
| `pkg/engine/bindings.go` | 241 | `db`, `console`, `globalState`, `JSON` bindings |
| `pkg/engine/http_bindings.go` | 288 | Express.js-compatible `app`, `req`, `res` bindings |
| `pkg/engine/request_logger.go` | 365 | In-memory request/execution logging |
| `pkg/web/routes.go` | 70 | Router setup for both servers |
| `pkg/web/handlers.templ.go` | 270 | Page handlers and static file serving |
| `pkg/web/admin.go` | 93 | Admin handler struct and routing |
| `pkg/web/admin/logs.go` | 165 | Logs REST API endpoints |
| `pkg/web/admin/sse.go` | 128 | SSE real-time update broadcasting |
| `pkg/web/admin/globalstate.go` | 64 | Global state GET/POST endpoints |
| `pkg/web/docs.go` | 316 | Preset examples and documentation data |
| `pkg/web/docsapi.go` | ~50 | Docs API endpoint |
| `pkg/web/templates/base.templ` | 94 | HTML shell — navbar, CDN resources |
| `pkg/web/templates/playground.templ` | 223 | Playground page layout |
| `pkg/web/templates/repl.templ` | 107 | REPL page layout |
| `pkg/web/templates/history.templ` | 244 | History page with filtering/pagination |
| `pkg/web/templates/docs.templ` | 111 | Documentation browser |
| `pkg/web/templates/admin.templ` | 330 | Request log dashboard components |
| `pkg/web/static/js/app.js` | 615 | Client-side JS — JSPlaygroundApp class |
| `pkg/web/static/css/app.css` | 403 | Global dark-theme styles |
| `pkg/web/static/admin/logs.html` | 75 | Admin logs standalone HTML |
| `pkg/web/static/admin/logs.js` | 493 | Admin logs client logic + SSE |
| `pkg/web/static/admin/logs.css` | 441 | Admin logs styles |
| `pkg/web/static/admin/globalstate.html` | 74 | Global state editor HTML |
| `pkg/web/static/admin/globalstate.js` | 121 | Global state editor logic |
| `pkg/web/static/admin/globalstate.css` | 231 | Global state editor styles |

---

## 13. Technology Stack Summary

| Layer | Technology | Version |
|-------|-----------|---------|
| **Backend language** | Go | — |
| **JS runtime** | Goja (ES5.1+) | — |
| **HTTP router** | gorilla/mux | — |
| **CLI framework** | Glazed | — |
| **Template engine** | Templ | — |
| **Markdown** | Goldmark + extensions | — |
| **Database** | SQLite (via Go driver) | — |
| **Frontend framework** | Bootstrap | 5.3.0 |
| **Code editor** | CodeMirror | 6.65.7 |
| **Icons** | Bootstrap Icons | 1.11.0 |
| **CDN** | jsdelivr, cdnjs | — |

---

## 14. Glossary

| Term | Definition |
|------|-----------|
| **Admin Server** | The server on port 9090 hosting the IDE UI and admin tools. |
| **JS Web Server** | The server on port 9922 serving routes registered by JavaScript. |
| **Dispatcher** | The single goroutine that sequentially executes EvalJobs. |
| **EvalJob** | A unit of work submitted to the engine (code string or handler invocation). |
| **EvalResult** | The return value + console output + error from a JavaScript execution. |
| **Handler** | A JavaScript function registered via `app.get()` etc. to serve HTTP traffic. |
| **Global State** | A persistent in-memory JavaScript object shared across all executions. |
| **RequestLogger** | In-memory circular buffer tracking the last 100 HTTP requests. |
| **Goja** | A pure-Go JavaScript runtime implementing ECMAScript 5.1+. |
| **Templ** | A typed Go template language that compiles to Go code for server-side HTML rendering. |
