package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Layout is config file + data directories.
type Layout struct {
	ConfigFile  string
	DataDir     string
	Store       string
	SourcesDir  string
	BackupsDir  string
	LocalSource string
}

func Resolve() Layout {
	if h := strings.TrimSpace(os.Getenv("SKILLSYNC_HOME")); h != "" {
		return forRoot(h)
	}
	home, _ := os.UserHomeDir()
	cfgHome := os.Getenv("XDG_CONFIG_HOME")
	dataHome := os.Getenv("XDG_DATA_HOME")
	if runtime.GOOS == "windows" {
		if cfgHome == "" {
			cfgHome = os.Getenv("APPDATA")
		}
		if dataHome == "" {
			dataHome = os.Getenv("LOCALAPPDATA")
		}
	}
	if cfgHome == "" {
		cfgHome = filepath.Join(home, ".config")
	}
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	l := Layout{
		ConfigFile: filepath.Join(cfgHome, "skillsyncrc"),
		DataDir:    filepath.Join(dataHome, "skillsync"),
	}
	l.fillData()
	return l
}

func forRoot(root string) Layout {
	l := Layout{
		ConfigFile: filepath.Join(root, "skillsyncrc"),
		DataDir:    root,
	}
	l.fillData()
	return l
}

func (l *Layout) fillData() {
	l.Store = filepath.Join(l.DataDir, "store")
	l.SourcesDir = filepath.Join(l.DataDir, "sources")
	l.BackupsDir = filepath.Join(l.DataDir, "backups")
	l.LocalSource = filepath.Join(l.SourcesDir, "local")
}

func ExpandOne(p string) string {
	p = expandVar(p)
	home, _ := os.UserHomeDir()
	switch {
	case p == "~":
		return home
	case strings.HasPrefix(p, "~/"):
		return filepath.Join(home, p[2:])
	default:
		return p
	}
}

func expandVar(p string) string {
	if !strings.HasPrefix(p, "${") {
		return p
	}
	inner := strings.TrimPrefix(p, "${")
	end := strings.IndexByte(inner, '}')
	if end < 0 {
		return p
	}
	varpart := inner[:end]
	suffix := inner[end+1:]
	name, def, ok := strings.Cut(varpart, ":-")
	if !ok {
		name = varpart
		def = ""
	}
	for _, c := range name {
		if (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '_' {
			return p
		}
	}
	if v, set := os.LookupEnv(name); set && v != "" {
		return v + suffix
	}
	return def + suffix
}

// ExpandAgent expands ${VAR:-default}, ~, and | alternates (first whose parent exists).
func ExpandAgent(raw string) string {
	alts := strings.Split(raw, "|")
	var first string
	for i, alt := range alts {
		p := ExpandOne(alt)
		if i == 0 {
			first = p
		}
		if parent := filepath.Dir(p); dirExists(parent) {
			return p
		}
	}
	return first
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}

// LooksLikeLocalPath reports filesystem paths, including POSIX-absolute
// forms that filepath.IsAbs does not treat as absolute on Windows.
func LooksLikeLocalPath(s string) bool {
	switch {
	case filepath.IsAbs(s), s == "~", strings.HasPrefix(s, "~/"):
		return true
	case strings.HasPrefix(s, "./"), strings.HasPrefix(s, "../"):
		return true
	case strings.HasPrefix(s, "/"), strings.HasPrefix(s, `\`):
		return true
	case len(s) >= 2 && s[1] == ':':
		return true
	default:
		return false
	}
}

func HasDotDotSegment(p string) bool {
	p = strings.ReplaceAll(p, `\`, "/")
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}
