package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/formenosland/skillsync/internal/agentregistry"
	"github.com/formenosland/skillsync/internal/config"
	"github.com/formenosland/skillsync/internal/fsops"
	"github.com/formenosland/skillsync/internal/gitignore"
	"github.com/formenosland/skillsync/internal/gitx"
	"github.com/formenosland/skillsync/internal/skill"
)

const agentsSkillsRel = ".agents/skills"

func (a *App) cmdApply(args []string) error {
	global, prune := false, false
	for _, arg := range args {
		switch arg {
		case "--global":
			global = true
		case "--prune":
			prune = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown option for apply: %s", arg)
			}
			return fmt.Errorf("unexpected argument: %s", arg)
		}
	}
	root, man, err := config.FindManifest(".")
	if err != nil {
		return fmt.Errorf("no %s found (walk up from the current directory)", config.ManifestName)
	}
	proj, err := config.LoadProject(man)
	if err != nil {
		return err
	}
	a.header("skillsync apply " + man)
	if !a.Yes && !a.DryRun {
		if !a.interactive() {
			return fmt.Errorf("non-interactive apply requires --yes")
		}
		if !a.confirmApply(root) {
			a.warn("apply cancelled")
			return nil
		}
	}
	wanted, err := a.projectWanted(root, proj)
	if err != nil {
		return err
	}
	views, err := a.projectViews(root, proj)
	if err != nil {
		return err
	}
	if prune {
		if err := a.pruneProject(views, wanted); err != nil {
			return err
		}
	}
	if err := a.materializeProject(views, wanted); err != nil {
		return err
	}
	if global {
		if err := a.promoteProjectGlobal(root, proj, wanted); err != nil {
			return err
		}
	}
	a.end("apply done " + a.ui.dim + fmt.Sprintf("(%d ok, %d warnings)", a.nOK, a.nWarn) + a.ui.reset)
	return nil
}

func (a *App) cmdUnapply(args []string) error {
	var names []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			return fmt.Errorf("unknown option for unapply: %s", arg)
		}
		names = append(names, arg)
	}
	root, man, err := config.FindManifest(".")
	if err != nil {
		return fmt.Errorf("no %s found (walk up from the current directory)", config.ManifestName)
	}
	proj, err := config.LoadProject(man)
	if err != nil {
		return err
	}
	a.header("skillsync unapply")
	views, err := a.projectViews(root, proj)
	if err != nil {
		return err
	}
	want := map[string]struct{}{}
	if len(names) == 0 {
		for _, n := range a.managedNames(views) {
			want[n] = struct{}{}
		}
	} else {
		for _, n := range names {
			if !skill.IsSafeName(n) {
				return fmt.Errorf("unsafe skill name: %s", n)
			}
			want[n] = struct{}{}
		}
	}
	if err := a.removeProjectLinks(views, want); err != nil {
		return err
	}
	var remain []string
	for _, n := range a.managedNames(views) {
		if _, drop := want[n]; drop {
			continue
		}
		remain = append(remain, n)
	}
	sort.Strings(remain)
	if err := a.writeProjectGitignores(views, remain); err != nil {
		return err
	}
	a.end("unapply done " + a.ui.dim + fmt.Sprintf("(%d ok, %d warnings)", a.nOK, a.nWarn) + a.ui.reset)
	return nil
}

func (a *App) confirmApply(root string) bool {
	fmt.Fprintf(a.Stderr, "apply project skills in %s? [y/N] ", root)
	line, _ := a.readLine()
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

func (a *App) applyProjectIfPresent() error {
	root, man, err := config.FindManifest(".")
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	proj, err := config.LoadProject(man)
	if err != nil {
		return err
	}
	a.info("project: " + man)
	wanted, err := a.projectWanted(root, proj)
	if err != nil {
		return err
	}
	views, err := a.projectViews(root, proj)
	if err != nil {
		return err
	}
	return a.materializeProject(views, wanted)
}

type wantedSkill struct {
	name, dir, src string
}

func (a *App) projectWanted(repoRoot string, proj config.Project) (map[string]wantedSkill, error) {
	out := map[string]wantedSkill{}
	for _, s := range proj.Skills.Sources {
		root, err := a.projectSourceRoot(repoRoot, s)
		if err != nil {
			return nil, err
		}
		found := map[string]string{}
		for _, f := range skill.FindInSource(root) {
			found[f.Name] = f.Dir
		}
		selectAll := len(s.Skills) == 0
		var pick []string
		for _, n := range s.Skills {
			n = strings.TrimSpace(n)
			if n == "*" || n == "" {
				selectAll = true
				continue
			}
			pick = append(pick, n)
		}
		if selectAll {
			pick = nil
			for n := range found {
				pick = append(pick, n)
			}
			sort.Strings(pick)
		}
		for _, n := range pick {
			if !skill.IsSafeName(n) {
				return nil, fmt.Errorf("unsafe skill name %q in source %s", n, s.URL)
			}
			dir, ok := found[n]
			if !ok {
				return nil, fmt.Errorf("skill %q not found in source %s", n, s.URL)
			}
			if prev, ok := out[n]; ok {
				return nil, fmt.Errorf("skill %q provided by both %s and %s", n, prev.src, s.URL)
			}
			out[n] = wantedSkill{name: n, dir: dir, src: s.URL}
		}
	}
	return out, nil
}

func (a *App) projectSourceRoot(repoRoot string, s config.ProjectSource) (string, error) {
	u := strings.TrimSpace(s.URL)
	if gitx.IsGitURL(u) {
		if a.DryRun {
			a.info("[dry-run] clone/pull " + gitx.NormalizeGitURL(u))
			dest, err := gitx.CloneDir(a.layout.SourcesDir, u)
			if err != nil {
				return "", err
			}
			return dest, nil
		}
		if err := gitx.CloneOrPull(a.layout.SourcesDir, u); err != nil {
			return "", fmt.Errorf("fetch %s: %w", u, err)
		}
		dest, err := gitx.CloneDir(a.layout.SourcesDir, u)
		if err != nil {
			return "", err
		}
		if err := gitx.CheckoutRef(dest, s.Ref); err != nil {
			return "", err
		}
		return dest, nil
	}
	p := u
	if !filepath.IsAbs(p) {
		p = filepath.Join(repoRoot, p)
	}
	st, err := os.Stat(p)
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("local source not found: %s", s.URL)
	}
	return fsops.ResolveDir(p)
}

