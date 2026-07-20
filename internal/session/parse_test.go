package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeSession(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assistantLine(text string) string {
	return fmt.Sprintf(`{"type":"assistant","isSidechain":false,"message":{"role":"assistant","content":[{"type":"text","text":%q}]}}`, text)
}

func toolUseLine() string {
	return `{"type":"assistant","isSidechain":false,"message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{}}]}}`
}

func userLine(text string) string {
	return fmt.Sprintf(`{"type":"user","isSidechain":false,"cwd":"/Users/tsuji/Repository/foo","gitBranch":"main","message":{"role":"user","content":%q}}`, text)
}

func TestLastAssistantText_SkipsToolUseTail(t *testing.T) {
	path := writeSession(t,
		userLine("こんにちは"),
		assistantLine("最初の返答"),
		assistantLine("これが最後のテキスト返答です"),
		toolUseLine(),
		toolUseLine(),
	)
	got, err := LastAssistantText(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "これが最後のテキスト返答です" {
		t.Errorf("got %q", got)
	}
}

func TestLastAssistantText_SkipsSidechain(t *testing.T) {
	side := `{"type":"assistant","isSidechain":true,"message":{"role":"assistant","content":[{"type":"text","text":"サブエージェントの返答"}]}}`
	path := writeSession(t,
		assistantLine("本体の返答"),
		side,
	)
	got, err := LastAssistantText(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "本体の返答" {
		t.Errorf("got %q", got)
	}
}

func TestLastAssistantText_NoAssistant(t *testing.T) {
	path := writeSession(t,
		userLine("質問だけして中断"),
		toolUseLine(),
	)
	_, err := LastAssistantText(path)
	if !errors.Is(err, ErrNoAssistantText) {
		t.Errorf("want ErrNoAssistantText, got %v", err)
	}
}

func TestLastAssistantText_HugeLineAcrossChunks(t *testing.T) {
	// chunkSize より大きい1行（file-history-snapshot 相当）が境界をまたいでも
	// carry の連結で正しくスキップ・パースできること
	huge := fmt.Sprintf(`{"type":"file-history-snapshot","snapshot":{"data":%q}}`,
		strings.Repeat("x", chunkSize+chunkSize/2))
	path := writeSession(t,
		assistantLine("巨大行の前の返答"),
		huge,
		toolUseLine(),
	)
	got, err := LastAssistantText(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "巨大行の前の返答" {
		t.Errorf("got %q", got)
	}
}

func TestLastAssistantText_HugeAssistantTextAcrossChunks(t *testing.T) {
	long := strings.Repeat("あ", chunkSize) // マルチバイトで chunk 境界をまたぐ
	path := writeSession(t, assistantLine(long))
	got, err := LastAssistantText(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != long {
		t.Errorf("length mismatch: got %d want %d", len(got), len(long))
	}
}

func TestLastAssistantText_JoinsMultipleTextBlocks(t *testing.T) {
	line := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"text","text":"前半"},{"type":"tool_use","id":"t","name":"Bash","input":{}},{"type":"text","text":"後半"}]}}`
	path := writeSession(t, line)
	got, err := LastAssistantText(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "前半\n\n後半" {
		t.Errorf("got %q", got)
	}
}

func TestLastAssistantText_RemovesTerminalControls(t *testing.T) {
	line := `{"type":"assistant","isSidechain":false,"message":{"role":"assistant","content":[{"type":"text","text":"before\u001b]52;c;ZXZpbA==\u0007after\u001b[2J"}]}}`
	path := writeSession(t, line)
	got, err := LastAssistantText(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "beforeafter" {
		t.Fatalf("got %q", got)
	}
}

func TestEnrich_AiTitleFromTail(t *testing.T) {
	lines := []string{userLine("最初の質問です")}
	for range 50 {
		lines = append(lines, toolUseLine())
	}
	lines = append(lines, `{"type":"ai-title","aiTitle":"日本語のセッションタイトル","sessionId":"x"}`)
	lines = append(lines, assistantLine("done"))
	path := writeSession(t, lines...)

	s := &Session{ID: "test", Path: path}
	enrich(s)
	if s.Title != "日本語のセッションタイトル" {
		t.Errorf("Title = %q", s.Title)
	}
	if s.CWD != "/Users/tsuji/Repository/foo" {
		t.Errorf("CWD = %q", s.CWD)
	}
}

func TestEnrich_FallbackToFirstUserPrompt(t *testing.T) {
	longPrompt := strings.Repeat("長い質問文", 30) // 表示幅60超
	path := writeSession(t,
		`{"type":"mode","mode":"normal"}`,
		userLine("<command-name>skip me</command-name>"),
		userLine(longPrompt),
		assistantLine("返答"),
	)
	s := &Session{ID: "test", Path: path}
	enrich(s)
	if !strings.HasPrefix(s.Title, "長い質問文") || !strings.HasSuffix(s.Title, "…") {
		t.Errorf("Title = %q", s.Title)
	}
	if s.GitBranch != "main" {
		t.Errorf("GitBranch = %q", s.GitBranch)
	}
}

func TestEnrich_EmptySession(t *testing.T) {
	path := writeSession(t, `{"type":"mode","mode":"normal"}`)
	s := &Session{ID: "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", Path: path}
	enrich(s)
	if !strings.HasPrefix(s.Title, "(無題)") {
		t.Errorf("Title = %q", s.Title)
	}
}
