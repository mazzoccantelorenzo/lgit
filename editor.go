package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// editorFinishedMsg is the result delivered when Neovim returns control to lgit.
type editorFinishedMsg struct {
	err error
}

// openFileInEditor is the command that opens a repository file while preserving the current UI model.
func openFileInEditor(filePath string) tea.Cmd {
	return func() tea.Msg {
		repositoryCommand := exec.Command("git", "rev-parse", "--show-toplevel")
		repositoryOutput, err := repositoryCommand.Output()
		if err != nil {
			return editorFinishedMsg{err: fmt.Errorf("find repository root: %w", err)}
		}

		repositoryRoot := strings.TrimSpace(string(repositoryOutput))
		editorCommand := exec.Command("nvim", "--", filepath.Join(repositoryRoot, filePath))
		editorCommand.Dir = repositoryRoot
		return tea.ExecProcess(editorCommand, func(err error) tea.Msg {
			return editorFinishedMsg{err: err}
		})()
	}
}
