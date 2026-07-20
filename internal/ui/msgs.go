package ui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"ccresume/internal/resume"
	"ccresume/internal/session"
)

type indexLoadedMsg struct {
	projects []*session.Project
	err      error
}

type previewTickMsg struct{ seq int }

type previewMsg struct {
	seq int
	raw string // 生の Markdown（リサイズ時の再レンダリング用）
	md  string // レンダリング済み
	err error
}

type rerenderMsg struct {
	seq int
	md  string
}

type deletedMsg struct {
	seq         uint64
	projectPath string
	path        string
	err         error
}

type cmuxDoneMsg struct{ err error }

func loadIndexCmd() tea.Cmd {
	return func() tea.Msg {
		projects, err := session.Scan()
		return indexLoadedMsg{projects: projects, err: err}
	}
}

func previewTickCmd(seq int) tea.Cmd {
	return tea.Tick(120*time.Millisecond, func(time.Time) tea.Msg {
		return previewTickMsg{seq: seq}
	})
}

func renderMarkdown(raw string, width int) string {
	raw = session.SanitizeDisplay(raw)
	if width < 10 {
		width = 10
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return raw
	}
	out, err := r.Render(raw)
	if err != nil {
		return raw
	}
	return out
}

func loadPreviewCmd(path string, seq, width int) tea.Cmd {
	return func() tea.Msg {
		raw, err := session.LastAssistantText(path)
		if err != nil {
			return previewMsg{seq: seq, err: err}
		}
		raw = session.SanitizeDisplay(raw)
		return previewMsg{seq: seq, raw: raw, md: renderMarkdown(raw, width)}
	}
}

func rerenderCmd(raw string, seq, width int) tea.Cmd {
	return func() tea.Msg {
		return rerenderMsg{seq: seq, md: renderMarkdown(raw, width)}
	}
}

func deleteCmd(seq uint64, projectPath, path string) tea.Cmd {
	return func() tea.Msg {
		return deletedMsg{seq: seq, projectPath: projectPath, path: path, err: removeFile(path)}
	}
}

func resumeCmuxCmd(cwd, sessionID string) tea.Cmd {
	return func() tea.Msg {
		return cmuxDoneMsg{err: resume.OpenResumeTab(cwd, sessionID)}
	}
}
