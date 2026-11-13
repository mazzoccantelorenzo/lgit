package main

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/loreenzo/lorenzogit/core"
)

// TestDiffViewportBoundsMatchesRenderedScreen verifies that computed mouse bounds align with the rendered diff text across layouts.
func TestDiffViewportBoundsMatchesRenderedScreen(t *testing.T) {
	sampleDiff := strings.Join([]string{
		"diff --git a/service.go b/service.go",
		"--- a/service.go",
		"+++ b/service.go",
		"@@ -1,3 +1,3 @@",
		" func handleRequest() {",
		"-\toldImplementation()",
		"+\tnewImplementation()",
		" }",
	}, "\n")

	testCases := []struct {
		name   string
		state  int
		width  int
		height int
	}{
		{name: "split commit preview", state: 0, width: 160, height: 40},
		{name: "compact commit preview", state: 0, width: 100, height: 24},
		{name: "commit detail view", state: 1, width: 160, height: 40},
		{name: "compact commit detail", state: 1, width: 50, height: 24},
		{name: "pull request detail view", state: 2, width: 160, height: 40},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			selectedCommit := core.Commit{ID: "abc1234", Message: "refactor service handler"}
			current := model{
				commitList: list.New([]list.Item{commitItem{commit: selectedCommit}}, newCustomDelegate(false), 0, 0),
				fileList: list.New([]list.Item{
					fileItem{file: core.FileChange{Status: "M", Path: "committed_file_one.go"}},
					fileItem{file: core.FileChange{Status: "M", Path: "committed_file_two.go"}},
				}, newFileDelegate(), 0, 0),
				diffView:       viewport.New(0, 0),
				state:          testCase.state,
				selectedCommit: selectedCommit,
				branches:       []string{"feature/selection"},
				baseBranch:     "main",
			}
			current.setDiffContent(sampleDiff, true)

			updated, _ := current.Update(tea.WindowSizeMsg{Width: testCase.width, Height: testCase.height})
			current = updated.(model)

			left, top, width, height, visible := current.diffViewportBounds()
			if !visible || width <= 0 || height <= 0 {
				t.Fatalf("expected visible diff viewport, got visible=%v (%d×%d at %d,%d)", visible, width, height, left, top)
			}

			screenRows := strings.Split(current.View(), "\n")
			if top >= len(screenRows) {
				t.Fatalf("diff top %d exceeds rendered screen height %d", top, len(screenRows))
			}

			firstDiffRow := ansi.Strip(screenRows[top])
			expectedFirstLine := "diff --git a/service.go b/service.go"
			byteIndex := strings.Index(firstDiffRow, expectedFirstLine[:10])
			if byteIndex < 0 {
				t.Fatalf("diff row %q does not contain expected header %q", firstDiffRow, expectedFirstLine[:10])
			}
			visualColumn := ansi.StringWidth(firstDiffRow[:byteIndex])
			if visualColumn != left {
				t.Fatalf("diff text starts at visual column %d in row %q, want left bound %d", visualColumn, firstDiffRow, left)
			}
		})
	}
}

