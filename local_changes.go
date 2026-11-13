package main

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/loreenzo/lorenzogit/core"
)

const localChangesWarning = "local changes found, be aware of uncommitted changes."

// refreshLocalChanges is the status refresh used at startup, on ticks, and after editing a file.
func (m *model) refreshLocalChanges() {
	changes, err := core.FetchLocalChanges()
	if err != nil {
		return
	}
	filePath := selectedFilePath(m.fileList)
	previouslyChanged := m.localChanges[filePath]
	m.localChanges = changes
	if previouslyChanged != changes[filePath] {
		m.applyLayout()
	}
}

// renderLocalChangesWarning is the red notice shown below the selected file's diff.
func (m model) renderLocalChangesWarning(width, height int) string {
	if width < 1 || height < 1 || !m.localChanges[selectedFilePath(m.fileList)] {
		return ""
	}
	warning := lipgloss.NewStyle().Foreground(lipgloss.Color("#f85149")).Width(width).Render(localChangesWarning)
	return clipRows(warning, height)
}
