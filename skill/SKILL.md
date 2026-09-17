---
name: skillsync
description: >
  Unified AI agent skill installation. Use when bootstrapping a machine for
  multi-agent skill setup, adding skill repos, removing skills, syncing skill
  sources, uninstalling, diagnosing broken skill symlinks, or interpreting
  skillsync doctor output. Triggers on "skillsync", "unify skills",
  "sync agent skills", "remove a skill everywhere",
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
- **Add a repo or folder** — `add` then pick names (TTY) or `--yes` for uniques.
- **Repo skills** — `apply` from a directory that has `skillsync.toml`.
- **Drop a skill everywhere** — `remove <name>` (or bare `remove` for a picker).
- **Something broken** — `doctor` finds drifted views and broken links.
- **Leaving** — `uninstall` (clean reverse of init), `--purge` to erase all data.

## Bootstrap

```sh
brew install formenosland/tap/skillsync
# or: go install github.com/formenosland/skillsync/cmd/skillsync@latest
skillsync init
```

From a checkout: `make` then run `./bin/skillsync init` (or `make install` onto `GOBIN`).

## Commands

```sh
skillsync --yes init                 # non-interactive init (link all candidates)
skillsync init                       # bootstrap: migrate + link agents (picker on tty)
skillsync --yes add acme-corp/skills # unique names only; warn on collisions
skillsync add ~/dev/my-skills        # local folder as a pointer
skillsync sync                       # pull sources, fill vacant names; also apply if skillsync.toml is present
skillsync --yes apply                # install/update project links from skillsync.toml
skillsync --yes apply --prune        # drop project links that left the manifest
skillsync --yes apply --global       # also install those names into the home store
skillsync unapply                    # remove managed project links (not the home store)
skillsync remove terse               # gone from every agent, instantly
skillsync remove                     # interactive picker (tty, no args)
skillsync --yes remove --all         # remove every installed skill (scripts)
skillsync remove --source ~/dev/my-skills
skillsync list                       # catalog on a tty; names when piped
skillsync list --names               # always one name per line (completion)
skillsync status                     # dashboard: paths, source health, views
skillsync doctor                     # exit 1 on actionable findings only
skillsync uninstall                  # remove views; store/config kept
skillsync uninstall --keep           # views become real copies instead
skillsync uninstall --purge          # type nuke to confirm; --yes skips prompts
```

Global flags work before or after the subcommand: `skillsync --dry-run sync`,
`skillsync init --yes`. Flags: `--dry-run` (preview), `--yes` / `-y` (no
prompts), `--copy` (no-symlink filesystems). Without a TTY, `init`/`add` need
`--yes`; bare `remove` needs skill names or `--all`. Occupied store names are
not replaced unless the user checks override on `add`. Store skill names
follow the Agent Skills `name` rules: `^[a-z0-9]+(-[a-z0-9]+)*$`, max 64 chars.

## Key semantics

- `remove` deletes the store symlink and records the name in
  `skillsyncrc` (`excludes`) so `sync` won't restore it. Re-`add`
  the source (or edit that file) to bring it back. No backups needed —
  source files are untouched. Unchecked unique names on `add` are excluded
  the same way.
- `init` migrates skills found in real agent folders into
  `~/.local/share/skillsync/sources/local/` and registers that path in
  `skillsyncrc`, backs up what it replaces, and only links agents that are
  actually installed.
- Git clones live under `sources/<host>/<owner>/<repo>/`. Path sources are
  not copied. Skills are only those with `SKILL.md` in a root folder,
  `skills/<name>`, or `skills/<category>/<name>`. The store is flat by name;
  `list` / `add` group by category. `add` names these layouts if it finds none.
- Real directories in the store are unmanaged: sync skips them, remove
  refuses them, doctor tells you to move them into a source and `add` it.

## Doctor output

Exit code 1 only for **actionable** errors (broken links, drifted/wrong/not-linked views). Warnings (unmanaged dir, missing clone) do not fail the command.

| Finding | Meaning | Fix |
|---------|---------|-----|
| broken link | Source moved or deleted | `sync`, or `remove --source` |
| drifted view | Agent recreated a real folder | `init` |
| wrong link | View points somewhere else | `init` |
| not linked | Installed agent without a view | `init` |
| unmanaged dir in store | Files placed in store by hand | Move to a source, `add` |

## Cautions

- **Two uninstalls:** `brew uninstall skillsync` (or delete a `go install`
  binary) removes the tool only. `skillsync uninstall` removes agent view
  symlinks; `--purge` also deletes store, sources, and config.
- `uninstall --purge` deletes the store, cloned sources (including
  migrated `sources/local/`), config, and backups. Confirm by typing
  `nuke`; `skillsync --yes uninstall --purge` skips that prompt. Everything
  else is non-destructive to skill files.
- Registry gaps: add `[[agents]]` tables to `skillsyncrc`
  (`id`, `display_name`, `global_path`, `project_path`).
- Project skills: commit `skillsync.toml` (`[skills].sources`, optional
  `[views].ids`). `apply` links into `.agents/skills/` (flat names) and
  extra harness `project_path`s. First-party occupants collide and stop
  apply. `unapply` does not call `remove`.

## Environment

```sh
SKILLSYNC_HOME=/one/root      # force single-root layout (default: XDG dirs)
SKILLSYNC_REGISTRY=/path.tsv  # alternate registry
NO_COLOR=1                    # plain output
```

Shell completion: `eval "$(skillsync completion bash)"` (or `zsh`).
Full design: `docs/DESIGN.md`. Agent registry: `internal/agentregistry/agents.tsv`
(generated file; do not hand-edit). Local path overrides: `[[agents]]`
in `skillsyncrc`.
