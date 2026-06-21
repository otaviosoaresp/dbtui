package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestQuitNormalModeAsksConfirm(t *testing.T) {
	a := App{}

	model, cmd := a.handleNormalMode(keyMsg("q"))
	got := model.(App)

	if !got.quitConfirm {
		t.Error("expected quitConfirm true after q in normal mode")
	}
	if isQuitCmd(cmd) {
		t.Error("expected no quit cmd, app must wait for confirmation")
	}
}

func TestQuitConfirmYesQuits(t *testing.T) {
	a := App{quitConfirm: true}

	_, cmd := a.handleQuitConfirm(keyMsg("y"))

	if !isQuitCmd(cmd) {
		t.Error("expected tea.Quit on y")
	}
}

func TestQuitConfirmCancel(t *testing.T) {
	for _, key := range []string{"n", "esc"} {
		a := App{quitConfirm: true}
		model, cmd := a.handleQuitConfirm(keyMsg(key))
		got := model.(App)

		if got.quitConfirm {
			t.Errorf("key %q: expected quitConfirm false after cancel", key)
		}
		if isQuitCmd(cmd) {
			t.Errorf("key %q: expected no quit cmd on cancel", key)
		}
	}
}
