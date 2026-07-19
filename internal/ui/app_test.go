package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"ccresume/internal/session"
)

func TestJKeyMovesCursor(t *testing.T) {
	m := New()
	var mdl tea.Model = m
	mdl, _ = mdl.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	projects := []*session.Project{{
		DirName: "-test", Path: "/tmp/x", CWD: "/tmp/x",
		Sessions: []*session.Session{
			{ID: "s1", Title: "one", ModTime: time.Now()},
			{ID: "s2", Title: "two", ModTime: time.Now()},
			{ID: "s3", Title: "three", ModTime: time.Now()},
		},
	}}
	mdl, _ = mdl.Update(indexLoadedMsg{projects: projects})
	app := mdl.(App)
	t.Logf("mode=%v index=%d", app.mode, app.sessionList.Index())

	mdl, _ = mdl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	app = mdl.(App)
	t.Logf("after j: index=%d filterState=%v", app.sessionList.Index(), app.sessionList.FilterState())
	if app.sessionList.Index() != 1 {
		t.Errorf("j did not move cursor: index=%d", app.sessionList.Index())
	}

	mdl, _ = mdl.Update(tea.KeyMsg{Type: tea.KeyDown})
	app = mdl.(App)
	if app.sessionList.Index() != 2 {
		t.Errorf("down did not move cursor: index=%d", app.sessionList.Index())
	}
}
