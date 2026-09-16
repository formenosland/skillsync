# skillsync

> One skill store. Every agent. Install a skill once — it shows up in Claude Code, Cursor, Codex, Gemini CLI, and the rest of your toolchain.

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go](https://img.shields.io/badge/Go-1.22+-00ADD8)](https://go.dev)
[![CI](https://github.com/formenosland/skillsync/actions/workflows/ci.yml/badge.svg)](https://github.com/formenosland/skillsync/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/formenosland/skillsync)](https://github.com/formenosland/skillsync/releases)

AI coding agents support [Agent Skills](https://agentskills.io) — but each one wants them in its own folder (`~/.claude/skills`, `~/.codex/skills`, `~/.cursor/skills`, …). The same skill ends up copied into many global directories.

**skillsync** ends it with a *one store, many views* model (think GNU Stow, for agent skills):

- Skills live in **one canonical store**.
- Every agent's skills folder becomes a **symlink into that store**.
- Git repos are cloned; local folders are pointers. You pick which names to install.

```console
$ skillsync add acme-corp/skills
skillsync add https://github.com/acme-corp/skills
  ✓ foo
  ✓ bar
add done (2 ok, 0 warnings)

$ skillsync list
github.com/acme-corp/skills
  foo             Foo skill description
  bar             Bar skill description

$ skillsync status
skillsync 0.3.4
  store     ~/.local/share/skillsync/store
  config    ~/.config/skillsyncrc
  skills    2 from 1 source

Sources
  up to date github.com/acme-corp/skills

Agent views
  linked     ~/.claude/skills        claude-code
  linked     ~/.cursor/skills        cursor
  linked     ~/.codex/skills         codex
  linked     ~/.agents/skills        cline dexto warp zed …
```

The store only ever contains symlinks created by skillsync. Your skill files stay in git repos or folders you control — skillsync never deletes them. `skillsync --version` prints the installed version.

Zero extra runtime tools: one static binary (git clone/pull via the library).

## Install

```sh
brew install formenosland/tap/skillsync
```

```sh
go install github.com/formenosland/skillsync/cmd/skillsync@latest
```

```sh
git clone https://github.com/formenosland/skillsync.git
cd skillsync
make          # ./bin/skillsync
make install  # go install ./cmd/skillsync  (GOBIN / GOPATH/bin)
```

Homebrew tracks tagged releases via [formenosland/homebrew-tap](https://github.com/formenosland/homebrew-tap); each GitHub Release bumps `Formula/skillsync.rb` through that tap’s `bump.yml` workflow. `go install @latest` follows the module proxy. Do not mix Homebrew and `go install` on `PATH` unless you know which binary wins.

## Quickstart

```sh
skillsync init                    # link every installed agent to the store
skillsync add acme-corp/company-skills
skillsync add alice/my-skills     # GitHub shorthand
skillsync add ~/dev/my-skills     # local folder (pointer, not a copy)
skillsync sync                    # pull git sources, refresh vacant names
```

`init` detects installed agents, moves any existing per-agent skills into `sources/local/` (then registers that path), and replaces each agent's skills folder with a symlink to the store. Anything replaced is backed up first (`~/.local/share/skillsync/backups/`).

On a TTY, `add` shows a checkbox list: new names on by default; names already in the store are marked `override` with the current occupant. Checking an override replaces that symlink. `--yes` installs unique names only and warns on conflicts.

## Commands

| Command | Description |
|---------|-------------|
| `init` | Create the store, migrate per-agent skills, link agent views |
| `add <url\|path>` | Register a git repo or local folder; pick which skill names to install |
| `sync` | Pull all git sources; fill vacant names; never steal occupied names |
| `remove [names...]` (`rm`) | Remove skills from everywhere; bare `remove` opens an interactive picker |
| `remove --all` | Remove every installed skill (use `skillsync --yes remove --all` in scripts) |
| `remove --source <url\|path>` | Unregister a source and drop its skills |
| `list` (`ls`) | Catalog grouped by source with descriptions (tty); one name per line when piped. `--pretty` / `--names` (`-1`) force either form |
| `status` | Dashboard: paths, counts, source health vs last fetch, agent views, excludes |
| `doctor` | Diagnose broken links, drifted views, missing links (exit 1 on actionable findings; warnings alone do not fail) |
| `uninstall [--keep] [--purge]` (`nuke`) | Reverse `init` (see below) |
| `completion bash\|zsh` | Shell completion (`skillsync remove <TAB>` completes skills) |

**Global flags** (before or after the subcommand): `--dry-run` (preview everything), `--yes` / `-y` (skip prompts; non-interactive init/add/remove), `--copy` (filesystems without symlinks).

```sh
skillsync --yes init
skillsync init --yes
skillsync --yes add acme-corp/skills
skillsync --dry-run remove foo
skillsync remove foo --dry-run
```

Non-interactive: `skillsync init --yes` when agents need linking; `skillsync --yes add <source>`; `skillsync remove foo` or `skillsync remove --all --yes` (bare `remove` with no TTY needs names or `--all`).

`remove` drops the skill from the store — and therefore every agent — immediately. Source files are never touched. The name is recorded in `skillsyncrc` (`excludes`) so `sync` won't resurrect it; re-`add` the source (or edit the file) to bring it back. Names you uncheck on `add` (new names only) are excluded the same way.

### Two kinds of uninstall

| Command | Removes |
|---------|---------|
| `skillsync uninstall` | Agent view symlinks only (clean reverse of `init`); add `--keep` to leave real copies in each agent folder, `--purge` to also delete store, sources, and config (type `nuke` to confirm; `skillsync --yes uninstall --purge` skips prompts) |
| `brew uninstall skillsync` | The Homebrew keg only — never your skill data |

A `go install` binary is just a file on `PATH` (`$(go env GOPATH)/bin/skillsync` unless `GOBIN` is set); delete it to remove the tool. If you previously used the old curl `install.sh`, remove `~/.local/bin/skillsync` and `~/.local/share/skillsync/app/` — that copy is unused now and is not skill data.

### Shell completion

```sh
eval "$(skillsync completion bash)"   # ~/.bashrc
eval "$(skillsync completion zsh)"    # ~/.zshrc
```

## How it works

```
   sources (yours)                    store (installed names)            views (vendor folders)
┌──────────────────────────┐   ┌────────────────────────────────┐   ┌────────────────────────┐
│ git clones under sources/│   │ ~/.local/share/skillsync/store │   │ ~/.claude/skills     ──┼─┐
│ dir: ~/dev/my-skills     │──>│   skill-a -> sources/host/…    │<──┼ ~/.codex/skills        │ │ symlink
│ sources/local (init dump)│   │   skill-b -> ~/dev/my-skills/… │   │ ~/.agents/skills     ──┼─┘ to store
└──────────────────────────┘   └────────────────────────────────┘   │ … ~75 agents           │
                                                                    └────────────────────────┘
```

1. **Store** — one symlink per *installed* skill name, pointing into a source. Agent views are a single symlink to this directory.
2. **Views** — each agent's global skills dir is a symlink to the store.
3. **Sources** — git URLs cloned under `sources/<host>/<owner>/<repo>/`; local folders referenced in place. Occupied names are not replaced unless you check override on `add`.
4. **`skillsyncrc`** — TOML list of git URLs or absolute paths, plus `excludes` and optional `[[agents]]`.

Everything lives in XDG paths (no new dotfolder in your home):

| What | Path |
|------|------|
| Config (`skillsyncrc`) | `${XDG_CONFIG_HOME:-~/.config}/skillsyncrc` |
| Data (`store/`, `sources/`, `backups/`, `app/`) | `${XDG_DATA_HOME:-~/.local/share}/skillsync/` |
| Single-root override | `SKILLSYNC_HOME=<dir>` (config at `$SKILLSYNC_HOME/skillsyncrc`) |

`backups/` is only from `init` (leftovers when replacing a real agent skills folder), not a library backup.

## Supported agents

[`internal/agentregistry/agents.tsv`](internal/agentregistry/agents.tsv) covers ~75 agents — Claude Code, Cursor, Codex, Gemini CLI, GitHub Copilot, OpenCode, Zed, Cline, Warp, Goose, Windsurf, Kiro, Junie, Amp, Hermes, OpenClaw, and more. It is **generated, never hand-edited** (see [Contributing](CONTRIBUTING.md) to refresh it). Paths may use env-overridable homes (`CODEX_HOME`, `CLAUDE_CONFIG_DIR`, `HERMES_HOME`, …) and `|` fallback chains (OpenClaw's `~/.openclaw` → `~/.clawdbot` → `~/.moltbot`). `init` only links agents that are actually installed — it never litters your home directory.

Agent missing or path wrong? Add an `[[agents]]` table to `skillsyncrc` (fields `id`, `display_name`, `global_path`, `project_path`; wins by `id`) — and please open an issue or PR.

## Compared to vercel-labs/skills

[vercel-labs/skills](https://github.com/vercel-labs/skills) (`npx skills`) and skillsync solve adjacent jobs. Neither replaces the other; the table is approach, not ranking.

| | skillsync | vercel-labs/skills |
|---|---|---|
| Skill format | [Agent Skills](https://agentskills.io) (`SKILL.md`) | same |
| Git / local sources | git URL, `owner/repo`, local path | same, plus GitLab, archives, direct file URLs |
| Agent coverage | ~75 filesystem agents | same ecosystem (overlapping path list) |
| Layout | one canonical store; each agent's **skills directory** is a view (Stow) | install **into each chosen agent directory** (per-skill symlink or copy) |
| Name collisions | occupant stays; `add` override is explicit | project directory vs `-g` global |
| Project skills (`.agents/skills/` in a repo) | not managed — the repo owns them | default install scope |
| Discovery / marketplace | out of scope ([skills.sh](https://skills.sh) if you want it) | `skills find`, skills.sh |
| Use without installing | — | `skills use` (temp files + prompt) |
| Author a skill template | — | `skills init` |
| Runtime | static Go binary | Node (`npx`) |
| Telemetry | none | on by default (`DISABLE_TELEMETRY` / `DO_NOT_TRACK`) |
| Install safety | owns only store/view **links**; never deletes skill files | writes/removes under agent skill dirs |
| Diagnose layout | `doctor` (drifted views, broken links) | — |

After `skillsync init`, agent folders are views into the store, so `npx skills add …` still works: it writes into a view and the skill is visible to every linked agent.

## FAQ

**What if an agent recreates its skills folder as a real directory?** `skillsync doctor` flags it as a drifted view; `skillsync init` heals it (migrating any new skills it finds).

**Windows?** Directory views use a symlink when the OS allows it, otherwise a junction. `--copy` remains for filesystems without links.

**Is output scriptable?** Yes: colors and symbols degrade automatically when piped (or with `NO_COLOR`/`TERM=dumb`), `list` emits plain names when piped (or with `--names`), global `--yes` skips confirmations (including the `nuke` typed confirm for `--purge`), and `doctor` exits 1 only for actionable problems (broken links, drifted/wrong/missing views)—not for informational warnings such as missing clones.

## Documentation

- [AGENTS.md](AGENTS.md) — instructions for coding agents working in this repo
- [Design & Specification](docs/DESIGN.md) — architecture and vendor guidance
- [Agent Skill](skill/SKILL.md) — teach your agent to *operate* the skillsync CLI (not to change this repo)
- [Contributing](CONTRIBUTING.md) · [Changelog](CHANGELOG.md) · [Code of Conduct](CODE_OF_CONDUCT.md)

## License

[MIT](LICENSE) © formenos.land
