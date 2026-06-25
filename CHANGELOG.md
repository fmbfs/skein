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

### Fixed
- `draw -m <method>` on a name declared by multiple unrelated
  classes/namespaces (e.g. `Get` in both `leveldb::DB` and
  `leveldb::Version`) could silently resolve to the wrong one, picked by
  arbitrary `workspace/symbol` response order. Now prints a stderr warning
  naming the other candidates and pointing at `-c`. Found via project-agnostic
  validation against Catch2, nlohmann/json, and leveldb.

[Unreleased]: https://github.com/fmbfs/skein/commits/main
