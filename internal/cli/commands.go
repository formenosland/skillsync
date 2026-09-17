package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/formenosland/skillsync/internal/agentregistry"
	"github.com/formenosland/skillsync/internal/fsops"
	"github.com/formenosland/skillsync/internal/gitx"
	"github.com/formenosland/skillsync/internal/paths"
	"github.com/formenosland/skillsync/internal/skill"
)

func (a *App) cmdInit(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument: %s", args[0])
	}
	a.header("skillsync init")
	a.info("store:  " + a.layout.Store)
	a.info("config: " + a.layout.ConfigFile)
	if err := a.do("mkdir data", func() error {
		return os.MkdirAll(a.layout.SourcesDir, 0o755)
	}); err != nil {
		return err
	}
	if err := a.do("mkdir store", func() error {
		return os.MkdirAll(a.layout.Store, 0o755)
	}); err != nil {
		return err
	}

	type cand struct{ path, ids string }
	var cands []cand
	for _, v := range a.views() {
		if a.viewIsNative(v.Path) {
			continue
		}
		ids := agentregistry.IDList(v.IDs)
		st := a.viewState(v.Path)
		switch st {
		case stLinked:
			a.info(ids + ": already linked")
		case stAbsent:
			if _, err := os.Stat(filepath.Dir(v.Path)); err == nil {
				cands = append(cands, cand{v.Path, ids})
			}
		default:
			cands = append(cands, cand{v.Path, ids})
		}
	}
	if len(cands) > 0 {
		if !a.interactive() && !a.Yes {
			return fmt.Errorf("non-interactive init requires --yes (skillsync --yes init)")
		}
		var lines []string
		for _, c := range cands {
			lines = append(lines, c.ids+"  ->  "+c.path)
		}
		picked, err := a.pickMulti("link these agent skill folders to the store?", lines)
		if err != nil {
			return err
		}
		sel := map[string]struct{}{}
		for _, s := range picked {
			sel[s] = struct{}{}
		}
		migratedAny := false
		for _, c := range cands {
			key := c.ids + "  ->  " + c.path
			if _, ok := sel[key]; !ok {
				a.info(c.ids + ": skipped by selection")
				continue
			}
			moved, err := a.makeView(c.path, c.ids)
			if err != nil {
				return err
			}
			if moved {
				migratedAny = true
			}
		}
		if migratedAny {
			local := a.layout.LocalSource
			if r, err := fsops.ResolveDir(local); err == nil {
				local = r
			}
			a.cfg.AppendSource(local)
			if err := a.save(); err != nil {
				return err
			}
			a.info("migrated skills now live in " + local + " (registered as a path source)")
		}
	} else {
		a.info("no unlinked agents detected")
	}
	if err := a.materialize(); err != nil {
		return err
	}
	a.end("init done " + a.ui.dim + fmt.Sprintf("(%d ok, %d warnings)", a.nOK, a.nWarn) + a.ui.reset)
	return nil
}

