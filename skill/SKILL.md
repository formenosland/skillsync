---
name: skillsync
description: >
  Unified AI agent skill installation. Use when bootstrapping a machine for
  multi-agent skill setup, adding org or personal skill repos, removing
  skills, syncing skill sources, uninstalling, diagnosing broken skill
  symlinks, or interpreting skillsync doctor output. Triggers on "skillsync",
  "unify skills", "sync agent skills", "remove a skill everywhere",
  "skills not showing up in Cursor/Claude/Codex", or agent skill path
  questions.
---

# skillsync

One canonical skill store, many agent views. Skills live in
`~/.local/share/skillsync/store/` (symlinks into sources you control); every
agent's global skills folder (`~/.claude/skills`, `~/.codex/skills`,
`~/.agents/skills`, ~75 agents total) is a symlink to that store.

Invariant: skillsync owns the store; humans own sources. It never deletes
skill files — only links.

## When to use

- **New machine** — install the CLI, then `init` once.
- **Company skills** — `add` the org repo with `--layer org`.
- **Personal skills** — `add` your repo or a local folder (default `user`).
- **Skills stale** — `sync` pulls all git sources and refreshes links.
- **Drop a skill everywhere** — `remove <name>` (or bare `remove` for a picker).
- **Something broken** — `doctor` finds drifted views, broken links, collisions.
- **Leaving** — `uninstall` (clean reverse of init), `--purge` to erase all data.

## Bootstrap

```sh
brew install formenosland/tap/skillsync
# or: curl -fsSL https://raw.githubusercontent.com/formenosland/skillsync/main/install.sh | sh
skillsync init
```

From a checkout: `./install.sh` then `skillsync init`.

## Commands

```sh
skillsync --yes init                 # non-interactive init (link all candidates)
skillsync init                       # bootstrap: migrate + link agents (picker on tty)
skillsync add acme-corp/skills --layer org
skillsync add ~/dev/my-skills        # local folder as a source
skillsync sync                       # pull sources, refresh store
skillsync remove terse               # gone from every agent, instantly
skillsync remove                     # interactive picker (tty, no args)
skillsync --yes remove --all         # remove every installed skill (scripts)
skillsync remove --source ~/dev/my-skills
skillsync list                       # plain names (scripts, completion)
skillsync status                     # skills, origins, view states
skillsync doctor                     # exit 1 on actionable findings only
skillsync uninstall                  # remove views; store/config kept
skillsync uninstall --keep           # views become real copies instead
skillsync uninstall --purge          # type nuke to confirm; --yes skips prompts
```

Global flags work before or after the subcommand: `skillsync --dry-run sync`,
`skillsync init --yes`. Flags: `--dry-run` (preview), `--yes` / `-y` (no
prompts), `--copy` (no-symlink filesystems). Without a TTY, bare `remove`
needs skill names or `--all`. Layer precedence: `org` < `user` < `local`
(highest wins on name collision). Store skill names follow the Agent Skills
`name` rules: `^[a-z0-9]+(-[a-z0-9]+)*$`, max 64 chars.

## Key semantics

- `remove` deletes the store symlink and records the name in
  `~/.config/skillsync/exclude.conf` so `sync` won't restore it. Re-`add`
  the source (or edit that file) to bring it back. No backups needed —
  source files are untouched.
- `init` migrates skills found in real agent folders into
  `~/.local/share/skillsync/sources/local/` (still yours; edit or move
  them), backs up what it replaces, and only links agents that are
  actually installed.
- Real directories in the store are unmanaged: sync skips them, remove
  refuses them, doctor tells you to move them into a source and `add` it.

## Doctor output

Exit code 1 only for **actionable** errors (broken links, drifted/wrong/not-linked views). Warnings (unmanaged dir, missing clone, collision info) do not fail the command.

| Finding | Meaning | Fix |
|---------|---------|-----|
| broken link | Source moved or deleted | `sync`, or `remove --source` |
| drifted view | Agent recreated a real folder | `init` |
| wrong link | View points somewhere else | `init` |
| not linked | Installed agent without a view | `init` |
| unmanaged dir in store | Files placed in store by hand | Move to a source, `add` |
| collision (info) | Same name in two layers | Expected; highest layer wins |

## Cautions

- **Two uninstalls:** `install.sh --uninstall` (or `curl ... | sh -s -- --uninstall`)
  removes only the curl-installed binary and app under
  `~/.local/share/skillsync/app/`. `brew uninstall skillsync` removes the
  Homebrew keg. Neither touches skill data. `skillsync uninstall` removes
  agent view symlinks; `--purge` also deletes store, sources, and config.
- `uninstall --purge` deletes the store, cloned sources (including
  migrated `sources/local/`), config, and backups. Confirm by typing
  `nuke`; `skillsync --yes uninstall --purge` skips that prompt. Everything
  else is non-destructive to skill files.
- Registry gaps: add rows to `~/.config/skillsync/agents.local.tsv`
  (format: `agent_id<TAB>display_name<TAB>global_path<TAB>project_path`).
- Project scope (`.agents/skills/` in a repo) is the repo's business,
  not skillsync's.

## Environment

```sh
SKILLSYNC_HOME=/one/root      # force single-root layout (default: XDG dirs)
SKILLSYNC_REGISTRY=/path.tsv  # alternate registry
NO_COLOR=1                    # plain output
```

Shell completion: `eval "$(skillsync completion bash)"` (or `zsh`).
Full design: `docs/DESIGN.md`. Registry: `registry/agents.tsv`
(generated from vercel-labs/skills; regenerate via `registry/generate.sh`).