func (a *App) projectViews(repoRoot string, proj config.Project) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	add := func(rel string) {
		rel = filepath.ToSlash(rel)
		if rel == "" || rel == "-" {
			return
		}
		abs := filepath.Join(repoRoot, filepath.FromSlash(rel))
		key := abs
		if r, err := filepath.Abs(abs); err == nil {
			key = r
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	add(agentsSkillsRel)
	rows := agentregistry.Load(a.cfg)
	for _, id := range proj.Views.IDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		r, ok := agentregistry.RowByID(rows, id)
		if !ok {
			return nil, fmt.Errorf("unknown view id %q (not in the agent registry)", id)
		}
		pp := strings.TrimSpace(r.ProjectPath)
		if pp == "" || pp == "-" {
			return nil, fmt.Errorf("view id %q has no project_path", id)
		}
		add(pp)
	}
	sort.Strings(out)
	return out, nil
}

func (a *App) materializeProject(views []string, wanted map[string]wantedSkill) error {
	names := make([]string, 0, len(wanted))
	for n := range wanted {
		names = append(names, n)
	}
	sort.Strings(names)
	var conflicts []string
	for _, view := range views {
		for _, n := range names {
			dest := filepath.Join(view, n)
			src := wanted[n].dir
			if err := a.projectOccupancy(view, dest, src); err != nil {
				conflicts = append(conflicts, fmt.Sprintf("%s (%s)", dest, err.Error()))
			}
		}
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("name collision (resolve first-party skills or drop names from %s):\n  %s",
			config.ManifestName, strings.Join(conflicts, "\n  "))
	}
	for _, view := range views {
		if err := a.do("mkdir "+view, func() error { return os.MkdirAll(view, 0o755) }); err != nil {
			return err
		}
		for _, n := range names {
			dest := filepath.Join(view, n)
			src := wanted[n].dir
			if fsops.IsSymlink(dest) && fsops.PathsEqual(dest, src) && !a.Copy {
				continue
			}
			if fsops.IsSymlink(dest) {
				if err := a.do("rm "+dest, func() error { return os.Remove(dest) }); err != nil {
					return err
				}
			}
			if a.Copy {
				if fsops.IsRealDir(dest) && a.ourProjectCopy(dest, src) {
					if err := a.do("rm copy "+dest, func() error { return os.RemoveAll(dest) }); err != nil {
						return err
					}
				}
				if err := a.do("copy "+n+" -> "+view, func() error { return fsops.LinkDir(src, dest, true) }); err != nil {
					return err
				}
				a.ok(n + a.ui.dim + " -> " + view + " (copy)" + a.ui.reset)
				continue
			}
			if err := a.do("link "+n+" -> "+view, func() error { return fsops.LinkDir(src, dest, false) }); err != nil {
				return err
			}
			a.ok(n + a.ui.dim + " -> " + view + a.ui.reset)
		}
	}
	return a.writeProjectGitignores(views, names)
}

func (a *App) projectOccupancy(view, dest, src string) error {
	if !fsops.Exists(dest) {
		return nil
	}
	name := filepath.Base(dest)
	if fsops.IsSymlink(dest) {
		if fsops.PathsEqual(dest, src) {
			return nil
		}
		if a.ourProjectLink(dest) || a.gitignoreHas(view, name) {
			return nil
		}
		return fmt.Errorf("unmanaged link")
	}
	if fsops.IsRealDir(dest) {
		if a.Copy && a.ourProjectCopy(dest, src) {
			return nil
		}
		return fmt.Errorf("unmanaged directory")
	}
	return fmt.Errorf("unexpected file")
}

func (a *App) gitignoreHas(view, name string) bool {
	for _, n := range gitignore.Names(filepath.Join(view, ".gitignore")) {
		if n == name {
			return true
		}
	}
	return false
}

