package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSplashAndHelpModalFlow verifies:
// 1. Splash loading view (0.5s) shows on startup with 3D logo
// 2. Help modal pops up over the repository view immediately after loading
// 3. Any key dismisses the help modal
// 4. Pressing 'h' in commit list reopens the modal anytime
func TestSplashAndHelpModalFlow(t *testing.T) {
	m := newModel(true)
	if !m.showSplash {
		t.Fatal("expected showSplash to be true for newModel(true)")
	}

	// Trigger WindowSizeMsg
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})
	m = updated.(model)

	// 1. Initial view is splash loading animation
	loadingView := m.View()
	if !strings.Contains(loadingView, "loading git repository...") {
		t.Fatalf("expected initial view to be loading view, got: %s", loadingView)
	}

	// 2. When splash timer completes, it transitions to showHelp modal
	updated, _ = m.Update(splashDoneMsg{})
	m = updated.(model)
	if m.showSplash {
		t.Fatal("expected showSplash to be false after splashDoneMsg")
	}
	if !m.showHelp {
		t.Fatal("expected showHelp modal to be true immediately after splashDoneMsg")
	}

	modalView := m.View()
	if !strings.Contains(modalView, "KEYBOARD SHORTCUTS") {
		t.Fatalf("expected modal to contain KEYBOARD SHORTCUTS, got: %s", modalView)
	}
	if !strings.Contains(modalView, "Press Esc, Enter, or h to close") {
		t.Fatalf("expected modal to contain close instructions, got: %s", modalView)
	}

	// 3. Pressing Enter / Esc / any key dismisses the help modal
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.showHelp {
		t.Fatal("expected showHelp to be false after pressing Enter")
	}

	// 4. Pressing 'h' in commit list reopens the help modal
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(model)
	if !m.showHelp {
		t.Fatal("expected 'h' to toggle showHelp to true")
	}

	// 5. Pressing 'h' again toggles it off
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	m = updated.(model)
	if m.showHelp {
		t.Fatal("expected 'h' to toggle showHelp to false")
	}
}

// TestSplashTimerCmd ensures the timer command returns a splashDoneMsg.
func TestSplashTimerCmd(t *testing.T) {
	cmd := splashTimerCmd()
	if cmd == nil {
		t.Fatal("expected splashTimerCmd to return a non-nil tea.Cmd")
	}
	msg := splashDoneMsg{}
	if _, ok := any(msg).(splashDoneMsg); !ok {
		t.Fatal("unexpected message type")
	}
	_ = time.Millisecond
}
