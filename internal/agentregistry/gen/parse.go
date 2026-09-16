package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type row struct {
	id, display, global, project string
}

var prefixes = map[string]string{
	"home":         "~",
	"configHome":   "${XDG_CONFIG_HOME:-~/.config}",
	"codexHome":    "${CODEX_HOME:-~/.codex}",
	"claudeHome":   "${CLAUDE_CONFIG_DIR:-~/.claude}",
	"vibeHome":     "${VIBE_HOME:-~/.vibe}",
	"hermesHome":   "${HERMES_HOME:-~/.hermes}",
	"autohandHome": "${AUTOHAND_HOME:-~/.autohand}",
	"grokHome":     "${GROK_HOME:-~/.grok}",
}

const openClawGlobal = "~/.openclaw/skills|~/.clawdbot/skills|~/.moltbot/skills"

var (
	reName     = regexp.MustCompile(`^\s+name: '([^']+)'`)
	reDisplay  = regexp.MustCompile(`^\s+displayName: '([^']+)'`)
	reSkills   = regexp.MustCompile(`^\s+skillsDir: '([^']+)'`)
	reUndef    = regexp.MustCompile(`^\s+globalSkillsDir: undefined`)
	reOpenClaw = regexp.MustCompile(`^\s+globalSkillsDir: getOpenClawGlobalSkillsDir\(\)`)
	reJoin     = regexp.MustCompile(`^\s+globalSkillsDir: join\((.*)\)\s*,?\s*$`)
)

func parseAgentsTS(src string) ([]row, error) {
	var rows []row
	cur := row{global: "-", project: "-"}
	flush := func() {
		if cur.id == "" {
			return
		}
		rows = append(rows, cur)
		cur = row{global: "-", project: "-"}
	}
	for _, line := range strings.Split(src, "\n") {
		if m := reName.FindStringSubmatch(line); m != nil {
			flush()
			cur.id = m[1]
			continue
		}
		if m := reDisplay.FindStringSubmatch(line); m != nil {
			cur.display = m[1]
			continue
		}
		if m := reSkills.FindStringSubmatch(line); m != nil {
			cur.project = m[1]
			continue
		}
		if reUndef.MatchString(line) {
			cur.global = "-"
			continue
		}
		if reOpenClaw.MatchString(line) {
			cur.global = openClawGlobal
			continue
		}
		if m := reJoin.FindStringSubmatch(line); m != nil {
			p, err := parseJoin(m[1])
			if err != nil {
				return nil, err
			}
			cur.global = p
		}
	}
	flush()
	if len(rows) == 0 {
		return nil, fmt.Errorf("no agents parsed (upstream format changed?)")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	for _, r := range rows {
		if strings.Contains(r.global, "UNKNOWN(") {
			return nil, fmt.Errorf("unrecognized path token in %s: %s (extend prefixes in parse.go)", r.id, r.global)
		}
	}
	return rows, nil
}

func parseJoin(inner string) (string, error) {
	inner = strings.TrimSpace(inner)
	parts := splitJoinArgs(inner)
	if len(parts) == 0 {
		return "", fmt.Errorf("empty join()")
	}
	var out string
	for i, tok := range parts {
		tok = strings.TrimSpace(tok)
		tok = strings.Trim(tok, `"'`)
		if i == 0 && isIdent(tok) {
			if p, ok := prefixes[tok]; ok {
				out = p
			} else {
				out = "UNKNOWN(" + tok + ")"
			}
			continue
		}
		out = out + "/" + tok
	}
	return out, nil
}

func splitJoinArgs(s string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			parts = append(parts, s[start:i])
			start = i + 1
		}
	}
	parts = append(parts, s[start:])
	return parts
}

func isIdent(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r < 'A' || r > 'z' || (r > 'Z' && r < 'a') {
				return false
			}
			continue
		}
		if r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
			continue
		}
		return false
	}
	return true
}
