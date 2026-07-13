# skein (skein) — Design Specification

> Version: 0.1-draft  
> Status: Pre-implementation  
> Last updated: 2026-06

---

## 1. Elevator pitch

`skein` is a **clangd-powered codebase exploration tool** for C++ projects.  
It answers *"what does the world look like from inside this symbol?"* and
renders the answer as an interactive, navigable relationship map — either
as a terminal tree (fast) or a full TUI session (slow/deep).

Visual inspiration: **lazygit** — keyboard-first, panel-based, no mouse required.  
Engine: **clangd** over LSP stdio — we compose its answers, we don't reimplement them.

---

## 2. Vocabulary (the textile theme)

| Term     | Definition |
|----------|------------|
| **thread**  | The focal symbol currently at the center of the map. |
| **follow**  | The act of navigating from the current thread to a connected node, making it the new thread. |
| **spool**   | Navigation history. Ordered list of past threads. Supports back/forward. |
| **bundle**  | A named, persistent collection of pinned threads. Multiple bundles per session. |
| **tangle**  | The no-thread entry state. Shows workspace symbol search. |
| **ply**     | How many hops out from the thread the map renders. Default: 1. Max: 3. |
| **strand limit** | Maximum visible nodes per render. Default: 50. Truncates with warning. |
| **draw**    | Fast mode. One thread, ply 1, stdout tree output, process exits. |

---

## 3. Modes

### 3.1 Draw (fast mode)

Triggered by the `draw` subcommand. Fires 2–3 LSP calls, prints a static tree to
stdout, exits. Designed for scripts, CI, piping.

```bash
skein draw -m <method>           # thread = method, show definitions + call sites
skein draw -m <method> -c <class> # same, scoped to one class — disambiguates a
                                   # name declared by more than one class/namespace
skein draw -c <class>            # thread = class, show hierarchy + members
skein draw -f <file>             # thread = file, show all symbols defined within
skein draw -s <symbol>           # thread = any symbol, generic relationship map
```

Output is a `tree(1)`-style Unicode tree. Supports `--json` for machine
consumption. Colour auto-disabled when stdout is not a TTY.

**Method name ambiguity.** A bare `-m <method>` is resolved via
`workspace/symbol`, which can return candidates from multiple unrelated
classes/namespaces sharing the same method name — common in real-world
codebases (operator overloads, template instantiations, unrelated classes
both implementing a `Get`/`Put`/`push_back`-shaped interface). When that
happens, skein still picks one (preferring a candidate that resolves into a
concrete source-file definition over a bare declaration) but prints a
stderr warning naming the other containers found, and the picked one, so
the result isn't silently wrong. Pass `-c <class>` to scope the search and
make the choice deterministic instead of order-dependent.

### 3.2 TUI (slow mode)

Triggered by `skein` (no args) or `skein <symbol>` (symbol becomes first thread).

Opens a full bubbletea TUI. LSP calls are made lazily as the user navigates.
Nodes expand on demand. The map rebuilds around each new thread.

```bash
skein              # tangle view: fuzzy symbol search, pick thread interactively
skein foo          # TUI opens with foo as initial thread
skein foo bar      # TUI opens with foo as thread, bar pre-pinned in bundle
```

---

## 4. TUI layout

```
┌─────────────────────────────────────────────────────────────────┐
│ skein  ·  processFrame  [method]  ply:1  strands:12  spool:3     │  ← status bar
├─────────────────────────────────────────────────────────────────┤
│ [processFrame] [Pipeline] [+]                                   │  ← bundle tabs
├───────────────────────────┬─────────────────────────────────────┤
│                           │                                     │
│   processFrame [method]   │   > void Pipeline::processFrame(   │  ← detail panel
│   ├── defined in          │       RawBuffer& buf)               │
│   │   └── Pipeline.cpp:42 │                                     │
│   ├── called in (9)       │   Called by:                        │
│   │   ├── main.cpp:101    │     Scheduler::run() :77            │
│   │   ├── main.cpp:214    │                                     │
│   │   └── Scheduler.cpp:77│   Calls:                            │
│   ├── calls (3)           │     Buffer::acquire() :44           │
│   │   ├── acquire() ──────┼──▶  [follow with <enter>]          │
│   │   ├── format()        │                                     │
│   │   └── emit()          │                                     │
│   └── inherits             │                                     │
│       └── IProcessor      │                                     │
│                           │                                     │
├───────────────────────────┴─────────────────────────────────────┤
│ > _                                                             │  ← search bar
├─────────────────────────────────────────────────────────────────┤
│ <enter> follow  <tab> panel  [/] search  [p] pin  [u] up        │  ← key hints
└─────────────────────────────────────────────────────────────────┘
```

