package skill

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// FindHint describes the only layouts FindInSource accepts.
const FindHint = "only SKILL.md in root skill folders, skills/<name>, or skills/<category>/<name>"

var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func IsSafeName(s string) bool {
	return s != "" && len(s) <= 64 && nameRe.MatchString(s)
}

type Found struct {
	Dir      string
	Name     string
	Category string
}

func NameFromDir(dir string) string {
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	name := ""
	if err == nil {
		name = frontmatterName(b)
	}
	if IsSafeName(name) {
		return name
	}
	bn := filepath.Base(dir)
	if IsSafeName(bn) {
		return bn
	}
	return ""
}

func frontmatterName(b []byte) string {
	fm := extractFM(b)
	if fm == nil {
		return ""
	}
	if n, ok := fm["name"].(string); ok {
		return strings.TrimSpace(strings.Trim(n, "\"'"))
	}
	return ""
}

func Blurb(dir string) string {
	fm := readFM(dir)
	if fm == nil {
		return ""
	}
	d, ok := fm["description"]
	if !ok {
		return ""
	}
	s := strings.TrimSpace(stringify(d))
	s = strings.ReplaceAll(s, "\t", " ")
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	return s
}

// UserFlag marks skills with disable-model-invocation: true.
const UserFlag = "[user]"

// Invocation is UserFlag when SKILL.md has disable-model-invocation: true, else "".
func Invocation(dir string) string {
	fm := readFM(dir)
	if fm == nil {
		return ""
	}
	v, ok := fm["disable-model-invocation"]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case bool:
		if t {
			return UserFlag
		}
	default:
		if strings.EqualFold(stringify(t), "true") {
			return UserFlag
		}
	}
	return ""
}

func readFM(dir string) map[string]any {
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil
	}
	return extractFM(b)
}

func Shorten(s string, fancy bool) string {
	if s == "" {
		return ""
	}
	r := []rune(s)
	if len(r) <= 72 {
		return s
	}
	if fancy {
		return string(r[:71]) + "…"
	}
	return string(r[:71]) + "..."
}

func stringify(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return strings.TrimSpace(strings.Trim(fmtSprint(t), "\"'"))
	}
}

func fmtSprint(v any) string {
	b, err := yaml.Marshal(v)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func extractFM(b []byte) map[string]any {
	s := string(b)
	s = strings.TrimPrefix(s, "\uFEFF")
	if !strings.HasPrefix(s, "---") {
		return nil
	}
	rest := s[3:]
	if strings.HasPrefix(rest, "\r\n") {
		rest = rest[2:]
	} else if strings.HasPrefix(rest, "\n") {
		rest = rest[1:]
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		end = strings.Index(rest, "\r\n---")
	}
	if end < 0 {
		return nil
	}
	block := rest[:end]
	m := map[string]any{}
	if err := yaml.Unmarshal([]byte(block), &m); err != nil {
		return nil
	}
	return m
}

func skipDotDir(name string) bool {
	return strings.HasPrefix(name, ".")
}

func hasSkillMD(dir string) bool {
	st, err := os.Stat(filepath.Join(dir, "SKILL.md"))
	return err == nil && !st.IsDir()
}

func listSubdirs(path string) []string {
	ents, err := os.ReadDir(path)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range ents {
		if !e.IsDir() || skipDotDir(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names
}

// Category is the skills/<category>/<name> folder, or empty for root and skills/<name> layouts.
func Category(sourceRoot, skillDir string) string {
	rel, err := filepath.Rel(sourceRoot, skillDir)
	if err != nil {
		return ""
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 3 && parts[0] == "skills" && parts[1] != "" && parts[2] != "." {
		return parts[1]
	}
	return ""
}

func FindInSource(root string) []Found {
	var out []Found
	seen := map[string]struct{}{}
	add := func(dir, category string) {
		if !hasSkillMD(dir) {
			return
		}
		n := NameFromDir(dir)
		if n == "" {
			return
		}
		if _, ok := seen[n]; ok {
			return
		}
		seen[n] = struct{}{}
		out = append(out, Found{Dir: dir, Name: n, Category: category})
	}

	add(root, "")
	for _, name := range listSubdirs(root) {
		if name == "skills" {
			continue
		}
		add(filepath.Join(root, name), "")
	}

	skillsDir := filepath.Join(root, "skills")
	st, err := os.Stat(skillsDir)
	if err != nil || !st.IsDir() {
		return out
	}
	for _, name := range listSubdirs(skillsDir) {
		d := filepath.Join(skillsDir, name)
		if hasSkillMD(d) {
			add(d, "")
			continue
		}
		for _, nested := range listSubdirs(d) {
			add(filepath.Join(d, nested), name)
		}
	}
	return out
}
