package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/loreenzo/lorenzogit/core"
)

// editorRoundTripModel is the test harness that stops after the editor hands control back to lgit.
type editorRoundTripModel struct {
	model
	command tea.Cmd
	err     error
}

// Init is the command that starts the requested editor round trip without background refreshes.
func (current editorRoundTripModel) Init() tea.Cmd {
	return current.command
}

// Update is the message handler that exercises lgit's editor completion before ending the test program.
func (current *editorRoundTripModel) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	updated, command := current.model.Update(message)
	current.model = updated.(model)
	if finished, ok := message.(editorFinishedMsg); ok {
		current.err = finished.err
		return current, tea.Quit
	}
	return current, command
}

// TestEditorRoundTrip is the integration check for opening the selected file and preserving the current view.
func TestEditorRoundTrip(t *testing.T) {
	testCases := []struct {
		name      string
		state     int
		diffFocus bool
		width     int
		exitCode  string
	}{
		{name: "commit files", state: 1, width: 160, exitCode: "0"},
		{name: "commit diff", state: 1, diffFocus: true, width: 160, exitCode: "0"},
		{name: "PR files", state: 2, width: 160, exitCode: "0"},
		{name: "PR diff", state: 2, diffFocus: true, width: 160, exitCode: "0"},
		{name: "compact files", state: 1, width: 50, exitCode: "0"},
		{name: "editor failure", state: 1, width: 160, exitCode: "1"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
			t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
			t.Chdir(t.TempDir())
			repositoryRoot, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			repositoryRoot, err = filepath.EvalSymlinks(repositoryRoot)
			if err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("git", "init", "-q").CombinedOutput(); err != nil {
				t.Fatalf("initialize repository: %v\n%s", err, output)
			}
			if err := os.Mkdir("nested", 0o700); err != nil {
				t.Fatal(err)
			}
			filePath := "nested/target file with spaces.txt"
			if err := os.WriteFile(filePath, []byte("original contents\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			if output, err := exec.Command("git", "add", ".").CombinedOutput(); err != nil {
				t.Fatalf("stage selected file: %v\n%s", err, output)
			}
			if output, err := exec.Command("git", "-c", "user.name=Test Author", "-c", "user.email=test@example.com", "commit", "-q", "-m", "initial file").CombinedOutput(); err != nil {
				t.Fatalf("commit selected file: %v\n%s", err, output)
			}

			editorDirectory := t.TempDir()
			invocationPath := filepath.Join(editorDirectory, "invocation")
			editorScript := `#!/bin/sh
printf '%s\n' "$PWD" "$@" > "$LGIT_EDITOR_INVOCATION"
if [ "$LGIT_EDITOR_EXIT_CODE" != 0 ]; then exit "$LGIT_EDITOR_EXIT_CODE"; fi
printf 'edited locally\n' >> "$2"
`
			if err := os.WriteFile(filepath.Join(editorDirectory, "nvim"), []byte(editorScript), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", editorDirectory+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("LGIT_EDITOR_INVOCATION", invocationPath)
			t.Setenv("LGIT_EDITOR_EXIT_CODE", testCase.exitCode)
			t.Chdir(filepath.Join(repositoryRoot, "nested"))

			selectedCommit := core.Commit{ID: "abc123", Message: "selected commit"}
			current := model{
				commitList: list.New([]list.Item{commitItem{commit: selectedCommit}}, newCustomDelegate(false), 0, 0),
				fileList: list.New([]list.Item{
					fileItem{file: core.FileChange{Status: "M", Path: "other.txt"}},
					fileItem{file: core.FileChange{Status: "M", Path: "nested/target first.txt"}},
					fileItem{file: core.FileChange{Status: "M", Path: filePath}},
				}, newFileDelegate(), 0, 0),
				diffView:       viewport.New(0, 0),
				state:          testCase.state,
				diffFocus:      testCase.diffFocus,
				selectedCommit: selectedCommit,
				branches:       []string{"feature/editor"},
				baseBranch:     "main",
				expanded:       true,
			}
			updated, _ := current.Update(tea.WindowSizeMsg{Width: testCase.width, Height: 40})
			current = updated.(model)
			current.fileList.SetFilterText("target")
			current.fileList.Select(1)
			current.diffView.SetContent(strings.Repeat("+diff content\n", 100))
			current.diffView.SetYOffset(17)
			originalScreen := current.View()

			updated, editorCommand := current.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if editorCommand == nil {
				t.Fatal("Enter did not request the editor")
			}
			roundTrip := &editorRoundTripModel{model: updated.(model), command: editorCommand}
			programContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			program := tea.NewProgram(roundTrip, tea.WithContext(programContext), tea.WithInput(&bytes.Buffer{}), tea.WithOutput(io.Discard), tea.WithoutSignals())
			if _, err := program.Run(); err != nil {
				t.Fatal(err)
			}

			invocation, err := os.ReadFile(invocationPath)
			if err != nil {
				t.Fatal(err)
			}
			expectedInvocation := repositoryRoot + "\n--\n" + filepath.Join(repositoryRoot, filePath) + "\n"
			if string(invocation) != expectedInvocation {
				t.Fatalf("editor invocation = %q, want %q", invocation, expectedInvocation)
			}
			if roundTrip.state != current.state || roundTrip.diffFocus != current.diffFocus || roundTrip.selectedCommit != selectedCommit || !roundTrip.expanded {
				t.Fatal("returning from the editor changed the current view or commit")
			}
			if selectedFilePath(roundTrip.fileList) != filePath || roundTrip.fileList.FilterValue() != "target" || roundTrip.fileList.FilterState() != list.FilterApplied || roundTrip.fileList.Index() != 1 || roundTrip.diffView.YOffset != 17 {
				t.Fatal("returning from the editor changed the file selection, filter, or scroll position")
			}
			if testCase.exitCode != "0" {
				if roundTrip.err == nil || !strings.Contains(roundTrip.View(), "Could not open nvim:") {
					t.Fatal("editor failure was not reported in lgit")
				}
				return
			}
			if roundTrip.err != nil {
				t.Fatal(roundTrip.err)
			}
			if roundTrip.View() == originalScreen || !strings.Contains(roundTrip.View(), "local changes found") {
				t.Fatal("returning from the editor did not show the local changes warning")
			}
			contents, err := os.ReadFile(filepath.Join(repositoryRoot, filePath))
			if err != nil || string(contents) != "original contents\nedited locally\n" {
				t.Fatalf("editor did not modify the selected working-tree file: %q, %v", contents, err)
			}
		})
	}
}

// TestEnterAcceptsFileFilter is the regression check that confirming a search does not launch Neovim.
func TestEnterAcceptsFileFilter(t *testing.T) {
	current := model{
		state: 1,
		fileList: list.New([]list.Item{
			fileItem{file: core.FileChange{Status: "M", Path: "target.txt"}},
		}, newFileDelegate(), 80, 20),
	}
	current.fileList.SetFilterText("target")
	current.fileList.SetFilterState(list.Filtering)
	updated, command := current.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.(model).fileList.FilterState() != list.FilterApplied {
		t.Fatal("Enter did not apply the file filter")
	}
	if command != nil {
		t.Fatal("confirming the filter started an unexpected command")
	}
}
