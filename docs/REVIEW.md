# skein — Full Project Review Document
> Prepared for: Claude Opus review  
> Purpose: Critical review, gap analysis, and recommendations before implementation begins  
> Status: Decisions finalized 2026-06-16 (see §14). Implementation in progress.

---

## How to review this document

This is a **proposed** design, not a settled one. The sections below read as
decisions for clarity, but every one is open to challenge. Several load-bearing
assumptions could invalidate parts of the architecture if wrong:

- **Sync vs async LSP** (Q1) changes the entire client design. If async is
  required, the compositor and TUI interfaces change shape.
- **Graceful degradation without compile_commands.json** (Q4) determines
  whether the TUI needs a fallback file-browser mode at all.
- **Config file vs flags-only** (Q6) affects how much state management the
  tool needs from v0.1.

Please challenge the assumptions directly. A confident "this is wrong because
X" is more valuable than validation. The open questions in section 14 are the
priority — answers there unblock implementation.

---

## 1. What we are building

`skein` is a **clangd-powered codebase exploration tool** for C++ projects.

The core insight: existing tools (clangd, cscope, ctags) answer **point queries** — jump to definition, find references. `skein` answers **shape queries** — *show me the full relationship map of this symbol, and let me walk it.*

It is not a replacement for clangd. It is a compositor and navigator built on top of clangd's LSP output.

### The two modes

```bash
skein                  # default: open TUI, start in tangle (field view)
skein foo              # default: open TUI with foo as first thread
skein draw -m foo      # fast mode: stdout tree, pipe-friendly, exits
skein draw -c Bar      # fast mode: class map
skein draw -f file.cpp # fast mode: all symbols in file
skein draw -m foo --json # fast mode: JSON output for scripts/CI
```

---

## 2. Identity

| | |
|---|---|
| **Tool name** | `skein` |
| **Binary** | `skein` |
| **Mascot** | Skein — a sheep. Logo: a continuous thread that begins as a dense knot (left) and resolves into a sheep silhouette (right). The knot IS the head. The untangled body IS the wool. |
| **Tagline** | *Untangle your codebase.* |
| **Visual inspiration** | lazygit — panel layout, keyboard-first, tab-based multi-context |
| **Credit** | README explicitly credits lazygit by Jesse Duffield for UX inspiration |

---

## 3. Vocabulary (the textile register)

Every term lives in the textile/wool domain. No exceptions.

| Concept | Term | Notes |
|---|---|---|
| Focal symbol | **thread** | The current anchor. What the map radiates from. |
| Navigate to node | **follow** | `enter` or `f` in TUI. Node becomes new thread. |
| Navigation history | **spool** | Back/forward. Thread spools back. |
| Pinned collection | **bundle** | A tab of threads being tracked simultaneously. |
| Blank start state | **tangle** | No thread set. Field view. Fuzzy search to pick first thread. |
| Depth | **ply** | How many hops out. `--ply N`. Default 1, max 3. |
| Fast mode subcommand | **draw** | Drawing a single thread from a skein. One question, stdout, exits. |
| Slow/TUI mode | *(default, no name needed)* | Just `skein`. |
| Node cap | **strand limit** | Max visible nodes. Default 50. Truncates with warning. |

---

## 4. Architecture

### Language and stack

| Layer | Technology | Rationale |
|---|---|---|
| Language | Go 1.22 | Single static binary, trivial cross-compile, strong stdlib |
| TUI | bubbletea + lipgloss | Same stack as lazygit. Event-loop native, proven at scale. |
| LSP engine | clangd via stdio JSON-RPC | No libclang dep. Any installed clangd works. Accurate. |
| Testing | testify + gomock | testify for assertions, gomock for LSP client mocking |
| Lint | golangci-lint v1.59 | Configured in .golangci.yml |
| Coverage | go test -cover → Codecov | Native Go coverage, free for OSS |

### Project structure

