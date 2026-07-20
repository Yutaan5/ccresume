package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"ccresume/internal/resume"
	"ccresume/internal/session"
)

type mode int

const (
	modeLoading mode = iota
	modeBrowse
	modeProjects
	modeConfirmDelete
)

type App struct {
	mode     mode
	projects []*session.Project
	curProj  int

	sessionList list.Model
	projectList list.Model
	preview     viewport.Model
	spinner     spinner.Model

	previewSeq   int
	previewRaw   string
	previewState string // "", "loading", "ready", "empty", "error"

	curSessionID string
	deleting     *deleteTarget
	execTarget   *resume.ExecTarget
	status       string
	statusIsErr  bool
	loadErr      error

	nextDeleteSeq  uint64
	pendingDeletes map[uint64]deleteTarget
	deletedPaths   map[string]struct{}

	width, height int
	ready         bool
}

type deleteTarget struct {
	projectPath string
	session     *session.Session
}

// ExecTarget は TUI 終了後に main が exec すべき対象（nil なら何もしない）。
func (m App) ExecTarget() *resume.ExecTarget { return m.execTarget }

func New() App {
	sp := spinner.New(spinner.WithSpinner(spinner.Dot))

	sl := list.New(nil, newDelegate(), 0, 0)
	sl.SetShowHelp(false)
	sl.SetShowStatusBar(false)
	sl.DisableQuitKeybindings()
	sl.SetStatusBarItemName("セッション", "セッション")

	pl := list.New(nil, newDelegate(), 0, 0)
	pl.Title = "ディレクトリを選択"
	pl.SetShowHelp(false)
	pl.SetShowStatusBar(false)
	pl.DisableQuitKeybindings()

	return App{
		mode:           modeLoading,
		spinner:        sp,
		sessionList:    sl,
		projectList:    pl,
		preview:        viewport.New(0, 0),
		pendingDeletes: make(map[uint64]deleteTarget),
		deletedPaths:   make(map[string]struct{}),
	}
}

func newDelegate() list.DefaultDelegate {
	d := list.NewDefaultDelegate()
	return d
}

func (m App) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadIndexCmd())
}

