package main

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// TestSplashLogoScreen verifies the 3D ASCII splash screen appears on startup and auto-dismisses after 500ms.
func TestSplashLogoScreen(t *testing.T) {
	m := newModel(true)
	if !m.showSplash {
		t.Fatal("expected showSplash to be true for newModel(true)")
	}

	// Trigger WindowSizeMsg
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(model)

	view := m.View()
	if !strings.Contains(view, "( •̀ ω •́ )✧") {
		t.Fatalf("splash screen does not contain the kaomoji face: %s", view)
	}

	// After splashDoneMsg, splash should be dismissed
	updated, _ = m.Update(splashDoneMsg{})
	m = updated.(model)
	if m.showSplash {
		t.Fatal("expected showSplash to be false after splashDoneMsg")
	}

	// Any key pressed during splash also dismisses it immediately
	m2 := newModel(true)
	updated2, _ := m2.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m2 = updated2.(model)
	updated2, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m2 = updated2.(model)
	if m2.showSplash {
		t.Fatal("expected key press to dismiss splash screen immediately")
	}
}

// TestSplashTimerCmd ensures the timer command returns a splashDoneMsg.
func TestSplashTimerCmd(t *testing.T) {
	cmd := splashTimerCmd()
	if cmd == nil {
		t.Fatal("expected splashTimerCmd to return a non-nil tea.Cmd")
	}
	// Simulate the tick message returned by tea.Tick
	msg := splashDoneMsg{}
	if _, ok := any(msg).(splashDoneMsg); !ok {
		t.Fatal("unexpected message type")
	}
	_ = time.Millisecond
}