```
skein/
├── cmd/skein/
│   └── main.go                  — entry point, mode dispatch
├── internal/
│   ├── lsp/
│   │   ├── client.go            — clangd subprocess + JSON-RPC stdio
│   │   ├── protocol.go          — LSP types (Location, Symbol, etc.)
│   │   └── client_test.go
│   ├── compositor/
│   │   ├── types.go             — Thread, RelationMap, Bundle, Spool
│   │   ├── method.go            — MethodCompositor
│   │   ├── class.go             — ClassCompositor
│   │   ├── file.go              — FileCompositor
│   │   └── compositor_test.go
│   ├── tree/
│   │   ├── printer.go           — fast mode: tree(1)-style stdout renderer
│   │   └── json.go              — fast mode: JSON renderer
│   └── tui/
│       ├── model.go             — bubbletea root model
│       ├── map.go               — left panel: relationship tree
│       ├── detail.go            — right panel: symbol detail + hover
│       ├── search.go            — bottom: fuzzy symbol search
│       ├── bundle.go            — tab bar + bundle management
│       ├── keys.go              — all keybindings in one place
│       └── styles.go            — lipgloss styles
├── tests/
│   ├── e2e/                     — end-to-end against fixture project
│   └── fixtures/simple_cpp/     — small C++ project for E2E tests
├── docs/
│   ├── SPEC.md                  — full design specification
│   └── skein-logo.svg           — Skein the sheep logo
├── .github/
│   ├── workflows/
│   │   ├── ci.yml               — lint + unit + build + E2E + audit
│   │   ├── release.yml          — triggered by vX.Y.Z tags
│   │   └── nightly.yml          — 02:00 UTC daily, uploads to nightly release
│   ├── ISSUE_TEMPLATE/
│   │   ├── bug_report.yml
│   │   └── feature_request.yml
│   ├── labels.yml               — full label taxonomy
│   └── pull_request_template.md
├── scripts/
│   └── setup-labels.sh          — applies labels via gh CLI
├── Dockerfile.ci                — CI image: clangd 18 + cmake + bear
├── .golangci.yml
├── go.mod                       — pinned deps
└── README.md
```

---

## 5. TUI layout

```
┌─────────────────────────────────────────────────────────────────────┐
│ skein  ·  processFrame [method]  ply:1  strands:12  spool:3         │ ← status
├─────────────────────────────────────────────────────────────────────┤
│ [processFrame] [Pipeline] [+]                                       │ ← bundles
├───────────────────────────┬─────────────────────────────────────────┤
│                           │                                         │
│  processFrame [method]    │  void Pipeline::processFrame(           │ ← detail
│  ├── defined in           │    RawBuffer& buf)                      │
│  │   └── Pipeline.cpp:42  │                                         │
│  ├── called in  (9)       │  Incoming:                              │
│  │   ├── main.cpp:101     │    Scheduler::run() :77                 │
│  │   └── Scheduler.cpp:77 │                                         │
│  ├── calls  (3)           │  Outgoing:                              │
│  │   ├── acquire() ───────┼─▶ [follow with enter]                  │
│  │   ├── format()         │                                         │
│  │   └── emit()           │                                         │
│  └── inherits             │                                         │
│      └── IProcessor       │                                         │
│                           │                                         │
├───────────────────────────┴─────────────────────────────────────────┤
│ > _                                                                 │ ← search
├─────────────────────────────────────────────────────────────────────┤
│ enter:follow  tab:panel  /:search  p:pin  u:back  i:incoming  o:out │ ← hints
└─────────────────────────────────────────────────────────────────────┘
```

---

## 6. Relationship directions

All three rendered simultaneously, colour-coded. Shortcuts are filters, not mode switches — the full picture is always the default.

| Direction | Colour | Meaning |
|---|---|---|
| Incoming | cyan | Who calls this, who inherits this |
| Outgoing | green | What this calls, what this inherits from |
| Bidirectional | yellow | Appears in both |

| Key | Action |
|---|---|
| `i` | Toggle incoming visibility |
| `o` | Toggle outgoing visibility |

---

## 7. Full keyboard reference

### Global

| Key | Action |
|---|---|
| `q` / `ctrl+c` | Quit |
| `/` | Focus search bar |
| `esc` | Clear search / back to map |
| `?` | Help overlay |
| `tab` | Cycle panels: map → detail → search |

### Map navigation

| Key | Action |
|---|---|
| `j` / `↓` | Down |
| `k` / `↑` | Up |
| `enter` | **Follow** — selected node becomes new thread |
| `u` | Back (spool) |
| `ctrl+r` | Forward (spool) |
| `r` | Reset to first thread |

### Bundle (multi-thread tabs)

