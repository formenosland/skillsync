package cli

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/formenosland/skillsync/internal/agentregistry"
	"github.com/formenosland/skillsync/internal/config"
	"github.com/formenosland/skillsync/internal/fsops"
	"github.com/formenosland/skillsync/internal/gitx"
	"github.com/formenosland/skillsync/internal/paths"
	"github.com/formenosland/skillsync/internal/skill"
	"golang.org/x/term"
)

var errHelpOrVersion = errors.New("handled")

// Version is set by the maintainer (not by this rewrite).
const Version = "1.4.1"

type App struct {
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer

	DryRun bool
	Yes    bool
	Copy   bool

	layout paths.Layout
	cfg    config.File
	ui     style
	nOK    int
	nWarn  int
}

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	a := &App{Args: args, Stdin: stdin, Stdout: stdout, Stderr: stderr}
	if err := a.run(); err != nil {
		if errors.Is(err, errQuiet) {
			return 1
		}
		a.err(err.Error())
		return 1
	}
	return 0
}

var errQuiet = errors.New("exit 1")

func (a *App) run() error {
	a.layout = paths.Resolve()
	a.ui = newStyle(a.stderrTTY())
	rest, err := a.parseGlobals()
	if errors.Is(err, errHelpOrVersion) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		a.cmdHelp()
		return nil
	}
	cmd := rest[0]
	if strings.HasPrefix(cmd, "-") {
		return fmt.Errorf("unknown option: %s (try --help)", cmd)
	}
	cfg, err := config.Load(a.layout.ConfigFile)
	if err != nil {
		return err
	}
	a.cfg = cfg
	args := rest[1:]
	switch cmd {
	case "init":
		return a.cmdInit(args)
	case "add":
		return a.cmdAdd(args)
	case "sync":
		return a.cmdSync(args)
	case "apply":
		return a.cmdApply(args)
	case "unapply":
		return a.cmdUnapply(args)
	case "remove", "rm":
		return a.cmdRemove(args)
	case "list", "ls":
		return a.cmdList(args)
	case "status":
		return a.cmdStatus(args)
	case "doctor":
		return a.cmdDoctor(args)
	case "uninstall", "nuke":
		return a.cmdUninstall(args)
	case "completion":
		return a.cmdCompletion(args)
	case "help":
		a.cmdHelp()
		return nil
	default:
		return fmt.Errorf("unknown command: %s (try help)", cmd)
	}
}

func (a *App) parseGlobals() ([]string, error) {
	var rest []string
	args := a.Args
	if len(args) > 0 {
		args = args[1:]
	}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--dry-run":
			a.DryRun = true
		case "--yes", "-y":
			a.Yes = true
		case "--copy":
			a.Copy = true
		case "-h", "--help":
			a.cmdHelp()
			return nil, errHelpOrVersion
		case "-V", "--version":
			fmt.Fprintln(a.Stdout, Version)
			return nil, errHelpOrVersion
		default:
			rest = append(rest, args[i])
		}
	}
	return rest, nil
}

func (a *App) save() error {
	if a.DryRun {
		a.info("[dry-run] write " + a.layout.ConfigFile)
		return nil
	}
	return config.Save(a.layout.ConfigFile, a.cfg)
}

func (a *App) do(label string, fn func() error) error {
	if a.DryRun {
		a.info("[dry-run] " + label)
		return nil
	}
	return fn()
}

func (a *App) sourceRoot(entry string) (string, error) {
	if gitx.IsGitURL(entry) {
		return gitx.CloneDir(a.layout.SourcesDir, entry)
	}
	return paths.ExpandOne(entry), nil
}

func (a *App) normalizeSource(raw string) (string, error) {
	if gitx.IsGitURL(raw) {
		return gitx.NormalizeGitURL(raw), nil
	}
	p := paths.ExpandOne(raw)
	st, err := os.Stat(p)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("local source not found: %s", p)
	}
	return fsops.ResolveDir(p)
}

func (a *App) fetchSource(raw string) error {
	if gitx.IsGitURL(raw) {
		if a.DryRun {
			a.info("[dry-run] clone/pull " + gitx.NormalizeGitURL(raw))
			return nil
		}
		return gitx.CloneOrPull(a.layout.SourcesDir, raw)
	}
	p := paths.ExpandOne(raw)
	st, err := os.Stat(p)
	if err != nil || !st.IsDir() {
		return fmt.Errorf("local source not found: %s", p)
	}
	return nil
}

