package gitignore

import (
	"os"
	"strings"
)

const (
	Begin = "# skillsync-managed"
	End   = "# end skillsync-managed"
)

func Rewrite(path string, names []string) error {
	var body []byte
	if b, err := os.ReadFile(path); err == nil {
		body = b
	} else if !os.IsNotExist(err) {
		return err
	}
	next := splice(string(body), names)
	if next == string(body) {
		return nil
	}
	return os.WriteFile(path, []byte(next), 0o644)
}

func Names(path string) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	_, _, names, ok := extract(string(b))
	if !ok {
		return nil
	}
	return names
}

func splice(content string, names []string) string {
	block := blockFor(names)
	before, after, _, ok := extract(content)
	if !ok {
		if strings.TrimSpace(content) == "" {
			return block
		}
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		return content + "\n" + block
	}
	return before + block + after
}

func extract(content string) (before, after string, names []string, ok bool) {
	i := strings.Index(content, Begin)
	if i < 0 {
		return content, "", nil, false
	}
	rest := content[i:]
	j := strings.Index(rest, End)
	if j < 0 {
		return content, "", nil, false
	}
	inner := rest[len(Begin):j]
	afterEnd := j + len(End)
	if afterEnd < len(rest) && rest[afterEnd] == '\n' {
		afterEnd++
	}
	before = content[:i]
	after = rest[afterEnd:]
	for _, line := range strings.Split(inner, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		names = append(names, line)
	}
	return before, after, names, true
}

func blockFor(names []string) string {
	var b strings.Builder
	b.WriteString(Begin)
	b.WriteByte('\n')
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		b.WriteString(n)
		b.WriteByte('\n')
	}
	b.WriteString(End)
	b.WriteByte('\n')
	return b.String()
}