| Key | Action |
|---|---|
| `p` | Pin current thread to bundle (new tab) |
| `[` | Previous bundle tab |
| `]` | Next bundle tab |
| `x` | Close current bundle tab |
| `1`–`9` | Jump to bundle tab by number |

### Depth and filters

| Key | Action |
|---|---|
| `+` / `=` | Ply +1 (max 3) |
| `-` | Ply -1 (min 1) |
| `i` | Toggle incoming |
| `o` | Toggle outgoing |

---

## 8. LSP calls used

| LSP method | skein use |
|---|---|
| `initialize` | Handshake |
| `workspace/symbol` | Fuzzy symbol search (tangle view + thread resolution) |
| `textDocument/references` | All usages |
| `textDocument/definition` | Definition location |
| `textDocument/documentSymbol` | All symbols in a file |
| `callHierarchy/incomingCalls` | Who calls this |
| `callHierarchy/outgoingCalls` | What this calls |
| `typeHierarchy/supertypes` | Base classes |
| `typeHierarchy/subtypes` | Derived classes |
| `textDocument/hover` | Signature + doc for detail panel |

---

## 9. CI/CD pipeline

### Workflows

| Workflow | Trigger | Jobs |
|---|---|---|
| `ci.yml` | push to main, PRs | lint, unit tests + coverage, build (5 platforms), E2E, dep audit |
| `release.yml` | push tag `vX.Y.Z` | build + sign + upload to GitHub Release (5 platforms) |
| `nightly.yml` | 02:00 UTC daily + manual | unit tests, build, update `nightly` release tag |

### Platforms built

- linux/amd64
- linux/arm64
- darwin/amd64
- darwin/arm64

Windows is best-effort, not a built/released platform (§14 Q5).

### Coverage

- Minimum: 85% (enforced in CI, fails build if below)
- Uploaded to Codecov on every push to main
- `go test -race` always enabled

### E2E

- Runs in `Dockerfile.ci` container: clangd 18 + cmake + bear
- Fixture project: `tests/fixtures/simple_cpp/`
- compile_commands.json generated fresh in CI from fixture

### Dependency audit

- `go mod tidy` verified (diff must be clean)
- `govulncheck` on every push

---

## 10. Release planning

### Versioning

Semantic versioning. Tags trigger release workflow.
- `vX.Y.Z` → stable release
- `vX.Y.Z-rcN` → release candidate (draft release, prerelease flag)
- `nightly` → rolling tag, updated daily

### Milestone plan

| Version | Goal | Key deliverables |
|---|---|---|
| v0.1.0 | draw mode works | LSP client, compositors, tree printer, JSON printer |
| v0.2.0 | TUI works | bubbletea model, map panel, detail panel, search |
| v0.3.0 | bundles | multi-thread tabs, bundle management |
| v0.4.0 | multi-path | `skein draw --path foo bar` — find relationship path between symbols |
| v1.0.0 | stable | hardened LSP client, full test coverage, docs complete |
| vX.0 | split view | two threads side by side (separate layout engine) |
| vX.1 | overlay mode | pinned threads highlighted in unified map |

---

## 11. GitHub project setup

### Labels taxonomy

**Type** (on every PR):
`type: feat` `type: fix` `type: refactor` `type: test` `type: docs` `type: chore` `type: perf`

**Component**:
`component: lsp` `component: compositor` `component: tui` `component: draw` `component: ci`

**Status**:
`status: triage` `status: ready` `status: blocked` `status: wip` `status: stale`

**Priority**:
`priority: critical` `priority: high` `priority: medium` `priority: low`

**Special**:
`good first issue` `help wanted` `breaking change`

### Branch naming

```
feat/short-description
fix/short-description
chore/short-description
docs/short-description
```

### PR requirements (enforced by branch protection)

- CI must pass (all jobs green)
- At least 1 approving review (owner can self-merge on solo work)
- No direct pushes to main
- Commits must be signed (`git commit -S`)
- PR template filled out

### Commit convention

Conventional commits enforced by CI lint:
```
feat(lsp): add callHierarchy/incomingCalls support
fix(tui): correct spool back navigation on empty history
chore(ci): pin golangci-lint to v1.59
```

---

## 12. Installation instructions (to be in README)