func (a *App) storeOccupied(name string) bool {
	dest := filepath.Join(a.layout.Store, name)
	return fsops.IsSymlink(dest) && destExists(dest)
}

func destExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (a *App) linkSkill(name, src string, force bool) error {
	if !skill.IsSafeName(name) {
		a.warn(fmt.Sprintf("'%s': unsafe skill name, skipped", name))
		return nil
	}
	if a.cfg.IsExcluded(name) {
		return nil
	}
	if err := a.do("mkdir store", func() error { return os.MkdirAll(a.layout.Store, 0o755) }); err != nil {
		return err
	}
	dest := filepath.Join(a.layout.Store, name)
	if fsops.IsRealDir(dest) {
		a.warn(fmt.Sprintf("'%s': unmanaged directory in store, skipped (move it into a source and 'add' it)", name))
		return nil
	}
	if fsops.IsSymlink(dest) {
		if fsops.PathsEqual(dest, src) {
			return nil
		}
		if !force {
			a.warn(fmt.Sprintf("'%s': already installed, skipped", name))
			return nil
		}
		if err := a.do("rm "+dest, func() error { return os.Remove(dest) }); err != nil {
			return err
		}
	} else if fsops.Exists(dest) {
		a.warn(fmt.Sprintf("'%s': unexpected file in store, skipped", name))
		return nil
	}
	if a.Copy {
		if err := a.do("copy "+name, func() error { return fsops.LinkDir(src, dest, true) }); err != nil {
			return err
		}
		a.ok(name + a.ui.dim + " (copy)" + a.ui.reset)
		return nil
	}
	if err := a.do("link "+name, func() error { return fsops.LinkDir(src, dest, false) }); err != nil {
		return err
	}
	a.ok(name)
	return nil
}

func (a *App) materialize() error {
	if err := a.do("mkdir store", func() error { return os.MkdirAll(a.layout.Store, 0o755) }); err != nil {
		return err
	}
	type prov struct{ name, dir, src string }
	var all []prov
	for _, src := range a.cfg.Sources {
		root, err := a.sourceRoot(src)
		if err != nil {
			a.warn("source missing: " + src)
			continue
		}
		st, err := os.Stat(root)
		if err != nil || !st.IsDir() {
			a.warn("source missing: " + src)
			continue
		}
		for _, f := range skill.FindInSource(root) {
			all = append(all, prov{f.Name, f.Dir, src})
		}
	}
	names := []string{}
	seen := map[string]struct{}{}
	byName := map[string][]prov{}
	for _, p := range all {
		byName[p.name] = append(byName[p.name], p)
		if _, ok := seen[p.name]; !ok {
			seen[p.name] = struct{}{}
			names = append(names, p.name)
		}
	}
	for _, name := range names {
		if a.cfg.IsExcluded(name) {
			continue
		}
		dest := filepath.Join(a.layout.Store, name)
		if fsops.IsRealDir(dest) {
			a.warn(fmt.Sprintf("'%s': unmanaged directory in store, skipped (move it into a source and 'add' it)", name))
			continue
		}
		if fsops.IsSymlink(dest) && destExists(dest) {
			continue
		}
		ps := byName[name]
		if len(ps) > 1 {
			a.warn(fmt.Sprintf("'%s': %d sources provide this name; not linking (add and pick an override)", name, len(ps)))
			continue
		}
		if err := a.linkSkill(name, ps[0].dir, false); err != nil {
			return err
		}
	}
	ents, err := os.ReadDir(a.layout.Store)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range ents {
		p := filepath.Join(a.layout.Store, e.Name())
		if fsops.IsSymlink(p) && !destExists(p) {
			a.info("pruning broken link: " + e.Name())
			_ = a.do("rm "+p, func() error { return os.Remove(p) })
		}
	}
	return nil
}

func (a *App) views() []agentregistry.View {
	return agentregistry.Views(agentregistry.Load(a.cfg))
}

type viewState string

const (
	stLinked    viewState = "linked"
	stAbsent    viewState = "absent"
	stRealDir   viewState = "real-dir"
	stWrongLink viewState = "wrong-link"
	stOther     viewState = "other"
	stNative    viewState = "native"
)

