package paths

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSKILLSYNCHome(t *testing.T) {
	t.Setenv("SKILLSYNC_HOME", "/tmp/ss-home")
	l := Resolve()
	if l.ConfigFile != filepath.Join("/tmp/ss-home", "skillsyncrc") {
		t.Fatalf("%s", l.ConfigFile)
	}
	if l.Store != filepath.Join("/tmp/ss-home", "store") {
		t.Fatalf("%s", l.Store)
	}
}

func TestXDGConfigFile(t *testing.T) {
	t.Setenv("SKILLSYNC_HOME", "")
	os.Unsetenv("SKILLSYNC_HOME")
	t.Setenv("XDG_CONFIG_HOME", "/cfg")
	t.Setenv("XDG_DATA_HOME", "/data")
	l := Resolve()
	if l.ConfigFile != filepath.Join("/cfg", "skillsyncrc") {
		t.Fatalf("config %s", l.ConfigFile)
	}
	if l.DataDir != filepath.Join("/data", "skillsync") {
		t.Fatalf("data %s", l.DataDir)
	}
}

func TestLooksLikeLocalPath(t *testing.T) {
	if !LooksLikeLocalPath("/tmp/foo") {
		t.Fatal("posix abs")
	}
	if !LooksLikeLocalPath(`C:\skills`) {
		t.Fatal("windows drive")
	}
	if !LooksLikeLocalPath("./vendor-skills") {
		t.Fatal("dot relative")
	}
	if LooksLikeLocalPath("acme/skills") {
		t.Fatal("github shorthand")
	}
}

func TestHasDotDot(t *testing.T) {
	if !HasDotDotSegment("github.com/foo/../../../tmp") {
		t.Fatal("expected dotdot")
	}
	if HasDotDotSegment("github.com/foo/bar") {
		t.Fatal("false positive")
	}
}
