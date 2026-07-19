package resume

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
)

// ExecTarget は TUI 終了後に exec で claude に切り替えるための情報。
type ExecTarget struct {
	SessionID string
	CWD       string
}

// Exec は現在のプロセスを `claude --resume <id>` に置き換える。
// 成功した場合この関数から戻らない。
func (t *ExecTarget) Exec() error {
	if t.CWD != "" {
		if err := os.Chdir(t.CWD); err != nil {
			fmt.Fprintf(os.Stderr, "警告: %s に移動できません（%v）。ホームから起動します。\n", t.CWD, err)
			home, _ := os.UserHomeDir()
			_ = os.Chdir(home)
		}
	}
	claude, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude コマンドが見つかりません: %w", err)
	}
	return syscall.Exec(claude, []string{"claude", "--resume", t.SessionID}, os.Environ())
}

// shellQuote はシングルクォートで安全に囲む（日本語・空白対応）。
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// ResumeCommand は新しいタブに流し込むシェルコマンドを組み立てる。
func ResumeCommand(cwd, sessionID string) string {
	cmd := "claude --resume " + sessionID
	if cwd != "" {
		cmd = "cd " + shellQuote(cwd) + " && " + cmd
	}
	return cmd
}
