package main

import (
	"fmt"
	"io"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type fileDelegate struct {
	Styles list.DefaultItemStyles
}

func newFileDelegate() fileDelegate {
	s := list.NewDefaultItemStyles()
	s.SelectedTitle = s.SelectedTitle.Foreground(lipgloss.Color("#58a6ff")).Border(lipgloss.HiddenBorder()).Padding(0, 0, 0, 1)
	s.NormalTitle = s.NormalTitle.Foreground(lipgloss.Color("#c9d1d9")).Padding(0, 0, 0, 1)
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

	title := f.Title()
	
	if index == m.Index() {
		title = d.Styles.SelectedTitle.Render("┃ " + title)
	} else {
		title = d.Styles.NormalTitle.Render("  " + title)
	}

	fmt.Fprint(w, title)
}
