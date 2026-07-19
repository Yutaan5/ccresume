package resume

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const (
	defaultCmuxPath = "/Applications/cmux.app/Contents/Resources/bin/cmux"
	cmuxTimeout     = 3 * time.Second
	shellWaitMax    = 2500 * time.Millisecond
	shellPollEvery  = 150 * time.Millisecond
	enterKey        = "enter"
)

// InCmux は cmux のターミナル内で動いているかを判定する。
func InCmux() bool {
	return os.Getenv("CMUX_SURFACE_ID") != "" || os.Getenv("CMUX_SOCKET_PATH") != ""
}

func cmuxBin() string {
	if p := os.Getenv("CMUX_BUNDLED_CLI_PATH"); p != "" {
		return p
	}
	if _, err := os.Stat(defaultCmuxPath); err == nil {
		return defaultCmuxPath
	}
	p, _ := exec.LookPath("cmux")
	return p
}

func runCmux(args ...string) (string, error) {
	bin := cmuxBin()
	if bin == "" {
		return "", fmt.Errorf("cmux CLI が見つかりません")
	}
	ctx, cancel := context.WithTimeout(context.Background(), cmuxTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("cmux %s: %w: %s", args[0], err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

var (
	surfaceRefRe  = regexp.MustCompile(`surface:\d+`)
	surfaceUUIDRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)
)

func parseSurfaceRef(out string) (string, bool) {
	if m := surfaceRefRe.FindString(out); m != "" {
		return m, true
	}
	if m := surfaceUUIDRe.FindString(out); m != "" {
		return m, true
	}
	return "", false
}

// OpenResumeTab は現在の cmux ワークスペースに新しいターミナルタブを作り、
// そこで claude --resume を実行させる。失敗したら new-workspace にフォールバック。
func OpenResumeTab(cwd, sessionID string) error {
	if err := openInNewSurface(cwd, sessionID); err != nil {
		if fbErr := openInNewWorkspace(cwd, sessionID); fbErr != nil {
			return fmt.Errorf("%v（フォールバックも失敗: %v）", err, fbErr)
		}
	}
	return nil
}

func openInNewSurface(cwd, sessionID string) error {
	args := []string{"new-surface", "--type", "terminal", "--focus", "true"}
	if ws := os.Getenv("CMUX_WORKSPACE_ID"); ws != "" {
		args = append(args, "--workspace", ws)
	}
	out, err := runCmux(args...)
	if err != nil {
		return err
	}
	ref, ok := parseSurfaceRef(out)
	if !ok {
		return fmt.Errorf("new-surface の出力から surface を特定できません: %q", strings.TrimSpace(out))
	}
	waitForShell(ref)
	if _, err := runCmux("send", "--surface", ref, ResumeCommand(cwd, sessionID)); err != nil {
		return err
	}
	if _, err := runCmux("send-key", "--surface", ref, enterKey); err != nil {
		return err
	}
	return nil
}

// waitForShell はプロンプトらしき文字が現れるまで read-screen でポーリングする。
// タイムアウトしてもエラーにはせず、そのまま送信を試みる。
func waitForShell(ref string) {
	deadline := time.Now().Add(shellWaitMax)
	for time.Now().Before(deadline) {
		out, err := runCmux("read-screen", "--surface", ref)
		if err == nil && strings.ContainsAny(out, "%$❯>") {
			return
		}
		time.Sleep(shellPollEvery)
	}
}

func openInNewWorkspace(cwd, sessionID string) error {
	args := []string{"new-workspace", "--command", "claude --resume " + sessionID, "--focus", "true"}
	if cwd != "" {
		args = append(args, "--cwd", cwd)
	}
	_, err := runCmux(args...)
	return err
}