func (a *App) cmdAdd(args []string) error {
	var source string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("unknown option for add: %s", arg)
		}
		if source != "" {
			return fmt.Errorf("unexpected argument: %s", arg)
		}
		source = arg
	}
	if source == "" {
		return fmt.Errorf("usage: skillsync add <git-url|owner/repo|path>")
	}
	if !a.interactive() && !a.Yes {
		return fmt.Errorf("non-interactive add requires --yes (skillsync --yes add <source>)")
	}
	entry, err := a.normalizeSource(source)
	if err != nil {
		return err
	}
	a.header("skillsync add " + entry)
	if err := a.fetchSource(source); err != nil {
		if gitx.IsGitURL(source) && !strings.Contains(err.Error(), "unsafe") {
			return fmt.Errorf("could not fetch source: %w", err)
		}
		return err
	}
	a.cfg.AppendSource(entry)
	if err := a.save(); err != nil {
		return err
	}
	root, err := a.sourceRoot(entry)
	if err != nil {
		return err
	}
	idx := a.sourceIndex()
	var cands []installCand
	for _, f := range skill.FindInSource(root) {
		c := installCand{name: f.Name, dir: f.Dir, kind: "new", cat: f.Category}
		dest := filepath.Join(a.layout.Store, f.Name)
		if a.storeOccupied(f.Name) && !fsops.PathsEqual(dest, f.Dir) {
			c.kind = "override"
			c.occ = a.skillGroup(dest, idx)
		}
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].cat != cands[j].cat {
			return cands[i].cat < cands[j].cat
		}
		return cands[i].name < cands[j].name
	})
	if len(cands) == 0 {
		a.info("no skills found in source (" + skill.FindHint + ")")
		a.end("add done " + a.ui.dim + fmt.Sprintf("(%d ok, %d warnings)", a.nOK, a.nWarn) + a.ui.reset)
		return nil
	}
	if a.Yes {
		for _, c := range cands {
			if c.kind == "override" {
				occ := c.occ
				if occ == "" {
					occ = "another source"
				}
				a.warn(fmt.Sprintf("'%s': already installed from %s, skipped", c.name, occ))
			}
		}
	}
	chosen, err := a.pickInstall("install skills from this source?", cands)
	if err != nil {
		return err
	}
	sel := map[string]struct{}{}
	for _, n := range chosen {
		sel[n] = struct{}{}
	}
	for _, c := range cands {
		if _, ok := sel[c.name]; ok {
			a.cfg.ExcludeDel(c.name)
			if err := a.linkSkill(c.name, c.dir, true); err != nil {
				return err
			}
		} else if c.kind == "new" {
			a.cfg.ExcludeAdd(c.name)
			a.info("skipped '" + c.name + "' (excluded; sync will not install it)")
		}
	}
	if err := a.save(); err != nil {
		return err
	}
	if err := a.materialize(); err != nil {
		return err
	}
	a.end("add done " + a.ui.dim + fmt.Sprintf("(%d ok, %d warnings)", a.nOK, a.nWarn) + a.ui.reset)
	return nil
}

func (a *App) cmdSync(args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument: %s", args[0])
	}
	a.header("skillsync sync")
	for _, src := range a.cfg.Sources {
		if !gitx.IsGitURL(src) {
			continue
		}
		if err := a.fetchSource(src); err != nil {
			a.warn("could not update source: " + src + " (continuing)")
		}
	}
	if err := a.materialize(); err != nil {
		return err
	}
	if err := a.applyProjectIfPresent(); err != nil {
		return err
	}
	a.end("sync done " + a.ui.dim + fmt.Sprintf("(%d ok, %d warnings)", a.nOK, a.nWarn) + a.ui.reset)
	return nil
}

func (a *App) cmdList(args []string) error {
	pretty, names := false, false
	for _, arg := range args {
		switch arg {
		case "--pretty":
			pretty = true
		case "--names", "-1":
			names = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option for list: %s", arg)
			}
			return fmt.Errorf("unexpected argument: %s", arg)
		}
	}
	if names || (!pretty && !a.stdoutTTY()) {
		for _, n := range a.listNames() {
			fmt.Fprintln(a.Stdout, n)
		}
		return nil
	}
	a.listPretty()
	return nil
}

