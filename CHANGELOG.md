# Changelog

All notable changes to skein are documented here.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Initial project scaffolding: CI, release, and nightly workflows.
- Design specification (`docs/SPEC.md`).
- Project review document (`docs/REVIEW.md`).
- Skein the sheep logo (`docs/skein-logo.svg`).
- `skein draw -m <method> -c <class>`: scope a method query to one class,
  disambiguating names declared by more than one class/namespace.
- Interactive TUI mode v1: bubbletea-based map view with search, follow,
  and status bar (#4).
- v0.1 draw-mode gaps closed: strand-limit truncation, `-s` generic symbol
  lookup, `--no-color`, `--absolute`; TUI bundle-jump (`1`-`9`); fixed
  cold-start symbol resolution (#5).
- TUI bundle tabs: pin/unpin threads, previous/next/close bundle keys,
  multi-thread tracking across tabs (#10, #13).
- TUI goto-to-editor key (`g`), spooled back/forward rework (`h`/`ctrl+r`),
  tab-switch fixes (#13).
- Warm-index fast path, search debounce, serialized LSP client, first-run
  onboarding hint (#11).

### Changed
- TUI search UX: improved contrast, history recall, fixed layout overflow (#9).
- TUI follow-target visibility rework and missing key legend added; unpin
  behavior fixed (#10).

### Fixed
- `draw -m <method>` on a name declared by multiple unrelated
  classes/namespaces (e.g. `Get` in both `leveldb::DB` and
  `leveldb::Version`) could silently resolve to the wrong one, picked by
  arbitrary `workspace/symbol` response order. Now prints a stderr warning
  naming the other candidates and pointing at `-c`. Found via project-agnostic
  validation against Catch2, nlohmann/json, and leveldb.
- TUI outgoing-call resolution, search spool-push, and an LSP client pipe
  leak (#6).
- Bare `skein` launch (no initial thread) never nudged clangd's indexer,
  breaking all search until an unrelated query warmed it (#8).
- Single call-site rendering folded onto its file row instead of a separate
  nested line, in both TUI map view (#12) and `draw`-mode tree output (#14).
- Test coverage raised 87.3% -> 89.7% by covering previously-untested
  compositor error paths (#7).

[Unreleased]: https://github.com/fmbfs/skein/commits/main
