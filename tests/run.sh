#!/bin/sh
# skillsync test suite — self-contained, sandboxed, no network.
#
# Runs every scenario in throwaway directories with fake HOME / registries.
# Never touches real agent folders or skill data.
#
# Usage: tests/run.sh
#
# shellcheck disable=SC2016
# (literal ${VAR:-default} strings are intentional registry syntax)

set -eu

ROOT=$(CDPATH='' cd -- "$(dirname "$0")/.." && pwd)
SKILLSYNC="$ROOT/bin/skillsync"
PASS=0

say() { printf '%s\n' "$*" >&2; }

fail() {
	say "FAIL: $*"
	exit 1
}

check() {
	_desc=$1
	shift
	if "$@"; then
		PASS=$((PASS + 1))
		say "  ok: $_desc"
	else
		fail "$_desc"
	fi
}

out_has() { printf '%s' "$OUT" | grep -q "$1"; }
out_lacks() { ! printf '%s' "$OUT" | grep -qE "$1"; }

# make_skill <dir> <name>
make_skill() {
	mkdir -p "$1"
	printf -- '---\nname: %s\ndescription: test skill %s\n---\nbody\n' "$2" "$2" >"$1/SKILL.md"
}

# make_sandbox: sets globals T, HOME_DIR, REG; exports env for skillsync.
make_sandbox() {
	T=$(mktemp -d)
	HOME_DIR="$T/home"
	REG="$T/reg.tsv"
	mkdir -p "$HOME_DIR/.claude" "$HOME_DIR/.codex"
	{
		printf 'agent_id\tdisplay_name\tglobal_path\tproject_path\n'
		printf 'fake-claude\tFake Claude\t%s/.claude/skills\t.claude/skills\n' "$HOME_DIR"
		printf 'fake-codex\tFake Codex\t${FAKE_CODEX_HOME:-%s/.codex}/skills\t.agents/skills\n' "$HOME_DIR"
		printf 'fake-absent\tNot Installed\t%s/.nope/skills\t.nope/skills\n' "$HOME_DIR"
	} >"$REG"
	export SKILLSYNC_HOME="$T/sync" SKILLSYNC_REGISTRY="$REG"
}

drop_sandbox() {
	unset SKILLSYNC_HOME SKILLSYNC_REGISTRY
	rm -rf "$T"
}

s() { "$SKILLSYNC" --yes "$@"; }

# --- 0. static checks ---------------------------------------------------------

say "== static checks"
check "bin/skillsync parses" sh -n "$SKILLSYNC"
check "install.sh parses" sh -n "$ROOT/install.sh"
check "generate.sh parses" sh -n "$ROOT/registry/generate.sh"
# Version pin matches Makefile / CI (SHELLCHECK_VERSION).
_sc_want="${SHELLCHECK_VERSION:-0.11.0}"
if ! command -v shellcheck >/dev/null 2>&1; then
	fail "shellcheck ${_sc_want} required (brew install shellcheck)"
fi
_sc_have=$(shellcheck --version | awk '/^version:/{print $2; exit}')
[ "$_sc_have" = "$_sc_want" ] || fail "shellcheck ${_sc_have}, want ${_sc_want}"
check "shellcheck clean" shellcheck "$SKILLSYNC" "$ROOT/install.sh" "$ROOT/registry/generate.sh" "$0"
check "registry has data rows" sh -c "grep -cv '^#\\|^agent_id' '$ROOT/registry/agents.tsv' | grep -q '[0-9]'"
_ver=$(sed -n 's/^VERSION="\([^"]*\)"$/\1/p' "$SKILLSYNC" | head -n 1)
[ -n "$_ver" ] || fail "VERSION= not found in bin/skillsync"
_got=$("$SKILLSYNC" --version)
check "skillsync --version matches VERSION" test "$_got" = "$_ver"
_gotv=$("$SKILLSYNC" -V)
check "skillsync -V matches VERSION" test "$_gotv" = "$_ver"

# --- 1. init: migration, views, install detection ------------------------------

say "== init"
make_sandbox
make_skill "$HOME_DIR/.claude/skills/oldskill" oldskill