func (a *App) listPretty() {
	lc := struct{ reset, bold, dim string }{}
	if a.stdoutTTY() && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		lc.reset, lc.bold, lc.dim = a.ui.reset, a.ui.bold, a.ui.dim
	}
	idx := a.sourceIndex()
	type row struct{ name, group, cat, blurb string }
	var rows []row
	for _, n := range a.listNames() {
		e := filepath.Join(a.layout.Store, n)
		blurb := skill.Shorten(skill.Blurb(e), a.ui.fancy)
		group, cat := a.skillPlace(e, idx)
		rows = append(rows, row{n, group, cat, blurb})
	}
	if len(rows) == 0 {
		fmt.Fprintln(a.Stdout, lc.dim+"(empty — run init / add)"+lc.reset)
		return
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].group != rows[j].group {
			return rows[i].group < rows[j].group
		}
		if rows[i].cat != rows[j].cat {
			return rows[i].cat < rows[j].cat
		}
		return rows[i].name < rows[j].name
	})
	w := 12
	for _, r := range rows {
		if len(r.name) > w {
			w = len(r.name)
		}
	}
	if w > 32 {
		w = 32
	}
	prev := ""
	prevCat := "\x00"
	for i, r := range rows {
		if r.group != prev {
			if i > 0 {
				fmt.Fprintln(a.Stdout)
			}
			fmt.Fprintln(a.Stdout, lc.bold+r.group+lc.reset)
			prev = r.group
			prevCat = "\x00"
		}
		if r.cat != prevCat {
			if r.cat != "" {
				fmt.Fprintln(a.Stdout, "  "+lc.bold+r.cat+lc.reset)
			}
			prevCat = r.cat
		}
		indent := "  "
		if r.cat != "" {
			indent = "    "
		}
		pad := w - len(r.name)
		if pad < 1 {
			pad = 1
		}
		spaces := strings.Repeat(" ", pad)
		if r.blurb != "" {
			fmt.Fprintf(a.Stdout, "%s%s%s%s%s  %s%s%s\n", indent, lc.bold, r.name, lc.reset, spaces, lc.dim, r.blurb, lc.reset)
		} else {
			fmt.Fprintf(a.Stdout, "%s%s%s%s\n", indent, lc.bold, r.name, lc.reset)
		}
	}
}

func (a *App) cmdRemove(args []string) error {
	srcMode, all := false, false
	var names []string
	for _, arg := range args {
		switch arg {
		case "--source":
			srcMode = true
		case "--all":
			all = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option for remove: %s", arg)
			}
			names = append(names, arg)
		}
	}
	if srcMode {
		if len(names) == 0 {
			return fmt.Errorf("usage: skillsync remove --source <url|path>")
		}
		for _, s := range names {
			if err := a.removeSource(s); err != nil {
				return err
			}
		}
		if err := a.save(); err != nil {
			return err
		}
		if err := a.materialize(); err != nil {
			return err
		}
		a.end("remove done")
		return nil
	}
	if all {
		names = a.listNames()
	} else if len(names) == 0 {
		picked, err := a.pickMulti("remove which skills?", a.listNames())
		if err != nil {
			return err
		}
		if len(picked) == 0 {
			return fmt.Errorf("specify skill names or use --all (non-interactive)")
		}
		names = picked
	}
	a.header("skillsync remove")
	for _, n := range names {
		if n == "" {
			continue
		}
		if !skill.IsSafeName(n) {
			a.err("'" + n + "': unsafe skill name, not removing")
			continue
		}
		dest := filepath.Join(a.layout.Store, n)
		if fsops.IsRealDir(dest) {
			a.err("'" + n + "' is unmanaged (real directory at " + dest + ") — not touching it")
			continue
		}
		if !fsops.IsSymlink(dest) && !fsops.Exists(dest) {
			a.warn("'" + n + "': not installed")
			continue
		}
		if err := a.do("rm "+dest, func() error { return os.Remove(dest) }); err != nil {
			return err
		}
		a.cfg.ExcludeAdd(n)
		a.ok("removed " + n + a.ui.dim + " (gone from all agents; sync won't restore it)" + a.ui.reset)
	}
	if err := a.save(); err != nil {
		return err
	}
	a.end("remove done " + a.ui.dim + fmt.Sprintf("(%d removed, %d warnings)", a.nOK, a.nWarn) + a.ui.reset)
	return nil
}

func (a *App) removeSource(s string) error {
	var entry string
	if gitx.IsGitURL(s) {
		entry = gitx.NormalizeGitURL(s)
	} else {
		entry = paths.ExpandOne(s)
		if r, err := fsops.ResolveDir(entry); err == nil {
			entry = r
		}
	}
	if a.cfg.DropSource(entry) {
		a.ok("unregistered " + entry)
	} else {
		a.warn("not in manifest: " + entry)
	}
	cp, err := a.sourceRoot(entry)
	if err != nil {
		return nil
	}
	cpRes := cp
	if r, err := fsops.ResolveDir(cp); err == nil {
		cpRes = r
	}
	ents, _ := os.ReadDir(a.layout.Store)
	for _, e := range ents {
		p := filepath.Join(a.layout.Store, e.Name())
		if !fsops.IsSymlink(p) {
			continue
		}
		t, err := fsops.ResolveDir(p)
		if err != nil {
			continue
		}
		if t == cpRes || strings.HasPrefix(t, cpRes+string(os.PathSeparator)) {
			_ = a.do("rm "+p, func() error { return os.Remove(p) })
			a.ok("removed " + e.Name())
		}
	}
	srcAbs, _ := filepath.Abs(a.layout.SourcesDir)
	cpAbs, _ := filepath.Abs(cp)
	rel, err := filepath.Rel(srcAbs, cpAbs)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, "..") {
		if st, err := os.Stat(cp); err == nil && st.IsDir() {
			_ = a.do("rm clone "+cp, func() error { return os.RemoveAll(cp) })
			a.ok("deleted clone " + cp)
		}
	} else {
		a.info("local folder kept: " + cp)
	}
	return nil
}

