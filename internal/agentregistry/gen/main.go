// Command gen refreshes internal/agentregistry/agents.tsv from vercel-labs/skills src/agents.ts.
//
//	make agentregistry
//	make agentregistry SHA=<upstream-commit>
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const defaultSHA = "9230fe8d4879e4d9ba04e7a31147477ffbcc51ca"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	sha := defaultSHA
	if len(args) > 0 && strings.TrimSpace(args[0]) != "" {
		sha = strings.TrimSpace(args[0])
	}
	url := fmt.Sprintf("https://raw.githubusercontent.com/vercel-labs/skills/%s/src/agents.ts", sha)
	fmt.Fprintf(os.Stderr, "fetching agents.ts @ %s\n", sha)
	src, err := fetch(url)
	if err != nil {
		return err
	}
	rows, err := parseAgentsTS(src)
	if err != nil {
		return err
	}
	out, err := tsvPath()
	if err != nil {
		return err
	}
	body := renderTSV(sha, rows)
	if err := os.WriteFile(out, []byte(body), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s (%d agents)\n", out, len(rows))
	return nil
}

func fetch(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func tsvPath() (string, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("cannot resolve gen source path")
	}
	return filepath.Join(filepath.Dir(file), "..", "agents.tsv"), nil
}

func renderTSV(sha string, rows []row) string {
	var b strings.Builder
	b.WriteString(`# Agent skills path registry for skillsync — GENERATED FILE, do not edit by hand.
# Regenerate with: make agentregistry
#   or: go run ./internal/agentregistry/gen [commit-sha]
# Upstream: https://github.com/vercel-labs/skills src/agents.ts @ `)
	b.WriteString(sha)
	b.WriteString("\n# Generated: ")
	b.WriteString(time.Now().UTC().Format("2006-01-02"))
	b.WriteString(`
#
# Format: agent_id<TAB>display_name<TAB>global_path<TAB>project_path
#   - Paths may use ~ (home), ${VAR:-default} (env-overridable home dirs,
#     e.g. ${CODEX_HOME:-~/.codex}/skills), and | -separated alternates
#     (first alternate whose parent directory exists wins, else the first).
#   - global_path "-" means project-only agent: no global view is created.
#
# Local overrides: [[agents]] tables in $XDG_CONFIG_HOME/skillsyncrc
# (same fields); rows there win over this file by agent_id.
#
# Known out of scope: Perplexity Computer (skills live in a cloud library
# uploaded via its UI — no local filesystem folder to link).
agent_id	display_name	global_path	project_path
`)
	for _, r := range rows {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\n", r.id, r.display, r.global, r.project)
	}
	return b.String()
}