s init >/dev/null 2>&1
check "claude view linked" test -L "$HOME_DIR/.claude/skills"
check "codex view linked" test -L "$HOME_DIR/.codex/skills"
check "uninstalled agent untouched" test ! -e "$HOME_DIR/.nope"
check "migrated skill in sources/local" test -f "$T/sync/sources/local/oldskill/SKILL.md"
check "migrated skill linked in store" test -L "$T/sync/store/oldskill"
check "skill visible through view" test -f "$HOME_DIR/.claude/skills/oldskill/SKILL.md"

OUT=$(s init 2>&1)
check "second init reports already linked" out_has 'already linked'

# --- 2. add, layers, precedence, idempotent sync -------------------------------

say "== add / layers"
make_skill "$T/src-org/skills/alpha" alpha
make_skill "$T/src-org/skills/beta" beta
make_skill "$T/src-user/beta" beta

s add "$T/src-org" --layer org >/dev/null 2>&1
s add "$T/src-user" >/dev/null 2>&1
check "org skill linked" test -L "$T/sync/store/alpha"
OUT=$(readlink "$T/sync/store/beta")
check "user beta wins over org" out_has src-user

OUT=$(s sync 2>&1)
check "sync is idempotent (no relinks)" out_lacks 'linking|updating'

OUT=$(s list)
check "list shows alpha" out_has alpha
check "list shows beta" out_has beta
check "list shows oldskill" out_has oldskill

# --- 3. remove: exclusion, re-add, dry-run --------------------------------------

say "== remove"
s remove beta >/dev/null 2>&1
check "removed from store" test ! -e "$T/sync/store/beta"
check "removed from views" test ! -e "$HOME_DIR/.claude/skills/beta"
check "recorded in exclude.conf" grep -qx beta "$T/sync/exclude.conf"

s sync >/dev/null 2>&1
check "sync does not resurrect removed skill" test ! -e "$T/sync/store/beta"

s add "$T/src-user" >/dev/null 2>&1
check "re-add restores skill" test -L "$T/sync/store/beta"

s --dry-run remove alpha >/dev/null 2>&1 || true
check "dry-run remove changes nothing" test -L "$T/sync/store/alpha"

# --- 4. remove --source ----------------------------------------------------------

say "== remove --source"
s remove --source "$T/src-org" >/dev/null 2>&1
check "source skills gone from store" test ! -e "$T/sync/store/alpha"
check "local source folder kept" test -d "$T/src-org"
check "manifest entry dropped" sh -c "! grep -q src-org '$T/sync/sources.conf'"

# --- 5. ownership invariant: unmanaged entries ------------------------------------

say "== unmanaged entries"
mkdir -p "$T/sync/store/rogue"
printf 'not a symlink\n' >"$T/sync/store/rogue/file"

OUT=$(s sync 2>&1) || true
check "sync leaves unmanaged dir intact" test -f "$T/sync/store/rogue/file"

OUT=$(s remove rogue 2>&1) || true
check "remove refuses unmanaged dir" test -d "$T/sync/store/rogue"
rm -rf "$T/sync/store/rogue"

# --- 6. doctor -------------------------------------------------------------------

say "== doctor"
if s doctor >/dev/null 2>&1; then
	check "doctor clean on healthy setup" true
else
	fail "doctor clean on healthy setup"
fi

ln -s /nonexistent-target "$T/sync/store/deadlink"
if s doctor >/dev/null 2>&1; then
	fail "doctor detects broken link (should exit 1)"
else
	check "doctor detects broken link" true
fi
rm -f "$T/sync/store/deadlink"

rm -f "$HOME_DIR/.claude/skills"
mkdir -p "$HOME_DIR/.claude/skills"
if s doctor >/dev/null 2>&1; then
	fail "doctor detects drifted view (should exit 1)"
else
	check "doctor detects drifted view" true
fi
s init >/dev/null 2>&1
check "init heals drifted view" test -L "$HOME_DIR/.claude/skills"

# --- 7. env-overridable registry paths ----------------------------------------------

say "== registry env override"
mkdir -p "$T/custom-codex"
OUT=$(FAKE_CODEX_HOME="$T/custom-codex" s status)
check "env var overrides agent home" out_has custom-codex

# --- 8. agents.local.tsv override ----------------------------------------------------

