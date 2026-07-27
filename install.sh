#!/bin/sh
# skillsync installer — pipe-safe (main at end).
#
# Install:  curl -fsSL https://get.formenos.land/skillsync/install.sh | sh
# Update:    re-run the same command
# Remove:    curl -fsSL ... | sh -s -- --uninstall
#
# From a checkout: ./install.sh
#
# Environment:
#   SKILLSYNC_INSTALL_REF     git branch/tag/commit (default: main)
#   SKILLSYNC_INSTALL_SOURCE  local directory to copy (skips git clone)
#   SKILLSYNC_INSTALL_REPO    override git URL
#
# shellcheck disable=SC1007,SC2088

set -eu

REPO_DEFAULT="https://github.com/formenosland/skillsync.git"
REF="${SKILLSYNC_INSTALL_REF:-main}"
REPO="${SKILLSYNC_INSTALL_REPO:-$REPO_DEFAULT}"

DATA_ROOT="${XDG_DATA_HOME:-$HOME/.local/share}/skillsync"
APP_DIR="$DATA_ROOT/app"
BIN_DIR="$HOME/.local/bin"
BIN_LINK="$BIN_DIR/skillsync"

YES=0
UNINSTALL=0

# --- ui (minimal; matches skillsync degradation) --------------------------------

if [ -t 2 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-}" != "dumb" ]; then
	C_BOLD=$(printf '\033[1m')
	C_DIM=$(printf '\033[2m')
	C_GREEN=$(printf '\033[32m')
	C_YELLOW=$(printf '\033[33m')
	C_CYAN=$(printf '\033[36m')
	C_RESET=$(printf '\033[0m')
	S_OK="✓"
else
	C_BOLD="" C_DIM="" C_GREEN="" C_YELLOW="" C_CYAN="" C_RESET=""
	S_OK="+"
fi

info() { printf '%s\n' "${C_CYAN}skillsync install:${C_RESET} $*" >&2; }
ok() { printf '%s\n' "${C_GREEN}${S_OK}${C_RESET} $*" >&2; }
warn() { printf '%s\n' "${C_YELLOW}!${C_RESET} $*" >&2; }
die() { warn "$*"; exit 1; }

confirm() {
	_msg=$1
	if [ "$YES" -eq 1 ]; then
		return 0
	fi
	if ! [ -t 0 ]; then
		return 1
	fi
	printf '%s [y/N] ' "$_msg" >&2
	read -r _ans || _ans=""
	case $_ans in y | Y | yes | YES) return 0 ;; esac
	return 1
}

