# AGENTS.md

Instructions for coding agents working in this repository.
([agents.md](https://agents.md) convention.)

## Project

skillsync: one canonical skill store, many agent views (GNU Stow model for [Agent Skills](https://agentskills.io)). Architecture: [docs/DESIGN.md](docs/DESIGN.md). Human usage: [README.md](README.md). Contribution process: [CONTRIBUTING.md](CONTRIBUTING.md).

The CLI is a Go binary (`cmd/skillsync`, `internal/…`). End-user agents *operating* the installed CLI use [skill/SKILL.md](skill/SKILL.md) — that is not this file.

## Commands

```sh
make                 # CGO_ENABLED=0 go build -o bin/skillsync ./cmd/skillsync
make test            # go test ./...
make agentregistry   # refresh internal/agentregistry/agents.tsv from pinned vercel-labs/skills
make install         # go install ./cmd/skillsync
./bin/skillsync help # after make
```

`make agentregistry SHA=<upstream-commit>` bumps the pin. CI `agentregistry-drift` re-runs the generator against the SHA in the TSV header. Extend `prefixes` in `internal/agentregistry/gen/parse.go` if upstream adds a path token.

## Non-negotiables

1. **Ownership invariant:** skillsync owns the store (links it created only); humans own sources. Never delete skill *files* — only store/view links skillsync made. Unmanaged real dirs in the store: skip / refuse / flag — do not touch.
2. **Go CLI, CGO off** — no host awk/sed/git required at runtime (go-git for HTTPS). Maintainer agent-registry refresh is `go run ./internal/agentregistry/gen` (`make agentregistry`).
3. **`internal/agentregistry/agents.tsv` is generated** — never hand-edit. Regenerate with `make agentregistry` or `make agentregistry SHA=<upstream-sha>`; extend `prefixes` in `internal/agentregistry/gen/parse.go` if upstream adds a path token. Embedded next to `Load` (`//go:embed` is package-relative).
4. Keep behavior **`--dry-run`-able** and **`--yes`-able** (automation first).
5. **Docs travel with behavior:** same change updates `README.md`, `docs/DESIGN.md`, and `skill/SKILL.md` when semantics change. Do not invent git tags — [cocogitto](https://docs.cocogitto.io/) (`cog.toml`) owns version bumps. Version string lives in `internal/cli/app.go` (`Version`).
6. **Config is one file:** `${XDG_CONFIG_HOME:-$HOME/.config}/skillsyncrc` (TOML), or `$SKILLSYNC_HOME/skillsyncrc`. No `sources.conf`, `exclude.conf`, `config.toml`, or `agents.local.tsv`.

## Boundaries

- Do not weaken the ownership invariant to “helpfully” delete `sources/local/` or other user trees.
- Do not rename this project to `skills` (collides with vercel’s npm package / common paths).
- Store skill names must match Agent Skills `name` rules: `^[a-z0-9]+(-[a-z0-9]+)*$`, max 64 chars.
- Two uninstalls by design: `brew uninstall skillsync` (or delete the `go install` binary) vs `skillsync uninstall` (views/data; `--purge` needs typed `nuke` unless `--yes`).
- Occupancy: `sync` never steals an occupied name; do not reintroduce org/user/local layers.
- Do not add a curl `install.sh` that clones the repo or copies `bin/` (gitignored). Distribution is Homebrew, `go install`, or `make`.

## Pitfalls (do not reintroduce)

- Do not pipe into interactive pickers: stdin must stay a TTY.
- Path equality: resolve with `EvalSymlinks` / `fsops.PathsEqual`. Linked views resolve to the store; use `viewIsNative` when checking “is this the store directory itself?”.
- Character classes for skill names must be ASCII-only (`IsSafeName`), not locale-dependent `[a-z]`.
- Windows directory views: symlink, then junction if privilege is missing; `--copy` for non-NTFS.

## Tests

Every behavior change needs a test; every bug fix needs a regression test. `go test ./...` is sandboxed (`SKILLSYNC_HOME` / fake `HOME`); no network; never touch real agent dirs.