func (m App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true
		m.layout()
		if m.previewState == "ready" && m.previewRaw != "" {
			return m, rerenderCmd(m.previewRaw, m.previewSeq, m.previewWidth())
		}
		return m, nil

	case indexLoadedMsg:
		if msg.err != nil {
			m.loadErr = msg.err
			m.mode = modeBrowse
			return m, nil
		}
		return m.applyIndex(msg.projects)

	case spinner.TickMsg:
		if m.mode == modeLoading {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case previewTickMsg:
		if msg.seq != m.previewSeq {
			return m, nil
		}
		if s := m.selectedSession(); s != nil {
			return m, loadPreviewCmd(s.Path, msg.seq, m.previewWidth())
		}
		return m, nil

	case previewMsg:
		if msg.seq != m.previewSeq {
			return m, nil
		}
		if msg.err != nil {
			if msg.err == session.ErrNoAssistantText {
				m.previewState = "empty"
				m.preview.SetContent(placeholderStyle.Render("（このセッションにはアシスタントのテキスト応答がありません）"))
			} else {
				m.previewState = "error"
				m.preview.SetContent(placeholderStyle.Render("読み込みエラー: " + session.SanitizeDisplay(msg.err.Error())))
			}
			return m, nil
		}
		m.previewState = "ready"
		m.previewRaw = msg.raw
		m.preview.SetContent(msg.md)
		m.preview.GotoTop()
		return m, nil

	case rerenderMsg:
		if msg.seq == m.previewSeq {
			m.preview.SetContent(msg.md)
		}
		return m, nil

	case deletedMsg:
		return m.applyDeleted(msg)

	case cmuxDoneMsg:
		if msg.err != nil {
			m.setError("cmux でのタブ作成に失敗: " + msg.err.Error())
			return m, nil
		}
		return m, tea.Quit

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m App) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.status = ""
	m.statusIsErr = false
	if os.Getenv("CCRESUME_DEBUG_KEYS") != "" {
		m.status = fmt.Sprintf("key=%q type=%d paste=%v", msg.String(), msg.Type, msg.Paste)
	}

	switch m.mode {
	case modeLoading:
		if key.Matches(msg, keys.Quit) {
			return m, tea.Quit
		}
		return m, nil

	case modeConfirmDelete:
		switch {
		case key.Matches(msg, keys.ConfirmYes):
			target := m.deleting
			m.deleting = nil
			m.mode = modeBrowse
			if target == nil || target.session == nil {
				return m, nil
			}
			m.nextDeleteSeq++
			seq := m.nextDeleteSeq
			m.pendingDeletes[seq] = *target
			return m, deleteCmd(seq, target.projectPath, target.session.Path)
		default:
			m.deleting = nil
			m.mode = modeBrowse
			return m, nil
		}

	case modeProjects:
		if m.projectList.FilterState() == list.Filtering {
			return m.updateProjectList(msg)
		}
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.CancelOrEsc):
			if m.projectList.FilterState() != list.Unfiltered {
				return m.updateProjectList(msg) // フィルタ解除は list に任せる
			}
			m.mode = modeBrowse
			return m, nil
		case key.Matches(msg, keys.Resume):
			if it, ok := m.projectList.SelectedItem().(projectItem); ok {
				for i, p := range m.projects {
					if p.DirName == it.p.DirName {
						m.curProj = i
						break
					}
				}
				m.setSessionItems(0)
				m.mode = modeBrowse
				return m, m.schedulePreview()
			}
			return m, nil
		default:
			return m.updateProjectList(msg)
		}

	case modeBrowse:
		if m.loadErr != nil {
			if key.Matches(msg, keys.Quit) || key.Matches(msg, keys.CancelOrEsc) {
				return m, tea.Quit
			}
			if key.Matches(msg, keys.Rescan) {
				m.loadErr = nil
				m.mode = modeLoading
				return m, tea.Batch(m.spinner.Tick, loadIndexCmd())
			}
			return m, nil
		}
		if m.sessionList.FilterState() == list.Filtering {
			return m.updateSessionList(msg)
		}
		switch {
		case key.Matches(msg, keys.Quit):
			return m, tea.Quit
		case key.Matches(msg, keys.CancelOrEsc):
			if m.sessionList.FilterState() != list.Unfiltered {
				return m.updateSessionList(msg)
			}
			return m, tea.Quit
		case key.Matches(msg, keys.Resume):
			return m.startResume()
		case key.Matches(msg, keys.Delete):
			if s := m.selectedSession(); s != nil {
				if p := m.currentProject(); p != nil {
					m.deleting = &deleteTarget{projectPath: p.Path, session: s}
					m.mode = modeConfirmDelete
				}
			}
			return m, nil
		case key.Matches(msg, keys.Projects):
			m.mode = modeProjects
			return m, nil
		case key.Matches(msg, keys.Rescan):
			m.mode = modeLoading
			return m, tea.Batch(m.spinner.Tick, loadIndexCmd())
		case key.Matches(msg, keys.ScrollDown):
			m.preview.ScrollDown(2)
			return m, nil
		case key.Matches(msg, keys.ScrollUp):
			m.preview.ScrollUp(2)
			return m, nil
		case key.Matches(msg, keys.HalfPgDown):
			m.preview.HalfPageDown()
			return m, nil
		case key.Matches(msg, keys.HalfPgUp):
			m.preview.HalfPageUp()
			return m, nil
		default:
			return m.updateSessionList(msg)
		}
	}
	return m, nil
}

func (m App) updateSessionList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.sessionList, cmd = m.sessionList.Update(msg)
	if pc := m.schedulePreview(); pc != nil {
		return m, tea.Batch(cmd, pc)
	}
	return m, cmd
}

func (m App) updateProjectList(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.projectList, cmd = m.projectList.Update(msg)
	return m, cmd
}

