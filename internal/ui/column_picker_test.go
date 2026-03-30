package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestColumnPicker() ColumnPicker {
	cp := NewColumnPicker()
	cp.Show([]string{"id", "name", "email", "customer_id", "status", "created_at"}, 60, 20)
	return cp
}

func TestColumnPicker_CursorPreservedOnArrowKeys(t *testing.T) {
	cp := newTestColumnPicker()

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})

	if cp.cursor != 1 {
		t.Errorf("expected cursor 1 after down, got %d", cp.cursor)
	}

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})

	if cp.cursor != 2 {
		t.Errorf("expected cursor 2 after second down, got %d", cp.cursor)
	}

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyUp})

	if cp.cursor != 1 {
		t.Errorf("expected cursor 1 after up, got %d", cp.cursor)
	}
}

func TestColumnPicker_CursorNotResetOnNonKeyMsg(t *testing.T) {
	cp := newTestColumnPicker()

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})
	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})

	cursorBefore := cp.cursor

	cp.applyFilter()

	if cp.cursor != cursorBefore {
		t.Errorf("expected cursor %d after applyFilter with no text change, got %d", cursorBefore, cp.cursor)
	}
}

func TestColumnPicker_CursorResetsOnNewText(t *testing.T) {
	cp := newTestColumnPicker()

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})
	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})

	if cp.cursor == 0 {
		t.Fatal("cursor should have moved from 0")
	}

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})

	if cp.cursor != 0 {
		t.Errorf("expected cursor reset to 0 after typing, got %d", cp.cursor)
	}
}

func TestColumnPicker_FuzzyFilter(t *testing.T) {
	cp := newTestColumnPicker()

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if len(cp.filtered) == 0 {
		t.Fatal("expected filtered results for 'id'")
	}

	found := false
	for _, col := range cp.filtered {
		if col == "id" || col == "customer_id" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'id' or 'customer_id' in filtered results")
	}
}

func TestColumnPicker_EscCloses(t *testing.T) {
	cp := newTestColumnPicker()

	if !cp.visible {
		t.Fatal("expected visible after Show")
	}

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if cp.visible {
		t.Error("expected hidden after Esc")
	}
}

func TestColumnPicker_EnterSelects(t *testing.T) {
	cp := newTestColumnPicker()

	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyDown})
	cp, _ = cp.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if cp.selected == "" {
		t.Error("expected a column to be selected")
	}
	if cp.selected != "name" {
		t.Errorf("expected 'name' selected (index 1), got %q", cp.selected)
	}
	if cp.visible {
		t.Error("expected hidden after Enter")
	}
}
