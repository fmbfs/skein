# skein (`skein`)

> A skein finds what's hidden beneath the surface. So does `skein`.

`skein` is a **clangd-powered codebase exploration tool** for C++ projects.  
It answers *"what does the world look like from inside this symbol?"* —
rendering a live relationship map you can navigate, not just a list of search results.

---

## Two modes

### Draw (fast) — pipe-friendly tree output

```
$ skein draw -m processFrame

processFrame  [method]  ply:1
├── defined in
│   └── src/pipeline/Pipeline.cpp :42
│         void Pipeline::processFrame(RawBuffer& buf)
├── called in  (9)
│   ├── src/main.cpp
│   │   ├── :101
│   │   └── :214
│   ├── src/scheduler/Scheduler.cpp
│   │   └── :77
│   └── tests/test_pipeline.cpp
│       └── :55
└── calls
    ├── Buffer::acquire()
    ├── Formatter::format()
    └── Emitter::emit()
```

### TUI (slow) — interactive exploration

```
$ skein processFrame
```

Opens a full terminal UI. Follow the map into connected symbols, pin
multiple threads into a bundle, search the workspace — all without
leaving the terminal.

---

## Core concepts

| Term | Meaning |
|------|---------|
| **thread** | The symbol at the centre of the current map |
| **follow** | Navigate to a connected node; it becomes the new thread |
| **spool** | History of past threads — back/forward like a browser |
| **bundle** | A tab of pinned threads you're tracking simultaneously |
| **tangle** | The blank-start state — fuzzy search to pick your first thread |
| **draw** | Fast mode: static tree to stdout, exits |
| **ply** | How many hops out the map renders (default 1, max 3) |
| **strand limit** | Max visible nodes per render (default 50, truncates with warning) |

---

## Why not clangd / cscope / ctags?

Those tools answer **point queries**: jump to definition, find references.  
`skein` answers **shape queries**: *show me the full relationship map of this
thing, composed into one readable view.*

It uses clangd as its engine — not to replace it, but to compose its
answers into something you can navigate.

---

## Visual inspiration

The TUI is directly inspired by **[lazygit](https://github.com/jesseduffield/lazygit)**
by Jesse Duffield — panel layout, keyboard-first navigation, tab-based
multi-context management, and the general philosophy that a terminal tool
should feel as fast as thought.

The keybindings deliberately mirror lazygit's conventions where possible
so that users already familiar with it feel at home immediately.

---

## Keyboard shortcuts (TUI mode)

| Key | Action |
|-----|--------|
| `enter` / `l` | **Follow** — selected node becomes new thread |
| `h` | Back (spool) |
| `ctrl+r` | Forward (spool) |
| `r` | Reset to first thread |
| `j` / `k` | Move up/down |
| `tab` | Cycle panels |
| `/` | Search workspace symbols |
| `g` | Goto definition in `$EDITOR` |
| `p` | Pin thread to bundle (new tab) |
| `u` | Unpin thread from bundle |
| `[` / `]` | Previous / next bundle tab |
| `x` | Close current bundle tab |
| `1`-`9` | Jump to bundle tab by number |
| `i` | Toggle incoming edges |
| `o` | Toggle outgoing edges |
| `+` / `-` | Increase / decrease ply (max 3) |
| `?` | Help overlay |
| `q` | Quit |

---

## Draw mode flags

| Flag | Action |
|------|--------|
| `-m <method>` | Thread = method/function |
| `-c <class>` | Thread = class (with `-m`, scopes the method lookup to this class) |
| `-f <file>` | Thread = file |
| `-s <symbol>` | Thread = any symbol, resolved generically (method or class) |
| `--ply N` | Traversal depth (default 1, max 3) |
| `--strands N` | Max visible nodes before truncation (default 50) |
| `--json` | JSON output |
| `--no-color` | Disable ANSI colour |
| `--absolute` | Absolute file paths instead of root-relative |
| `--db <path>` | Path to `compile_commands.json` |
| `--clangd <path>` | clangd binary (default: `$PATH`) |

---

## Status

✅ **v0.1 and v0.2 complete** — draw mode and TUI mode are both implemented,
tested (≥85% coverage on `internal/...`), and merged to `main`.

### Roadmap

- [x] Design spec (`docs/SPEC.md`)
- [x] v0.1 — draw mode: clangd LSP client + tree printer
- [x] v0.2 — TUI mode: bubbletea model, thread/follow/spool
- [x] v0.3 — bundle (multi-thread tabs)
- [ ] v0.4 — multi-path find (`skein draw --path foo bar`)
- [ ] vX.0 — split view (two threads side by side)
- [ ] vX.1 — overlay mode (pinned threads in unified map)
- [ ] Future — Vim/Neovim plugin (`skein.nvim`, separate repo)
- [ ] Future — Language support beyond C++ (Rust, Python via LSP)

---

## Requirements

- `clangd` ≥ 14 on `$PATH`
- `compile_commands.json` in your project root
- Go ≥ 1.22 (to build `skein`)

```bash
# Generate compile_commands.json (CMake)
cmake -B build -DCMAKE_EXPORT_COMPILE_COMMANDS=ON

# Non-CMake projects
bear -- make
```

---

## Building

```bash
git clone https://github.com/fmbfs/skein
cd skein
go build ./cmd/skein
# binary: ./skein
```

---

## Usage

```bash
skein draw -m foo         # draw: all definitions and call sites of foo
skein draw -m foo -c Bar  # draw: foo, scoped to class Bar (disambiguates overloaded/common names)
skein draw -c Bar         # draw: class Bar — hierarchy, members
skein draw -f Pipeline.cpp # draw: all symbols in file
skein draw -m foo --json  # draw: JSON output
skein                      # TUI: tangle view, pick thread interactively
skein foo                  # TUI: open with foo as first thread
skein foo bar              # TUI: foo as thread, bar pre-pinned in bundle
```

---

## License

MIT