// schedulePreview は選択セッションが変わっていたら debounce 付きで読み込みを予約する。
func (m *App) schedulePreview() tea.Cmd {
	s := m.selectedSession()
	if s == nil {
		if m.curSessionID != "" {
			m.curSessionID = ""
			m.previewSeq++
			m.previewState = ""
			m.preview.SetContent("")
		}
		return nil
	}
	if s.ID == m.curSessionID {
		return nil
	}
	m.curSessionID = s.ID
	m.previewSeq++
	m.previewState = "loading"
	m.previewRaw = ""
	m.preview.SetContent(placeholderStyle.Render("読み込み中…"))
	return previewTickCmd(m.previewSeq)
}

func (m App) startResume() (tea.Model, tea.Cmd) {
	s := m.selectedSession()
	if s == nil {
		return m, nil
	}
	if err := resume.ValidateTarget(s.CWD, s.ID); err != nil {
		m.setError("resume できません: " + err.Error())
		return m, nil
	}
	if resume.InCmux() {
		m.status = "新しいタブを開いています…"
		return m, resumeCmuxCmd(s.CWD, s.ID)
	}
	m.execTarget = &resume.ExecTarget{SessionID: s.ID, CWD: s.CWD}
	return m, tea.Quit
}

func (m App) applyIndex(projects []*session.Project) (tea.Model, tea.Cmd) {
	prevDir := ""
	if len(m.projects) > 0 && m.curProj < len(m.projects) {
		prevDir = m.projects[m.curProj].DirName
	}
	filtered := projects[:0]
	for _, p := range projects {
		sessions := p.Sessions[:0]
		for _, s := range p.Sessions {
			if _, deleted := m.deletedPaths[s.Path]; !deleted {
				sessions = append(sessions, s)
			}
		}
		p.Sessions = sessions
		if len(p.Sessions) > 0 {
			filtered = append(filtered, p)
		}
	}
	projects = filtered
	m.projects = projects
	m.curProj = 0
	for i, p := range projects {
		if p.DirName == prevDir {
			m.curProj = i
			break
		}
	}

	items := make([]list.Item, len(projects))
	for i, p := range projects {
		items[i] = projectItem{p: p}
	}
	m.projectList.SetItems(items)

	m.mode = modeBrowse
	if len(projects) == 0 {
		m.loadErr = fmt.Errorf("セッションが見つかりません（~/.claude/projects が空です）")
		m.sessionList.SetItems(nil)
		return m, nil
	}
	m.loadErr = nil
	m.setSessionItems(0)
	m.layout()
	return m, m.schedulePreview()
}

func (m *App) setSessionItems(cursor int) {
	p := m.currentProject()
	if p == nil {
		m.sessionList.SetItems(nil)
		m.curSessionID = ""
		m.previewState = ""
		m.previewRaw = ""
		m.preview.SetContent("")
		return
	}
	items := make([]list.Item, len(p.Sessions))
	for i, s := range p.Sessions {
		items[i] = sessionItem{s: s}
	}
	m.sessionList.SetItems(items)
	m.sessionList.ResetFilter()
	m.sessionList.Title = p.DisplayName()
	if cursor >= len(items) {
		cursor = len(items) - 1
	}
	if cursor < 0 {
		cursor = 0
	}
	m.sessionList.Select(cursor)
	m.curSessionID = "" // プレビューを必ず更新させる
}

func (m App) applyDeleted(msg deletedMsg) (tea.Model, tea.Cmd) {
	target, ok := m.pendingDeletes[msg.seq]
	if !ok || target.projectPath != msg.projectPath || target.session == nil || target.session.Path != msg.path {
		return m, nil
	}
	delete(m.pendingDeletes, msg.seq)
	if msg.err != nil {
		m.setError("削除に失敗: " + msg.err.Error())
		return m, nil
	}
	m.deletedPaths[msg.path] = struct{}{}
	m.status = "削除しました"
	m.statusIsErr = false

	originIndex := -1
	for i, p := range m.projects {
		if p.Path == msg.projectPath {
			originIndex = i
			break
		}
	}
	if originIndex < 0 {
		return m, nil
	}

	currentPath := ""
	if p := m.currentProject(); p != nil {
		currentPath = p.Path
	}
	originWasCurrent := originIndex == m.curProj
	cursor := m.sessionList.Index()
	p := m.projects[originIndex]
	kept := make([]*session.Session, 0, len(p.Sessions))
	for _, s := range p.Sessions {
		if s.Path != msg.path {
			kept = append(kept, s)
		}
	}
	p.Sessions = kept
	if len(p.Sessions) == 0 {
		m.projects = append(m.projects[:originIndex], m.projects[originIndex+1:]...)
	}

	if len(m.projects) == 0 {
		m.curProj = 0
		m.projectList.SetItems(nil)
		m.setSessionItems(0)
		m.loadErr = fmt.Errorf("セッションが見つかりません（~/.claude/projects が空です）")
		return m, nil
	}

	m.curProj = 0
	for i, q := range m.projects {
		if q.Path == currentPath {
			m.curProj = i
			break
		}
	}
	m.setProjectItems()
	if originWasCurrent {
		m.setSessionItems(cursor)
		return m, m.schedulePreview()
	}
	return m, nil
}

