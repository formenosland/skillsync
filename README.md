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

The store only ever contains symlinks created by skillsync. Your skill files stay in git repos or folders you control — skillsync never deletes them.

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

`init` migrates per-agent skills into `sources/local/` and replaces each agent's skills folder with a symlink to the store (originals go under `backups/`).

## Commands

| Command | Description |
|---------|-------------|
| `init` | Create the store, migrate per-agent skills, link agent views |
| `add <url\|path>` | Register a git repo or local folder; pick which skills to install |
| `sync` | Pull git sources; fill vacant names. Also `apply` if `skillsync.toml` is found |
| `apply` | Install/update repo skills from `skillsync.toml` (`--global`, `--prune`) |
| `unapply [names…]` | Remove skillsync-managed project links (home store untouched) |
| `remove [names...]` (`rm`) | Unlink skills from the store (every agent); bare `remove` picks |
| `remove --all` | Remove every installed skill |
| `remove --source <url\|path>` | Unregister a source and drop its skills |
| `list` (`ls`) | Installed skills (grouped on a tty; names when piped) |
| `status` | Paths, source health, agent views |
| `doctor` | Broken links, drifted views |
| `uninstall [--keep] [--purge]` (`nuke`) | Reverse `init` (see below) |
| `completion bash\|zsh` | Shell completion (`skillsync remove <TAB>` completes skills) |

**Global flags** (before or after the subcommand): `--dry-run`, `--yes` / `-y`, `--copy`.

```sh
skillsync --dry-run remove foo
skillsync remove foo --dry-run
```

`remove` unlinks a store name immediately. Source files stay. The name is recorded in `skillsyncrc` (`excludes`) so `sync` will not restore it.

### Two kinds of uninstall

| Command | Removes |
|---------|---------|
| `skillsync uninstall` | View symlinks (`--keep` copies, `--purge` wipes data; type `nuke`) |
| `brew uninstall skillsync` | The keg, not skill data |

A `go install` binary is just a file on `PATH`. Leftover from the old curl `install.sh`: `~/.local/bin/skillsync` and `~/.local/share/skillsync/app/`.

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
3. **Sources** — git URLs cloned under `sources/<host>/<owner>/<repo>/`; local folders referenced in place. Occupied names are not replaced by default.
4. **`skillsyncrc`** — TOML list of git URLs or absolute paths, plus `excludes` and optional `[[agents]]`.
5. **Repo `skillsync.toml`** — team allowlist. `apply` links those names under `.agents/skills/` (and extra `[views]` agent `project_path`s). Managed names are gitignored; first-party skill dirs stay yours.

### Project skills

Commit `skillsync.toml` at the repo root:

```toml
[skills]
sources = [
  { url = "https://github.com/acme/skills", ref = "v1.2.0", skills = ["foo", "bar"] },
]

[views]
ids = ["claude-code"]   # optional extra folders (.claude/skills); .agents/skills is always used
```

```sh
skillsync --yes apply              # install/update project links
skillsync --yes apply --prune      # drop names that left the file
skillsync --yes apply --global     # also put those names in your home store
skillsync unapply foo              # remove project links only
```

A first-party skill occupying the same name in that folder is a conflict: apply stops. `sync` from inside the repo also refreshes project links. `remove` still only affects the home store.

Everything lives in XDG paths (no new dotfolder in your home):

| What | Path |
|------|------|
| Config (`skillsyncrc`) | `${XDG_CONFIG_HOME:-~/.config}/skillsyncrc` |
| Data (`store/`, `sources/`, `backups/`, `app/`) | `${XDG_DATA_HOME:-~/.local/share}/skillsync/` |
| Single-root override | `SKILLSYNC_HOME=<dir>` (config at `$SKILLSYNC_HOME/skillsyncrc`) |

`backups/` is only from `init` (leftovers when replacing a real agent skills folder), not a library backup.

## Supported agents

[`internal/agentregistry/agents.tsv`](internal/agentregistry/agents.tsv) covers ~75 agents — Claude Code, Cursor, Codex, Gemini CLI, GitHub Copilot, OpenCode, Zed, Cline, Warp, Goose, Windsurf, Kiro, Junie, Amp, Hermes, OpenClaw, and more. It is **generated, never hand-edited** (see [Contributing](CONTRIBUTING.md) to refresh it). Paths may use env-overridable homes (`CODEX_HOME`, `CLAUDE_CONFIG_DIR`, `HERMES_HOME`, …) and `|` fallback chains (OpenClaw's `~/.openclaw` → `~/.clawdbot` → `~/.moltbot`). `init` only links agents that are actually installed.

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
| Project skills | `skillsync.toml` + `apply` (gitignored links in `.agents/skills`) | default: copy into the project and typically commit |
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

**Which skill folders are scanned?** Only these layouts (dot folders at the source root are ignored):

1. `SKILL.md` in the source root, or in an immediate child folder (`<name>/SKILL.md`)
2. `skills/<name>/SKILL.md`
3. `skills/<category>/<name>/SKILL.md` — e.g. [mattpocock/skills](https://github.com/mattpocock/skills/tree/main/skills)

Nothing else is collected. The store stays flat by skill `name`.

**Windows?** Directory views use a symlink when possible, otherwise a junction. `--copy` for filesystems without links.

**Scripts?** Piped `list` is names only; `--yes` skips prompts; `doctor` exits 1 only on actionable problems.

## Documentation

- [AGENTS.md](AGENTS.md) — instructions for coding agents working in this repo
- [Design & Specification](docs/DESIGN.md) — architecture and vendor guidance
- [Agent Skill](skill/SKILL.md) — teach your agent to *operate* the skillsync CLI (not to change this repo)
- [Contributing](CONTRIBUTING.md) · [Changelog](CHANGELOG.md) · [Code of Conduct](CODE_OF_CONDUCT.md)

## License

[MIT](LICENSE) © formenos.land
