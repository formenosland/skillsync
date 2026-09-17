# skillsync — Design & Specification

**Status:** Stable (v0.3 series) · **Maintainer:** formenos.land

skillsync defines a vendor-neutral convention for installing and managing AI agent skills across tools: a canonical skill store with per-agent directory views. This document is the reference for the design; the [README](../README.md) covers day-to-day usage.

## 1. Problem

### 1.1 The N × M mess

AI agent skills are an open standard ([agentskills.io](https://agentskills.io), Linux Foundation AAIF). A skill is a directory containing `SKILL.md` with YAML frontmatter (`name`, `description`, …) and markdown instructions.

Adoption is broad (70+ tools), but installation is fragmented:

| Dimension | Fragmentation |
|-----------|---------------|
| **Scope** | Global (user home) vs. project (repo) vs. org (company) |
| **Agent** | Each tool expects skills in its own folder |
| **Tooling** | Per-agent CLIs, manual copying, ad-hoc scripts |

A developer using Claude Code, Cursor, Codex, and Copilot may maintain four copies of the same skill in four global directories, plus project copies.

### 1.2 What already works

- **Project scope** is converging on `.agents/skills/` (Codex, Cursor, Gemini CLI, Copilot, Zed, Cline, Warp, Amp, …).
- **The SKILL.md format** is standardized and interoperable.
- **The vercel-labs `skills` CLI** (`npx skills add …`) fetches skills into chosen agent (and project) directories — useful distribution, a different layout model.

What's missing is a **single write path** (one store, many views).

## 2. Prior Art

| System | Analogy |
|--------|---------|
| [agentskills.io](https://agentskills.io) | Skill *format* spec — what a skill is |
| [vercel-labs/skills](https://github.com/vercel-labs/skills) | Per-agent installer + discovery; overlapping agent-path list (optional registry refresh) |
| **GNU Stow** | Symlink farm — one source tree, many "views" |
| **XDG Base Directory** | Predictable config/data locations without home-dir clutter |
| **npm / cargo / go modules** | Manifest + lock + vendor dir (heavier than needed here) |

skillsync applies the Stow pattern: one canonical store, many symlinks.

## 3. Design

### 3.1 Core model

```
   sources (humans own these)              store (installed names)             views (vendor folders)
┌────────────────────────────┐      ┌───────────────────────────────┐    ┌──────────────────────────┐
│ git clones (host/owner/repo)│      │ ~/.local/share/skillsync/store│    │ ~/.agents/skills      ───┼──┐
│ dir: ~/dev/my-skills        │ ───> │   skill-a -> sources/…        │<───┼ ~/.claude/skills         │  │
│ sources/local (init dump)   │      │   skill-b -> ~/dev/my-skills/…│    │ ~/.codex/skills          │  │ every view is a
└────────────────────────────┘      └───────────────────────────────┘    │ … (~75 agents)        ───┼──┘ store
                                                                         └──────────────────────────┘
```

**Store** — a single canonical directory of per-skill symlinks (the installed name index). Not a vendor path: it lives in skillsync's own XDG data directory, so no vendor semantics can ever collide with it. Agent views are a whole-directory symlink to this folder.

**Views** — every agent's global skills directory (including `~/.agents/skills`, which some vendors read natively) is a whole-directory symlink to the store. Adding a skill once makes it visible to every agent. No per-agent bookkeeping.

**Ownership invariant** — *skillsync owns the store; humans own sources.* The store contains only symlinks created by skillsync. A real directory found in the store is unmanaged: `sync` skips it, `remove` refuses it, `doctor` flags it with advice to move it into a source. skillsync never deletes skill files — only links.

**Occupancy** — a store name is occupied or vacant. `add` does not replace an occupant unless the user checks that row (override). `--yes add` installs unique names only and warns. `sync` never steals an occupied name. A vacant name with two or more providers is left unlinked (warn). In a repo, `apply` links selected names under `.agents/skills/` (and extra `[views]` paths); an unmanaged occupant there is a hard failure. Agents resolve project vs global nearest-first.

### 3.2 Locations (XDG)

| What | Path |
|------|------|
| Config (`skillsyncrc`) | `${XDG_CONFIG_HOME:-~/.config}/skillsyncrc` |
| Store, cloned sources, backups | `${XDG_DATA_HOME:-~/.local/share}/skillsync/` |
| Single-root override | `SKILLSYNC_HOME` env var (`$SKILLSYNC_HOME/skillsyncrc`) |

No new dotfolder in `~`. This matches the practice of OpenCode, Amp, Goose, and the vercel CLI on both Linux and macOS.

### 3.3 Sources file

File: `${XDG_CONFIG_HOME:-~/.config}/skillsyncrc` — TOML:

```toml
sources = [
  "https://github.com/alice/personal-skills",
  "/home/alice/dev/experimental-skills",
]
excludes = ["old-skill"]
```

- `#` comments in TOML; `sources` is a list of git URLs or absolute paths.
- Git sources are cloned to `~/.local/share/skillsync/sources/<host>/<owner>/<repo>/` (scheme and trailing `.git` stripped; `..` segments rejected). `owner/repo` shorthand is GitHub HTTPS. Local paths are referenced in place (no copy).
- `init` may evacuate real agent-folder skills into `sources/local/` and **register that path in `skillsyncrc`** like any other path source. It is not an implicit extra source.

### 3.3a Project manifest

File: `<repo>/skillsync.toml` (walk up from cwd). Unknown keys are rejected.

```toml
[skills]
sources = [
  { url = "https://github.com/acme/skills", ref = "v1.2.0", skills = ["foo", "bar"] },
  { url = "./vendor-skills", skills = ["*"] },
]

[views]
ids = ["claude-code", "codex"]
```

- `[skills].sources` is the allowlist (`ref` optional; omit `skills` or use `"*"` for all names). Git URLs clone into the same XDG `sources/` tree. Paths must be relative to the repo (no absolute/`~` paths).
- Always own `.agents/skills`. `[views].ids` are registry agent ids; we also own each id’s `project_path` (deduped). Do not list `.claude` itself — only the skills subdir from the registry.
- `apply` creates **flat** per-skill symlinks (`<view>/<name>` → clone). A real directory or foreign link in that slot is a conflict and apply stops. A generated `.gitignore` block lists managed names so Git stays clean; first-party skill dirs are not listed.
- `apply --global` also installs those names into the home store (occupancy same as `add --yes`). `remove` is home-store only; `unapply` drops project links.

Skills under a skills root stay one level deep (`<root>/<name>/SKILL.md`). Nested namespace folders are not used (many clients, including Zed, scan one level).

### 3.4 Skill discovery

Within each source, skill directories (containing `SKILL.md`) are found at depth 1, depth 2, and under a `skills/` container. The store name comes from the `name:` frontmatter field, falling back to the directory basename. Names must match the [Agent Skills](https://agentskills.io/specification) `name` rules: 1–64 characters, `/^[a-z0-9]+(-[a-z0-9]+)*$/` (lowercase, digits, single hyphens; no leading/trailing/consecutive hyphens). Unsafe names are skipped during materialize/migration and refused by `remove`.

### 3.5 Agent registry

`internal/agentregistry/agents.tsv` maps agent ids to global and project paths. Requirements:

- **Generated, not hand-written.** The committed TSV is produced by `go run ./internal/agentregistry/gen` (`make agentregistry`). That command refreshes rows from vercel-labs/skills `src/agents.ts` (pinned SHA in the file header) so path coverage stays aligned with that CLI; runtime still needs no network. This is a maintenance convenience, not the origin of the store/view model.
- **Expressive paths.** Rows may use `~`, `${VAR:-default}` (env-overridable homes such as `CODEX_HOME`, `CLAUDE_CONFIG_DIR`, `HERMES_HOME`), and `|`-separated alternates (first whose parent directory exists wins — e.g. OpenClaw's `~/.openclaw` → `~/.clawdbot` → `~/.moltbot`).
- **Project-only agents** carry `-` as global path and get no view.
- **Local override:** `[[agents]]` in `skillsyncrc` (fields `id`, `display_name`, `global_path`, `project_path`) merges over the shipped registry, winning by `id`. Users can add unlisted agents or correct paths without touching the installation.
- **Install detection:** a view is only created when the agent's parent folder exists, so uninstalled agents never cause folder litter.

Non-filesystem vendors (e.g. Perplexity Computer, whose skills live in a cloud library) are documented as out of scope.

### 3.6 Collision rules

| Situation | Resolution |
|-----------|------------|
| `add` unique name | Linked after picker / `--yes` |
| `add` name already in store | Unchecked by default (`override`); `--yes` skips and warns |
| `sync`, vacant, one provider | Link |
| `sync`, vacant, two+ providers | Leave vacant; warn |
| Occupied store name | Never retargeted by `sync` |
| Unmanaged real directory in store | Never touched; flagged by doctor |
| Project apply vs first-party dir | Hard-fail (every colliding view+name) |
| Project skill vs global skill | Agent resolves nearest scope itself |

## 4. Operations

| Command | Semantics |
|---------|-----------|
| `init` | Create store; for each installed agent: migrate real skills to `sources/local/`, register that path, back up the folder, replace it with a view symlink. Idempotent. Interactive agent selection on a tty; non-interactive init requires global `--yes`. |
| `add` | Register a source, fetch it, pick names to symlink. TTY checkbox (overrides explicit). `--yes` installs unique names only. Unchecked unique names go to `excludes` in `skillsyncrc`. Non-TTY without `--yes` refuses. |
| `sync` | Pull all git sources, fill vacant names, prune broken links. Never steal occupied names. If `skillsync.toml` is found walking up from cwd, also materialize that project. |
| `apply` | Fetch project sources (optional `ref`), link allowlisted names into `.agents/skills` and `[views]` paths, rewrite managed gitignore blocks. Non-interactive requires `--yes`. |
| `apply --global` | Same, then install those names into the home store and register git/path sources in `skillsyncrc`. Occupied home names are skipped (warn). |
| `apply --prune` | Remove project symlinks whose names left the manifest. |
| `unapply [names…]` | Remove skillsync-managed project symlinks (all, or named). Does not touch the home store. |
| `remove` | Delete the skill's store symlink (visible everywhere instantly) and record the name in `excludes` so sync won't restore it. No backups — source files are never touched, so nothing is lost. Refuses unmanaged entries. Bare `remove` on a tty opens a picker; without names, non-interactive use requires skill arguments or `--all` (`skillsync --yes remove --all` removes everything). |
| `remove --all` | Remove every skill currently in the store (same per-skill semantics as `remove <name>`). |
| `remove --source` | Drop the manifest entry, delete the clone (managed clones only — local folders are kept), remove its store links, re-materialize. |
| `list` | On a tty: skills grouped by source, with the first line of each `description`. Piped / `--names` (`-1`): plain names for scripts and completion. `--pretty` forces the catalog when stdout is not a tty. |
| `status` | Dashboard: version, store/config paths, skill and source counts, source health vs last fetched origin (`up to date` / `behind` / `ahead` / `diverged` / `local` / `missing`; path sources: `path`). No network. Not a skill catalog (`list` is). |
| `doctor` | Broken links, drifted views, wrong links, missing sources, unlinked installed agents, project apply drift/collisions. Exit 1 on actionable findings. |
| `uninstall` | Reverse of init: remove all view symlinks (default), or convert views to real copies (`--keep`). `--purge` deletes all skillsync data after typing the confirmation word `nuke`; global `--yes` skips that prompt. |

**Global options** (before or after the subcommand): `--dry-run`, `--yes` / `-y`, `--copy`. Examples: `skillsync --dry-run remove foo`, `skillsync remove foo --dry-run`, `skillsync init --yes`.

All commands honor `--dry-run`; prompts (including typed purge confirm) honor `--yes` for automation.

### Safety rules

- Never delete a real directory without backing it up first (`~/.local/share/skillsync/backups/<timestamp>/`).
- Only remove symlinks that point into the store or sources.
- `--purge` is the single exception (type `nuke` to confirm; `--yes` skips it).

## 5. Migration Path

- **From per-agent installs:** `skillsync init` — skills found in agent folders move to `sources/local/` (then listed in `skillsyncrc`), folders become views, originals backed up.
- **From the vercel `skills` CLI:** no conflict; it installs into agent dirs, which are views into the store after init. Skills added by either tool appear everywhere.
- **From manual copies:** put them in a folder, `skillsync add <folder>`.

## 6. Distribution

The reference CLI is installed out-of-band from skill data:

| Piece | Location |
|-------|----------|
| Application (Homebrew) | keg under the Homebrew prefix (`libexec` + `bin/skillsync` symlink) |
| Application (`go install`) | `$(go env GOPATH)/bin/skillsync` (or `$GOBIN`) |
| Application (checkout) | `./bin/skillsync` after `make` |
| Skill config / data | `${XDG_CONFIG_HOME:-~/.config}/skillsyncrc` and `${XDG_DATA_HOME:-~/.local/share}/skillsync/` (store, sources, backups) |

Install/update is Homebrew, `go install`, or building from a checkout. Homebrew is the formula in [formenosland/homebrew-tap](https://github.com/formenosland/homebrew-tap) (`brew install formenosland/tap/skillsync`); it uses GitHub’s tagged source archive, not a custom binary. The skillsync Release workflow hashes that archive and calls the tap’s reusable `bump.yml` so `url` / `sha256` stay in lockstep with the tag.

```sh
brew install formenosland/tap/skillsync
go install github.com/formenosland/skillsync/cmd/skillsync@latest
```

Removing the tool (`brew uninstall skillsync`, or deleting the `go install` binary) does not touch skill data. `skillsync uninstall [--purge]` manages views and skill data. An older curl installer used `${XDG_DATA_HOME}/skillsync/app/` and `~/.local/bin/skillsync`; that path is obsolete.

## 7. Pinning and Updates

- Git sources: `sync` fetches and updates clones (HTTPS via go-git). Pin by checking out a tag/commit in the clone; home `sync` does not rewrite HEADs. Project `apply` honors per-source `ref` in `skillsync.toml`.
- Local paths: always live.
- Registry: pinned upstream commit, explicit regeneration.
- Future: optional `sources.lock` with commit SHAs (out of scope for v0).

## 8. Agent Vendor Guidance

**Short term — nothing required.** Views make existing per-agent paths work transparently.

**Long term** — vendors should converge on *one shared global path* (the ecosystem is drifting toward `~/.agents/skills/`) and keep `.agents/skills/` for projects. Whatever the convergence point turns out to be, skillsync treats it as just another view, so users are covered before, during, and after the transition.

**Registry maintenance** — regenerating from vercel-labs/skills `agents.ts` is optional coverage sync. Users bridge gaps instantly via `[[agents]]` in `skillsyncrc`.

## 9. Out of Scope

| Topic | Notes |
|-------|-------|
| Marketplace / registry hosting | [skills.sh](https://skills.sh) and similar serve discovery |
| Skill format changes | Governed by agentskills.io / AAIF |
| Nested skill namespaces under a skills root | Clients typically scan one level (`<root>/<name>/SKILL.md`) |
| Windows directory links | Symlink when allowed; junction fallback; `--copy` for non-NTFS |
| Cloud-library vendors | No filesystem surface (e.g. Perplexity Computer) |

## 10. Open Questions

1. Should `sources.lock` be standardized in v1?
2. Should `remove` of a `sources/local/` skill offer to delete the files too (currently: never)?

## 11. References

- [Agent Skills Specification](https://agentskills.io)
- [vercel-labs/skills](https://github.com/vercel-labs/skills) — per-agent installer and discovery CLI
- [GNU Stow](https://www.gnu.org/software/stow/)
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)
