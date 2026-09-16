package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// File is the TOML document stored as skillsyncrc.
type File struct {
	Sources  []string `toml:"sources,omitempty"`
	Excludes []string `toml:"excludes,omitempty"`
	Agents   []Agent  `toml:"agents,omitempty"`
}

// Agent is a user override or extra registry row (wins by id).
type Agent struct {
	ID          string `toml:"id"`
	DisplayName string `toml:"display_name"`
	GlobalPath  string `toml:"global_path"`
	ProjectPath string `toml:"project_path"`
}

func Load(path string) (File, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return File{}, nil
		}
		return File{}, err
	}
	var f File
	if err := toml.Unmarshal(b, &f); err != nil {
		return File{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return f, nil
}

func Save(path string, f File) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := toml.Marshal(f)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if !bytes.HasSuffix(b, []byte("\n")) {
		b = append(b, '\n')
	}
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (f *File) HasSource(src string) bool {
	return slices.Contains(f.Sources, src)
}

func (f *File) AppendSource(src string) {
	if src == "" || f.HasSource(src) {
		return
	}
	f.Sources = append(f.Sources, src)
}

func (f *File) DropSource(src string) bool {
	n := slices.DeleteFunc(f.Sources, func(s string) bool { return s == src })
	if len(n) == len(f.Sources) {
		return false
	}
	f.Sources = n
	return true
}

func (f *File) IsExcluded(name string) bool {
	return slices.Contains(f.Excludes, name)
}

func (f *File) ExcludeAdd(name string) {
	if name == "" || f.IsExcluded(name) {
		return
	}
	f.Excludes = append(f.Excludes, name)
}

func (f *File) ExcludeDel(name string) {
	f.Excludes = slices.DeleteFunc(f.Excludes, func(s string) bool { return s == name })
}

func (f File) AgentMap() map[string]Agent {
	m := make(map[string]Agent, len(f.Agents))
	for _, a := range f.Agents {
		id := strings.TrimSpace(a.ID)
		if id == "" {
			continue
		}
		m[id] = a
	}
	return m
}