### Panels
- **Left**: relationship tree rooted at current thread
- **Right**: detail panel — symbol signature, hover info, selected node context
- **Bottom**: always-available symbol search bar (fuzzy, workspace-wide)
- **Top**: bundle tabs + status bar

---

## 5. Relationship directions

All three directions are rendered **simultaneously** in the map, colour-coded.
Shortcuts act as **filters** (show/hide), not mode switches — the full picture
is always the default.

| Direction  | Colour  | Example                          |
|------------|---------|----------------------------------|
| Incoming   | cyan    | who calls `foo`, who inherits `Bar` |
| Outgoing   | green   | what `foo` calls, what `Bar` inherits |
| Bidirectional | yellow | same symbol appears in both    |

**Shortcuts for direction filters:**

| Key | Action |
|-----|--------|
| `i` | Toggle incoming edges |
| `o` | Toggle outgoing edges |

---

## 6. Ply control

| Key | Action |
|-----|--------|
| `+` / `=` | Increase ply (max 3) |
| `-`        | Decrease ply (min 1) |

When the strand limit (default 50, override with `--strands N`) is reached,
truncated sections print a warning — e.g. `called-in truncated: showing 50 of
80 call sites (30 hidden)` — to stderr in draw mode, or in the TUI's status
bar. The cap is intentional — real codebases have methods called in hundreds
of places; showing all of them destroys the map. Each section (called-in,
calls, members, symbols) gets its own independent budget, so one high-traffic
list doesn't crowd out another section's visibility.

---

## 7. Navigation & shortcuts

Lazygit-inspired. Keyboard-first. No mouse required.

### Global

| Key | Action |
|-----|--------|
| `q` / `ctrl+c` | Quit |
| `/` | Focus search bar |
| `esc` | Clear search / back to map |
| `?` | Toggle help overlay |
| `tab` | Cycle focus: left panel → right panel → search |

### Map navigation

| Key | Action |
|-----|--------|
| `j` / `↓` | Move selection down |
| `k` / `↑` | Move selection up |
| `enter` / `l` | **Follow** — selected node becomes new thread |
| `h` | Back (spool) |
| `ctrl+r` | Forward (spool) |
| `r` | Reset to first thread |
| `g` | Goto definition in `$EDITOR` |

### Bundle (multi-thread)

| Key | Action |
|-----|--------|
| `p` | Pin current thread to bundle (new tab) |
| `u` | Unpin current thread from bundle |
| `[` | Previous bundle tab |
| `]` | Next bundle tab |
| `x` | Close current bundle tab |

### Layout (v1: tabs only)

| Key | Action |
|-----|--------|
| `1`–`9` | Jump to bundle tab by number |

> **TODO (v2):** Split view — two threads side by side.  
> **TODO (v3):** Overlay mode — pinned threads highlighted in unified map.

### Ply & filters

| Key | Action |
|-----|--------|
| `+` / `=` | Ply +1 (max 3) |
| `-` | Ply -1 (min 1) |
| `i` | Toggle incoming edges |
| `o` | Toggle outgoing edges |

### Draw mode only

| Flag | Action |
|------|--------|
| `--json` | JSON output |
| `--no-color` | Disable ANSI colour |
| `--absolute` | Absolute file paths |
| `--ply N` | Override default ply (1) |
| `--strands N` | Override default strand limit (50) |
| `--db <path>` | Path to compile_commands.json |
| `--clangd <path>` | clangd binary (default: $PATH) |

---

## 8. Multi-path find (TODO)

> **Status: not started. Planned for v2.**

The ability to find the *relationship path* between two symbols:

```bash
skein draw --path foo bar
# → foo calls baz() which is overridden by Bar::baz() which uses bar
```

In TUI: select a second thread and trigger pathfinding.  
This requires graph traversal over the LSP call hierarchy — non-trivial but
well-defined once the LSP client layer is solid.

---

## 9. Vim/Neovim integration (TODO)

> **Status: not started. Deferred deliberately.**

