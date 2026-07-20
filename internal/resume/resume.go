package resume

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"ccresume/internal/session"
)

// ExecTarget は TUI 終了後に exec で claude に切り替えるための情報。
type ExecTarget struct {
	SessionID string
	CWD       string
}

// Exec は現在のプロセスを `claude --resume <id>` に置き換える。
// 成功した場合この関数から戻らない。
func (t *ExecTarget) Exec() error {
	if err := ValidateTarget(t.CWD, t.SessionID); err != nil {
		return err
	}
	if err := os.Chdir(t.CWD); err != nil {
		return fmt.Errorf("作業ディレクトリ %q に移動できません: %w", t.CWD, err)
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude コマンドが見つかりません: %w", err)
	}
	return syscall.Exec(claude, []string{"claude", "--resume", t.SessionID}, os.Environ())
}

// ValidateTarget checks all data needed to resume before starting Claude or
// creating a cmux surface.
func ValidateTarget(cwd, sessionID string) error {
	if !session.ValidID(sessionID) {
		return fmt.Errorf("不正なセッション ID です: %q", sessionID)
	}
	if cwd == "" {
		return fmt.Errorf("セッションの作業ディレクトリが記録されていません")
	}
	info, err := os.Stat(cwd)
	if err != nil {
		return fmt.Errorf("作業ディレクトリ %q を利用できません: %w", cwd, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("作業ディレクトリ %q はディレクトリではありません", cwd)
	}
	if err := unix.Access(cwd, unix.X_OK); err != nil {
		return fmt.Errorf("作業ディレクトリ %q に移動する権限がありません: %w", cwd, err)
	}
	return nil
}

// shellQuote はシングルクォートで安全に囲む（日本語・空白対応）。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func resumeShellCommand(sessionID string) string {
	return "claude --resume " + shellQuote(sessionID)
}

// ResumeCommand は新しいタブに流し込むシェルコマンドを組み立てる。
func ResumeCommand(cwd, sessionID string) string {
	cmd := resumeShellCommand(sessionID)
	if cwd != "" {
		cmd = "cd " + shellQuote(cwd) + " && " + cmd
	}
	return cmd
}
