package cli

import "fmt"

func (a *App) cmdHelp() {
	fmt.Fprintf(a.Stdout, `%sskillsync%s %s — unified AI agent skill installation

%sUsage:%s
  skillsync [options] <command> [args]

%sCommands:%s
  init                        Create store, migrate per-agent skills, link views
                              (non-interactive: skillsync --yes init)
  add <url|path>              Register a skill source; pick names to install
                              (SKILL.md within 4 directory levels; --yes skips picker)
  apply                       Install/update repo skills from skillsync.toml
                              (non-interactive: skillsync --yes apply)
  apply --global              Also install those names into the home store
  apply --prune               Remove project links that left the manifest
  unapply [names...]          Remove skillsync-managed project links
  sync                        Pull all sources, refresh the store
                              (also apply if skillsync.toml is present)
  remove [names...] (rm)      Remove skills everywhere (picker when tty, no args)
  remove --all                Remove every installed skill
  remove --source <url|path>  Unregister a source and its skills
  list (ls)                   Skills grouped by source (tty); names when piped
                              --pretty always catalog; --names / -1 always names
  status                      Dashboard: paths, source health, views, excludes
  doctor                      Diagnose problems (exit 1 if issues)
  uninstall [--keep] [--purge] (nuke)
                              Reverse init; --keep converts views to real
                              copies; --purge deletes all skillsync data
  completion bash|zsh         Shell completion script
  help                        This help

%sOptions:%s  (global; before or after the command)
  --dry-run    Print actions without changing anything
  --yes, -y    Skip prompts; non-interactive init/add/remove/apply
  --copy       Copy instead of symlink (no-symlink filesystems)

%sEnvironment:%s
  SKILLSYNC_HOME       Force all state under one root (default: XDG dirs)
  SKILLSYNC_REGISTRY   Registry TSV (default: embedded agents.tsv)
  XDG_CONFIG_HOME      Config file: $XDG_CONFIG_HOME/skillsyncrc (~/.config/skillsyncrc)
                              Optional repo file: skillsync.toml (walk up from cwd)
  XDG_DATA_HOME        Data:   $XDG_DATA_HOME/skillsync (~/.local/share/skillsync)
  NO_COLOR             Disable colors and symbols
`, a.ui.bold, a.ui.reset, Version, a.ui.bold, a.ui.reset, a.ui.bold, a.ui.reset, a.ui.bold, a.ui.reset, a.ui.bold, a.ui.reset)
}

func (a *App) cmdCompletion(args []string) error {
	sh := ""
	if len(args) > 0 {
		sh = args[0]
	}
	switch sh {
	case "bash":
		fmt.Fprint(a.Stdout, bashCompletion)
	case "zsh":
		fmt.Fprint(a.Stdout, zshCompletion)
	default:
		return fmt.Errorf("usage: skillsync completion bash|zsh")
	}
	return nil
}

const bashCompletion = `# bash completion for skillsync. Enable with:
#   eval "$(skillsync completion bash)"
_skillsync() {
	local cur prev
	cur="${COMP_WORDS[COMP_CWORD]}"
	prev="${COMP_WORDS[COMP_CWORD - 1]}"
	local cmds="init add apply unapply sync remove list status doctor uninstall completion help"
	local flags="--dry-run --yes --copy --help"
	if [ "$COMP_CWORD" -eq 1 ]; then
		COMPREPLY=($(compgen -W "$cmds $flags" -- "$cur"))
		return
	fi
	case "${COMP_WORDS[1]}" in
		apply)
			COMPREPLY=($(compgen -W "--global --prune" -- "$cur"))
			;;
		remove | rm)
			COMPREPLY=($(compgen -W "$(skillsync list --names 2>/dev/null) --source --all" -- "$cur"))
			;;
		uninstall | nuke)
			COMPREPLY=($(compgen -W "--keep --purge" -- "$cur"))
			;;
		completion)
			COMPREPLY=($(compgen -W "bash zsh" -- "$cur"))
			;;
	esac
}
complete -F _skillsync skillsync
`

const zshCompletion = `# zsh completion for skillsync. Enable with:
#   eval "$(skillsync completion zsh)"
_skillsync() {
	local -a cmds
	cmds=(init add apply unapply sync remove list status doctor uninstall completion help)
	if (( CURRENT == 2 )); then
		_describe 'command' cmds
		return
	fi
	case $words[2] in
		apply) _arguments '--global' '--prune' ;;
		unapply)
			local -a skills
			skills=(${(f)"$(skillsync list --names 2>/dev/null)"})
			_describe 'skill' skills
			;;
		remove | rm)
			local -a skills
			skills=(${(f)"$(skillsync list --names 2>/dev/null)"})
			_describe 'skill' skills
			;;
		uninstall | nuke) _arguments '--keep' '--purge' ;;
		completion) _values 'shell' bash zsh ;;
	esac
}
compdef _skillsync skillsync
`