func (a *App) sourceHealth(src string) (state, color string) {
	root, err := a.sourceRoot(src)
	if err != nil {
		return "missing", a.ui.yellow
	}
	st, err := os.Stat(root)
	if err != nil || !st.IsDir() {
		return "missing", a.ui.yellow
	}
	if !gitx.IsGitURL(src) {
		return "path", a.ui.green
	}
	s := gitx.SyncState(root)
	switch s {
	case "up to date":
		return s, a.ui.green
	case "local":
		return s, a.ui.dim
	default:
		return s, a.ui.yellow
	}
}

func (a *App) cmdStatus(args []string) error {
	fmt.Fprintf(a.Stdout, "%sskillsync %s%s\n", a.ui.bold, Version, a.ui.reset)
	fmt.Fprintf(a.Stdout, "  %sstore%s     %s\n", a.ui.dim, a.ui.reset, a.layout.Store)
	fmt.Fprintf(a.Stdout, "  %sconfig%s    %s\n", a.ui.dim, a.ui.reset, a.layout.ConfigFile)
	nSkills := len(a.listNames())
	nSources := len(a.cfg.Sources)
	srcWord := "sources"
	if nSources == 1 {
		srcWord = "source"
	}
	fmt.Fprintf(a.Stdout, "  %sskills%s    %d from %d %s\n", a.ui.dim, a.ui.reset, nSkills, nSources, srcWord)
	if n := len(a.cfg.Excludes); n > 0 {
		fmt.Fprintf(a.Stdout, "  %sexcluded%s  %d\n", a.ui.dim, a.ui.reset, n)
	}

	if nSources > 0 {
		fmt.Fprintf(a.Stdout, "\n%sSources%s\n", a.ui.bold, a.ui.reset)
		for _, src := range a.cfg.Sources {
			st, color := a.sourceHealth(src)
			fmt.Fprintf(a.Stdout, "  %s%-10s%s %s\n", color, st, a.ui.reset, a.sourceHeading(src))
		}
	}

	fmt.Fprintf(a.Stdout, "\n%sAgent views%s\n", a.ui.bold, a.ui.reset)
	for _, v := range a.views() {
		st := a.viewState(v.Path)
		if a.viewIsNative(v.Path) {
			st = stNative
		}
		if st == stAbsent {
			if _, err := os.Stat(filepath.Dir(v.Path)); err != nil {
				continue
			}
		}
		c := a.ui.yellow
		switch st {
		case stLinked, stNative:
			c = a.ui.green
		case stAbsent:
			c = a.ui.dim
		}
		fmt.Fprintf(a.Stdout, "  %s%-10s%s %-46s %s\n", c, st, a.ui.reset, v.Path, a.ui.dim+agentregistry.IDList(v.IDs)+a.ui.reset)
	}
	if len(a.cfg.Excludes) > 0 {
		fmt.Fprintf(a.Stdout, "\n%sExcluded%s (removed skills; edit %s to restore)\n", a.ui.bold, a.ui.reset, a.layout.ConfigFile)
		for _, e := range a.cfg.Excludes {
			fmt.Fprintf(a.Stdout, "  %s\n", e)
		}
	}
	return nil
}

