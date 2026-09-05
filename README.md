# skillsync

> One skill store. Every agent. Install a skill once — it shows up in Claude Code, Cursor, Codex, Gemini CLI, and the rest of your toolchain.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![POSIX sh](https://img.shields.io/badge/shell-POSIX%20sh-lightgrey.svg)](bin/skillsync)
[![CI](https://github.com/formenosland/skillsync/actions/workflows/ci.yml/badge.svg)](https://github.com/formenosland/skillsync/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/formenosland/skillsync)](https://github.com/formenosland/skillsync/releases)

AI coding agents support [Agent Skills](https://agentskills.io) — but each one wants them in its own folder (`~/.claude/skills`, `~/.codex/skills`, `~/.cursor/skills`, …). Multiply that by org, personal, and project scopes, and skills management becomes an N × M mess.

**skillsync** ends it with a *one store, many views* model (think GNU Stow, for agent skills):

- Skills live in **one canonical store**.
- Every agent's skills folder becomes a **symlink into that store**.
- Sources (your org's repo, your personal repo, local folders) **merge with clear precedence**: `org` < `user` < `local`.

```console
$ skillsync add acme-corp/skills --layer org
◆ skillsync add (org) https://github.com/acme-corp/skills
│ ✓ code-review (org)
│ ✓ terse (org)
└ add done (2 ok, 0 warnings)

$ skillsync status
Agent views
  linked     ~/.claude/skills        claude-code
  linked     ~/.cursor/skills        cursor
  linked     ~/.codex/skills         codex
  linked     ~/.agents/skills        cline dexto warp zed …
```

The store only ever contains symlinks created by skillsync. Your skill files stay in git repos or folders you control — skillsync never deletes them. `skillsync --version` prints the installed version.

Zero runtime dependencies beyond POSIX `sh`, `git`, `ln`, and standard coreutils.

## Install

```sh
brew install formenosland/tap/skillsync
```

```sh
curl -fsSL https://raw.githubusercontent.com/formenosland/skillsync/main/install.sh | sh
```

Homebrew tracks tagged releases via [formenosland/homebrew-tap](https://github.com/formenosland/homebrew-tap); each GitHub Release bumps `Formula/skillsync.rb` through that tap’s `bump.yml` workflow. The curl installer also installs the latest GitHub release (re-run to update). From a checkout, `./install.sh` copies that tree with no network. Do not mix the two on `PATH` unless you know which binary wins.

<details>
<summary>Override the git ref</summary>

`SKILLSYNC_INSTALL_REF` is optional — a tag, branch, or commit when you do not want the latest release (`main` for unreleased HEAD).

```sh
SKILLSYNC_INSTALL_REF=main curl -fsSL https://raw.githubusercontent.com/formenosland/skillsync/main/install.sh | sh
```

The installer copies the app to `~/.local/share/skillsync/app/` and symlinks `~/.local/bin/skillsync` (printing a one-line PATH fix if `~/.local/bin` isn't on your PATH).

</details>

## Quickstart

```sh
skillsync init                                  # link every installed agent to the store
skillsync add acme-corp/company-skills --layer org
skillsync add alice/my-skills                   # GitHub shorthand
skillsync add ~/dev/my-skills                   # local folder
skillsync sync                                  # pull sources, refresh everywhere
```

`init` detects installed agents, migrates any existing per-agent skills into a managed local source, and replaces each agent's skills folder with a symlink to the store. Anything replaced is backed up first (`~/.local/share/skillsync/backups/`).

## Commands

| Command | Description |
|---------|-------------|
| `init` | Create the store, migrate per-agent skills, link agent views |
| `add <url\|path> [--layer org\|user]` | Register a git repo or local folder as a skill source |
| `sync` | Pull all git sources and refresh the store |
| `remove [names...]` (`rm`) | Remove skills from everywhere; bare `remove` opens an interactive picker |
| `remove --all` | Remove every installed skill (use `skillsync --yes remove --all` in scripts) |
| `remove --source <url\|path>` | Unregister a source and drop its skills |
| `list` (`ls`) | Installed skill names, one per line |
| `status` | Rich overview: skills, origins, agent view states, sources |
| `doctor` | Diagnose broken links, drifted views, missing links (exit 1 on actionable findings; warnings alone do not fail) |
| `uninstall [--keep] [--purge]` (`nuke`) | Reverse `init` (see below) |
| `completion bash\|zsh` | Shell completion (`skillsync remove <TAB>` completes skills) |

**Global flags** (before or after the subcommand): `--dry-run` (preview everything), `--yes` / `-y` (skip prompts; non-interactive init/remove), `--copy` (filesystems without symlinks).

```sh
skillsync --yes init
skillsync init --yes
skillsync --dry-run remove foo
skillsync remove foo --dry-run
```

Non-interactive: `skillsync init --yes` when agents need linking; `skillsync remove foo` or `skillsync remove --all --yes` (bare `remove` with no TTY needs names or `--all`).

`remove` drops the skill from the store — and therefore every agent — immediately. Source files are never touched. The name is recorded in `exclude.conf` so `sync` won't resurrect it; re-`add` the source (or edit the file) to bring it back.

### Two kinds of uninstall

| Command | Removes |
|---------|---------|
| `skillsync uninstall` | Agent view symlinks only (clean reverse of `init`); add `--keep` to leave real copies in each agent folder, `--purge` to also delete store, sources, and config (type `nuke` to confirm; `skillsync --yes uninstall --purge` skips prompts) |
| `install.sh --uninstall` | The curl-installed tool (`~/.local/bin/skillsync` + app copy) — never your skill data |
| `brew uninstall skillsync` | The Homebrew keg only — never your skill data |

### Shell completion

```sh
eval "$(skillsync completion bash)"   # ~/.bashrc
eval "$(skillsync completion zsh)"    # ~/.zshrc
```

## How it works

```
   sources (yours)                    store (skillsync's)                views (vendor folders)
┌──────────────────────────┐   ┌────────────────────────────────┐   ┌────────────────────────┐
│ git: acme-corp/skills    │   │ ~/.local/share/skillsync/store │   │ ~/.claude/skills     ──┼─┐
│ git: alice/skills        │──>│   skill-a -> sources/…         │<──┼ ~/.codex/skills        │ │ symlinks
│ dir: ~/dev/my-skills     │   │   skill-b -> ~/dev/my-skills/… │   │ ~/.agents/skills     ──┼─┘ to store
│ sources/local (migrated) │   └────────────────────────────────┘   │ … ~75 agents           │
└──────────────────────────┘                                        └────────────────────────┘
```

1. **Store** — one symlink per skill, pointing into a source.
2. **Views** — each agent's global skills dir is a symlink to the store.
3. **Layers** — `org` < `user` < `local`; highest wins on name collision.
4. **Sources** — git repos cloned under `sources/`, local folders referenced in place. `sync` = pull + refresh.

Everything lives in XDG paths (no new dotfolder in your home):

| What | Path |
|------|------|
| Config (`sources.conf`, `exclude.conf`, `agents.local.tsv`) | `${XDG_CONFIG_HOME:-~/.config}/skillsync/` |
| Data (`store/`, `sources/`, `backups/`, `app/`) | `${XDG_DATA_HOME:-~/.local/share}/skillsync/` |
| Single-root override | `SKILLSYNC_HOME=<dir>` |

## Supported agents

[`registry/agents.tsv`](registry/agents.tsv) covers ~75 agents — Claude Code, Cursor, Codex, Gemini CLI, GitHub Copilot, OpenCode, Zed, Cline, Warp, Goose, Windsurf, Kiro, Junie, Amp, Hermes, OpenClaw, and more. It is **generated, never hand-edited**, from the maintained agent map in [vercel-labs/skills](https://github.com/vercel-labs/skills), pinned to a commit recorded in the file header:

```sh
registry/generate.sh [newer-commit-sha]   # refresh from upstream
```

The registry handles env-overridable homes (`CODEX_HOME`, `CLAUDE_CONFIG_DIR`, `HERMES_HOME`, …) and fallback chains (OpenClaw's `~/.openclaw` → `~/.clawdbot` → `~/.moltbot`). `init` only links agents that are actually installed — it never litters your home directory.

Agent missing or path wrong? Add a row to `~/.config/skillsync/agents.local.tsv` (same TSV format; wins by `agent_id`) — and please open an issue or PR.

## Interop

The [vercel `skills` CLI](https://github.com/vercel-labs/skills) (`npx skills add …`) keeps working: it installs into agent directories, which are views into the same store after `init`. Skills added by either tool appear everywhere.

## FAQ

**What if an agent recreates its skills folder as a real directory?** `skillsync doctor` flags it as a drifted view; `skillsync init` heals it (migrating any new skills it finds).

**Windows?** Not yet — symlink semantics differ. `--copy` exists as a stopgap; proper support is future work.

**Is output scriptable?** Yes: colors and symbols degrade automatically when piped (or with `NO_COLOR`/`TERM=dumb`), `list` emits plain names, global `--yes` skips confirmations (including the `nuke` typed confirm for `--purge`), and `doctor` exits 1 only for actionable problems (broken links, drifted/wrong/missing views)—not for informational warnings such as layer collisions or missing clones.

## Documentation

- [AGENTS.md](AGENTS.md) — instructions for coding agents working in this repo
- [Design & Specification](docs/DESIGN.md) — architecture, precedence rules, and vendor guidance
- [Agent Skill](skill/SKILL.md) — teach your agent to *operate* the skillsync CLI (not to change this repo)
- [Contributing](CONTRIBUTING.md) · [Changelog](CHANGELOG.md) · [Code of Conduct](CODE_OF_CONDUCT.md)

## License

[MIT](LICENSE) © formenos.land
