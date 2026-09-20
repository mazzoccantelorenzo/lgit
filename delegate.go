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
	Styles   list.DefaultItemStyles
	Expanded bool
}

func newCustomDelegate(expanded bool) customDelegate {
	return customDelegate{Styles: list.NewDefaultItemStyles(), Expanded: expanded}
}

func (d customDelegate) Height() int {
	if d.Expanded {
		return 14
	}
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

	commitID := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Render(i.commit.ID)
	commitMsg := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Render(i.commit.Message)

	title := fmt.Sprintf("%s  %s", commitID, commitMsg)
	descStr := fmt.Sprintf("%s • %s", i.commit.Author, i.commit.Date)

	bodyLines := []string{}
	if i.commit.Body != "" {
		wrappedBody := wordwrap.String(strings.TrimSpace(i.commit.Body), m.Width()-6)
		bodyLines = strings.Split(wrappedBody, "\n")
	}

	maxLines := 4
	if d.Expanded {
		maxLines = 12
	}

	for j := 0; j < maxLines; j++ {
		if j < len(bodyLines) {
			descStr += "\n" + bodyLines[j]
		} else {
			descStr += "\n"
		}
	}

	if len(bodyLines) > maxLines {
		descStr = descStr[:len(descStr)-3] + "..."
	}

	// Optional: render description in white as well if requested "body bianco"
	descWhite := lipgloss.NewStyle().Foreground(lipgloss.Color("15")).Render(descStr)

	if index == m.Index() {
		title = d.Styles.SelectedTitle.Render("> ") + title
		descWhite = d.Styles.SelectedDesc.Render(strings.ReplaceAll(descWhite, "\n", "\n  "))
		descWhite = "  " + descWhite
	} else {
		title = d.Styles.NormalTitle.Render("  ") + title
		descWhite = d.Styles.NormalDesc.Render(strings.ReplaceAll(descWhite, "\n", "\n  "))
		descWhite = "  " + descWhite
	}

	block := fmt.Sprintf("%s\n%s", title, descWhite)
	block = lipgloss.NewStyle().MaxWidth(m.Width() - 2).Render(block)

	fmt.Fprint(w, block)
}