say "== agents.local.tsv"
mkdir -p "$HOME_DIR/.custom"
printf 'my-agent\tMy Agent\t%s/.custom/skills\t.custom/skills\n' "$HOME_DIR" >"$T/sync/agents.local.tsv"
s init >/dev/null 2>&1
check "locally-declared agent linked" test -L "$HOME_DIR/.custom/skills"

# --- 9. uninstall ---------------------------------------------------------------------

say "== uninstall"
s uninstall --keep >/dev/null 2>&1
check "keep: view is a real dir" sh -c "test -d '$HOME_DIR/.claude/skills' && test ! -L '$HOME_DIR/.claude/skills'"
check "keep: copies present" test -f "$HOME_DIR/.claude/skills/beta/SKILL.md"

s init >/dev/null 2>&1
s uninstall >/dev/null 2>&1
check "default: views removed" test ! -e "$HOME_DIR/.claude/skills"
check "default: store kept" test -d "$T/sync/store"

s uninstall --purge >/dev/null 2>&1
check "purge: all data gone" test ! -d "$T/sync"
check "purge: outside sources kept" test -d "$T/src-user"

drop_sandbox

# --- 10. security / ux ---------------------------------------------------------------

say "== security / ux"
make_sandbox
mkdir -p "$T/src-evil/skills/safebasename"
printf -- '---\nname: ../../../evil\ndescription: escape attempt\n---\nbody\n' \
	>"$T/src-evil/skills/safebasename/SKILL.md"

