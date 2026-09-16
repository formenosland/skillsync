package agentregistry

import (
	"strings"
	"testing"

	"github.com/formenosland/skillsync/internal/config"
)

func TestShippedRegistryHasRows(t *testing.T) {
	rows := Load(config.File{})
	if len(rows) < 50 {
		t.Fatalf("embedded registry too small: %d", len(rows))
	}
	var found bool
	for _, r := range rows {
		if r.ID == "claude-code" && strings.Contains(r.GlobalPath, "claude") {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("missing claude-code")
	}
}