Point-query from cursor, result in floating window or quickfix list.  
When this grows beyond a single file, extract to a separate repository:
`skein.nvim` or `skein.vim`, consumed here as a git submodule.

---

## 10. Architecture

```
skein/
├── cmd/skein/
│   └── main.go              — entry point, mode dispatch (draw vs TUI)
├── internal/
│   ├── lsp/
│   │   ├── client.go        — clangd subprocess + JSON-RPC stdio transport
│   │   ├── protocol.go      — LSP types (Location, Symbol, etc.)
│   │   └── client_test.go
│   ├── compositor/
│   │   ├── types.go         — Thread, RelationMap, Relation, Spool, Bundle
│   │   ├── method.go        — MethodCompositor: definitions + call sites
│   │   ├── class.go         — ClassCompositor: hierarchy + members
│   │   ├── file.go          — FileCompositor: all symbols in file
│   │   └── compositor_test.go
│   ├── tree/
│   │   ├── printer.go       — draw mode: tree(1)-style stdout renderer
│   │   └── json.go          — draw mode: JSON renderer
│   └── tui/
│       ├── model.go         — bubbletea root model (state, Update, View)
│       ├── map.go           — left panel: relationship tree
│       ├── detail.go        — right panel: symbol detail
│       ├── search.go        — bottom search bar
│       ├── bundle.go        — tab bar + bundle management
│       ├── keys.go          — all keybindings in one place
│       └── styles.go        — lipgloss styles (colours, borders)
├── docs/
│   └── SPEC.md              — this file
├── go.mod
├── go.sum
└── README.md
```

---

## 11. LSP calls used

| LSP method | skein use |
|---|---|
| `initialize` | Handshake, capability negotiation |
| `workspace/symbol` | Fuzzy symbol search (tangle view + thread resolution) |
| `textDocument/references` | All usages of a symbol |
| `textDocument/definition` | Definition location |
| `textDocument/documentSymbol` | All symbols in a file |
| `callHierarchy/incomingCalls` | Who calls this function |
| `callHierarchy/outgoingCalls` | What this function calls |
| `typeHierarchy/supertypes` | Base classes |
| `typeHierarchy/subtypes` | Derived classes |
| `textDocument/hover` | Signature + doc comment for detail panel |

---

## 11a. Performance notes

- **Index warm-up.** clangd's background indexer is incremental: right after
  the LSP handshake, `workspace/symbol` queries can return a partial result
  (e.g. only a header declaration) with the concrete `.cpp` definition
  landing moments later. To avoid handing back a stale/incomplete snapshot,
  the first resolution of any symbol waits for the result count to stay
  stable for ~800ms before returning. Once a client's index has stabilised
  once, it's marked *warm* and every later `workspace/symbol` lookup (i.e.
  every follow/navigation for the rest of the session) short-circuits to a
  single round trip instead of repeating that wait — the fast path falls
  back to the full stabilisation loop only if a symbol genuinely isn't
  found yet (e.g. a file edited moments ago). See
  `internal/compositor/shared.go`'s `findWorkspaceSymbol`.
- **Search debounce.** The bottom search bar waits ~120ms after the last
  keystroke before querying clangd, so a normal typing burst collapses into
  one `workspace/symbol` round trip instead of one per character. A
  superseded query (the user kept typing before it fired, or before its
  result came back) is dropped rather than overwriting newer search state.
  See `internal/tui/model.go`'s `searchDebounceMsg`/`searchState.generation`.
- **Serial LSP transport.** `*lsp.Client` is a single blocking stdio
  connection to clangd (one call in flight at a time — see
  `internal/lsp/client.go`); overlapping callers queue rather than racing.

---

## 12. Requirements

| Dependency | Purpose | Minimum version |
|---|---|---|
| `clangd` | Symbol resolution engine | 14 |
| `compile_commands.json` | Build context for clangd | — |
| Go | Building skein | 1.22 |

`compile_commands.json` generation:
```bash
# CMake
cmake -B build -DCMAKE_EXPORT_COMPILE_COMMANDS=ON

# Non-CMake projects
bear -- make
```

---

## 13. What skein is NOT

- Not a replacement for clangd. It is a consumer of clangd.
- Not a code formatter, linter, or refactor tool.
- Not a debugger or profiler.
- Not a generic LSP client (it is C++-first; other languages are future work).
