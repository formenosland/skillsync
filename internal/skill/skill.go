package skill

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// MaxFindDepth is how many directory levels below a source root we walk for SKILL.md.
const MaxFindDepth = 4

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

func skipFindDir(name string) bool {
	switch name {
	case ".git", "node_modules":
		return true
	}
	return strings.HasPrefix(name, ".")
}

func FindInSource(root string) []Found {
	var out []Found
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path != root && skipFindDir(d.Name()) {
			return filepath.SkipDir
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		depth := 0
		if rel != "." {
			depth = strings.Count(rel, string(filepath.Separator)) + 1
		}
		if depth > MaxFindDepth {
			return filepath.SkipDir
		}
		if _, err := os.Stat(filepath.Join(path, "SKILL.md")); err != nil {
			return nil
		}
		n := NameFromDir(path)
		if n != "" {
			out = append(out, Found{Dir: path, Name: n})
		}
		return filepath.SkipDir
	})
	return out
}
