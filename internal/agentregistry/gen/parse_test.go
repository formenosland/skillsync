package main

import "testing"

const fixture = `
export const agents: Record<AgentType, AgentConfig> = {
  'aider-desk': {
    name: 'aider-desk',
    displayName: 'AiderDesk',
    skillsDir: '.aider-desk/skills',
    globalSkillsDir: join(home, '.aider-desk/skills'),
  },
  amp: {
    name: 'amp',
    displayName: 'Amp',
    skillsDir: '.agents/skills',
    globalSkillsDir: join(configHome, 'agents/skills'),
  },
  eve: {
    name: 'eve',
    displayName: 'Eve',
    skillsDir: 'agent/skills',
    globalSkillsDir: undefined,
  },
  openclaw: {
    name: 'openclaw',
    displayName: 'OpenClaw',
    skillsDir: 'skills',
    globalSkillsDir: getOpenClawGlobalSkillsDir(),
  },
  'autohand-code': {
    name: 'autohand-code',
    displayName: 'Autohand Code CLI',
    skillsDir: '.autohand/skills',
    globalSkillsDir: join(autohandHome, 'skills'),
  },
};
`

func TestParseAgentsTS(t *testing.T) {
	rows, err := parseAgentsTS(fixture)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]row{
		"aider-desk":    {id: "aider-desk", display: "AiderDesk", global: "~/.aider-desk/skills", project: ".aider-desk/skills"},
		"amp":           {id: "amp", display: "Amp", global: "${XDG_CONFIG_HOME:-~/.config}/agents/skills", project: ".agents/skills"},
		"eve":           {id: "eve", display: "Eve", global: "-", project: "agent/skills"},
		"openclaw":      {id: "openclaw", display: "OpenClaw", global: openClawGlobal, project: "skills"},
		"autohand-code": {id: "autohand-code", display: "Autohand Code CLI", global: "${AUTOHAND_HOME:-~/.autohand}/skills", project: ".autohand/skills"},
	}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows", len(rows))
	}
	for _, r := range rows {
		w, ok := want[r.id]
		if !ok {
			t.Fatalf("unexpected %s", r.id)
		}
		if r != w {
			t.Fatalf("%s: got %+v want %+v", r.id, r, w)
		}
	}
}

func TestParseJoinUnknownToken(t *testing.T) {
	src := `
  foo: {
    name: 'foo',
    displayName: 'Foo',
    skillsDir: '.foo/skills',
    globalSkillsDir: join(zedAppDataHome, 'skills'),
  },
`
	_, err := parseAgentsTS(src)
	if err == nil {
		t.Fatal("want unknown token error")
	}
}
