---
title: "Jesus IDE — Notebook UI & System 7 Theme Design"
doc-type: design-doc
ticket: JESUS-IDE-01
topics:
  - javascript
  - architecture
  - refactor
status: active
created: 2026-03-16
---

# Jesus IDE — Notebook UI & System 7 Theme Design

This document specifies the visual design and component architecture for the Jesus IDE
frontend rewrite.  The design draws directly from the CozoDB Editor
(`~/code/wesen/2026-03-14--cozodb-editor`) which implements a Classic Macintosh System 7
aesthetic for a notebook-style query editor.

The core idea: **replace the current split-pane REPL/Playground with a single
notebook interface** where each cell is a System 7-style window card containing an
editor, and output appears inline beneath it — like Jupyter, but with Classic Mac
chrome.

---

## 1. Design Language: Classic Macintosh System 7

### 1.1 Why System 7

The System 7 aesthetic works well for a developer tool because:

- **Hard pixel borders** make panels and regions instantly visually distinct.
- **No gradients or blur** — everything is crisp and legible, even at small sizes.
- **High contrast** — black-on-white with carefully chosen accent colors.
- **Information density** — small font sizes (11–13px) with tight spacing let you see
  more code and output at once.
- **Nostalgic charm** — it looks distinctive and memorable, not generic.

### 1.2 Reference Implementation

The CozoDB Editor has already implemented this design language.  These are the key
CSS files to reference:

| File | What It Defines |
|------|----------------|
| `frontend/src/theme/tokens.css` | All CSS custom properties (colors, borders, shadows, fonts) |
| `frontend/src/theme/layout.css` | Window chrome, menubar, buttons, scrollbars |
| `frontend/src/theme/cards.css` | AI cards, diagnosis cards, code panels, query result tables |
| `frontend/src/notebook/notebook.css` | Cell cards, editor, markdown preview, status badges, empty states |

All paths relative to `~/code/wesen/2026-03-14--cozodb-editor/`.

---

## 2. Token System

The entire theme is driven by CSS custom properties scoped to a `.mac-desktop` root
class.  Jesus should adopt this same system, extended with tokens specific to
JavaScript execution.

### 2.1 Complete Token Reference

```css
/* ===== Jesus IDE — System 7 Design Tokens ===== */

.mac-desktop {
    /* ── Desktop & Window Chrome ── */
    --bg-desktop:        #a8a8a8;
    --bg-window:         #ffffff;
    --bg-titlebar:       linear-gradient(180deg, #fff 0%, #ddd 50%, #bbb 100%);
    --bg-titlebar-stripe:#000;
    --bg-field:          #ffffff;

    /* ── Semantic Backgrounds ── */
    --bg-code:           #f5f5f0;   /* Code cell editor background */
    --bg-result:         #f0f8f0;   /* Successful execution result (light green) */
    --bg-error:          #fff0f0;   /* Error output (light red) */
    --bg-console:        #fffff0;   /* Console output (cream) */
    --bg-warning:        #fffff0;
    --bg-main:           #ffffff;

    /* ── Text Colors ── */
    --text-primary:      #000000;
    --text-secondary:    #333333;
    --text-muted:        #666666;
    --text-code:         #222222;
    --text-error:        #cc0000;
    --text-warning:      #886600;
    --text-line-num:     #999999;

    /* ── Borders ── */
    --border-window:     #000000;   /* Hard black window borders */
    --border-subtle:     #cccccc;
    --border-field:      #999999;
    --border-code:       #ccccbb;
    --border-error:      #cc6666;
    --border-error-dim:  #ddaaaa;
    --border-result:     #88bb88;
    --border-console:    #bbbb88;

    /* ── Accent & Links ── */
    --accent:            #000000;
    --accent-dim:        #666666;
    --accent-highlight:  #0066cc;

    /* ── Shadows ── */
    --shadow-window:     2px 2px 0px #000;
    --shadow-btn:        1px 1px 0px #000;

    /* ── Typography ── */
    font-family: "IBM Plex Sans", "Geneva", "Helvetica", sans-serif;
    font-size: 13px;
    line-height: 1.4;

    /* ── Desktop dither pattern ── */
    min-height: 100vh;
    background: var(--bg-desktop);
    background-image:
        repeating-conic-gradient(#a8a8a8 0% 25%, #b0b0b0 0% 50%) 0 0 / 4px 4px;
    color: var(--text-primary);
}
```

### 2.2 Mapping Current Jesus CSS → System 7 Tokens

