package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestTableList() TableList {
	tl := NewTableList()
	tl.SetTables([]string{"users", "orders", "products", "categories", "order_items"}, nil)
	tl.SetSize(20, 10)
	return tl
}

func TestTableList_FuzzyFilter_CursorPreservedOnArrowKeys(t *testing.T) {
	tl := newTestTableList()
	tl.Focus()
	tl.StartFiltering()

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})

	if len(tl.filtered) == 0 {
		t.Fatal("expected filtered results for 'o'")
	}

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyDown})

	if tl.cursor != 1 {
		t.Errorf("expected cursor at 1 after down, got %d", tl.cursor)
	}

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyDown})

	if tl.cursor < 1 {
		t.Errorf("expected cursor >= 1 after second down, got %d", tl.cursor)
	}

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyUp})

	expected := tl.cursor
	if expected > 1 {
		t.Errorf("expected cursor to move up, got %d", expected)
	}
}

func TestTableList_FuzzyFilter_CursorNotResetOnBlinkTick(t *testing.T) {
	tl := newTestTableList()
	tl.Focus()
	tl.StartFiltering()

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyDown})

	cursorBefore := tl.cursor

	tl, _ = tl.Update(struct{}{})

	if tl.cursor != cursorBefore {
		t.Errorf("expected cursor %d after non-key message, got %d", cursorBefore, tl.cursor)
	}
}

func TestTableList_FuzzyFilter_CursorResetsOnNewText(t *testing.T) {
	tl := newTestTableList()
	tl.Focus()
	tl.StartFiltering()

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyDown})
	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyDown})

	cursorBefore := tl.cursor
	if cursorBefore == 0 {
		t.Skip("not enough filtered items to move cursor")
	}

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})

	if tl.cursor != 0 {
		t.Errorf("expected cursor reset to 0 after new text, got %d", tl.cursor)
	}
}

func TestTableList_FuzzyFilter_EscCancels(t *testing.T) {
	tl := newTestTableList()
	tl.Focus()
	tl.StartFiltering()

	if !tl.filtering {
		t.Fatal("expected filtering to be true")
	}

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if tl.filtering {
		t.Error("expected filtering to be false after Esc")
	}
}

func TestTableList_FuzzyFilter_EnterSelects(t *testing.T) {
	tl := newTestTableList()
	tl.Focus()
	tl.StartFiltering()

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if tl.selected == "" {
		t.Error("expected a table to be selected after Enter")
	}
	if tl.filtering {
		t.Error("expected filtering to stop after Enter")
	}
}

func TestTableList_NormalMode_JKNavigation(t *testing.T) {
	tl := newTestTableList()
	tl.Focus()

	if tl.cursor != 0 {
		t.Errorf("expected initial cursor 0, got %d", tl.cursor)
	}

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	if tl.cursor != 1 {
		t.Errorf("expected cursor 1 after j, got %d", tl.cursor)
	}

	tl, _ = tl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	if tl.cursor != 0 {
		t.Errorf("expected cursor 0 after k, got %d", tl.cursor)
	}
}
