package session

import (
	"encoding/json"
	"strings"

	"github.com/mattn/go-runewidth"
)

const titleMaxWidth = 60

// record はセッション JSONL の1行のうち、この用途で必要なフィールドだけを持つ。
type record struct {
	Type        string `json:"type"`
	IsSidechain bool   `json:"isSidechain"`
	AiTitle     string `json:"aiTitle"`
	Summary     string `json:"summary"`
	Cwd         string `json:"cwd"`
	GitBranch   string `json:"gitBranch"`
	Message     struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// assistantText は1行をパースし、sidechain でない assistant レコードの
// text ブロックを連結して返す。テキストが空なら ok=false。
func assistantText(line []byte) (string, bool) {
	var r record
	if err := json.Unmarshal(line, &r); err != nil {
		return "", false
	}
	if r.Type != "assistant" || r.IsSidechain {
		return "", false
	}
	var blocks []contentBlock
	if err := json.Unmarshal(r.Message.Content, &blocks); err != nil {
		return "", false
	}
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
			parts = append(parts, b.Text)
		}
	}
	if len(parts) == 0 {
		return "", false
	}
	return strings.Join(parts, "\n\n"), true
}

// userPromptText は user レコードからタイトル候補となるテキストを取り出す。
// コマンドラッパー（"<command-name>..." など）や添付は除外する。
func userPromptText(r *record) (string, bool) {
	if r.Type != "user" || r.IsSidechain {
		return "", false
	}
	var text string
	if err := json.Unmarshal(r.Message.Content, &text); err != nil {
		var blocks []contentBlock
		if err := json.Unmarshal(r.Message.Content, &blocks); err != nil {
			return "", false
		}
		for _, b := range blocks {
			if b.Type == "text" && strings.TrimSpace(b.Text) != "" {
				text = b.Text
				break
			}
		}
	}
	text = strings.TrimSpace(text)
	if text == "" || strings.HasPrefix(text, "<") || strings.HasPrefix(text, "Caveat:") ||
		strings.HasPrefix(text, "Base directory for this skill:") {
		return "", false
	}
	return text, true
}

// truncateTitle は改行を潰し、表示幅ベースで切り詰める（日本語安全）。
func truncateTitle(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return runewidth.Truncate(s, titleMaxWidth, "…")
}