# Resolve install.sh location (follow symlinks).
installer_dir() {
	_p=$1
	while [ -L "$_p" ]; do
		_t=$(readlink "$_p") || break
		case $_t in
			/*) _p=$_t ;;
			*) _p=$(CDPATH= cd -- "$(dirname "$_p")" && pwd)/$_t ;;
		esac
	done
	CDPATH= cd -- "$(dirname "$_p")" && pwd
}

local_source_dir() {
	if [ -n "${SKILLSYNC_INSTALL_SOURCE:-}" ]; then
		if [ -d "$SKILLSYNC_INSTALL_SOURCE" ] && [ -x "$SKILLSYNC_INSTALL_SOURCE/bin/skillsync" ]; then
			printf '%s' "$SKILLSYNC_INSTALL_SOURCE"
			return 0
		fi
		die "SKILLSYNC_INSTALL_SOURCE is not a skillsync tree (need bin/skillsync): $SKILLSYNC_INSTALL_SOURCE"
	fi
	_root=$(installer_dir "$0")
	if [ -x "$_root/bin/skillsync" ]; then
		printf '%s' "$_root"
		return 0
	fi
	return 1
}

copy_tree() {
	_src=$1
	_dst=$2
	mkdir -p "$_dst"
	# Prefer rsync for efficient updates; fall back to cp.
	if command -v rsync >/dev/null 2>&1; then
		rsync -a --delete \
			--exclude '.git' \
			--exclude '.DS_Store' \
			"$_src/" "$_dst/"
	else
		rm -rf "$_dst"
		mkdir -p "$_dst"
		cp -R "$_src/." "$_dst/"
		rm -rf "$_dst/.git" 2>/dev/null || true
	fi
}

install_from_local() {
	_src=$1
	info "installing from local tree: $_src"
	mkdir -p "$(dirname "$APP_DIR")"
	copy_tree "$_src" "$APP_DIR"
}

install_from_git() {
	mkdir -p "$(dirname "$APP_DIR")"
	if [ -d "$APP_DIR/.git" ]; then
		info "updating existing install in $APP_DIR"
		git -C "$APP_DIR" fetch --depth 1 origin "$REF" 2>/dev/null ||
			git -C "$APP_DIR" fetch origin "$REF"
		git -C "$APP_DIR" checkout "$REF" 2>/dev/null ||
			git -C "$APP_DIR" checkout -B "$REF" "origin/$REF" 2>/dev/null ||
			git -C "$APP_DIR" reset --hard "origin/$REF"
		git -C "$APP_DIR" pull --ff-only origin "$REF" 2>/dev/null || true
	else
		if [ -e "$APP_DIR" ]; then
			die "cannot clone: $APP_DIR exists and is not a git checkout (remove it or use --uninstall)"
		fi
		info "cloning $REPO (ref: $REF)"
		git clone --depth 1 --branch "$REF" "$REPO" "$APP_DIR" 2>/dev/null ||
			git clone --depth 1 "$REPO" "$APP_DIR"
		if [ "$REF" != "main" ] && [ "$REF" != "master" ]; then
			if git -C "$APP_DIR" fetch --depth 1 origin "$REF"; then
				git -C "$APP_DIR" checkout "$REF" 2>/dev/null || true
			fi
		fi
	fi
}

link_binary() {
	[ -x "$APP_DIR/bin/skillsync" ] || die "install incomplete: missing $APP_DIR/bin/skillsync"
	mkdir -p "$BIN_DIR"
	ln -sf "$APP_DIR/bin/skillsync" "$BIN_LINK"
	ok "linked $BIN_LINK -> $APP_DIR/bin/skillsync"
}

path_hint() {
	case ":$PATH:" in
		*":$BIN_DIR:"*) return 0 ;;
	esac
	warn "~/.local/bin is not on your PATH"
	case ${SHELL:-} in
		*/zsh)
			printf '  Add to ~/.zshrc:  %s\n' "${C_DIM}export PATH=\"\$HOME/.local/bin:\$PATH\"${C_RESET}" >&2
			;;
		*/bash)
			printf '  Add to ~/.bashrc:  %s\n' "${C_DIM}export PATH=\"\$HOME/.local/bin:\$PATH\"${C_RESET}" >&2
			;;
		*)
			printf '  Add to your shell rc:  %s\n' "${C_DIM}export PATH=\"\$HOME/.local/bin:\$PATH\"${C_RESET}" >&2
			;;
	esac
}

cmd_install() {
	_local=$(local_source_dir) || _local=""
	if [ -n "$_local" ]; then
		install_from_local "$_local"
	else
		command -v git >/dev/null 2>&1 || die "git is required (or run install.sh from a skillsync checkout)"
		install_from_git
	fi
	link_binary
	path_hint
	ok "skillsync installed — run: ${C_BOLD}skillsync init${C_RESET}"
}

cmd_uninstall() {
	if [ -e "$BIN_LINK" ] || [ -L "$BIN_LINK" ]; then
		rm -f "$BIN_LINK"
		ok "removed $BIN_LINK"
	else
		info "no binary at $BIN_LINK"
	fi
	if [ -d "$APP_DIR" ]; then
		if confirm "Remove application files at $APP_DIR?"; then
			rm -rf "$APP_DIR"
			ok "removed $APP_DIR"
		else
			info "kept $APP_DIR"
		fi
	else
		info "no app directory at $APP_DIR"
	fi
	warn "skill data (store, views) was not changed"
	info "to remove agent views and data, run: skillsync uninstall [--purge]"
}

usage() {
	cat <<EOF
skillsync installer

  curl -fsSL https://get.formenos.land/skillsync/install.sh | sh
  ./install.sh

Options:
  --uninstall   Remove ~/.local/bin/skillsync and the app copy (not skill data)
  --yes         Skip confirmation prompts
EOF
}

main() {
	while [ $# -gt 0 ]; do
		case $1 in
			--uninstall) UNINSTALL=1 ;;
			--yes | -y) YES=1 ;;
			-h | --help)
				usage
				return 0
				;;
			*) die "unknown option: $1" ;;
		esac
		shift
	done
	if [ "$UNINSTALL" -eq 1 ]; then
		cmd_uninstall
	else
		cmd_install
	fi
}

main "$@"