func (a *App) cmdDoctor(args []string) error {
	a.header("skillsync doctor")
	issues := 0
	ents, _ := os.ReadDir(a.layout.Store)
	for _, e := range ents {
		p := filepath.Join(a.layout.Store, e.Name())
		if fsops.IsSymlink(p) && !destExists(p) {
			a.err("broken link: " + e.Name() + " -> " + fsops.Readlink(p) + "  (run sync)")
			issues++
		} else if fsops.IsRealDir(p) {
			a.warn("unmanaged dir in store: " + e.Name() + "  (move into a source, then 'add')")
		}
	}
	for _, v := range a.views() {
		if a.viewIsNative(v.Path) {
			continue
		}
		ids := agentregistry.IDList(v.IDs)
		switch a.viewState(v.Path) {
		case stRealDir:
			a.err("drifted view: " + v.Path + " (" + ids + ") is a real directory  (run init)")
			issues++
		case stWrongLink:
			a.err("wrong link: " + v.Path + " (" + ids + ") -> " + fsops.Readlink(v.Path) + "  (run init)")
			issues++
		case stAbsent:
			if _, err := os.Stat(filepath.Dir(v.Path)); err == nil {
				a.err("not linked: " + v.Path + " (" + ids + ")  (run init)")
				issues++
			}
		}
	}
	for _, src := range a.cfg.Sources {
		root, err := a.sourceRoot(src)
		if err != nil {
			a.warn("source missing on disk: " + src + "  (run sync or add)")
			continue
		}
		if st, err := os.Stat(root); err != nil || !st.IsDir() {
			a.warn("source missing on disk: " + src + "  (run sync or add)")
		}
	}
	issues += a.doctorProject()
	if issues > 0 {
		a.end(a.ui.red + fmt.Sprintf("%d issue(s) found", issues) + a.ui.reset)
		return errQuiet
	}
	a.end(a.ui.green + "all clear" + a.ui.reset)
	return nil
}

func (a *App) cmdUninstall(args []string) error {
	keep, purge := false, false
	for _, arg := range args {
		switch arg {
		case "--keep":
			keep = true
		case "--purge":
			purge = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option for uninstall: %s", arg)
			}
			return fmt.Errorf("unexpected argument: %s", arg)
		}
	}
	a.header("skillsync uninstall")
	for _, v := range a.views() {
		if a.viewIsNative(v.Path) {
			continue
		}
		if a.viewState(v.Path) != stLinked {
			continue
		}
		ids := agentregistry.IDList(v.IDs)
		if keep {
			if err := a.do("materialize view "+v.Path, func() error {
				if err := os.Remove(v.Path); err != nil {
					return err
				}
				if err := os.MkdirAll(v.Path, 0o755); err != nil {
					return err
				}
				for _, n := range a.listNames() {
					src := filepath.Join(a.layout.Store, n)
					if !destExists(src) {
						continue
					}
					if err := fsops.CopyDirFollow(src, filepath.Join(v.Path, n)); err != nil {
						return err
					}
				}
				return nil
			}); err != nil {
				return err
			}
			a.ok(ids + a.ui.dim + " (view -> real directory with copies)" + a.ui.reset)
		} else {
			if err := a.do("rm view "+v.Path, func() error { return os.Remove(v.Path) }); err != nil {
				return err
			}
			a.ok(ids + a.ui.dim + " (view removed)" + a.ui.reset)
		}
	}
	if purge {
		if a.confirmTyped("nuke", "--purge deletes the store, all cloned sources (including sources/local), skillsyncrc, and backups. This cannot be undone.") {
			if err := a.do("rm data", func() error { return os.RemoveAll(a.layout.DataDir) }); err != nil {
				return err
			}
			if filepath.Dir(a.layout.ConfigFile) != a.layout.DataDir && !strings.HasPrefix(a.layout.ConfigFile, a.layout.DataDir+string(os.PathSeparator)) {
				if err := a.do("rm rc", func() error { return os.Remove(a.layout.ConfigFile) }); err != nil && !os.IsNotExist(err) {
					return err
				}
			}
			a.ok("purged " + a.layout.DataDir + " and " + a.layout.ConfigFile)
		} else {
			a.warn("purge cancelled")
		}
	} else {
		a.info("store, sources, and config kept (use --purge to delete them)")
	}
	a.end("uninstall done")
	return nil
}