| Current Hard-Coded Value | Becomes Token | Usage |
|--------------------------|---------------|-------|
| Bootstrap's `data-bs-theme="dark"` | `.mac-desktop` root class | Page wrapper |
| `#1e1e1e` (editor background) | `var(--bg-code)` = `#f5f5f0` | Code cell background |
| `#0d1117` (console background) | `var(--bg-console)` = `#fffff0` | Console output |
| `text-success`, `text-danger` Bootstrap classes | `var(--text-error)`, `.is-ok`/`.is-error` badges | Status indicators |
| `.btn-outline-primary`, etc. | `.mac-btn` with hover inversion | All buttons |
| Bootstrap card/card-header | `.mac-window` + `.mac-window__titlebar` | Cell containers |
| CDN Bootstrap 5.3 | **Removed entirely** | — |

---

## 3. Notebook Paradigm

### 3.1 What Changes from Current IDE

| Current | Notebook |
|---------|----------|
| Separate Playground and REPL pages | Single notebook page with cells |
| One big editor + one output panel | Each cell has its own editor + inline output |
| Full page reload to switch views | All cells live on one scrollable page |
| Output replaces previous output | Output stacks beneath each cell, collapse/expand |
| REPL is a separate mode | A cell *is* a REPL entry; you can add more cells |
| History is a separate page | The notebook IS the history — past cells + outputs |
| Code examples load via localStorage hack | Code examples insert new cells |
| "Run" and "Execute & Store" are separate | Every cell has Run; the notebook auto-persists |

### 3.2 Cell Types

The Jesus notebook has two cell types (matching CozoDB):

#### Code Cell

```
┌─────────────────────────────────────────────────────────────────┐
│ □  [3] CODE  ●complete  12ms                   [Run] [+Code] [+MD] │
├─────────────────────────────────────────────────────────────────┤
│ app.get("/hello", (req, res) => {                               │
│     const count = db.query("SELECT COUNT(*) as n FROM users");  │
│     res.json({ message: "Hello!", users: count[0].n });         │
│ });                                                             │
│                                                                 │
│ ┌─ Console ─────────────────────────────────────────────────┐   │
│ │  Registered route: GET /hello                             │   │
│ └───────────────────────────────────────────────────────────┘   │
│                                                                 │
│ ┌─ Result ──────────────────────────────────────────────────┐   │
│ │  { message: "Hello!", users: 42 }                         │   │
│ └───────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

Components:
- **Title bar**: close button, execution count `[N]`, "CODE" label, status badge, timing, action buttons
- **Editor**: monospace textarea with 1px black border, focus ring
- **Console output**: cream background (`--bg-console`), shows `console.log` output
- **Result**: green-tinted background (`--bg-result`), shows return value
- **Error**: red-tinted background (`--bg-error`), shows error message with header

#### Markdown Cell

```
┌─────────────────────────────────────────────────────────────────┐
│ □  MARKDOWN                                     [+Code] [+MD]  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ## API Setup                                                   │
│                                                                 │
│  This section registers the Express routes for our REST API.    │
│  Use `db.query()` for read operations and `db.execute()` for   │
│  writes.                                                        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

In **preview mode** (default): rendered markdown with dashed border, click to edit.
In **edit mode**: raw textarea, Escape to preview.

### 3.3 Notebook Layout

```
┌──────────────────────────────────────────────────────────────────┐
│ 🍎 Jesus  File  Edit  Cell  View  Help              ● Connected  │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌────────────────── Untitled Notebook ──────────────────┐      │
│   │ □                                          [_]  [□]  │      │
│   ├──────────────────────────────────────────────────────┤      │
│   │                                                      │      │
│   │  ┌─ Cell 1: MARKDOWN ─────────────────────────────┐  │      │
│   │  │ # Hello World API                              │  │      │
│   │  │ A simple Express.js endpoint using the Jesus   │  │      │
│   │  │ JavaScript runtime.                            │  │      │
│   │  └────────────────────────────────────────────────┘  │      │
│   │                                                      │      │
│   │  ┌─ Cell 2: [1] CODE ●complete 12ms ──── [Run] ──┐  │      │
│   │  │ app.get("/hello", (req, res) => {              │  │      │
│   │  │     res.json({ message: "Hello, World!" });    │  │      │
│   │  │ });                                            │  │      │
│   │  │                                                │  │      │
│   │  │ ┌─ Result ─────────────────────────────────┐   │  │      │
│   │  │ │ undefined                                │   │  │      │
│   │  │ └─────────────────────────────────────────┘   │  │      │
│   │  └────────────────────────────────────────────────┘  │      │
│   │                                                      │      │
│   │  ┌─ Cell 3: [2] CODE ●error ────── [Run] ────────┐  │      │
│   │  │ db.query("SELEC * FROM users");                │  │      │
│   │  │                                                │  │      │
│   │  │ ┌─ ERROR ──────────────────────────────────┐   │  │      │
│   │  │ │ near "SELEC": syntax error               │   │  │      │
│   │  │ └─────────────────────────────────────────┘   │  │      │
│   │  └────────────────────────────────────────────────┘  │      │
│   │                                                      │      │
│   │         [ + Code ]  [ + Markdown ]                   │      │
│   │                                                      │      │
│   └──────────────────────────────────────────────────────┘      │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

### 3.4 Menubar

The top menubar replaces the Bootstrap navbar.  It uses the System 7 menubar style:
20px tall, white background, hard black 2px bottom border.

```css
.mac-menubar {
    position: sticky;
    top: 0;
    z-index: 100;
    height: 20px;
    background: #fff;
    border-bottom: 2px solid #000;
    display: flex;
    align-items: center;
    padding: 0 8px;
    font-size: 12px;
    font-weight: 700;
    gap: 16px;
}

