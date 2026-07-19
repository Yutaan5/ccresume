package ui

import (
	"fmt"
	"time"

	"ccresume/internal/session"
)

type sessionItem struct{ s *session.Session }

func (i sessionItem) Title() string { return i.s.Title }
func (i sessionItem) Description() string {
	desc := relTime(i.s.ModTime)
	if i.s.GitBranch != "" {
		desc += " · " + i.s.GitBranch
	}
	return desc
}
func (i sessionItem) FilterValue() string { return i.s.FilterValue() }

type projectItem struct{ p *session.Project }

func (i projectItem) Title() string { return i.p.DisplayName() }
func (i projectItem) Description() string {
	return fmt.Sprintf("%d件 · 最終 %s", len(i.p.Sessions), relTime(i.p.Sessions[0].ModTime))
}
func (i projectItem) FilterValue() string { return i.p.DisplayName() }

// relTime は日本語の相対時刻表示を返す。
func relTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "たった今"
	case d < time.Hour:
		return fmt.Sprintf("%d分前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d時間前", int(d.Hours()))
	case d < 48*time.Hour:
		return "昨日"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d日前", int(d.Hours()/24))
	default:
		return t.Format("2006/01/02")
	}
}
