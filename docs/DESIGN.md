# skillsync — Design & Specification

**Status:** Stable (v0.2 series) · **Maintainer:** formenos.land

skillsync defines a vendor-neutral convention for installing and managing AI agent skills across tools, scopes, and organizational boundaries: a canonical skill store with per-agent directory views and layered source manifests. This document is the reference for the design; the [README](../README.md) covers day-to-day usage.

## 1. Problem

### 1.1 The N × M mess

AI agent skills are an open standard ([agentskills.io](https://agentskills.io), Linux Foundation AAIF). A skill is a directory containing `SKILL.md` with YAML frontmatter (`name`, `description`, …) and markdown instructions.

Adoption is broad (70+ tools), but installation is fragmented:

| Dimension | Fragmentation |
|-----------|---------------|
| **Scope** | Global (user home) vs. project (repo) vs. org (company) |
| **Agent** | Each tool expects skills in its own folder |
| **Tooling** | Per-agent CLIs, manual copying, ad-hoc scripts |

A developer using Claude Code, Cursor, Codex, and Copilot may maintain four copies of the same skill in four global directories, plus project copies. Company-wide skills require yet another distribution mechanism.

### 1.2 What already works

- **Project scope** is converging on `.agents/skills/` (Codex, Cursor, Gemini CLI, Copilot, Zed, Cline, Warp, Amp, …).
- **The SKILL.md format** is standardized and interoperable.
- **The vercel-labs `skills` CLI** (`npx skills add …`) installs into agent-specific paths — useful, but multiplies installs per agent.

What's missing is a **unified global layer** with org/user precedence and a single write path.

## 2. Prior Art

| System | Analogy |
|--------|---------|
| [agentskills.io](https://agentskills.io) | Skill *format* spec — what a skill is |
| [vercel-labs/skills](https://github.com/vercel-labs/skills) | Per-agent installer CLI; also the maintained agent-path map this design builds on |
| **GNU Stow** | Symlink farm — one source tree, many "views" |
| **XDG Base Directory** | Predictable config/data locations without home-dir clutter |
| **npm / cargo / go modules** | Manifest + lock + vendor dir (heavier than needed here) |

skillsync applies the Stow pattern: one canonical store, many symlinks.

## 3. Design

### 3.1 Core model

```
   sources (humans own these)              store (skillsync owns this)         views (vendor folders)
┌────────────────────────────┐      ┌───────────────────────────────┐    ┌──────────────────────────┐
│ git: acme-corp/skills (org)│      │ ~/.local/share/skillsync/store│    │ ~/.agents/skills      ───┼──┐
│ git: alice/skills   (user) │ ───> │   skill-a -> sources/…        │<───┼ ~/.claude/skills         │  │
│ dir: ~/dev/my-skills (user)│      │   skill-b -> sources/…        │    │ ~/.codex/skills          │  │ every view is a
│ sources/local  (migrated)  │      │   skill-c -> ~/dev/my-skills/…│    │ ~/.gemini/skills         │  │ symlink to the
└────────────────────────────┘      └───────────────────────────────┘    │ … (~75 agents)        ───┼──┘ store
                                                                         └──────────────────────────┘
```

**Store** — a single canonical directory of per-skill symlinks. Not a vendor path: it lives in skillsync's own XDG data directory, so no vendor semantics can ever collide with it.

**Views** — every agent's global skills directory (including `~/.agents/skills`, which some vendors read natively) is a whole-directory symlink to the store. Adding a skill once makes it visible to every agent. No per-agent bookkeeping.

**Ownership invariant** — *skillsync owns the store; humans own sources.* The store contains only symlinks created by skillsync. A real directory found in the store is unmanaged: `sync` skips it, `remove` refuses it, `doctor` flags it with advice to move it into a source. skillsync never deletes skill files — only links.

**Layers** — sources merge with precedence:

```
org  <  user  <  local (sources/local/, migrated or hand-placed)
```

Highest layer wins on name collision. Project skills (`.agents/skills/` in a repo) are outside home-scope sync; agents resolve them nearest-first themselves.

### 3.2 Locations (XDG)

| What | Path |
|------|------|
| Manifest, excludes, registry override | `${XDG_CONFIG_HOME:-~/.config}/skillsync/` |
| Store, cloned sources, backups | `${XDG_DATA_HOME:-~/.local/share}/skillsync/` |
| Single-root override | `SKILLSYNC_HOME` env var |

No new dotfolder in `~`. This matches the practice of OpenCode, Amp, Goose, and the vercel CLI on both Linux and macOS.

### 3.3 Sources manifest

File: `~/.config/skillsync/sources.conf` — POSIX-sh-parseable, one source per line:

```
# layer  url-or-path
org      https://github.com/acme-corp/skills
user     https://github.com/alice/personal-skills
user     /home/alice/dev/experimental-skills
```

- `#` comments; first token is the layer (`org` or `user`), remainder is the URL or absolute path.
- Git sources are cloned to `~/.local/share/skillsync/sources/<owner>/<repo>/` (paths derived from the URL must stay under `sources/`; `..` segments are rejected). Local paths are referenced in place.
- `sources/local/` is an implicit final source (highest precedence): `init` migrates skills found in existing agent folders here, and users may place authored skills here directly.

### 3.4 Skill discovery

Within each source, skill directories (containing `SKILL.md`) are found at depth 1, depth 2, and under a `skills/` container. The store name comes from the `name:` frontmatter field, falling back to the directory basename. Names must match the [Agent Skills](https://agentskills.io/specification) `name` rules: 1–64 characters, `/^[a-z0-9]+(-[a-z0-9]+)*$/` (lowercase, digits, single hyphens; no leading/trailing/consecutive hyphens). Unsafe names are skipped during materialize/migration and refused by `remove`.

### 3.5 Agent registry

`registry/agents.tsv` maps agent ids to global and project paths. Requirements:

- **Generated, not hand-written.** The registry derives from the maintained agent map in vercel-labs/skills (`src/agents.ts`), pinned to a commit recorded in the file header. `registry/generate.sh` regenerates it; the output is committed so the tool needs no network at runtime.
- **Expressive paths.** Rows may use `~`, `${VAR:-default}` (env-overridable homes such as `CODEX_HOME`, `CLAUDE_CONFIG_DIR`, `HERMES_HOME`), and `|`-separated alternates (first whose parent directory exists wins — e.g. OpenClaw's `~/.openclaw` → `~/.clawdbot` → `~/.moltbot`).
- **Project-only agents** carry `-` as global path and get no view.
- **Local override:** `~/.config/skillsync/agents.local.tsv` (same format) merges over the shipped registry, winning by `agent_id`. Users can add unlisted agents or correct paths without touching the installation.
- **Install detection:** a view is only created when the agent's parent folder exists, so uninstalled agents never cause folder litter.

Non-filesystem vendors (e.g. Perplexity Computer, whose skills live in a cloud library) are documented as out of scope.

### 3.6 Collision rules

| Situation | Resolution |
|-----------|------------|
| Same name, org vs user layer | User wins |
| Same name, any source vs `sources/local/` | Local wins |
| Same name from two same-layer sources | Later manifest entry wins |
| Unmanaged real directory in store | Never touched; flagged by doctor |
| Project skill vs global skill | Agent resolves nearest scope itself |

## 4. Operations

| Command | Semantics |
|---------|-----------|
| `init` | Create store; for each installed agent: migrate real skills to `sources/local/`, back up the folder, replace it with a view symlink. Idempotent. Interactive agent selection on a tty; non-interactive init requires global `--yes` (`skillsync --yes init`, which selects all link candidates). |
| `add` | Register a source (git URL, `owner/repo`, or path), fetch it, materialize. Re-adding un-excludes its skills. |
| `sync` | Pull all git sources, re-materialize with layer precedence, prune broken links. |
| `remove` | Delete the skill's store symlink (visible everywhere instantly) and record the name in `exclude.conf` so sync won't restore it. No backups — source files are never touched, so nothing is lost. Refuses unmanaged entries. Bare `remove` on a tty opens a picker; without names, non-interactive use requires skill arguments or `--all` (`skillsync --yes remove --all` removes everything). |
| `remove --all` | Remove every skill currently in the store (same per-skill semantics as `remove <name>`). |
| `remove --source` | Drop the manifest entry, delete the clone (managed clones only — local folders are kept), remove its store links, re-materialize. |
| `list` | Plain skill names for scripts and shell completion. |
| `status` | Skills with origins, agent view states, sources, excludes. |
| `doctor` | Broken links, drifted views (agent recreated a real folder), wrong links, missing sources, unlinked installed agents, cross-layer collisions (informational). Exit 1 on actionable findings. |
| `uninstall` | Reverse of init: remove all view symlinks (default), or convert views to real copies (`--keep`). `--purge` deletes all skillsync data after typing the confirmation word `nuke`; global `--yes` skips that prompt. |

**Global options** (before or after the subcommand): `--dry-run`, `--yes` / `-y`, `--copy`. Examples: `skillsync --dry-run remove foo`, `skillsync remove foo --dry-run`, `skillsync init --yes`.

All commands honor `--dry-run`; prompts (including typed purge confirm) honor `--yes` for automation.

### Safety rules

- Never delete a real directory without backing it up first (`~/.local/share/skillsync/backups/<timestamp>/`).
- Only remove symlinks that point into the store or sources.
- `--purge` is the single exception (type `nuke` to confirm; `--yes` skips it).

## 5. Migration Path

- **From per-agent installs:** `skillsync init` — skills found in agent folders move to `sources/local/`, folders become views, originals backed up.
- **From the vercel `skills` CLI:** no conflict; it installs into agent dirs, which are views into the store after init. Skills added by either tool appear everywhere.
- **From manual copies:** put them in a folder, `skillsync add <folder>`.

## 6. Distribution

The reference CLI is installed out-of-band from skill data:

| Piece | Location |
|-------|----------|
| Application (script, registry, docs) | `${XDG_DATA_HOME:-~/.local/share}/skillsync/app/` |
| User command | `~/.local/bin/skillsync` → `app/bin/skillsync` |
| Skill config / data | `${XDG_CONFIG_HOME:-~/.config}/skillsync/` and `${XDG_DATA_HOME:-~/.local/share}/skillsync/` (store, sources, backups) |

Install/update is a pipe-safe `install.sh`. Today:

```sh
curl -fsSL https://raw.githubusercontent.com/formenosland/skillsync/main/install.sh | sh
```

Default git install is the latest GitHub Release tag. `SKILLSYNC_INSTALL_REF` overrides (tag, branch, or commit). A checkout of this repo copies the local tree and does not hit the network.

For formenos.land tools the planned stable URL is `https://get.formenos.land/<tool>/install.sh` (e.g. `/skillsync/install.sh`), backed by a small GitHub Pages repo with one static script per tool, synced from each tool repo on release (a Cloudflare redirect to the repository raw URL is an acceptable alternative). Re-running the installer updates the app copy; it does not run `skillsync init` or touch views. Removing the tool (`install.sh --uninstall`) deletes only the app and bin symlink; `skillsync uninstall [--purge]` manages views and skill data.

## 7. Pinning and Updates

- Git sources: `sync` runs `git pull --ff-only`. Pin by checking out a tag/commit in the clone; skillsync does not rewrite HEADs.
- Local paths: always live.
- Registry: pinned upstream commit, explicit regeneration.
- Future: optional `sources.lock` with commit SHAs (out of scope for v0).

## 8. Agent Vendor Guidance

**Short term — nothing required.** Views make existing per-agent paths work transparently.

**Long term** — vendors should converge on *one shared global path* (the ecosystem is drifting toward `~/.agents/skills/`) and keep `.agents/skills/` for projects. Whatever the convergence point turns out to be, skillsync treats it as just another view, so users are covered before, during, and after the transition.

**Registry maintenance** — upstream additions land in vercel-labs/skills; regenerating the registry picks them up. Users bridge gaps instantly via `agents.local.tsv`.

## 9. Out of Scope

| Topic | Notes |
|-------|-------|
| Marketplace / registry hosting | [skills.sh](https://skills.sh) and similar serve discovery |
| Skill format changes | Governed by agentskills.io / AAIF |
| Project-scope sync | Managed by the repo (git, submodules, copies) |
| Windows symlink semantics | Future work; `--copy` fallback available |
| Cloud-library vendors | No filesystem surface (e.g. Perplexity Computer) |

## 10. Open Questions

1. Should `sources.lock` be standardized in v1?
2. Should project-scope `.agents/sources.conf` be supported for team repos?
3. Should `remove` of a `sources/local/` skill offer to delete the files too (currently: never)?

## 11. References

- [Agent Skills Specification](https://agentskills.io)
- [vercel-labs/skills](https://github.com/vercel-labs/skills) — CLI and maintained agent path map
- [GNU Stow](https://www.gnu.org/software/stow/)
- [XDG Base Directory Specification](https://specifications.freedesktop.org/basedir-spec/basedir-spec-latest.html)