func (m *App) setError(s string) {
	m.status = session.SanitizeDisplay(s)
	m.statusIsErr = true
}

func (m *App) setProjectItems() {
	items := make([]list.Item, len(m.projects))
	for i, p := range m.projects {
		items[i] = projectItem{p: p}
	}
	m.projectList.SetItems(items)
	if m.curProj >= 0 && m.curProj < len(items) {
		m.projectList.Select(m.curProj)
	}
}

func (m *App) currentProject() *session.Project {
	if m.curProj < 0 || m.curProj >= len(m.projects) {
		return nil
	}
	return m.projects[m.curProj]
}

func (m App) selectedSession() *session.Session {
	if it, ok := m.sessionList.SelectedItem().(sessionItem); ok {
		return it.s
	}
	return nil
}

// --- layout & view ---

func (m *App) leftWidth() int {
	w := m.width * 2 / 5
	if w < 30 {
		w = 30
	}
	if w > m.width-20 {
		w = m.width / 2
	}
	return w
}

func (m *App) previewWidth() int {
	return m.width - m.leftWidth() - 4
}

func (m *App) layout() {
	if !m.ready {
		return
	}
	contentH := m.height - 1 // フッター分
	m.sessionList.SetSize(m.leftWidth(), contentH)
	m.projectList.SetSize(m.width, contentH)
	m.preview.Width = m.previewWidth()
	m.preview.Height = contentH - 2 // プレビューのタイトル行
}

func (m App) View() string {
	if !m.ready {
		return ""
	}
	switch m.mode {
	case modeLoading:
		return fmt.Sprintf("\n %s セッションを読み込んでいます…\n", m.spinner.View())
	case modeProjects:
		return m.projectList.View() + "\n" + statusStyle.Render(" "+helpProjects)
	default:
		if m.loadErr != nil {
			return placeholderStyle.Render(session.SanitizeDisplay(m.loadErr.Error())+"\n\nr: 再読込  q: 終了") + "\n"
		}
		return m.browseView()
	}
}

func (m App) browseView() string {
	left := leftPaneStyle.Render(m.sessionList.View())

	header := "最後の返答"
	if s := m.selectedSession(); s != nil {
		meta := relTime(s.ModTime)
		if s.GitBranch != "" {
			meta += " · " + session.SanitizeDisplay(s.GitBranch)
		}
		header += " " + previewMetaStyle.Render(meta)
	}
	rightTop := previewTitleStyle.Width(m.previewWidth()).Render(header)
	right := rightTop + "\n" + m.preview.View()

	body := lipgloss.JoinHorizontal(lipgloss.Top, left, right)

	footer := " " + helpBrowse
	if m.mode == modeConfirmDelete && m.deleting != nil {
		footer = confirmStyle.Render(fmt.Sprintf(" 本当に削除しますか？ %s (y/N)", session.SanitizeDisplay(m.deleting.session.Title)))
	} else if m.status != "" {
		status := session.SanitizeDisplay(m.status)
		if m.statusIsErr {
			footer = errorStyle.Render(" " + status)
		} else {
			footer = statusStyle.Render(" " + status)
		}
	} else {
		footer = statusStyle.Render(footer)
	}
	return body + "\n" + footer
}

func removeFile(path string) error { return os.Remove(path) }
