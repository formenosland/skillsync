package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/formenosland/skillsync/internal/paths"
	"github.com/pelletier/go-toml/v2"
)

const ManifestName = "skillsync.toml"

// Project is the committed repo manifest (skillsync.toml).
type Project struct {
	Skills ProjectSkills `toml:"skills"`
	Views  ProjectViews  `toml:"views"`
}

type ProjectSkills struct {
	Sources []ProjectSource `toml:"sources"`
}

type ProjectSource struct {
	URL    string   `toml:"url"`
	Ref    string   `toml:"ref,omitempty"`
	Skills []string `toml:"skills,omitempty"`
}

type ProjectViews struct {
	IDs []string `toml:"ids"`
}

func LoadProject(path string) (Project, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Project{}, err
	}
	var p Project
	dec := toml.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return Project{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := p.Validate(); err != nil {
		return Project{}, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}

func (p Project) Validate() error {
	seen := map[string]string{}
	for i, s := range p.Skills.Sources {
		u := strings.TrimSpace(s.URL)
		if u == "" {
			return fmt.Errorf("skills.sources[%d]: empty url", i)
		}
		if forbiddenAbsSource(u) {
			return fmt.Errorf("skills.sources[%d]: absolute path not allowed (use a git URL or a path relative to the repo)", i)
		}
		all := len(s.Skills) == 0
		for _, n := range s.Skills {
			n = strings.TrimSpace(n)
			if n == "" {
				continue
			}
			if n == "*" {
				all = true
				continue
			}
			if prev, ok := seen[n]; ok {
				return fmt.Errorf("skill %q listed by both %s and %s", n, prev, u)
			}
			seen[n] = u
		}
		if all {
			// "*" / omitted: uniqueness is checked after discovery
			_ = all
		}
	}
	return nil
}

func forbiddenAbsSource(u string) bool {
	if strings.HasPrefix(u, "~") {
		return true
	}
	if strings.HasPrefix(u, "./") || strings.HasPrefix(u, "../") {
		return false
	}
	return paths.LooksLikeLocalPath(u) && !strings.HasPrefix(u, ".")
}

// FindManifest walks from start toward filesystem root looking for skillsync.toml.
func FindManifest(start string) (repoRoot, manifest string, err error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", "", err
	}
	for {
		cand := filepath.Join(dir, ManifestName)
		st, err := os.Stat(cand)
		if err == nil && !st.IsDir() {
			return dir, cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", "", os.ErrNotExist
		}
		dir = parent
	}
}