```bash
# Go install (recommended)
go install github.com/fmbfs/skein/cmd/skein@latest

# Homebrew (planned for v1.0)
brew install fmbfs/tap/skein

# Binary download
# Download from GitHub Releases for your platform
# Verify checksum:
sha256sum -c skein-linux-amd64.sha256

# Build from source
git clone https://github.com/fmbfs/skein
cd skein
go build ./cmd/skein
```

### Requirements

- `clangd` ≥ 14 on `$PATH`
- `compile_commands.json` in project root

```bash
# CMake projects
cmake -B build -DCMAKE_EXPORT_COMPILE_COMMANDS=ON

# Non-CMake projects
bear -- make
```

---

## 13. What is explicitly NOT in scope

- Replacing clangd (skein is a consumer, not a reimplementation)
- Supporting languages other than C/C++ (future work)
- Code formatting, linting, refactoring
- Debugging or profiling
- Vim/Neovim plugin (documented TODO — when it grows, split to `skein.nvim`)
- Docker image for distribution (overkill for a CLI tool)

---

## 14. Decisions (resolved 2026-06-16)

These were the eight open questions raised for review. Each is now resolved;
the question is preserved for context, followed by the decision and rationale.

1. **LSP client design**: synchronous blocking RPC per call vs async with goroutines.
   **Decision: synchronous blocking.** clangd's typical response time (<200ms) is
   well within tolerance for blocking calls, and bubbletea's event loop already
   runs the LSP call inside its own goroutine via `tea.Cmd` — the TUI doesn't
   freeze even though the call itself blocks. Revisit only if v0.2 profiling
   shows real jank.

2. **compile_commands.json auto-detection**: walk upward from cwd (mirrors clangd's own logic).
   **Decision: confirmed, walk upward from cwd.** Edge case to handle explicitly:
   a parent-level `compile_commands.json` that exists but doesn't cover the file
   being queried — skein must surface clangd's error rather than silently using
   a stale/wrong database.

3. **Node cap strategy**: hard cap at 50 visible strands with truncation warning.
   **Decision: 50 stays the default, but it is configurable via `--strands N`
   from v0.1** (not deferred to a future config file). The flag already existed
   in the spec's sniff/draw-mode flag table; this decision makes explicit that
   it's a default, not a hard ceiling.

4. **TUI entry with no compile_commands.json**: graceful degradation vs fail fast.
   **Decision: fail fast** with a clear, actionable error that names the
   cmake/bear command to generate the missing file. A file-browser fallback mode
   is a second product surface to build and maintain for a degraded experience
   — not worth it for v0.1.

5. **Windows support**: supported platform or best-effort?
   **Decision: best-effort, not a supported platform**, for v0.1–v1.0. clangd on
   Windows for embedded C++ workflows is rare enough that maintaining a full
   CI/release matrix entry isn't worth it. Windows has been dropped from
   `ci.yml`, `release.yml`, and `nightly.yml`'s build matrices. Revisit if a
   user actually asks.

6. **Config file**: `~/.config/skein/config.toml` vs flags-only for v0.1.
   **Decision: flags only for v0.1.** A config file is easy to add later
   without breaking anything; building it now is speculative state management.

7. **Coverage strategy**: mock the LSP client in unit/compositor/TUI tests,
   cover the transport layer via E2E against a real clangd, merge coverage
   profiles before enforcing the 85% threshold.
   **Decision: confirmed as proposed**, no changes. This is the standard
   pattern for Go projects with a subprocess/IO boundary.

---

## 15. Logo generation prompt (for dedicated image AI)

> A minimalist monochrome logo for a developer CLI tool called **skein**. The concept: a single continuous thread begins as a dense, chaotic, overlapping knot on the left side — compressed, illegible as individual lines, reading as a solid tangled mass. The knot IS the sheep's head — there is no separate head drawn. As the thread moves rightward, the lines gradually separate, breathe, and resolve into the clean silhouette of a sheep: a rounded wool body drawn entirely from loosely spaced parallel curves, four simple geometric legs, and a small tail curl at the far right. No fills anywhere — lines only. Monochrome ink (#1a1a1a) on transparent background. Style: precise, technical, like an engineering schematic or circuit diagram. Not cute, not cartoonish. Must read at 16×16 pixels as a favicon (overall silhouette) and at 512×512 (individual thread lines visible). The single memorable element: the left-to-right journey from chaos to structure, from tangle to clarity, from knot to sheep.
