# Contributing to skillsync

Thanks for helping unify agent skills. This project values small, reviewable changes with tests.

**Coding agents:** follow [AGENTS.md](AGENTS.md) (setup, invariants, commands, boundaries).

## Development setup

No build step. You need `sh`, `git`, `awk`, and **ShellCheck v0.11.0** (same pin as CI).

```sh
git clone https://github.com/formenosland/skillsync.git
cd skillsync
brew install shellcheck        # or your package manager — must be v0.11.0
make test                      # or: tests/run.sh
./bin/skillsync help
```

Convenience targets: `make test`, `make shellcheck`, `make install`, `make uninstall`.

Lint rules: [`.shellcheckrc`](.shellcheckrc) (native ShellCheck config). Version pin: `SHELLCHECK_VERSION` in the [Makefile](Makefile) and [`.github/workflows/ci.yml`](.github/workflows/ci.yml) — bump both together.

## Code style

- **POSIX sh only** — no bashisms. Everything must run under `sh` (dash, macOS `/bin/sh`, busybox).
- Tabs for indentation in shell scripts.
- `make shellcheck` must pass (ShellCheck v0.11.0 + `.shellcheckrc`). Intentional idiom exceptions use file-level `# shellcheck disable=` with a reason comment.
- Keep the ownership invariant sacred: *skillsync owns the store; humans own sources.* No code path may delete skill files — only symlinks we created. Anything replaced (not removed) is backed up first.
- User-facing output goes through the `ui_*` helpers so it degrades correctly (non-tty, `NO_COLOR`, `TERM=dumb`). Machine output (`list`, `completion`) goes to stdout, plain.

## Tests

```sh
make test
# or: tests/run.sh
```

The suite is self-contained: it builds sandboxes under `mktemp -d` with fake `HOME`/`SKILLSYNC_HOME`/registries and never touches your real agent folders or skill data. Every behavior change needs a test; every bug fix needs a regression test.

## The agent registry

`registry/agents.tsv` is **generated** — do not edit it by hand. It derives from `src/agents.ts` in [vercel-labs/skills](https://github.com/vercel-labs/skills), pinned to a commit:

```sh
registry/generate.sh <upstream-commit-sha>
```

To update: run the generator with a newer SHA, review the diff (every changed row should correspond to an upstream change), and commit the regenerated file. If upstream introduces a new path token, the generator fails loudly — extend `prefix()` in `registry/generate.sh`.

Known gaps in upstream coverage belong in your local `agents.local.tsv` first, and in an upstream PR to vercel-labs/skills second.

## Pull requests

- One logical change per PR.
- Explain the *why* in the description; the diff shows the what.
- Conventional Commits (`feat:`, `fix:`, `docs:`, `chore:`, …). CI runs `cog check --from-latest-tag`.
- CI (shellcheck + test suite on Linux and macOS) must be green.
- Docs live next to behavior: if you change semantics, update `README.md`, `docs/DESIGN.md`, and `skill/SKILL.md` in the same PR.

## Releases

Versioning is [Semantic Versioning](https://semver.org/) via [cocogitto](https://docs.cocogitto.io/). Do not hand-edit `VERSION` in `bin/skillsync` or append changelog sections for a release — `cog bump` does both and tags `vX.Y.Z`.

**One-time seed** (until `v0.2.0` exists on the remote):

```sh
git tag -a v0.2.0 d55d7ac1a0f17d42de8097b1cf4906ea3f5cefa0 -m "skillsync 0.2.0"
git push origin v0.2.0
```

That tag marks the first public preview. Later `fix:` / `feat:` commits become `0.2.1` (or higher) on the next bump.

Ship a release from `main` after merging conventional commits:

1. GitHub → Actions → **Release** → **Run workflow** (bump defaults to `auto` from conventional commits; optionally choose `major` / `minor` / `patch` to force that increment), or locally: `cog bump --auto` (or `--major` / `--minor` / `--patch`) then `git push origin HEAD` and `git push origin vX.Y.Z`.
2. The Release workflow publishes a GitHub Release from `cog changelog --at <tag>` and hashes GitHub’s tagged source archive (not a Release asset).
3. A following job calls [formenosland/homebrew-tap](https://github.com/formenosland/homebrew-tap) `.github/workflows/bump.yml` with that `url` / `sha256` / version. It opens a formula PR, waits for tap CI, and squash-merges. `brew upgrade` follows that merge. `install.sh` (curl) tracks the GitHub Release tag as soon as it exists. `SKILLSYNC_INSTALL_REF` is only an override for curl (specific tag, or `main` for HEAD).

## Security

See [SECURITY.md](SECURITY.md). Report vulnerabilities privately to security@formenos.land — do not open a public issue for undisclosed issues.