func (a *App) ourProjectLink(dest string) bool {
	if !fsops.IsSymlink(dest) {
		return false
	}
	t, err := fsops.ResolveDir(dest)
	if err != nil {
		return false
	}
	srcRoot, err := filepath.Abs(a.layout.SourcesDir)
	if err != nil {
		srcRoot = a.layout.SourcesDir
	}
	return t == srcRoot || strings.HasPrefix(t, srcRoot+string(os.PathSeparator))
}

func (a *App) ourProjectCopy(dest, src string) bool {
	return fileExists(filepath.Join(dest, "SKILL.md")) && skill.NameFromDir(dest) == skill.NameFromDir(src)
}

func (a *App) pruneProject(views []string, wanted map[string]wantedSkill) error {
	stale := map[string]struct{}{}
	for _, n := range a.managedNames(views) {
		if _, ok := wanted[n]; !ok {
			stale[n] = struct{}{}
		}
	}
	if err := a.removeProjectLinks(views, stale); err != nil {
		return err
	}
	names := make([]string, 0, len(wanted))
	for n := range wanted {
		names = append(names, n)
	}
	sort.Strings(names)
	return a.writeProjectGitignores(views, names)
}

func (a *App) managedNames(views []string) []string {
	seen := map[string]struct{}{}
	for _, view := range views {
		for _, n := range gitignore.Names(filepath.Join(view, ".gitignore")) {
			seen[n] = struct{}{}
		}
		ents, _ := os.ReadDir(view)
		for _, e := range ents {
			p := filepath.Join(view, e.Name())
			if a.ourProjectLink(p) {
				seen[e.Name()] = struct{}{}
			}
		}
	}
	var out []string
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

func (a *App) removeProjectLinks(views []string, names map[string]struct{}) error {
	for _, view := range views {
		for n := range names {
			dest := filepath.Join(view, n)
			if fsops.IsSymlink(dest) {
				if err := a.do("rm "+dest, func() error { return os.Remove(dest) }); err != nil {
					return err
				}
				a.ok("removed " + n + a.ui.dim + " from " + view + a.ui.reset)
				continue
			}
			if fsops.IsRealDir(dest) {
				a.warn("'" + n + "' in " + view + " is a real directory, not removing")
			}
		}
	}
	return nil
}

func (a *App) writeProjectGitignores(views, names []string) error {
	sort.Strings(names)
	for _, view := range views {
		gi := filepath.Join(view, ".gitignore")
		if a.DryRun {
			a.info("[dry-run] gitignore " + gi)
			continue
		}
		if err := os.MkdirAll(view, 0o755); err != nil {
			return err
		}
		if err := gitignore.Rewrite(gi, names); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) promoteProjectGlobal(repoRoot string, proj config.Project, wanted map[string]wantedSkill) error {
	for _, s := range proj.Skills.Sources {
		u := strings.TrimSpace(s.URL)
		if gitx.IsGitURL(u) {
			a.cfg.AppendSource(gitx.NormalizeGitURL(u))
			continue
		}
		p := u
		if !filepath.IsAbs(p) {
			p = filepath.Join(repoRoot, p)
		}
		if r, err := fsops.ResolveDir(p); err == nil {
			a.cfg.AppendSource(r)
		}
	}
	if err := a.save(); err != nil {
		return err
	}
	names := make([]string, 0, len(wanted))
	for n := range wanted {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		w := wanted[n]
		dest := filepath.Join(a.layout.Store, n)
		if a.storeOccupied(n) && !fsops.PathsEqual(dest, w.dir) {
			a.warn(fmt.Sprintf("'%s': already in the home store, skipped", n))
			continue
		}
		a.cfg.ExcludeDel(n)
		if err := a.linkSkill(n, w.dir, false); err != nil {
			return err
		}
	}
	return a.save()
}

func (a *App) doctorProject() int {
	root, man, err := config.FindManifest(".")
	if err != nil {
		return 0
	}
	proj, err := config.LoadProject(man)
	if err != nil {
		a.err(err.Error())
		return 1
	}
	wanted, err := a.projectWanted(root, proj)
	if err != nil {
		a.err("project: " + err.Error())
		return 1
	}
	views, err := a.projectViews(root, proj)
	if err != nil {
		a.err("project: " + err.Error())
		return 1
	}
	issues := 0
	for _, view := range views {
		gi := filepath.Join(view, ".gitignore")
		listed := map[string]struct{}{}
		for _, n := range gitignore.Names(gi) {
			listed[n] = struct{}{}
		}
		for n := range wanted {
			if _, ok := listed[n]; !ok {
				a.err("project gitignore missing " + n + " in " + view + "  (run apply)")
				issues++
			}
			dest := filepath.Join(view, n)
			if !fsops.Exists(dest) && !fsops.IsSymlink(dest) {
				a.err("project skill not applied: " + dest + "  (run apply)")
				issues++
				continue
			}
			if err := a.projectOccupancy(view, dest, wanted[n].dir); err != nil {
				a.err("project collision: " + dest + "  (" + err.Error() + ")")
				issues++
			}
		}
	}
	return issues
}
