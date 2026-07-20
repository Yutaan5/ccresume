package resume

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validSessionID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"

func TestValidateTarget(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		cwd  string
		id   string
		ok   bool
	}{
		{name: "valid", cwd: dir, id: validSessionID, ok: true},
		{name: "missing cwd", cwd: "", id: validSessionID},
		{name: "nonexistent cwd", cwd: filepath.Join(dir, "missing"), id: validSessionID},
		{name: "cwd is file", cwd: file, id: validSessionID},
		{name: "command injection id", cwd: dir, id: "foo; open -a Calculator;#"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTarget(tt.cwd, tt.id)
			if tt.ok && err != nil {
				t.Fatalf("ValidateTarget() error = %v", err)
			}
			if !tt.ok && err == nil {
				t.Fatal("ValidateTarget() unexpectedly succeeded")
			}
		})
	}
}

func TestResumeCommand_QuotesAllUntrustedValues(t *testing.T) {
	got := ResumeCommand("/tmp/project's dir", "foo; open -a Calculator;#")
	want := "cd '/tmp/project'\\''s dir' && claude --resume 'foo; open -a Calculator;#'"
	if got != want {
		t.Fatalf("ResumeCommand() = %q, want %q", got, want)
	}
	if strings.Contains(got, "--resume foo;") {
		t.Fatal("session ID is not shell quoted")
	}
}

func TestWorkspaceCommandQuotesSessionID(t *testing.T) {
	got := resumeShellCommand("foo; open -a Calculator;#")
	if got != "claude --resume 'foo; open -a Calculator;#'" {
		t.Fatalf("resumeShellCommand() = %q", got)
	}
}
