package agentregistry

import (
	_ "embed"
	"os"
	"strings"

	"github.com/formenosland/skillsync/internal/config"
	"github.com/formenosland/skillsync/internal/paths"
)

//go:generate go run ./gen

//go:embed agents.tsv
var shippedTSV string

type Row struct {
	ID, DisplayName, GlobalPath, ProjectPath string
}

func Load(cfg config.File) []Row {
	raw := shippedTSV
	if p := strings.TrimSpace(os.Getenv("SKILLSYNC_REGISTRY")); p != "" {
		b, err := os.ReadFile(p)
		if err == nil {
			raw = string(b)
		}
	}
	rows := parseTSV(raw)
	ov := cfg.AgentMap()
	seen := map[string]struct{}{}
	var out []Row
	for _, r := range rows {
		if a, ok := ov[r.ID]; ok {
			out = append(out, rowFromAgent(a))
		} else {
			out = append(out, r)
		}
		seen[r.ID] = struct{}{}
	}
	for _, a := range cfg.Agents {
		if a.ID == "" {
			continue
		}
		if _, ok := seen[a.ID]; ok {
			continue
		}
		out = append(out, rowFromAgent(a))
		seen[a.ID] = struct{}{}
	}
	return out
}

func rowFromAgent(a config.Agent) Row {
	return Row{ID: a.ID, DisplayName: a.DisplayName, GlobalPath: a.GlobalPath, ProjectPath: a.ProjectPath}
}

func parseTSV(raw string) []Row {
	var out []Row
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 4 {
			continue
		}
		if parts[0] == "agent_id" {
			continue
		}
		out = append(out, Row{
			ID:          parts[0],
			DisplayName: parts[1],
			GlobalPath:  parts[2],
			ProjectPath: parts[3],
		})
	}
	return out
}

type View struct {
	Path string
	IDs  []string
}

func Views(rows []Row) []View {
	order := []string{}
	ids := map[string][]string{}
	for _, r := range rows {
		if r.GlobalPath == "-" {
			continue
		}
		p := paths.ExpandAgent(r.GlobalPath)
		if _, ok := ids[p]; !ok {
			order = append(order, p)
		}
		ids[p] = append(ids[p], r.ID)
	}
	var out []View
	for _, p := range order {
		out = append(out, View{Path: p, IDs: ids[p]})
	}
	return out
}

func IDList(ids []string) string {
	return strings.Join(ids, " ")
}