s add "$T/src-evil" >/dev/null 2>&1
s sync >/dev/null 2>&1
check "unsafe frontmatter uses safe basename in store" test -L "$T/sync/store/safebasename"
check "unsafe frontmatter does not create evil store entry" test ! -e "$T/sync/store/evil"
check "store has no dot-dot skill entries" sh -c '! ls -1 "$T/sync/store" 2>/dev/null | grep -q "\\.\\."'
_troot=$(CDPATH='' cd -- "$T" && pwd -P)
_outside=0
for _lnk in "$T/sync/store"/*; do
	[ -L "$_lnk" ] || continue
	_rt=$(CDPATH='' cd -- "$(readlink "$_lnk")" && pwd -P) || _outside=1
	case $_rt in
		"$_troot"/*) ;;
		*) _outside=1 ;;
	esac
done
check "store symlinks stay under sandbox" test "$_outside" -eq 0

mkdir -p "$T/outside-decoy"
touch "$T/outside-decoy/marker"
OUT=$("$SKILLSYNC" remove '../../../evil' 2>&1) || true
check "remove refuses unsafe skill name" out_has 'unsafe skill name'
check "remove unsafe name leaves outside decoy" test -f "$T/outside-decoy/marker"
check "remove unsafe name keeps store skill" test -L "$T/sync/store/safebasename"

# Dotted / underscored / uppercase names are rejected (Agent Skills kebab-case).
mkdir -p "$T/src-dot/skills/with.dot"
printf -- '---\nname: with.dot\ndescription: dotted\n---\nbody\n' \
	>"$T/src-dot/skills/with.dot/SKILL.md"
s add "$T/src-dot" >/dev/null 2>&1
check "dotted skill name not materialized" test ! -e "$T/sync/store/with.dot"

mkdir -p "$T/src-under/skills/with_under"
printf -- '---\nname: with_under\ndescription: underscore\n---\nbody\n' \
	>"$T/src-under/skills/with_under/SKILL.md"
s add "$T/src-under" >/dev/null 2>&1
check "underscore skill name not materialized" test ! -e "$T/sync/store/with_under"

mkdir -p "$T/src-upper/skills/CodeReview"
printf -- '---\nname: CodeReview\ndescription: upper\n---\nbody\n' \
	>"$T/src-upper/skills/CodeReview/SKILL.md"
s add "$T/src-upper" >/dev/null 2>&1
# Exact basename match (avoid case-insensitive -e false positives on macOS).
check "uppercase skill name not materialized" sh -c '
	for f in "$1"/*; do
		{ [ -e "$f" ] || [ -L "$f" ]; } || continue
		[ "$(basename "$f")" = CodeReview ] && exit 1
	done
	exit 0
' sh "$T/sync/store"

mkdir -p "$T/src-kebab/skills/code-review"
make_skill "$T/src-kebab/skills/code-review" code-review
s add "$T/src-kebab" >/dev/null 2>&1
check "valid kebab-case skill is materialized" test -L "$T/sync/store/code-review"

OUT=$("$SKILLSYNC" remove </dev/null 2>&1) && _rc=0 || _rc=$?
check "bare remove without tty exits nonzero" test "$_rc" -ne 0
check "bare remove without tty prompts for --all" out_has 'specify skill names or use --all'

s remove --all >/dev/null 2>&1
check "remove --all clears store skill" test ! -e "$T/sync/store/safebasename"
check "remove --all records exclude" grep -qx safebasename "$T/sync/exclude.conf"

# Global flags after the subcommand
make_skill "$T/src-user/postflag" postflag
s add "$T/src-user" >/dev/null 2>&1
"$SKILLSYNC" remove postflag --dry-run >/dev/null 2>&1 || true
check "global --dry-run after command is honored" test -L "$T/sync/store/postflag"
"$SKILLSYNC" remove postflag --yes >/dev/null 2>&1
check "global --yes after command is honored" test ! -e "$T/sync/store/postflag"

OUT=$("$SKILLSYNC" add 'https://github.com/foo/../../../tmp-evil' 2>&1) && _rc=0 || _rc=$?
check "unsafe source URL path exits nonzero" test "$_rc" -ne 0
check "unsafe source URL path error" out_has 'unsafe path'

drop_sandbox

make_sandbox
make_skill "$HOME_DIR/.claude/skills/linkme" linkme
OUT=$(</dev/null "$SKILLSYNC" init 2>&1) && _rc=0 || _rc=$?
check "non-interactive init without --yes exits nonzero" test "$_rc" -ne 0
check "non-interactive init requires --yes message" out_has 'non-interactive init requires --yes'
OUT=$(s init 2>&1)
check "init with --yes still completes" out_has 'init done'
check "init with --yes links agent view" test -L "$HOME_DIR/.claude/skills"

drop_sandbox

# --- 11. installer ---------------------------------------------------------------------

say "== installer"
_got=$(printf '%s' '{"url":"x","tag_name":"v3.1.4","name":"n"}' | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)
check "release json tag_name parse" test "$_got" = "v3.1.4"
T=$(mktemp -d)
export HOME="$T/home" XDG_DATA_HOME="$T/data" XDG_CONFIG_HOME="$T/config"
mkdir -p "$HOME"

"$ROOT/install.sh" --yes >/dev/null 2>&1
check "app installed" test -x "$XDG_DATA_HOME/skillsync/app/bin/skillsync"
check "bin symlink created" test -L "$HOME/.local/bin/skillsync"

OUT=$("$HOME/.local/bin/skillsync" status)
check "runs through bin symlink (registry found)" out_has 'skillsync 0'

"$ROOT/install.sh" --yes >/dev/null 2>&1
check "re-install is idempotent" test -x "$XDG_DATA_HOME/skillsync/app/bin/skillsync"

mkdir -p "$XDG_DATA_HOME/skillsync/store"
touch "$XDG_DATA_HOME/skillsync/store/.marker"
"$ROOT/install.sh" --uninstall --yes >/dev/null 2>&1
check "tool removed" test ! -e "$HOME/.local/bin/skillsync"
check "app removed" test ! -d "$XDG_DATA_HOME/skillsync/app"
check "skill data untouched" test -f "$XDG_DATA_HOME/skillsync/store/.marker"

unset XDG_DATA_HOME XDG_CONFIG_HOME
rm -rf "$T"

# --- 12. output hygiene -----------------------------------------------------------------

say "== output hygiene"
T=$(mktemp -d)
export SKILLSYNC_HOME="$T/sync"
printf 'agent_id\tdisplay_name\tglobal_path\tproject_path\n' >"$T/reg.tsv"
export SKILLSYNC_REGISTRY="$T/reg.tsv"
OUT=$("$SKILLSYNC" --yes init 2>&1)
ESC=$(printf '\033')
case $OUT in
	*"$ESC"*) fail "non-tty output contains ANSI escapes" ;;
	*) check "non-tty output is escape-free" true ;;
esac
"$SKILLSYNC" completion bash | bash -n
check "bash completion is valid bash" true
unset SKILLSYNC_HOME SKILLSYNC_REGISTRY
rm -rf "$T"

say ""
say "ALL PASS ($PASS checks)"
