#!/bin/sh
# generate.sh — regenerate agents.tsv from the maintained upstream registry.
#
# Source of truth: src/agents.ts in https://github.com/vercel-labs/skills
# (the machine-readable agent map used by the `skills` CLI, ~73 agents).
#
# Usage:
#   registry/generate.sh [commit-sha]
#
# With no argument, uses the pinned commit below. Pass a newer SHA to
# update the registry, review the diff, and commit the result.
#
# Requires: curl, awk, sort.
#
# shellcheck disable=SC1007

set -e

PINNED_SHA=${1:-9230fe8d4879e4d9ba04e7a31147477ffbcc51ca}
UPSTREAM_URL="https://raw.githubusercontent.com/vercel-labs/skills/$PINNED_SHA/src/agents.ts"
OUT_DIR=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
OUT_FILE="$OUT_DIR/agents.tsv"
TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT INT HUP

echo "fetching agents.ts @ $PINNED_SHA" >&2
curl -fsSL "$UPSTREAM_URL" -o "$TMP"

{
	cat <<EOF
# Agent skills path registry for skillsync — GENERATED FILE, do not edit by hand.
# Regenerate with: registry/generate.sh [commit-sha]
# Upstream: https://github.com/vercel-labs/skills src/agents.ts @ $PINNED_SHA
# Generated: $(date -u +%Y-%m-%d)
#
# Format: agent_id<TAB>display_name<TAB>global_path<TAB>project_path
#   - Paths may use ~ (home), \${VAR:-default} (env-overridable home dirs,
#     e.g. \${CODEX_HOME:-~/.codex}/skills), and | -separated alternates
#     (first alternate whose parent directory exists wins, else the first).
#   - global_path "-" means project-only agent: no global view is created.
#
# Local overrides: put rows in \$XDG_CONFIG_HOME/skillsync/agents.local.tsv
# (same format); rows there win over this file by agent_id.
#
# Known out of scope: Perplexity Computer (skills live in a cloud library
# uploaded via its UI — no local filesystem folder to link).
EOF
	printf 'agent_id\tdisplay_name\tglobal_path\tproject_path\n'
	awk '
	function flush() {
		if (id != "") {
			printf "%s\t%s\t%s\t%s\n", id, dn, gp, sd
		}
		id = ""; dn = ""; gp = "-"; sd = "-"
	}
	# Map a join(...) first token to a path prefix.
	function prefix(tok) {
		if (tok == "home") return "~"
		if (tok == "configHome") return "${XDG_CONFIG_HOME:-~/.config}"
		if (tok == "codexHome") return "${CODEX_HOME:-~/.codex}"
		if (tok == "claudeHome") return "${CLAUDE_CONFIG_DIR:-~/.claude}"
		if (tok == "vibeHome") return "${VIBE_HOME:-~/.vibe}"
		if (tok == "hermesHome") return "${HERMES_HOME:-~/.hermes}"
		if (tok == "autohandHome") return "${AUTOHAND_HOME:-~/.autohand}"
		if (tok == "grokHome") return "${GROK_HOME:-~/.grok}"
		return "UNKNOWN(" tok ")"
	}
	function parse_join(s,   inner, n, parts, i, tok, out) {
		sub(/^.*join\(/, "", s); sub(/\).*$/, "", s)
		n = split(s, parts, /,[ ]*/)
		out = ""
		for (i = 1; i <= n; i++) {
			tok = parts[i]
			gsub(/^['"'"'"]|['"'"'"]$/, "", tok)
			if (i == 1 && tok !~ /^['"'"'"]/ && tok ~ /^[A-Za-z]+$/ && tok !~ /^\./) {
				out = prefix(tok)
			} else {
				out = out "/" tok
			}
		}
		return out
	}
	/^[ ]+name: '"'"'/ {
		flush()
		id = $0
		sub(/^[ ]+name: '"'"'/, "", id); sub(/'"'"',.*$/, "", id)
		next
	}
	/^[ ]+displayName: '"'"'/ {
		dn = $0
		sub(/^[ ]+displayName: '"'"'/, "", dn); sub(/'"'"',.*$/, "", dn)
		next
	}
	/^[ ]+skillsDir: '"'"'/ {
		sd = $0
		sub(/^[ ]+skillsDir: '"'"'/, "", sd); sub(/'"'"',.*$/, "", sd)
		next
	}
	/^[ ]+globalSkillsDir: undefined/ { gp = "-"; next }
	/^[ ]+globalSkillsDir: getOpenClawGlobalSkillsDir\(\)/ {
		gp = "~/.openclaw/skills|~/.clawdbot/skills|~/.moltbot/skills"
		next
	}
	/^[ ]+globalSkillsDir: join\(/ { gp = parse_join($0); next }
	END { flush() }
	' "$TMP" | sort
} > "$OUT_FILE"

COUNT=$(grep -cv '^#\|^agent_id' "$OUT_FILE")
echo "wrote $OUT_FILE ($COUNT agents)" >&2

if grep -q 'UNKNOWN(' "$OUT_FILE"; then
	echo "error: unrecognized path token(s) — upstream format changed, update prefix() mapping:" >&2
	grep 'UNKNOWN(' "$OUT_FILE" >&2
	exit 1
fi
