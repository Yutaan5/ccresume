package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeDisplay_RemovesTerminalControls(t *testing.T) {
	input := "先頭\x1b]52;c;Y2xpcGJvYXJk\x07\n\t本文\x1b[2J\u0085末尾"
	got := SanitizeDisplay(input)
	want := "先頭\n\t本文末尾"
	if got != want {
		t.Fatalf("SanitizeDisplay() = %q, want %q", got, want)
	}
}

func TestValidID(t *testing.T) {
	if !ValidID("aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee") {
		t.Fatal("UUID-shaped session ID was rejected")
	}
	for _, id := range []string{
		"",
		"not-a-uuid",
		"foo; open -a Calculator;#",
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee-extra",
	} {
		if ValidID(id) {
			t.Errorf("ValidID(%q) = true", id)
		}
	}
}

func TestScanRoot_IgnoresNonUUIDJSONLNames(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, "-tmp-project")
	if err := os.Mkdir(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	validID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	for _, name := range []string{
		validID + ".jsonl",
		"foo; open -a Calculator;#.jsonl",
	} {
		content := `{"type":"user","cwd":"/tmp/project","message":{"role":"user","content":"test"}}` + "\n"
		if err := os.WriteFile(filepath.Join(projectDir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	projects, err := scanRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 {
		t.Fatalf("got %d projects", len(projects))
	}
	if len(projects[0].Sessions) != 1 {
		t.Fatalf("got %d sessions", len(projects[0].Sessions))
	}
	if got := projects[0].Sessions[0].ID; got != validID {
		t.Fatalf("session ID = %q", got)
	}
	if strings.Contains(projects[0].Sessions[0].ID, ";") {
		t.Fatal("command-like file name reached scan results")
	}
}

func TestProjectDisplayNameSanitizesCWD(t *testing.T) {
	p := &Project{CWD: "/tmp/project\x1b]0;spoofed-title\x07"}
	if got := p.DisplayName(); got != "/tmp/project" {
		t.Fatalf("DisplayName() = %q", got)
	}
}
