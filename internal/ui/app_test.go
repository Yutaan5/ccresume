package ui

import (
	"strings"
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

func TestApplyDeletedUpdatesOriginProjectAfterNavigation(t *testing.T) {
	p1s1 := &session.Session{ID: "s1", Path: "/sessions/p1-s1", Title: "one"}
	p1s2 := &session.Session{ID: "s2", Path: "/sessions/p1-s2", Title: "two"}
	p2s1 := &session.Session{ID: "s3", Path: "/sessions/p2-s1", Title: "three"}
	p1 := &session.Project{DirName: "p1", Path: "/projects/p1", Sessions: []*session.Session{p1s1, p1s2}}
	p2 := &session.Project{DirName: "p2", Path: "/projects/p2", Sessions: []*session.Session{p2s1}}

	m := New()
	m.projects = []*session.Project{p1, p2}
	m.curProj = 1
	m.setSessionItems(0)
	m.pendingDeletes[1] = deleteTarget{projectPath: p1.Path, session: p1s1}

	mdl, _ := m.applyDeleted(deletedMsg{seq: 1, projectPath: p1.Path, path: p1s1.Path})
	got := mdl.(App)
	if len(got.projects[0].Sessions) != 1 || got.projects[0].Sessions[0] != p1s2 {
		t.Fatalf("origin sessions = %#v", got.projects[0].Sessions)
	}
	if got.currentProject() != p2 {
		t.Fatalf("current project changed to %#v", got.currentProject())
	}
	if got.selectedSession() != p2s1 {
		t.Fatalf("visible session changed to %#v", got.selectedSession())
	}
}

func TestApplyDeletedAfterEmptyRescanDoesNotPanic(t *testing.T) {
	s := &session.Session{ID: "s1", Path: "/sessions/p1-s1", Title: "one"}
	p := &session.Project{DirName: "p1", Path: "/projects/p1", Sessions: []*session.Session{s}}
	m := New()
	m.projects = []*session.Project{p}
	m.curProj = 0
	m.setSessionItems(0)
	m.pendingDeletes[7] = deleteTarget{projectPath: p.Path, session: s}

	mdl, _ := m.applyIndex(nil)
	m = mdl.(App)
	mdl, _ = m.applyDeleted(deletedMsg{seq: 7, projectPath: p.Path, path: s.Path})
	got := mdl.(App)
	if _, ok := got.deletedPaths[s.Path]; !ok {
		t.Fatal("successful deletion was not recorded")
	}
	if len(got.projects) != 0 {
		t.Fatalf("projects = %#v", got.projects)
	}
}

func TestApplyDeletedIgnoresUnknownOperationGeneration(t *testing.T) {
	s := &session.Session{ID: "s1", Path: "/sessions/p1-s1", Title: "one"}
	p := &session.Project{DirName: "p1", Path: "/projects/p1", Sessions: []*session.Session{s}}
	m := New()
	m.projects = []*session.Project{p}
	m.curProj = 0
	m.setSessionItems(0)

	mdl, _ := m.applyDeleted(deletedMsg{seq: 99, projectPath: p.Path, path: s.Path})
	got := mdl.(App)
	if len(got.projects) != 1 || len(got.projects[0].Sessions) != 1 {
		t.Fatalf("unknown delete operation mutated state: %#v", got.projects)
	}
}

func TestApplyIndexDoesNotResurrectDeletedSession(t *testing.T) {
	s := &session.Session{ID: "s1", Path: "/sessions/p1-s1", Title: "one"}
	p := &session.Project{DirName: "p1", Path: "/projects/p1", Sessions: []*session.Session{s}}
	m := New()
	m.projects = []*session.Project{p}
	m.curProj = 0
	m.setSessionItems(0)
	m.pendingDeletes[3] = deleteTarget{projectPath: p.Path, session: s}

	mdl, _ := m.applyDeleted(deletedMsg{seq: 3, projectPath: p.Path, path: s.Path})
	m = mdl.(App)
	staleSession := &session.Session{ID: s.ID, Path: s.Path, Title: s.Title}
	staleProject := &session.Project{DirName: p.DirName, Path: p.Path, Sessions: []*session.Session{staleSession}}
	mdl, _ = m.applyIndex([]*session.Project{staleProject})
	got := mdl.(App)
	if len(got.projects) != 0 {
		t.Fatalf("deleted session was resurrected: %#v", got.projects)
	}
}

func TestDisplayBoundariesSanitizeSessionMetadata(t *testing.T) {
	s := &session.Session{
		Title:     "safe\x1b]52;c;ZXZpbA==\x07",
		GitBranch: "main\x1b[2J",
	}
	item := sessionItem{s: s}
	for _, got := range []string{item.Title(), item.Description()} {
		if strings.ContainsAny(got, "\x1b\x07") || strings.Contains(got, "ZXZpbA") {
			t.Fatalf("unsafe metadata reached display: %q", got)
		}
	}
}