.mac-menubar__item:hover {
    background: #000;
    color: #fff;
    padding: 0 6px;
}
```

**Menu items:**

| Menu | Items |
|------|-------|
| **Jesus** (bold, leftmost) | About, Preferences |
| **File** | New Notebook, Open, Save, Export |
| **Edit** | Undo, Redo, Cut, Copy, Paste |
| **Cell** | Run Cell (Shift+Enter), Run All, Insert Code Below, Insert Markdown Below, Delete Cell, Move Up, Move Down |
| **View** | Toggle Quick Reference, Toggle Admin Panel, Show Registered Routes |
| **Help** | Documentation, Keyboard Shortcuts, About Runtime |

Right side: status indicator (`● Connected` / `○ Disconnected`).

### 3.5 Cell Execution States

Each code cell shows a status badge in its title bar.  The CozoDB editor already
defines exactly the right set:

```css
/* Status badge base */
.mac-cell-status {
    font-size: 10px;
    padding: 0 6px;
    border: 1px solid #999;
    background: #eee;
    font-weight: 500;
}

/* Variants */
.mac-cell-status.is-ok      { border-color: #66aa66; background: #e0f0e0; color: #336633; }
.mac-cell-status.is-error   { border-color: #cc6666; background: #f0e0e0; color: #993333; }
.mac-cell-status.is-dirty   { border-color: #c28a3a; background: #f6ecd2; color: #8a5a00; }
.mac-cell-status.is-running { border-color: #3377aa; background: #e0eef8; color: #225588; }
```

| Badge | Meaning |
|-------|---------|
| `idle` (gray) | Cell has never been run |
| `running` (blue) | Execution in progress |
| `complete` (green) | Last run succeeded |
| `error` (red) | Last run failed |
| `dirty` (amber) | Code changed since last run |

### 3.6 Active Cell Highlight

The currently focused cell gets an emphasized drop shadow, exactly like CozoDB:

```css
.mac-cell-card.is-active {
    box-shadow: 3px 3px 0px #000;
}
```

Keyboard navigation:
- **j / ArrowDown**: move to next cell
- **k / ArrowUp**: move to previous cell
- **Enter**: focus editor in current cell
- **Escape**: exit editor, return to cell navigation
- **Shift+Enter**: run current cell

---

## 4. Component Architecture

### 4.1 Component Tree

```
<App>
  <div className="mac-desktop">
    <Menubar />
    <div className="mac-window" data-widget="notebook">
      <WindowTitlebar title="Untitled Notebook" />
      <NotebookBody>
        {cells.map(cell =>
          <NotebookCellCard
            key={cell.id}
            cell={cell}
            isActive={activeIndex === i}
            runtime={runtimes[cell.id]}
          >
            {cell.kind === 'code' ? (
              <>
                <CellEditor value={cell.source} onChange={...} />
                <CellConsoleOutput logs={runtime.consoleLog} />
                <CellResult value={runtime.result} />
                <CellError error={runtime.error} />
              </>
            ) : (
              <MarkdownPreview source={cell.source} onEdit={...} />
            )}
          </NotebookCellCard>
        )}
        <AddCellButtons />
      </NotebookBody>
    </div>
    <QuickReferencePanel />   {/* slide-in side panel */}
  </div>
</App>
```

### 4.2 Module Structure

```
frontend/src/
├── main.tsx
├── App.tsx
├── App.css                      (minimal — just body resets)
├── theme/
│   ├── tokens.css               ← System 7 design tokens
│   ├── layout.css               ← Window chrome, menubar, buttons
│   └── cards.css                ← Output cards, error cards, result tables
├── store/
│   ├── index.ts                 ← configureStore
│   ├── api.ts                   ← RTK Query API definition
│   ├── websocketMiddleware.ts
│   └── slices/
│       ├── notebookSlice.ts     ← cells[], activeIndex, execution state
│       └── uiSlice.ts           ← panels, preferences
├── notebook/
│   ├── index.ts
│   ├── NotebookPage.tsx         ← Top-level page (menubar + notebook window)
│   ├── NotebookBody.tsx         ← Cell list + add buttons
│   ├── NotebookCellCard.tsx     ← Single cell wrapper (window chrome)
│   ├── notebook.css             ← Cell card, editor, markdown styles
│   ├── parts.ts                 ← data-part constants
│   └── components/
│       ├── Menubar.tsx
│       ├── WindowTitlebar.tsx
│       ├── CellEditor.tsx       ← Textarea with monospace, focus ring
│       ├── CellConsoleOutput.tsx ← Console log display
│       ├── CellResult.tsx       ← Result value display
│       ├── CellError.tsx        ← Error card with header
│       ├── MarkdownPreview.tsx  ← Rendered markdown (click-to-edit)
│       ├── StatusBadge.tsx      ← ok/error/dirty/running badge
│       ├── AddCellButtons.tsx   ← [+Code] [+Markdown] buttons
│       └── QuickReferencePanel.tsx
├── admin/                        ← Admin dashboard (separate route)
│   ├── AdminPage.tsx
│   ├── components/
│   │   ├── RequestLogList.tsx
│   │   ├── StatsCards.tsx
│   │   └── GlobalStateEditor.tsx
│   └── admin.css
├── hooks/
│   ├── useWebSocket.ts
│   ├── useKeyboardNav.ts       ← j/k/Enter/Escape/Shift+Enter
│   └── useLocalStorage.ts
└── styles/
    └── reset.css                ← Minimal normalize
```

### 4.3 Parts Manifest

```typescript
// frontend/src/notebook/parts.ts

export const NOTEBOOK_PARTS = {
    // Desktop
    desktop:          'desktop',
    menubar:          'menubar',
    menuItem:         'menu-item',
    statusIndicator:  'status-indicator',

    // Notebook window
    notebook:         'notebook',
    notebookTitlebar: 'notebook-titlebar',
    notebookBody:     'notebook-body',

    // Cell
    cell:             'cell',
    cellTitlebar:     'cell-titlebar',
    cellLabel:        'cell-label',
    cellStatus:       'cell-status',
    cellBody:         'cell-body',
    cellEditor:       'cell-editor',
    cellOutput:       'cell-output',

    // Output regions
    consoleOutput:    'console-output',
    resultOutput:     'result-output',
    errorCard:        'error-card',
    errorHeader:      'error-header',
    errorBody:        'error-body',

    // Markdown
    markdownPreview:  'markdown-preview',

    // Actions
    addCellButtons:   'add-cell-buttons',
    quickReference:   'quick-reference',
} as const;
```

---

## 5. CSS Implementation

### 5.1 Window Chrome (from CozoDB `layout.css`)

Every cell is rendered inside a `.mac-window`, which provides the System 7 look:

```css
/* Window container */
.mac-window {
    border: 2px solid var(--border-window);
    background: var(--bg-window);
    box-shadow: var(--shadow-window);   /* 2px 2px 0px #000 */
}

/* Title bar with gradient + stripe overlay */
.mac-window__titlebar {
    height: 20px;
    background: var(--bg-titlebar);
    border-bottom: 2px solid var(--border-window);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 4px;
    position: relative;
    overflow: hidden;
    font-size: 11px;
}

/* System 7 horizontal scan lines */
.mac-window__titlebar::before {
    content: "";
    position: absolute;
    inset: 0;
    background-image:
        repeating-linear-gradient(
            0deg,
            transparent,
            transparent 1px,
            #000 1px,
            #000 2px,
            transparent 2px,
            transparent 3px
        );
    background-size: 100% 20px;
    opacity: 0.08;
    pointer-events: none;
}

/* Close button — the small box in the top left */
.mac-window__close {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 1px solid var(--border-window);
    background: var(--bg-window);
    cursor: pointer;
    flex-shrink: 0;
}

.mac-window__close:hover {
    background: #000;
}
```

### 5.2 Buttons

All buttons use the `.mac-btn` class — simple, crisp, invertable:

```css
.mac-btn {
    font-family: inherit;
    font-size: 11px;
    padding: 1px 8px;
    border: 1px solid #000;
    border-radius: 3px;
    background: #fff;
    color: #000;
    cursor: pointer;
    box-shadow: var(--shadow-btn);   /* 1px 1px 0px #000 */
}

.mac-btn:hover:not(:disabled) {
    background: #000;
    color: #fff;
}

.mac-btn:active:not(:disabled) {
    transform: translate(1px, 1px);
    box-shadow: none;
}

.mac-btn:disabled {
    color: #999;
    border-color: #999;
    box-shadow: none;
    cursor: not-allowed;
}
```

### 5.3 Cell Editor

The editor is a plain `<textarea>` (not CodeMirror) for simplicity in the initial
version.  The System 7 style gives it personality through borders and focus states:

```css
.mac-cell-editor {
    width: 100%;
    resize: vertical;
    border: 1px solid #000;
    background: var(--bg-field);
    color: var(--text-primary);
    padding: 8px;
    font-family: "IBM Plex Mono", monospace;
    font-size: 13px;
    line-height: 1.5;
    outline: none;
}

.mac-cell-editor:focus {
    box-shadow: 0 0 0 2px #000 inset;   /* Thick inset focus ring */
}
```

Later, CodeMirror 6 can be swapped in for syntax highlighting — the wrapper just
needs to receive the same CSS variables and maintain the same border/focus treatment.

### 5.4 Output Cards

Console output, results, and errors each have a distinct background color:

```css
/* Console output — cream */
.mac-console-output {
    margin-top: 8px;
    padding: 6px 8px;
    background: var(--bg-console);        /* #fffff0 */
    border: 1px solid var(--border-console);
    font-family: "IBM Plex Mono", monospace;
    font-size: 12px;
    line-height: 1.5;
    white-space: pre-wrap;
}

.mac-console-output__label {
    font-size: 10px;
    font-weight: 600;
    color: var(--text-muted);
    letter-spacing: 0.05em;
    margin-bottom: 2px;
}

/* Result — light green */
.mac-result-output {
    margin-top: 4px;
    padding: 6px 8px;
    background: var(--bg-result);         /* #f0f8f0 */
    border: 1px solid var(--border-result);
    font-family: "IBM Plex Mono", monospace;
    font-size: 12px;
    line-height: 1.5;
    white-space: pre-wrap;
}

/* Error — light red with header bar */
.mac-cell-error {
    margin-top: 8px;
    border: 1px solid var(--border-error);
    background: var(--bg-error);          /* #fff0f0 */
}

.mac-cell-error__header {
    padding: 4px 8px;
    font-size: 11px;
    font-weight: 700;
    letter-spacing: 0.05em;
    color: var(--text-error);             /* #cc0000 */
    background: var(--bg-error-header);   /* #ffdddd */
    border-bottom: 1px solid var(--border-error-dim);
}

.mac-cell-error__body {
    padding: 8px;
    white-space: pre-wrap;
    font-size: 12px;
    line-height: 1.5;
}
```

---

## 6. Key React Components — Pseudocode

### 6.1 NotebookPage

```tsx
function NotebookPage() {
    const dispatch = useAppDispatch();
    const { cells, activeIndex } = useAppSelector(s => s.notebook);
    const [executeCode] = useExecuteCodeMutation();

    // Keyboard navigation
    useKeyboardNav({
        onNext: () => dispatch(setActiveIndex(Math.min(activeIndex + 1, cells.length - 1))),
        onPrev: () => dispatch(setActiveIndex(Math.max(activeIndex - 1, 0))),
        onRun:  () => handleRunCell(cells[activeIndex].id),
    });

    async function handleRunCell(cellId: string) {
        const cell = cells.find(c => c.id === cellId);
        if (!cell || cell.kind !== 'code') return;

        dispatch(setCellStatus({ cellId, status: 'running' }));
        try {
            const result = await executeCode({
                code: cell.source,
                options: { store: true },
            }).unwrap();

            dispatch(setCellRuntime({
                cellId,
                runtime: {
                    status: result.ok ? 'complete' : 'error',
                    result: result.data.result,
                    consoleLog: result.data.consoleLog,
                    error: result.error,
                    sessionId: result.data.sessionId,
                    durationMs: result.meta.duration_ms,
                    executionCount: nextCount(),
                },
            }));
        } catch (err) {
            dispatch(setCellRuntime({
                cellId,
                runtime: { status: 'error', error: err.message },
            }));
        }
    }

    return (
        <div className="mac-desktop">
            <Menubar onNewCode={() => dispatch(addCell('code'))}
                     onNewMarkdown={() => dispatch(addCell('markdown'))}
                     onRunAll={() => cells.forEach(c => handleRunCell(c.id))} />
            <div className="mac-notebook-chrome">
                <div className="mac-window" data-widget="notebook">
                    <WindowTitlebar title="Untitled Notebook" />
                    <div className="mac-notebook-body">
                        {cells.length === 0 ? (
                            <EmptyState
                                onAddCode={() => dispatch(addCell('code'))}
                                onAddMarkdown={() => dispatch(addCell('markdown'))}
                            />
                        ) : (
                            cells.map((cell, i) => (
                                <NotebookCellCard
                                    key={cell.id}
                                    cell={cell}
                                    cellIndex={i}
                                    isActive={activeIndex === i}
                                    runtime={runtimes[cell.id]}
                                    onRun={handleRunCell}
                                    onFocus={(idx) => dispatch(setActiveIndex(idx))}
                                    onChangeSource={(id, src) => dispatch(updateCellSource({ id, source: src }))}
                                    onDelete={(id) => dispatch(deleteCell(id))}
                                    onInsertCodeBelow={(id) => dispatch(insertCellAfter({ afterId: id, kind: 'code' }))}
                                    onInsertMarkdownBelow={(id) => dispatch(insertCellAfter({ afterId: id, kind: 'markdown' }))}
                                />
                            ))
                        )}
                        <AddCellButtons
                            onAddCode={() => dispatch(addCell('code'))}
                            onAddMarkdown={() => dispatch(addCell('markdown'))}
                        />
                    </div>
                </div>
            </div>
        </div>
    );
}
```

### 6.2 NotebookCellCard

```tsx
function NotebookCellCard({
    cell, cellIndex, isActive, runtime,
    onRun, onFocus, onChangeSource, onDelete,
    onInsertCodeBelow, onInsertMarkdownBelow,
}: NotebookCellCardProps) {
    const [editing, setEditing] = useState(cell.kind === 'code');
    const editorRef = useRef<HTMLTextAreaElement>(null);
    const isCode = cell.kind === 'code';

    const statusClass =
        runtime?.status === 'complete' ? 'is-ok' :
        runtime?.status === 'error' ? 'is-error' :
        runtime?.status === 'running' ? 'is-running' : '';

    function handleKeyDown(e: React.KeyboardEvent) {
        if (e.key === 'Enter' && e.shiftKey && isCode) {
            e.preventDefault();
            onRun(cell.id);
        }
        if (e.key === 'Escape' && cell.kind === 'markdown') {
            setEditing(false);
        }
    }

    return (
        <div
            className={`mac-window mac-cell-card ${isActive ? 'is-active' : ''}`}
            data-part="cell"
            data-state={runtime?.status || 'idle'}
            onClick={() => onFocus(cellIndex)}
        >
            {/* Title bar */}
            <div className="mac-window__titlebar" data-part="cell-titlebar">
                <div className="mac-window__titlebar-left">
                    <span className="mac-window__close"
                          onClick={e => { e.stopPropagation(); onDelete(cell.id); }} />
                    <span className="mac-cell-label" data-part="cell-label">
                        {isCode ? `[${runtime?.executionCount ?? ' '}]` : ''}
                        {' '}{cell.kind.toUpperCase()}
                    </span>
                    {isCode && runtime?.status ? (
                        <span className={`mac-cell-status ${statusClass}`}
                              data-part="cell-status">
                            {runtime.status}
                        </span>
                    ) : null}
                    {runtime?.durationMs ? (
                        <span className="mac-cell-timestamp">
                            {runtime.durationMs}ms
                        </span>
                    ) : null}
                </div>
                <div className="mac-window__titlebar-right">
                    {isCode ? (
                        <button className="mac-btn" onClick={e => {
                            e.stopPropagation(); onRun(cell.id);
                        }}>Run</button>
                    ) : null}
                    <button className="mac-btn" onClick={e => {
                        e.stopPropagation(); onInsertCodeBelow(cell.id);
                    }}>+Code</button>
                    <button className="mac-btn" onClick={e => {
                        e.stopPropagation(); onInsertMarkdownBelow(cell.id);
                    }}>+MD</button>
                </div>
            </div>

            {/* Cell body */}
            <div className="mac-cell-body" data-part="cell-body">
                {cell.kind === 'markdown' && !editing ? (
                    <MarkdownPreview
                        source={cell.source}
                        onClick={() => { setEditing(true); onFocus(cellIndex); }}
                    />
                ) : (
                    <CellEditor
                        ref={editorRef}
                        value={cell.source}
                        onChange={src => onChangeSource(cell.id, src)}
                        onKeyDown={handleKeyDown}
                        placeholder={isCode
                            ? '// Enter JavaScript... (Shift+Enter to run)'
                            : 'Enter markdown... (Escape to preview)'}
                        rows={isCode ? 5 : 4}
                    />
                )}

                {/* Inline output */}
                {runtime?.consoleLog?.length ? (
                    <CellConsoleOutput logs={runtime.consoleLog} />
                ) : null}

                {runtime?.status === 'complete' && runtime.result !== undefined ? (
                    <CellResult value={runtime.result} />
                ) : null}

                {runtime?.status === 'error' && runtime.error ? (
                    <CellError error={runtime.error} />
                ) : null}
            </div>
        </div>
    );
}
```

### 6.3 notebookSlice

```typescript
// frontend/src/store/slices/notebookSlice.ts

interface NotebookCell {
    id: string;
    kind: 'code' | 'markdown';
    source: string;
}

interface CellRuntimeState {
    status: 'idle' | 'running' | 'complete' | 'error';
    result?: unknown;
    consoleLog?: string[];
    error?: string;
    sessionId?: string;
    durationMs?: number;
    executionCount?: number;
}

interface NotebookState {
    cells: NotebookCell[];
    activeIndex: number;
    runtimes: Record<string, CellRuntimeState>;
    executionCounter: number;
    title: string;
}

const initialState: NotebookState = {
    cells: [
        {
            id: nanoid(),
            kind: 'markdown',
            source: '# Welcome to Jesus Notebook\nWrite JavaScript, run it, see results inline.',
        },
        {
            id: nanoid(),
            kind: 'code',
            source: '// Try: app.get("/hello", (req, res) => res.json({ ok: true }));\n',
        },
    ],
    activeIndex: 1,
    runtimes: {},
    executionCounter: 0,
    title: 'Untitled Notebook',
};

export const notebookSlice = createSlice({
    name: 'notebook',
    initialState,
    reducers: {
        addCell: (state, action: PayloadAction<'code' | 'markdown'>) => {
            state.cells.push({
                id: nanoid(),
                kind: action.payload,
                source: '',
            });
            state.activeIndex = state.cells.length - 1;
        },

        insertCellAfter: (state, action: PayloadAction<{
            afterId: string;
            kind: 'code' | 'markdown';
            source?: string;
        }>) => {
            const idx = state.cells.findIndex(c => c.id === action.payload.afterId);
            const newCell = {
                id: nanoid(),
                kind: action.payload.kind,
                source: action.payload.source || '',
            };
            state.cells.splice(idx + 1, 0, newCell);
            state.activeIndex = idx + 1;
        },

        deleteCell: (state, action: PayloadAction<string>) => {
            const idx = state.cells.findIndex(c => c.id === action.payload);
            state.cells.splice(idx, 1);
            delete state.runtimes[action.payload];
            if (state.activeIndex >= state.cells.length) {
                state.activeIndex = Math.max(0, state.cells.length - 1);
            }
        },

        updateCellSource: (state, action: PayloadAction<{ id: string; source: string }>) => {
            const cell = state.cells.find(c => c.id === action.payload.id);
            if (cell) {
                cell.source = action.payload.source;
                // Mark as dirty if previously run
                const rt = state.runtimes[action.payload.id];
                if (rt && rt.status === 'complete') {
                    rt.status = 'idle'; // or a 'dirty' status
                }
            }
        },

        setActiveIndex: (state, action: PayloadAction<number>) => {
            state.activeIndex = action.payload;
        },

        setCellStatus: (state, action: PayloadAction<{ cellId: string; status: CellRuntimeState['status'] }>) => {
            if (!state.runtimes[action.payload.cellId]) {
                state.runtimes[action.payload.cellId] = { status: 'idle' };
            }
            state.runtimes[action.payload.cellId].status = action.payload.status;
        },

        setCellRuntime: (state, action: PayloadAction<{ cellId: string; runtime: CellRuntimeState }>) => {
            state.runtimes[action.payload.cellId] = action.payload.runtime;
        },

        moveCellUp: (state, action: PayloadAction<string>) => {
            const idx = state.cells.findIndex(c => c.id === action.payload);
            if (idx > 0) {
                [state.cells[idx - 1], state.cells[idx]] = [state.cells[idx], state.cells[idx - 1]];
                state.activeIndex = idx - 1;
            }
        },

        moveCellDown: (state, action: PayloadAction<string>) => {
            const idx = state.cells.findIndex(c => c.id === action.payload);
            if (idx < state.cells.length - 1) {
                [state.cells[idx], state.cells[idx + 1]] = [state.cells[idx + 1], state.cells[idx]];
                state.activeIndex = idx + 1;
            }
        },
    },
});
```

---

## 7. Admin Dashboard in System 7 Style

The admin features (logs, global state) live at a separate `/admin` route but share
the same theme.  Instead of the current standalone HTML pages, they become React
components inside a System 7 window:

```
┌──────────────────────────────────────────────────────────────────┐
│ 🍎 Jesus  File  Admin                               ● Connected  │
├──────────────────────────────────────────────────────────────────┤
│                                                                  │
│   ┌──────────────── Admin: Request Logs ──────────────────┐      │
│   │ □                                          [_]  [□]  │      │
│   ├──────────────────────────────────────────────────────┤      │
│   │                                                      │      │
│   │  Total: 42  Success: 95%  Avg: 23ms  Errors: 2      │      │
│   │  ──────────────────────────────────────────────────  │      │
│   │  GET  /hello     200  12ms  just now                 │      │
│   │  POST /users     201  45ms  2s ago                   │      │
│   │  GET  /bad       404   5ms  15s ago                  │      │
│   │                                                      │      │
│   └──────────────────────────────────────────────────────┘      │
│                                                                  │
│   ┌──────────────── Admin: Global State ─────────────────┐      │
│   │ □                                          [_]  [□]  │      │
│   ├──────────────────────────────────────────────────────┤      │
│   │                                                      │      │
│   │  {                                                   │      │
│   │      "counter": 42,                                  │      │
│   │      "lastUser": "alice"                             │      │
│   │  }                                                   │      │
│   │                                                      │      │
│   │  [Save]  [Reset]  [ ] Auto-refresh                   │      │
│   │                                                      │      │
│   └──────────────────────────────────────────────────────┘      │
│                                                                  │
└──────────────────────────────────────────────────────────────────┘
```

---

## 8. Comparison: Current vs. Notebook UI

| Aspect | Current IDE | Notebook UI |
|--------|-------------|-------------|
| **Visual language** | Dark Bootstrap, generic | System 7, distinctive |
| **Layout** | Two-column: editor + output | Vertical cells, inline output |
| **Editor** | One global CodeMirror | Per-cell textarea (upgradable to CM6) |
| **Output location** | Separate panel on the right | Directly below each cell |
| **Multiple scripts** | One at a time | Multiple cells, run independently |
| **Documentation** | Separate /docs page | Markdown cells inline |
| **Navigation** | Navbar + page reloads | Menubar + keyboard (j/k) |
| **State persistence** | localStorage hacks | Redux store |
| **Theming** | Hard-coded dark theme | CSS custom properties, light theme |
| **Framework** | Bootstrap 5 + CDN CodeMirror | Pure CSS + optional CM6 |
| **Code examples** | Dropdown menu | Insert as new cell |
| **History** | Separate /history page | The notebook itself is history |

---

## 9. What Is NOT Ported (Out of Scope)

The following features from the current IDE are not needed in the notebook UI:

- **Separate REPL page**: cells replace the REPL
- **Separate History page**: cell outputs + execution counts serve as history
- **Bootstrap framework**: replaced by System 7 CSS
- **CDN dependencies**: all local via npm
- **Vim mode in CodeMirror**: can be added later when CM6 is wired
- **Notebook hydration/persistence**: out of scope per requirements — the notebook
  starts fresh each session (persistence can be added later via a save/load API)

---

## 10. Implementation Priority

1. **Theme tokens** (`tokens.css`, `layout.css`) — copy from CozoDB, adapt colors
2. **Menubar** component — simple, sets the visual tone
3. **NotebookCellCard** — the core visual unit
4. **CellEditor** (textarea) — minimal, just needs focus ring
5. **CellConsoleOutput**, **CellResult**, **CellError** — output cards
6. **MarkdownPreview** — click-to-edit markdown rendering
7. **notebookSlice** — Redux state for cells
8. **RTK Query execute mutation** — wire to `/api/v1/execute`
9. **Keyboard navigation** — j/k/Enter/Escape/Shift+Enter
10. **Admin components** — logs and global state in System 7 windows
