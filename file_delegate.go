package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type fileDelegate struct {
	Styles list.DefaultItemStyles
}

func newFileDelegate() fileDelegate {
	s := list.NewDefaultItemStyles()
	return fileDelegate{Styles: s}
}

func (d fileDelegate) Height() int {
	return 1
}

func (d fileDelegate) Spacing() int {
	return 0
}

func (d fileDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d fileDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	f, ok := listItem.(fileItem)
	if !ok {
		return
	}

	icon := "📄"
	color := lipgloss.Color("#c9d1d9")
	status := f.file.Status

	if strings.HasPrefix(status, "M") {
		icon = "✎ "
		color = lipgloss.Color("#d29922") // yellow
	} else if strings.HasPrefix(status, "A") {
		icon = "+ "
		color = lipgloss.Color("#2ea043") // green
	} else if strings.HasPrefix(status, "D") {
		icon = "- "
		color = lipgloss.Color("#f85149") // red
	}

	pathParts := strings.Split(f.file.Path, "/")
	filename := pathParts[len(pathParts)-1]
	dir := ""
	if len(pathParts) > 1 {
		dir = strings.Join(pathParts[:len(pathParts)-1], "/") + "/"
	}

	dirStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e"))
	fileStyle := lipgloss.NewStyle().Foreground(color)

	line := fmt.Sprintf("%s %s%s", icon, dirStyle.Render(dir), fileStyle.Render(filename))

	if index == m.Index() {
		line = "┃ " + line
		line = lipgloss.NewStyle().Background(lipgloss.Color("#21262d")).Render(line)
	} else {
		line = "  " + line
	}

	block := lipgloss.NewStyle().MaxWidth(m.Width() - 2).Render(line)
	fmt.Fprint(w, block)
}
