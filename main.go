package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"ccresume/internal/ui"
)

func main() {
	p := tea.NewProgram(ui.New(), tea.WithAltScreen())
	final, err := p.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "エラー:", err)
		os.Exit(1)
	}
	app, ok := final.(ui.App)
	if !ok {
		return
	}
	if t := app.ExecTarget(); t != nil {
		// Bubble Tea がターミナルを復元した後に claude に置き換える
		if err := t.Exec(); err != nil {
			fmt.Fprintln(os.Stderr, "claude の起動に失敗:", err)
			os.Exit(1)
		}
	}
}
