# Changelog

All notable changes to skillsync are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Fixed

- Reject unsafe skill names per Agent Skills spec (`^[a-z0-9]+(-[a-z0-9]+)*$`, ≤64 chars) during discovery, materialize, and remove.
- Confine git clone paths under `sources/` and reject `..` in URL-derived paths.
- Non-interactive pickers no longer auto-select all without `--yes`; bare `remove` requires names or `--all` when not on a tty.
- `sync` pulls every manifested git source (not only the last entry).
- Global flags accepted before or after the subcommand.
- Document install URL, `remove --all`, init/remove automation, doctor exit semantics, and `--purge` / `nuke` confirmation.

### Added

- `remove --all` to drop every installed skill in one shot.
- `Makefile` targets: `test`, `shellcheck`, `install`, `uninstall`.
- Deny-by-default `.gitignore` (explicit includes only).
- ShellCheck v0.11.0 pin via Makefile/CI + native `.shellcheckrc` (no custom runner).

### Changed

- OSS scaffolding: CI workflow, `.editorconfig`, `.gitignore`, `SECURITY.md`; docs aligned with CLI behavior.
- Replaced transitional `HANDOVER.md` with root [AGENTS.md](AGENTS.md) as the single coding-agent entrypoint; operator guidance remains in `skill/SKILL.md`.

## [0.2.0] - 2026-07-25

First public preview.

### Added

- One-store-many-views core: canonical store in XDG data dir, whole-directory view symlinks for every agent in the registry.
- Commands: `init`, `add` (git URL / `owner/repo` / local path, `--layer org|user`), `sync`, `remove`/`rm` (with interactive picker and `--source`), `list`/`ls`, `status`, `doctor`, `uninstall`/`nuke` (`--keep`, `--purge`), `completion bash|zsh`.
- Ownership invariant: the store only contains skillsync-created symlinks; unmanaged entries are skipped, refused, and flagged — never touched.
- Layered precedence (`org` < `user` < `local`) with `exclude.conf` so removed skills stay removed across syncs.
- Generated agent registry (~75 agents) from vercel-labs/skills at a pinned commit, with `${VAR:-default}` env-overridable homes, `|` fallback chains, and `agents.local.tsv` user overrides.
- `install.sh`: pipe-safe curl installer with update-on-rerun, PATH guidance, and `--uninstall`.
- Interactive UI (colors, symbols, spinner, pickers) with automatic degradation for pipes, `NO_COLOR`, and `TERM=dumb`.
- Migration: `init` moves existing per-agent skills into a managed local source with timestamped backups.

[Unreleased]: https://github.com/formenosland/skillsync/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/formenosland/skillsync/releases/tag/v0.2.0
