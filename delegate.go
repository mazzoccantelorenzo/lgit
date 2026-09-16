package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/muesli/reflow/wordwrap"
)

type customDelegate struct {
	Styles list.DefaultItemStyles
}

func newCustomDelegate() customDelegate {
	return customDelegate{Styles: list.NewDefaultItemStyles()}
}

func (d customDelegate) Height() int {
	return 5
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
	if i.commit.Body != "" {
		wrappedBody := wordwrap.String(strings.TrimSpace(i.commit.Body), m.Width()-6)
		desc += "\n\n" + wrappedBody
	}

	if index == m.Index() {
		title = d.Styles.SelectedTitle.Render("> " + title)
		desc = d.Styles.SelectedDesc.Render(desc)
	} else {
		title = d.Styles.NormalTitle.Render("  " + title)
		desc = d.Styles.NormalDesc.Render(desc)
	}

	fmt.Fprintf(w, "%s\n%s", title, desc)
}
