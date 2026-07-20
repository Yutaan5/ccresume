package session

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

const (
	tailReadSize = 64 * 1024
	headReadSize = 32 * 1024
	scanWorkers  = 8
)

// ProjectsDir は Claude Code のセッション保存ルートを返す。
func ProjectsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "projects"), nil
}

// Scan は全プロジェクトのセッションを列挙し、メタデータ（タイトル・cwd 等）を
// 付与して返す。プロジェクトは最新セッション降順、セッションも更新日時降順。
func Scan() ([]*Project, error) {
	root, err := ProjectsDir()
	if err != nil {
		return nil, err
	}
	return scanRoot(root)
}

func scanRoot(root string) ([]*Project, error) {
	dirs, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var projects []*Project
	var all []*Session
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		p := &Project{DirName: d.Name(), Path: filepath.Join(root, d.Name())}
		entries, err := os.ReadDir(p.Path)
		if err != nil {
			continue
		}
		for _, e := range entries {
			// UUID 名のサブディレクトリ（サブエージェント等）は対象外
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
				continue
			}
			id := strings.TrimSuffix(e.Name(), ".jsonl")
			if !ValidID(id) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			s := &Session{
				ID:      id,
				Path:    filepath.Join(p.Path, e.Name()),
				ModTime: info.ModTime(),
				Size:    info.Size(),
			}
			p.Sessions = append(p.Sessions, s)
			all = append(all, s)
		}
		if len(p.Sessions) == 0 {
			continue
		}
		sort.Slice(p.Sessions, func(i, j int) bool {
			return p.Sessions[i].ModTime.After(p.Sessions[j].ModTime)
		})
		projects = append(projects, p)
	}

	enrichAll(all)

	for _, p := range projects {
		// 最新の enriched セッションの cwd をプロジェクトの実パスとする
		for _, s := range p.Sessions {
			if s.CWD != "" {
				p.CWD = s.CWD
				break
			}
		}
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Sessions[0].ModTime.After(projects[j].Sessions[0].ModTime)
	})
	return projects, nil
}

func enrichAll(sessions []*Session) {
	jobs := make(chan *Session)
	var wg sync.WaitGroup
	for range scanWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for s := range jobs {
				enrich(s)
			}
		}()
	}
	for _, s := range sessions {
		jobs <- s
	}
	close(jobs)
	wg.Wait()
}

// enrich は末尾 64KB から ai-title / cwd / gitBranch を、
// 取れなければ先頭 32KB から summary / 最初のユーザープロンプトを拾う。
func enrich(s *Session) {
	lines, err := tailLines(s.Path, tailReadSize)
	if err == nil {
		for i := len(lines) - 1; i >= 0; i-- {
			var r record
			if json.Unmarshal(lines[i], &r) != nil {
				continue
			}
			if s.Title == "" && r.Type == "ai-title" && r.AiTitle != "" {
				s.Title = truncateTitle(r.AiTitle)
			}
			if s.CWD == "" && r.Cwd != "" {
				s.CWD = r.Cwd
				s.GitBranch = r.GitBranch
			}
			if s.Title != "" && s.CWD != "" {
				break
			}
		}
	}
	if s.Title == "" || s.CWD == "" {
		enrichFromHead(s)
	}
	if s.Title == "" {
		s.Title = "(無題) " + shortID(s.ID)
	}
	s.Enriched = true
}

func enrichFromHead(s *Session) {
	lines, err := headLines(s.Path, headReadSize)
	if err != nil {
		return
	}
	for _, line := range lines {
		var r record
		if json.Unmarshal(line, &r) != nil {
			continue
		}
		if s.CWD == "" && r.Cwd != "" {
			s.CWD = r.Cwd
			s.GitBranch = r.GitBranch
		}
		if s.Title == "" {
			if r.Type == "summary" && r.Summary != "" {
				s.Title = truncateTitle(r.Summary)
			} else if txt, ok := userPromptText(&r); ok {
				s.Title = truncateTitle(txt)
			}
		}
		if s.Title != "" && s.CWD != "" {
			return
		}
	}
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}