func (a *App) viewIsNative(p string) bool {
	return !fsops.IsSymlink(p) && fsops.IsRealDir(p) && fsops.PathsEqual(p, a.layout.Store)
}

func (a *App) viewState(p string) viewState {
	if !fsops.Exists(p) {
		return stAbsent
	}
	if fsops.IsSymlink(p) && fsops.PathsEqual(p, a.layout.Store) {
		return stLinked
	}
	if fsops.IsRealDir(p) {
		return stRealDir
	}
	if fsops.IsSymlink(p) {
		return stWrongLink
	}
	return stOther
}

func (a *App) newBackupDir() (string, error) {
	base := filepath.Join(a.layout.BackupsDir, time.Now().Format("20060102-150405"))
	bp := base
	for n := 0; fsops.Exists(bp); n++ {
		bp = fmt.Sprintf("%s-%d", base, n+1)
	}
	if err := a.do("mkdir "+bp, func() error { return os.MkdirAll(bp, 0o755) }); err != nil {
		return "", err
	}
	return bp, nil
}

func (a *App) makeView(path string, ids string) (migrated bool, err error) {
	if a.viewIsNative(path) {
		return false, nil
	}
	switch a.viewState(path) {
	case stLinked:
		a.info(ids + ": already linked")
		return false, nil
	case stAbsent:
		if _, err := os.Stat(filepath.Dir(path)); err == nil {
			if err := a.do("link view "+path, func() error {
				return fsops.LinkDir(a.layout.Store, path, a.Copy)
			}); err != nil {
				return false, err
			}
			a.ok(ids + a.ui.dim + " -> store" + a.ui.reset)
		} else {
			a.info(ids + ": not installed, skipped")
		}
		return false, nil
	case stWrongLink:
		if err := a.do("relink "+path, func() error {
			_ = os.Remove(path)
			return fsops.LinkDir(a.layout.Store, path, a.Copy)
		}); err != nil {
			return false, err
		}
		a.ok(ids + a.ui.dim + " (relinked)" + a.ui.reset)
		return false, nil
	case stRealDir:
		moved := false
		ents, _ := os.ReadDir(path)
		for _, e := range ents {
			entry := filepath.Join(path, e.Name())
			st, err := os.Lstat(entry)
			if err != nil {
				continue
			}
			if st.IsDir() && fileExists(filepath.Join(entry, "SKILL.md")) {
				sn := skill.NameFromDir(entry)
				if !skill.IsSafeName(sn) {
					bp, err := a.newBackupDir()
					if err != nil {
						return moved, err
					}
					a.warn("unsafe skill name from '" + entry + "'; backed up only")
					_ = a.do("mv unsafe", func() error { return os.Rename(entry, filepath.Join(bp, e.Name())) })
					continue
				}
				dest := filepath.Join(a.layout.LocalSource, sn)
				if fsops.Exists(dest) {
					bp, err := a.newBackupDir()
					if err != nil {
						return moved, err
					}
					a.warn("'" + sn + "' already in local source; backing up copy from " + path)
					_ = a.do("mv dup", func() error { return os.Rename(entry, filepath.Join(bp, sn)) })
				} else {
					if err := a.do("mkdir local", func() error { return os.MkdirAll(a.layout.LocalSource, 0o755) }); err != nil {
						return moved, err
					}
					if err := a.do("migrate "+sn, func() error { return os.Rename(entry, dest) }); err != nil {
						return moved, err
					}
					a.ok("migrated '" + sn + "'" + a.ui.dim + " -> sources/local" + a.ui.reset)
					moved = true
				}
			} else {
				bp, err := a.newBackupDir()
				if err != nil {
					return moved, err
				}
				a.warn("non-skill entry '" + e.Name() + "' in " + path + "; backed up")
				_ = a.do("mv nonskill", func() error { return os.Rename(entry, filepath.Join(bp, e.Name())) })
			}
		}
		bp, err := a.newBackupDir()
		if err != nil {
			return moved, err
		}
		bakName := "agent-dir-" + filepath.Base(filepath.Dir(path)) + "-" + filepath.Base(path)
		if err := a.do("backup agent dir", func() error { return os.Rename(path, filepath.Join(bp, bakName)) }); err != nil {
			return moved, err
		}
		if err := a.do("link view", func() error { return fsops.LinkDir(a.layout.Store, path, a.Copy) }); err != nil {
			return moved, err
		}
		a.ok(ids + a.ui.dim + " -> store (migrated)" + a.ui.reset)
		return moved, nil
	default:
		a.warn(ids + ": unexpected state at " + path + ", skipped")
		return false, nil
	}
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func (a *App) stdinTTY() bool {
	f, ok := a.Stdin.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func (a *App) stderrTTY() bool {
	f, ok := a.Stderr.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func (a *App) stdoutTTY() bool {
	f, ok := a.Stdout.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

func (a *App) interactive() bool {
	return a.stdinTTY() && a.stderrTTY()
}

type installCand struct {
	name, dir, kind, occ, cat, blurb string
}

func (a *App) readLine() (string, error) {
	sc := bufio.NewScanner(a.Stdin)
	if !sc.Scan() {
		return "", sc.Err()
	}
	return sc.Text(), nil
}

func (a *App) confirmTyped(word, msg string) bool {
	if a.DryRun || a.Yes {
		return true
	}
	if !a.stdinTTY() {
		return false
	}
	fmt.Fprintln(a.Stderr, a.ui.red+msg+a.ui.reset)
	fmt.Fprintf(a.Stderr, "type %s%s%s to continue: ", a.ui.bold, word, a.ui.reset)
	line, _ := a.readLine()
	return strings.TrimSpace(line) == word
}

func (a *App) compactHome(p string) string {
	home, _ := os.UserHomeDir()
	if p == home {
		return "~"
	}
	if strings.HasPrefix(p, home+string(os.PathSeparator)) {
		return "~/" + filepath.ToSlash(p[len(home)+1:])
	}
	return p
}

func (a *App) sourceHeading(src string) string {
	if gitx.IsGitURL(src) {
		u := gitx.NormalizeGitURL(src)
		u = strings.TrimPrefix(u, "https://")
		u = strings.TrimPrefix(u, "http://")
		u = strings.TrimPrefix(u, "git@")
		u = strings.Replace(u, ":", "/", 1)
		u = strings.TrimSuffix(u, ".git")
		return u
	}
	p := paths.ExpandOne(src)
	if r, err := fsops.ResolveDir(p); err == nil {
		p = r
	}
	return a.compactHome(p)
}

func (a *App) sourceIndex() [][2]string {
	var idx [][2]string
	for _, src := range a.cfg.Sources {
		root, err := a.sourceRoot(src)
		if err != nil {
			continue
		}
		res, err := fsops.ResolveDir(root)
		if err != nil {
			continue
		}
		idx = append(idx, [2]string{res, a.sourceHeading(src)})
	}
	return idx
}

func (a *App) skillPlace(entry string, idx [][2]string) (group, category string) {
	g, cat, _ := a.skillLocate(entry, idx)
	return g, cat
}

func (a *App) occupantLabel(entry string, idx [][2]string) string {
	g, _, rel := a.skillLocate(entry, idx)
	if rel != "" && rel != "." {
		return g + "/" + rel
	}
	return g
}

func (a *App) skillLocate(entry string, idx [][2]string) (group, category, rel string) {
	if fsops.IsRealDir(entry) {
		return "unmanaged", "", ""
	}
	if fsops.IsSymlink(entry) && !destExists(entry) {
		return "(broken)", "", ""
	}
	t, err := fsops.ResolveDir(entry)
	if err != nil {
		return "(broken)", "", ""
	}
	best := ""
	bestRoot := ""
	bestLen := -1
	for _, row := range idx {
		root := row[0]
		if t == root || strings.HasPrefix(t, root+string(os.PathSeparator)) {
			if len(root) >= bestLen {
				bestLen = len(root)
				best = row[1]
				bestRoot = root
			}
		}
	}
	if best == "" {
		return a.compactHome(t), "", ""
	}
	rel, err = filepath.Rel(bestRoot, t)
	if err != nil {
		rel = ""
	} else {
		rel = filepath.ToSlash(rel)
	}
	return best, skill.Category(bestRoot, t), rel
}

func (a *App) listNames() []string {
	ents, err := os.ReadDir(a.layout.Store)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range ents {
		p := filepath.Join(a.layout.Store, e.Name())
		if fsops.Exists(p) || fsops.IsSymlink(p) {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}
