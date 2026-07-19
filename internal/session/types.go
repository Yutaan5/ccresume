package session

import "time"

// Project は ~/.claude/projects/ 直下の1ディレクトリ（= 作業ディレクトリ単位）。
type Project struct {
	DirName  string // エンコードされたディレクトリ名（表示フォールバック用）
	Path     string // ~/.claude/projects/<DirName>
	CWD      string // セッションレコードの cwd から解決した実パス
	Sessions []*Session
}

// DisplayName は一覧表示用のプロジェクト名を返す。
func (p *Project) DisplayName() string {
	if p.CWD != "" {
		return p.CWD
	}
	return p.DirName
}

// Session は1つの会話履歴（.jsonl ファイル）。
type Session struct {
	ID        string
	Path      string
	ModTime   time.Time
	Size      int64
	Title     string
	CWD       string
	GitBranch string
	Enriched  bool
}

// FilterValue は bubbles/list のフィルタ対象文字列。
func (s *Session) FilterValue() string { return s.Title + " " + s.ID }