// TestMouseDragSelectsOnlyDiffCodeWithoutSidebar verifies that multi-line mouse selection ignores the file list and copies clean code.
func TestMouseDragSelectsOnlyDiffCodeWithoutSidebar(t *testing.T) {
	sampleDiff := strings.Join([]string{
		"diff --git a/handler.go b/handler.go",
		"--- a/handler.go",
		"+++ b/handler.go",
		"@@ -10,4 +10,4 @@",
		" func executeAction() {",
		"+\tperformFirstStep()",
		"+\tperformSecondStep()",
		" }",
	}, "\n")

	var copiedText string
	selectedCommit := core.Commit{ID: "99aa11b", Message: "add execution steps"}
	current := model{
		commitList: list.New([]list.Item{commitItem{commit: selectedCommit}}, newCustomDelegate(false), 0, 0),
		fileList: list.New([]list.Item{
			fileItem{file: core.FileChange{Status: "M", Path: "sidebar_committed_file.go"}},
			fileItem{file: core.FileChange{Status: "A", Path: "another_committed_file.go"}},
		}, newFileDelegate(), 0, 0),
		diffView:       viewport.New(0, 0),
		state:          1,
		selectedCommit: selectedCommit,
		branches:       []string{"main"},
		clipboardWriter: func(text string) error {
			copiedText = text
			return nil
		},
	}
	current.setDiffContent(sampleDiff, true)

	updated, _ := current.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	current = updated.(model)

	left, top, _, _, visible := current.diffViewportBounds()
	if !visible {
		t.Fatal("expected diff viewport to be visible")
	}

	// Drag from line 4 (" func executeAction() {") down to line 7 (" }").
	pressEvent := tea.MouseMsg{
		X:      left + 1,
		Y:      top + 4,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
	}
	updated, _ = current.Update(pressEvent)
	current = updated.(model)

	motionEvent := tea.MouseMsg{
		X:      left + 30,
		Y:      top + 7,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionMotion,
	}
	updated, _ = current.Update(motionEvent)
	current = updated.(model)

	releaseEvent := tea.MouseMsg{
		X:      left + 30,
		Y:      top + 7,
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
	}
	updated, _ = current.Update(releaseEvent)
	current = updated.(model)

	expectedCopiedCode := strings.Join([]string{
		"func executeAction() {",
		"\tperformFirstStep()",
		"\tperformSecondStep()",
		"}",
	}, "\n")
	if copiedText != expectedCopiedCode {
		t.Fatalf("copied text = %q, want %q", copiedText, expectedCopiedCode)
	}
	if strings.Contains(copiedText, "sidebar_committed_file.go") || strings.Contains(copiedText, "another_committed_file.go") {
		t.Fatal("copied diff selection included committed file names from the sidebar")
	}
}

// TestSingleLineSubstringSelectionAndClear verifies partial line selection and clearing on single click.
func TestSingleLineSubstringSelectionAndClear(t *testing.T) {
	sampleDiff := strings.Join([]string{
		"@@ -1,1 +1,1 @@",
		"+const targetIdentifier = 42",
	}, "\n")

	var copiedText string
	current := model{
		commitList: list.New(nil, newCustomDelegate(false), 0, 0),
		fileList:   list.New([]list.Item{fileItem{file: core.FileChange{Status: "M", Path: "constants.go"}}}, newFileDelegate(), 0, 0),
		diffView:   viewport.New(0, 0),
		state:      1,
		branches:   []string{"main"},
		clipboardWriter: func(text string) error {
			copiedText = text
			return nil
		},
	}
	current.setDiffContent(sampleDiff, true)

	updated, _ := current.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	current = updated.(model)

	left, top, _, _, _ := current.diffViewportBounds()

	// Select "targetIdentifier" on line 1 ("+const targetIdentifier = 42"), which spans columns 7..22.
	updated, _ = current.Update(tea.MouseMsg{X: left + 7, Y: top + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	current = updated.(model)
	updated, _ = current.Update(tea.MouseMsg{X: left + 22, Y: top + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion})
	current = updated.(model)
	updated, _ = current.Update(tea.MouseMsg{X: left + 22, Y: top + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	current = updated.(model)

	if copiedText != "targetIdentifier" {
		t.Fatalf("copied single-line substring = %q, want %q", copiedText, "targetIdentifier")
	}
	if !current.selection.hasRange() {
		t.Fatal("expected selection to remain visible after mouse release")
	}

	// A single click without motion clears the selection.
	updated, _ = current.Update(tea.MouseMsg{X: left + 2, Y: top + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	current = updated.(model)
	updated, _ = current.Update(tea.MouseMsg{X: left + 2, Y: top + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	current = updated.(model)

	if current.selection.active {
		t.Fatal("single click without dragging did not clear the selection")
	}
}
