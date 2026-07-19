package ui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	Resume      key.Binding
	Delete      key.Binding
	Projects    key.Binding
	Rescan      key.Binding
	Quit        key.Binding
	ScrollDown  key.Binding
	ScrollUp    key.Binding
	HalfPgDown  key.Binding
	HalfPgUp    key.Binding
	ConfirmYes  key.Binding
	CancelOrEsc key.Binding
}

var keys = keyMap{
	Resume:      key.NewBinding(key.WithKeys("enter")),
	Delete:      key.NewBinding(key.WithKeys("d", "x")),
	Projects:    key.NewBinding(key.WithKeys("tab", "p")),
	Rescan:      key.NewBinding(key.WithKeys("r")),
	Quit:        key.NewBinding(key.WithKeys("q", "ctrl+c")),
	ScrollDown:  key.NewBinding(key.WithKeys("J")),
	ScrollUp:    key.NewBinding(key.WithKeys("K")),
	HalfPgDown:  key.NewBinding(key.WithKeys("ctrl+d", "pgdown")),
	HalfPgUp:    key.NewBinding(key.WithKeys("ctrl+u", "pgup")),
	ConfirmYes:  key.NewBinding(key.WithKeys("y", "Y")),
	CancelOrEsc: key.NewBinding(key.WithKeys("esc")),
}

const helpBrowse = "enter:呼び出し  d:削除  tab:ディレクトリ  /:検索  J/K:プレビュー  r:再読込  q:終了"
const helpProjects = "enter:選択  /:検索  esc:戻る  q:終了"
