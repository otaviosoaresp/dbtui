package ui

import (
	"strings"
	"testing"
)

func TestHelpOverlay_Toggle(t *testing.T) {
	var h HelpOverlay

	if h.Visible() {
		t.Error("expected help to be hidden initially")
	}

	h.Toggle()
	if !h.Visible() {
		t.Error("expected help to be visible after toggle")
	}

	h.Toggle()
	if h.Visible() {
		t.Error("expected help to be hidden after second toggle")
	}
}

func TestHelpOverlay_Hide(t *testing.T) {
	var h HelpOverlay
	h.Toggle()
	h.Hide()

	if h.Visible() {
		t.Error("expected help to be hidden after Hide()")
	}
}

func TestHelpOverlay_View_Hidden(t *testing.T) {
	var h HelpOverlay
	h.SetSize(80, 24)

	view := h.View()
	if view != "" {
		t.Error("expected empty view when hidden")
	}
}

func TestHelpOverlay_View_Visible(t *testing.T) {
	var h HelpOverlay
	h.SetSize(80, 24)
	h.Toggle()

	view := h.View()
	if view == "" {
		t.Error("expected non-empty view when visible")
	}

	expectedBindings := []string{
		"j / k",
		"d / u",
		"Tab",
		"Fuzzy jump to column",
		"Record view",
		"Filter column",
		"Command mode",
		"Edit cell",
		"Quit",
	}

	for _, binding := range expectedBindings {
		if !strings.Contains(view, binding) {
			t.Errorf("expected help view to contain %q", binding)
		}
	}
}

func TestHelpOverlay_View_ZeroSize(t *testing.T) {
	var h HelpOverlay
	h.Toggle()

	view := h.View()
	if view != "" {
		t.Error("expected empty view with zero size")
	}
}
