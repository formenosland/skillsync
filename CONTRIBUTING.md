# Contributing to skillsync

Thanks for helping unify agent skills. This project values small, reviewable changes with tests.

**Coding agents:** follow [AGENTS.md](AGENTS.md) (setup, invariants, commands, boundaries).

## Development setup

You need **Go 1.22+** (`CGO_ENABLED=0`).

```sh
git clone https://github.com/formenosland/skillsync.git
cd skillsync
make test
make && ./bin/skillsync help
```

Convenience targets: `make` / `make build`, `make test`, `make agentregistry`, `make install` (`go install ./cmd/skillsync`).

## Code style

- **Go CLI** (`cmd/skillsync`, `internal/…`), `CGO_ENABLED=0`.
- `gofmt` for Go.
- Keep the ownership invariant sacred: *skillsync owns the store; humans own sources.* No code path may delete skill files — only links we created. Anything replaced (not removed) is backed up first.
- User-facing command logs go to stderr and degrade (non-tty, `NO_COLOR`, `TERM=dumb`). Machine output (`list --names`, `completion`) goes to stdout, plain. `list` on a tty (or `--pretty`) is a catalog.
- Config is `${XDG_CONFIG_HOME:-$HOME/.config}/skillsyncrc` (TOML). Do not add `sources.conf` / `exclude.conf` / `agents.local.tsv`.

## Tests

```sh
make test
```

The suite is self-contained: `go test ./...` uses temp dirs with fake `HOME`/`SKILLSYNC_HOME`/registries. Never touches your real agent folders or skill data. Every behavior change needs a test; every bug fix needs a regression test.

## The agent registry

`internal/agentregistry/agents.tsv` is **generated** — do not edit it by hand. It derives from `src/agents.ts` in [vercel-labs/skills](https://github.com/vercel-labs/skills), pinned to a commit:

```sh
make agentregistry             # current pin in the TSV header / gen defaultSHA
make agentregistry SHA=<commit>  # bump upstream
```

To update: run the generator with a newer SHA, review the diff (every changed row should correspond to an upstream change), and commit the regenerated file. If upstream introduces a new path token, the generator fails loudly — extend `prefixes` in `internal/agentregistry/gen/parse.go`.

Known gaps in upstream coverage belong in `[[agents]]` in `skillsyncrc` first, and in an upstream PR to vercel-labs/skills second.

## Pull requests

- One logical change per PR.
- Explain the *why* in the description; the diff shows the what.
- Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, …). CI runs `cog check --from-latest-tag`.
- CI (`go test` on Linux/macOS/Windows; agentregistry-drift on Ubuntu) must be green.
- Docs live next to behavior: if you change semantics, update `README.md`, `docs/DESIGN.md`, and `skill/SKILL.md` in the same PR.

## Releases

Versioning is [Semantic Versioning](https://semver.org/) via [cocogitto](https://docs.cocogitto.io/). Do not invent tags — `cog bump` updates `internal/cli/app.go` `Version` and `CHANGELOG.md`.

**One-time seed** (until `v0.2.0` exists on the remote):

```sh
git tag -a v0.2.0 d55d7ac1a0f17d42de8097b1cf4906ea3f5cefa0 -m "skillsync 0.2.0"
git push origin v0.2.0
```

That tag marks the first public preview. Later `fix:` / `feat:` commits become `0.2.1` (or higher) on the next bump.

Ship a release from `main` after merging conventional commits:

1. GitHub → Actions → **Release** → **Run workflow** (bump defaults to `auto` from conventional commits; optionally choose `major` / `minor` / `patch` to force that increment), or locally: `cog bump --auto` (or `--major` / `--minor` / `--patch`) then `git push origin HEAD` and `git push origin vX.Y.Z`.
2. The Release workflow publishes a GitHub Release from `cog changelog --at <tag>` and hashes GitHub’s tagged source archive (not a Release asset).
3. A following job calls [formenosland/homebrew-tap](https://github.com/formenosland/homebrew-tap) `.github/workflows/bump.yml` with that `url` / `sha256` / version. It opens a formula PR, waits for tap CI, and squash-merges. `brew upgrade` follows that merge. `go install …@latest` follows the module proxy once the tag is published.

## Security

See [SECURITY.md](SECURITY.md). Report vulnerabilities privately to security@formenos.land — do not open a public issue for undisclosed issues.
