package skill

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

var nameRe = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)

func IsSafeName(s string) bool {
	return s != "" && len(s) <= 64 && nameRe.MatchString(s)
}

type Found struct {
	Dir  string
	Name string
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
	b, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return ""
	}
	fm := extractFM(b)
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

func FindInSource(root string) []Found {
	var out []Found
	seen := map[string]struct{}{}
	add := func(dir string) {
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err != nil {
			return
		}
		n := NameFromDir(dir)
		if n == "" {
			return
		}
		key := dir
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, Found{Dir: dir, Name: n})
	}
	skills := filepath.Join(root, "skills")
	if st, err := os.Stat(skills); err == nil && st.IsDir() {
		ents, _ := os.ReadDir(skills)
		for _, e := range ents {
			if !e.IsDir() {
				continue
			}
			add(filepath.Join(skills, e.Name()))
		}
	}
	ents, err := os.ReadDir(root)
	if err != nil {
		return out
	}
	for _, e := range ents {
		if !e.IsDir() {
			continue
		}
		if e.Name() == "skills" || e.Name() == ".git" {
			continue
		}
		d := filepath.Join(root, e.Name())
		if _, err := os.Stat(filepath.Join(d, "SKILL.md")); err == nil {
			add(d)
			continue
		}
		ents2, _ := os.ReadDir(d)
		for _, e2 := range ents2 {
			if !e2.IsDir() {
				continue
			}
			add(filepath.Join(d, e2.Name()))
		}
	}
	return out
}
