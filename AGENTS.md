# AGENTS.md

Instructions for coding agents working in this repository.
([agents.md](https://agents.md) convention.)

## Project

skillsync: one canonical skill store, many agent views (GNU Stow model for [Agent Skills](https://agentskills.io)). Architecture: [docs/DESIGN.md](docs/DESIGN.md). Human usage: [README.md](README.md). Contribution process: [CONTRIBUTING.md](CONTRIBUTING.md).

End-user agents *operating* the installed CLI use [skill/SKILL.md](skill/SKILL.md) — that is not this file.

## Commands

```sh
make test            # sandboxed suite (tests/run.sh); requires ShellCheck v0.11.0
make shellcheck      # lint; version must match SHELLCHECK_VERSION in Makefile
./bin/skillsync help # run from checkout
./install.sh         # local install / update
```

Bump ShellCheck pin in both `Makefile` and `.github/workflows/ci.yml` (`SHELLCHECK_VERSION`) together. Rules: `.shellcheckrc`.

## Non-negotiables

1. **Ownership invariant:** skillsync owns the store (symlinks it created only); humans own sources. Never delete skill *files* — only store/view links skillsync made. Unmanaged real dirs in the store: skip / refuse / flag — do not touch.
2. **POSIX sh only** — `bin/skillsync`, `install.sh`, `registry/generate.sh`, `tests/run.sh`. No bashisms. `make shellcheck` must stay clean.
3. **`registry/agents.tsv` is generated** — never hand-edit. Regenerate with `registry/generate.sh <upstream-sha>`; extend `prefix()` if upstream adds a path token.
4. Keep behavior **`--dry-run`-able** and **`--yes`-able** (automation first).
5. **Docs travel with behavior:** same change updates `README.md`, `docs/DESIGN.md`, and `skill/SKILL.md` when semantics change. Do not hand-edit `VERSION` in `bin/skillsync` or invent git tags — [cocogitto](https://docs.cocogitto.io/) (`cog.toml`) owns version bumps and `CHANGELOG.md` on release.

## Boundaries

- Do not weaken the ownership invariant to “helpfully” delete `sources/local/` or other user trees.
- Do not rename this project to `skills` (collides with vercel’s npm package / common paths).
- Store skill names must match Agent Skills `name` rules: `^[a-z0-9]+(-[a-z0-9]+)*$`, max 64 chars.
- Two uninstalls by design: `install.sh --uninstall` or `brew uninstall skillsync` (tool only) vs `skillsync uninstall` (views/data; `--purge` needs typed `nuke` unless `--yes`).

## Pitfalls (do not reintroduce)

- Avoid `cmd | while …` under `set -e` when the loop can fail or must update counters — use a temp file + `while read <file`.
- Path equality: resolve with `pwd -P` (`resolve_dir`). Linked views resolve to the store; use `view_is_native` when checking “is this the store directory itself?”.
- Prefer explicit `if` over `A && B || C` for control flow.
- Character classes like `[a-z]` in `case` can match uppercase under some UTF-8 locales — use `LC_ALL=C` (see `is_safe_skill_name`).

## Tests

Every behavior change needs a test; every bug fix needs a regression test. Suite is sandboxed (`SKILLSYNC_HOME` / fake `HOME`); no network; never touch real agent dirs.
