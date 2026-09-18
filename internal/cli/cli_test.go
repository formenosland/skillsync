package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/formenosland/skillsync/internal/skill"
)

func makeSkill(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\ndescription: test skill " + name + "\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

type sandbox struct {
	t    *testing.T
	root string
	home string
	sync string
	reg  string
}

func newSandbox(t *testing.T) *sandbox {
	t.Helper()
	root := t.TempDir()
	home := filepath.Join(root, "home")
	sync := filepath.Join(root, "sync")
	if err := os.MkdirAll(filepath.Join(home, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(home, ".codex"), 0o755); err != nil {
		t.Fatal(err)
	}
	reg := filepath.Join(root, "reg.tsv")
	tsv := "agent_id\tdisplay_name\tglobal_path\tproject_path\n" +
		"fake-claude\tFake Claude\t" + home + "/.claude/skills\t.claude/skills\n" +
		"fake-codex\tFake Codex\t${FAKE_CODEX_HOME:-" + home + "/.codex}/skills\t.agents/skills\n" +
		"fake-absent\tNot Installed\t" + home + "/.nope/skills\t.nope/skills\n"
	if err := os.WriteFile(reg, []byte(tsv), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKILLSYNC_HOME", sync)
	t.Setenv("SKILLSYNC_REGISTRY", reg)
	t.Setenv("HOME", home)
	t.Setenv("NO_COLOR", "1")
	return &sandbox{t: t, root: root, home: home, sync: sync, reg: reg}
}

func (s *sandbox) run(args ...string) (stdout, stderr string, code int) {
	s.t.Helper()
	var out, errb bytes.Buffer
	code = Main(append([]string{"skillsync"}, args...), bytes.NewReader(nil), &out, &errb)
	return out.String(), errb.String(), code
}

func (s *sandbox) yes(args ...string) (stdout, stderr string, code int) {
	return s.run(append([]string{"--yes"}, args...)...)
}

func isView(t *testing.T, path, store string) bool {
	t.Helper()
	a, err := filepath.EvalSymlinks(path)
	if err != nil {
		return false
	}
	b, err := filepath.EvalSymlinks(store)
	if err != nil {
		return false
	}
	return a == b
}

func isLinkOrView(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return true
	}
	return fi.IsDir()
}

func TestVersion(t *testing.T) {
	s := newSandbox(t)
	out, _, code := s.run("--version")
	if code != 0 || strings.TrimSpace(out) != Version {
		t.Fatalf("version %q code %d", out, code)
	}
	out, _, code = s.run("-V")
	if code != 0 || strings.TrimSpace(out) != Version {
		t.Fatalf("-V %q code %d", out, code)
	}
}

func TestInitMigrateAndViews(t *testing.T) {
	s := newSandbox(t)
	makeSkill(t, filepath.Join(s.home, ".claude", "skills", "oldskill"), "oldskill")
	_, errb, code := s.yes("init")
	if code != 0 {
		t.Fatalf("init: %s", errb)
	}
	store := filepath.Join(s.sync, "store")
	claude := filepath.Join(s.home, ".claude", "skills")
	if !isView(t, claude, store) {
		t.Fatal("claude view not linked")
	}
	if !isView(t, filepath.Join(s.home, ".codex", "skills"), store) {
		t.Fatal("codex view not linked")
	}
	if _, err := os.Stat(filepath.Join(s.home, ".nope")); !os.IsNotExist(err) {
		t.Fatal("uninstalled agent touched")
	}
	if _, err := os.Stat(filepath.Join(s.sync, "sources", "local", "oldskill", "SKILL.md")); err != nil {
		t.Fatal("migrated skill missing")
	}
	if !isLinkOrView(filepath.Join(s.sync, "store", "oldskill")) {
		t.Fatal("store link missing")
	}
	rc, err := os.ReadFile(filepath.Join(s.sync, "skillsyncrc"))
	if err != nil || !strings.Contains(filepath.ToSlash(string(rc)), "sources/local") {
		t.Fatalf("skillsyncrc should register local: %s %v", rc, err)
	}
	if _, err := os.Stat(filepath.Join(claude, "oldskill", "SKILL.md")); err != nil {
		t.Fatal("not visible through view")
	}
	_, errb, code = s.yes("init")
	if code != 0 || !strings.Contains(errb, "already linked") {
		t.Fatalf("second init: %s", errb)
	}
	if strings.ContainsAny(errb, "◆└") {
		t.Fatal("tree chrome")
	}
}

func TestAddOccupancyAndList(t *testing.T) {
	s := newSandbox(t)
	makeSkill(t, filepath.Join(s.home, ".claude", "skills", "oldskill"), "oldskill")
	s.yes("init")
	srcOrg := filepath.Join(s.root, "src-org")
	srcUser := filepath.Join(s.root, "src-user")
	makeSkill(t, filepath.Join(srcOrg, "skills", "alpha"), "alpha")
	makeSkill(t, filepath.Join(srcOrg, "skills", "beta"), "beta")
	makeSkill(t, filepath.Join(srcUser, "beta"), "beta")
	makeSkill(t, filepath.Join(srcUser, "gamma"), "gamma")
	if _, e, c := s.yes("add", srcOrg); c != 0 {
		t.Fatal(e)
	}
	_, errb, code := s.yes("add", srcOrg)
	if code != 0 {
		t.Fatal(errb)
	}
	if !strings.Contains(errb, "all skills from this source are already installed") {
		t.Fatalf("re-add: %s", errb)
	}
	if strings.Contains(errb, "excluded") {
		t.Fatalf("re-add excluded: %s", errb)
	}
	makeSkill(t, filepath.Join(srcOrg, "skills", "delta"), "delta")
	if _, e, c := s.yes("add", srcOrg); c != 0 {
		t.Fatal(e)
	}
	if !isLinkOrView(filepath.Join(s.sync, "store", "delta")) {
		t.Fatal("new skill after re-add")
	}
	if _, e, c := s.yes("add", srcUser); c != 0 {
		t.Fatal(e)
	}
	store := filepath.Join(s.sync, "store")
	for _, n := range []string{"alpha", "gamma"} {
		if !isLinkOrView(filepath.Join(store, n)) {
			t.Fatalf("%s not linked", n)
		}
	}
	tgt, err := filepath.EvalSymlinks(filepath.Join(store, "beta"))
	if err != nil || !strings.Contains(tgt, "src-org") {
		t.Fatalf("beta occupant %s %v", tgt, err)
	}
	if _, err := os.Stat(filepath.Join(s.sync, "sources", "src-org")); !os.IsNotExist(err) && err == nil {
		t.Fatal("path add copied into sources")
	}
	if !strings.Contains(tgt, "src-org") {
		t.Fatal("not a pointer")
	}
	_, errb, _ = s.yes("add", srcUser)
	if !strings.Contains(errb, "already installed") {
		t.Fatalf("want conflict warn: %s", errb)
	}
	_, errb, code = s.run("add", srcUser)
	if code == 0 || !strings.Contains(errb, "non-interactive add requires --yes") {
		t.Fatalf("code %d %s", code, errb)
	}
	_, errb, _ = s.yes("sync")
	if strings.Contains(errb, "linking") || strings.Contains(errb, "updating") {
		t.Fatalf("sync relinked: %s", errb)
	}
	out, _, _ := s.run("list")
	for _, n := range []string{"alpha", "beta", "oldskill"} {
		if !strings.Contains(out, n) {
			t.Fatalf("list missing %s: %s", n, out)
		}
	}
	if strings.Contains(out, "test skill") || strings.Contains(out, "skillsync list") {
		t.Fatalf("piped list should be names: %s", out)
	}
	out, _, _ = s.run("list", "--pretty")
	if !strings.Contains(out, "alpha") || !strings.Contains(out, "src-org") || !strings.Contains(out, "src-user") {
		t.Fatalf("pretty: %s", out)
	}
	if !strings.Contains(out, "local") || !strings.Contains(out, "test skill alpha") {
		t.Fatalf("pretty groups: %s", out)
	}
	if strings.Contains(out, "skill(s)") || strings.Contains(out, "skillsync list") {
		t.Fatal("tree chrome in pretty")
	}
	if strings.Contains(out, "  org") || strings.Contains(out, "\n  user\n") {
		t.Fatal("layer labels")
	}
	out, _, _ = s.run("list", "--names")
	if !strings.Contains(out, "alpha") || strings.Contains(out, "test skill") {
		t.Fatalf("names: %s", out)
	}

	srcDeep := filepath.Join(s.root, "src-deep")
	makeSkill(t, filepath.Join(srcDeep, "skills", "engineering", "verify"), "verify")
	if _, e, c := s.yes("add", srcDeep); c != 0 {
		t.Fatal(e)
	}
	if !isLinkOrView(filepath.Join(store, "verify")) {
		t.Fatal("nested category skill not linked")
	}
	empty := filepath.Join(s.root, "src-empty")
	if err := os.MkdirAll(empty, 0o755); err != nil {
		t.Fatal(err)
	}
	_, errb, code = s.yes("add", empty)
	if code != 0 {
		t.Fatal(errb)
	}
	if !strings.Contains(errb, "no skills found in source (only SKILL.md in root skill folders, skills/<name>, or skills/<category>/<name>)") {
		t.Fatalf("empty add: %s", errb)
	}
	out, _, _ = s.run("list", "--pretty")
	if !strings.Contains(out, "engineering") || !strings.Contains(out, "verify") {
		t.Fatalf("pretty categories: %s", out)
	}

	out, _, _ = s.yes("status")
	if strings.Contains(out, "test skill alpha") {
		t.Fatalf("status should not reprint the catalog: %s", out)
	}
	if !strings.Contains(out, "skills") || !strings.Contains(out, "from") {
		t.Fatalf("status missing skill counts: %s", out)
	}
	if !strings.Contains(out, "Sources") || !strings.Contains(out, "path") {
		t.Fatalf("status missing source health: %s", out)
	}
	if !strings.Contains(out, "src-org") || !strings.Contains(out, "src-user") {
		t.Fatalf("status missing source names: %s", out)
	}
	if !strings.Contains(out, "Agent views") {
		t.Fatalf("status missing views: %s", out)
	}
}

func TestListInvocation(t *testing.T) {
	s := newSandbox(t)
	src := filepath.Join(s.root, "src-inv")
	makeSkill(t, filepath.Join(src, "plain"), "plain")
	user := filepath.Join(src, "only-me")
	if err := os.MkdirAll(user, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: only-me\ndescription: user only\ndisable-model-invocation: true\n---\n"
	if err := os.WriteFile(filepath.Join(user, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	s.yes("init")
	if _, e, c := s.yes("add", src); c != 0 {
		t.Fatal(e)
	}
	out, _, _ := s.run("list", "--pretty")
	if !strings.Contains(out, "plain") || !strings.Contains(out, "only-me") || !strings.Contains(out, skill.UserFlag) {
		t.Fatalf("want [user]: %s", out)
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "plain") && strings.Contains(line, skill.UserFlag) {
			t.Fatalf("plain marked user: %s", line)
		}
	}
	out, _, _ = s.run("list", "--names")
	if strings.Contains(out, skill.UserFlag) || strings.Contains(out, "user only") {
		t.Fatalf("names should stay names: %s", out)
	}
}

func TestRemoveExcludeAndSource(t *testing.T) {
	s := newSandbox(t)
	makeSkill(t, filepath.Join(s.home, ".claude", "skills", "oldskill"), "oldskill")
	srcOrg := filepath.Join(s.root, "src-org")
	srcUser := filepath.Join(s.root, "src-user")
	makeSkill(t, filepath.Join(srcOrg, "skills", "alpha"), "alpha")
	makeSkill(t, filepath.Join(srcOrg, "skills", "beta"), "beta")
	makeSkill(t, filepath.Join(srcUser, "beta"), "beta")
	makeSkill(t, filepath.Join(srcUser, "gamma"), "gamma")
	s.yes("init")
	s.yes("add", srcOrg)
	s.yes("add", srcUser)
	s.yes("remove", "beta")
	if _, err := os.Lstat(filepath.Join(s.sync, "store", "beta")); !os.IsNotExist(err) {
		t.Fatal("beta still in store")
	}
	if _, err := os.Stat(filepath.Join(s.home, ".claude", "skills", "beta")); !os.IsNotExist(err) {
		t.Fatal("beta still in view")
	}
	rc, _ := os.ReadFile(filepath.Join(s.sync, "skillsyncrc"))
	if !strings.Contains(string(rc), "beta") {
		t.Fatalf("exclude missing: %s", rc)
	}
	s.yes("sync")
	if _, err := os.Lstat(filepath.Join(s.sync, "store", "beta")); !os.IsNotExist(err) {
		t.Fatal("sync resurrected beta")
	}
	s.yes("add", srcUser)
	if !isLinkOrView(filepath.Join(s.sync, "store", "beta")) {
		t.Fatal("re-add did not restore")
	}
	s.run("remove", "alpha", "--dry-run")
	if _, err := os.Lstat(filepath.Join(s.sync, "store", "alpha")); err != nil {
		t.Fatal("dry-run removed alpha")
	}
	s.yes("remove", "--source", srcOrg)
	if _, err := os.Lstat(filepath.Join(s.sync, "store", "alpha")); !os.IsNotExist(err) {
		t.Fatal("alpha still present")
	}
	if _, err := os.Stat(srcOrg); err != nil {
		t.Fatal("local folder deleted")
	}
	rc, _ = os.ReadFile(filepath.Join(s.sync, "skillsyncrc"))
	if strings.Contains(string(rc), "src-org") {
		t.Fatalf("manifest still has src-org: %s", rc)
	}
}

func TestUnmanagedDoctorUninstall(t *testing.T) {
	s := newSandbox(t)
	makeSkill(t, filepath.Join(s.home, ".claude", "skills", "oldskill"), "oldskill")
	srcUser := filepath.Join(s.root, "src-user")
	makeSkill(t, filepath.Join(srcUser, "beta"), "beta")
	s.yes("init")
	s.yes("add", srcUser)
	rogue := filepath.Join(s.sync, "store", "rogue")
	if err := os.MkdirAll(rogue, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(rogue, "file"), []byte("x\n"), 0o644)
	s.yes("sync")
	if _, err := os.Stat(filepath.Join(rogue, "file")); err != nil {
		t.Fatal("unmanaged deleted")
	}
	s.yes("remove", "rogue")
	if _, err := os.Stat(rogue); err != nil {
		t.Fatal("remove touched unmanaged")
	}
	os.RemoveAll(rogue)
	if _, e, c := s.yes("doctor"); c != 0 {
		t.Fatalf("doctor clean: %s", e)
	}
	os.Symlink("/nonexistent-target", filepath.Join(s.sync, "store", "deadlink"))
	if _, _, c := s.yes("doctor"); c == 0 {
		t.Fatal("doctor should fail on broken link")
	}
	os.Remove(filepath.Join(s.sync, "store", "deadlink"))
	os.Remove(filepath.Join(s.home, ".claude", "skills"))
	os.MkdirAll(filepath.Join(s.home, ".claude", "skills"), 0o755)
	if _, _, c := s.yes("doctor"); c == 0 {
		t.Fatal("doctor should fail on drift")
	}
	s.yes("init")
	if !isView(t, filepath.Join(s.home, ".claude", "skills"), filepath.Join(s.sync, "store")) {
		t.Fatal("init did not heal")
	}
	t.Setenv("FAKE_CODEX_HOME", filepath.Join(s.root, "custom-codex"))
	os.MkdirAll(filepath.Join(s.root, "custom-codex"), 0o755)
	out, _, _ := s.yes("status")
	if !strings.Contains(out, "custom-codex") {
		t.Fatalf("env override: %s", out)
	}
	custom := filepath.Join(s.home, ".custom")
	os.MkdirAll(custom, 0o755)
	rcPath := filepath.Join(s.sync, "skillsyncrc")
	rc, _ := os.ReadFile(rcPath)
	extra := string(rc) + "\n[[agents]]\nid = \"my-agent\"\ndisplay_name = \"My Agent\"\nglobal_path = \"" + filepath.ToSlash(filepath.Join(s.home, ".custom/skills")) + "\"\nproject_path = \".custom/skills\"\n"
	os.WriteFile(rcPath, []byte(extra), 0o644)
	s.yes("init")
	if !isView(t, filepath.Join(s.home, ".custom", "skills"), filepath.Join(s.sync, "store")) {
		t.Fatal("local agent not linked")
	}
	s.yes("uninstall", "--keep")
	st, err := os.Lstat(filepath.Join(s.home, ".claude", "skills"))
	if err != nil || st.Mode()&os.ModeSymlink != 0 {
		t.Fatal("keep should be real dir")
	}
	if _, err := os.Stat(filepath.Join(s.home, ".claude", "skills", "beta", "SKILL.md")); err != nil {
		t.Fatal("keep copies missing")
	}
	s.yes("init")
	s.yes("uninstall")
	if _, err := os.Stat(filepath.Join(s.home, ".claude", "skills")); !os.IsNotExist(err) {
		t.Fatal("views not removed")
	}
	if _, err := os.Stat(filepath.Join(s.sync, "store")); err != nil {
		t.Fatal("store should remain")
	}
	s.yes("uninstall", "--purge")
	if _, err := os.Stat(s.sync); !os.IsNotExist(err) {
		t.Fatal("purge left data")
	}
	if _, err := os.Stat(srcUser); err != nil {
		t.Fatal("outside source deleted")
	}
}

func TestSecurityAndFlags(t *testing.T) {
	s := newSandbox(t)
	s.yes("init")
	evil := filepath.Join(s.root, "src-evil", "skills", "safebasename")
	os.MkdirAll(evil, 0o755)
	os.WriteFile(filepath.Join(evil, "SKILL.md"), []byte("---\nname: ../../../evil\ndescription: escape\n---\nbody\n"), 0o644)
	s.yes("add", filepath.Join(s.root, "src-evil"))
	s.yes("sync")
	if !isLinkOrView(filepath.Join(s.sync, "store", "safebasename")) {
		t.Fatal("safe basename not used")
	}
	if _, err := os.Stat(filepath.Join(s.sync, "store", "evil")); !os.IsNotExist(err) {
		t.Fatal("evil store entry")
	}
	ents, _ := os.ReadDir(filepath.Join(s.sync, "store"))
	for _, e := range ents {
		if strings.Contains(e.Name(), "..") {
			t.Fatal("dot-dot name")
		}
	}
	os.MkdirAll(filepath.Join(s.root, "outside-decoy"), 0o755)
	os.WriteFile(filepath.Join(s.root, "outside-decoy", "marker"), []byte("x"), 0o644)
	_, errb, _ := s.run("remove", "../../../evil")
	if !strings.Contains(errb, "unknown skill") || !strings.Contains(errb, "Usage:") {
		t.Fatalf("%s", errb)
	}
	if _, err := os.Stat(filepath.Join(s.root, "outside-decoy", "marker")); err != nil {
		t.Fatal("decoy")
	}
	dot := filepath.Join(s.root, "src-dot", "skills", "with.dot")
	os.MkdirAll(dot, 0o755)
	os.WriteFile(filepath.Join(dot, "SKILL.md"), []byte("---\nname: with.dot\ndescription: d\n---\n"), 0o644)
	s.yes("add", filepath.Join(s.root, "src-dot"))
	if _, err := os.Stat(filepath.Join(s.sync, "store", "with.dot")); !os.IsNotExist(err) {
		t.Fatal("dotted name")
	}
	under := filepath.Join(s.root, "src-under", "skills", "with_under")
	os.MkdirAll(under, 0o755)
	os.WriteFile(filepath.Join(under, "SKILL.md"), []byte("---\nname: with_under\ndescription: u\n---\n"), 0o644)
	s.yes("add", filepath.Join(s.root, "src-under"))
	if _, err := os.Stat(filepath.Join(s.sync, "store", "with_under")); !os.IsNotExist(err) {
		t.Fatal("underscore name")
	}
	upper := filepath.Join(s.root, "src-upper", "skills", "CodeReview")
	os.MkdirAll(upper, 0o755)
	os.WriteFile(filepath.Join(upper, "SKILL.md"), []byte("---\nname: CodeReview\ndescription: u\n---\n"), 0o644)
	s.yes("add", filepath.Join(s.root, "src-upper"))
	ents, _ = os.ReadDir(filepath.Join(s.sync, "store"))
	for _, e := range ents {
		if e.Name() == "CodeReview" {
			t.Fatal("uppercase name")
		}
	}
	makeSkill(t, filepath.Join(s.root, "src-kebab", "skills", "code-review"), "code-review")
	s.yes("add", filepath.Join(s.root, "src-kebab"))
	if !isLinkOrView(filepath.Join(s.sync, "store", "code-review")) {
		t.Fatal("kebab not linked")
	}
	_, errb, code := s.run("remove")
	if code == 0 || !strings.Contains(errb, "specify skill names or use --all") {
		t.Fatalf("%d %s", code, errb)
	}
	s.yes("remove", "--all")
	if _, err := os.Lstat(filepath.Join(s.sync, "store", "safebasename")); !os.IsNotExist(err) {
		t.Fatal("remove --all")
	}
	rc, _ := os.ReadFile(filepath.Join(s.sync, "skillsyncrc"))
	if !strings.Contains(string(rc), "safebasename") {
		t.Fatalf("exclude: %s", rc)
	}
	makeSkill(t, filepath.Join(s.root, "src-user", "postflag"), "postflag")
	s.yes("add", filepath.Join(s.root, "src-user"))
	s.run("remove", "postflag", "--dry-run")
	if _, err := os.Lstat(filepath.Join(s.sync, "store", "postflag")); err != nil {
		t.Fatal("dry-run after command")
	}
	s.run("remove", "postflag", "--yes")
	if _, err := os.Lstat(filepath.Join(s.sync, "store", "postflag")); !os.IsNotExist(err) {
		t.Fatal("--yes after command")
	}
	_, errb, code = s.yes("add", "https://github.com/foo/../../../tmp-evil")
	if code == 0 || !strings.Contains(errb, "unsafe path") {
		t.Fatalf("%d %s", code, errb)
	}
}

func TestNonInteractiveInitRequiresYes(t *testing.T) {
	s := newSandbox(t)
	makeSkill(t, filepath.Join(s.home, ".claude", "skills", "linkme"), "linkme")
	_, errb, code := s.run("init")
	if code == 0 || !strings.Contains(errb, "non-interactive init requires --yes") {
		t.Fatalf("init %d %s", code, errb)
	}
	_, errb, code = s.yes("init")
	if code != 0 || !strings.Contains(errb, "init done") {
		t.Fatalf("yes init: %s", errb)
	}
	if !isView(t, filepath.Join(s.home, ".claude", "skills"), filepath.Join(s.sync, "store")) {
		t.Fatal("init with --yes did not link")
	}
}

func TestOutputHygiene(t *testing.T) {
	s := newSandbox(t)
	_, errb, _ := s.yes("init")
	if strings.Contains(errb, "\033") {
		t.Fatal("ansi in non-tty")
	}
	out, _, code := s.run("completion", "bash")
	if code != 0 || !strings.Contains(out, "_skillsync") {
		t.Fatalf("completion: %s", out)
	}
}

func TestCopyMode(t *testing.T) {
	s := newSandbox(t)
	src := filepath.Join(s.root, "src")
	makeSkill(t, filepath.Join(src, "skills", "copied"), "copied")
	s.yes("init")
	if _, e, c := s.yes("--copy", "add", src); c != 0 {
		t.Fatal(e)
	}
	st, err := os.Lstat(filepath.Join(s.sync, "store", "copied"))
	if err != nil || st.Mode()&os.ModeSymlink != 0 {
		t.Fatal("copy should not be symlink")
	}
	if _, err := os.Stat(filepath.Join(s.sync, "store", "copied", "SKILL.md")); err != nil {
		t.Fatal("copy missing files")
	}
}

func writeManifest(t *testing.T, repo, body string) {
	t.Helper()
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "skillsync.toml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(wd) })
}

func TestApplyProjectLinksAndGitignore(t *testing.T) {
	s := newSandbox(t)
	repo := filepath.Join(s.root, "repo")
	makeSkill(t, filepath.Join(repo, "vendor-skills", "foo"), "foo")
	makeSkill(t, filepath.Join(repo, "vendor-skills", "bar"), "bar")
	writeManifest(t, repo, `[skills]
sources = [{ url = "./vendor-skills", skills = ["foo", "bar"] }]
[views]
ids = ["fake-claude"]
`)
	chdir(t, repo)
	if _, e, c := s.yes("apply"); c != 0 {
		t.Fatalf("apply: %s", e)
	}
	for _, view := range []string{
		filepath.Join(repo, ".agents", "skills"),
		filepath.Join(repo, ".claude", "skills"),
	} {
		for _, n := range []string{"foo", "bar"} {
			p := filepath.Join(view, n)
			fi, err := os.Lstat(p)
			if err != nil || fi.Mode()&os.ModeSymlink == 0 {
				t.Fatalf("%s not a symlink: %v", p, err)
			}
		}
		gi, err := os.ReadFile(filepath.Join(view, ".gitignore"))
		if err != nil || !strings.Contains(string(gi), "foo") {
			t.Fatalf("gitignore %s: %s %v", view, gi, err)
		}
	}
}

func TestApplyCollisionHardFail(t *testing.T) {
	s := newSandbox(t)
	repo := filepath.Join(s.root, "repo")
	makeSkill(t, filepath.Join(repo, "vendor-skills", "foo"), "foo")
	makeSkill(t, filepath.Join(repo, ".agents", "skills", "foo"), "foo")
	writeManifest(t, repo, `[skills]
sources = [{ url = "./vendor-skills", skills = ["foo"] }]
`)
	chdir(t, repo)
	_, e, c := s.yes("apply")
	if c == 0 || !strings.Contains(e, "collision") {
		t.Fatalf("want collision, code %d err %s", c, e)
	}
}

func TestApplyUnknownViewID(t *testing.T) {
	s := newSandbox(t)
	repo := filepath.Join(s.root, "repo")
	makeSkill(t, filepath.Join(repo, "vendor-skills", "foo"), "foo")
	writeManifest(t, repo, `[skills]
sources = [{ url = "./vendor-skills", skills = ["foo"] }]
[views]
ids = ["not-an-agent"]
`)
	chdir(t, repo)
	_, e, c := s.yes("apply")
	if c == 0 || !strings.Contains(e, "unknown view id") {
		t.Fatalf("code %d err %s", c, e)
	}
}

func TestApplyPathDedupeAndUnapply(t *testing.T) {
	s := newSandbox(t)
	repo := filepath.Join(s.root, "repo")
	makeSkill(t, filepath.Join(repo, "vendor-skills", "foo"), "foo")
	writeManifest(t, repo, `[skills]
sources = [{ url = "./vendor-skills", skills = ["*"] }]
[views]
ids = ["fake-codex"]
`)
	chdir(t, repo)
	if _, e, c := s.yes("apply"); c != 0 {
		t.Fatalf("apply: %s", e)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".agents", "skills", "foo")); err != nil {
		t.Fatal(err)
	}
	if _, e, c := s.yes("unapply", "foo"); c != 0 {
		t.Fatalf("unapply: %s", e)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".agents", "skills", "foo")); !os.IsNotExist(err) {
		t.Fatal("link should be gone")
	}
}

func TestApplyPruneAndGlobal(t *testing.T) {
	s := newSandbox(t)
	s.yes("init")
	repo := filepath.Join(s.root, "repo")
	makeSkill(t, filepath.Join(repo, "vendor-skills", "foo"), "foo")
	makeSkill(t, filepath.Join(repo, "vendor-skills", "bar"), "bar")
	writeManifest(t, repo, `[skills]
sources = [{ url = "./vendor-skills", skills = ["foo", "bar"] }]
`)
	chdir(t, repo)
	if _, e, c := s.yes("apply"); c != 0 {
		t.Fatalf("apply: %s", e)
	}
	writeManifest(t, repo, `[skills]
sources = [{ url = "./vendor-skills", skills = ["foo"] }]
`)
	if _, e, c := s.yes("apply", "--prune"); c != 0 {
		t.Fatalf("prune: %s", e)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".agents", "skills", "bar")); !os.IsNotExist(err) {
		t.Fatal("bar should be pruned")
	}
	if _, e, c := s.yes("apply", "--global"); c != 0 {
		t.Fatalf("global: %s", e)
	}
	if _, err := os.Lstat(filepath.Join(s.sync, "store", "foo")); err != nil {
		t.Fatal("home store missing foo")
	}
}

func TestApplyDryRun(t *testing.T) {
	s := newSandbox(t)
	repo := filepath.Join(s.root, "repo")
	makeSkill(t, filepath.Join(repo, "vendor-skills", "foo"), "foo")
	writeManifest(t, repo, `[skills]
sources = [{ url = "./vendor-skills", skills = ["foo"] }]
`)
	chdir(t, repo)
	if _, e, c := s.run("--dry-run", "--yes", "apply"); c != 0 {
		t.Fatalf("%s", e)
	}
	if _, err := os.Lstat(filepath.Join(repo, ".agents", "skills", "foo")); !os.IsNotExist(err) {
		t.Fatal("dry-run wrote a link")
	}
}
