package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/wordwrap"
)

type customDelegate struct {
	Styles list.DefaultItemStyles
}

func newCustomDelegate() customDelegate {
	return customDelegate{Styles: list.NewDefaultItemStyles()}
}

func (d customDelegate) Height() int {
	return 6
}

func (d customDelegate) Spacing() int {
	return 1
}

func (d customDelegate) Update(msg tea.Msg, m *list.Model) tea.Cmd {
	return nil
}

func (d customDelegate) Render(w io.Writer, m list.Model, index int, listItem list.Item) {
	i, ok := listItem.(commitItem)
	if !ok {
		return
	}

	title := i.Title()
	desc := fmt.Sprintf("%s • %s", i.commit.Author, i.commit.Date)

	bodyLines := []string{}
	if i.commit.Body != "" {
		wrappedBody := wordwrap.String(strings.TrimSpace(i.commit.Body), m.Width()-6)
		bodyLines = strings.Split(wrappedBody, "\n")
	}

	for j := 0; j < 4; j++ {
		if j < len(bodyLines) {
			desc += "\n" + bodyLines[j]
		} else {
			desc += "\n"
		}
	}

	if len(bodyLines) > 4 {
		desc = desc[:len(desc)-3] + "..."
	}

	if index == m.Index() {
		title = d.Styles.SelectedTitle.Render("> " + title)
		desc = d.Styles.SelectedDesc.Render(strings.ReplaceAll(desc, "\n", "\n  "))
		desc = "  " + desc
	} else {
		title = d.Styles.NormalTitle.Render("  " + title)
		desc = d.Styles.NormalDesc.Render(strings.ReplaceAll(desc, "\n", "\n  "))
		desc = "  " + desc
	}

	block := fmt.Sprintf("%s\n%s", title, desc)
	block = lipgloss.NewStyle().MaxWidth(m.Width() - 2).Render(block)

	fmt.Fprint(w, block)
}
