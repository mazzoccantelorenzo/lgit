package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/loreenzo/lorenzogit/core"
)

// UI Constants
var (
	docStyle     = lipgloss.NewStyle().Margin(1, 2)
	sidebarStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363d")).Padding(1, 2)
	diffStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363d")).Padding(1, 2)
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d1d9")).Bold(true).PaddingBottom(1)
	bodyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e")).PaddingBottom(1).Italic(true)
)

type commitItem struct {
	commit core.Commit
}

func (i commitItem) Title() string { return fmt.Sprintf("%s  %s", i.commit.ID, i.commit.Message) }
func (i commitItem) Description() string {
	return fmt.Sprintf("%s • %s", i.commit.Author, i.commit.Date)
}
func (i commitItem) FilterValue() string { return i.commit.Message }

type fileItem struct {
	file core.FileChange
}

func (f fileItem) Title() string       { return fmt.Sprintf("[%s] %s", f.file.Status, f.file.Path) }
func (f fileItem) Description() string { return "" }
func (f fileItem) FilterValue() string { return f.file.Path }

type model struct {
	commitList list.Model
	fileList   list.Model
	diffView   viewport.Model

	state          int
	width          int
	height         int
	selectedCommit core.Commit
}

func initialModel() model {
	commits, _ := core.FetchCommits()

	var items []list.Item
	for _, c := range commits {
		items = append(items, commitItem{commit: c})
	}

	m := list.New(items, newCustomDelegate(), 0, 0)
	m.Title = "Git Log TUI"
	m.SetShowStatusBar(false)

	fList := list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0)
	fList.Title = "Files Changed"
	fList.SetShowStatusBar(false)

	vp := viewport.New(0, 0)

	return model{
		commitList: m,
		fileList:   fList,
		diffView:   vp,
		state:      0,
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

		if m.state == 0 {
			if msg.String() == "right" || msg.String() == "enter" {
				if i, ok := m.commitList.SelectedItem().(commitItem); ok {
					m.selectedCommit = i.commit
					files, _ := core.FetchCommitFiles(m.selectedCommit.ID)

					var fItems []list.Item
					for _, f := range files {
						fItems = append(fItems, fileItem{file: f})
					}
					m.fileList.SetItems(fItems)

					if len(files) > 0 {
						diff, _ := core.FetchFileDiff(m.selectedCommit.ID, files[0].Path)
						m.diffView.SetContent(colorizeDiff(diff))
					} else {
						m.diffView.SetContent("No diff available.")
					}

					m.state = 1
					return m, nil
				}
			}
			m.commitList, cmd = m.commitList.Update(msg)
			cmds = append(cmds, cmd)

		} else if m.state == 1 {
			if msg.String() == "left" || msg.String() == "esc" {
				m.state = 0
				return m, nil
			}

			oldIndex := m.fileList.Index()
			m.fileList, cmd = m.fileList.Update(msg)
			cmds = append(cmds, cmd)

			if m.fileList.Index() != oldIndex {
				if f, ok := m.fileList.SelectedItem().(fileItem); ok {
					diff, _ := core.FetchFileDiff(m.selectedCommit.ID, f.file.Path)
					m.diffView.SetContent(colorizeDiff(diff))
					m.diffView.GotoTop()
				}
			}

			m.diffView, cmd = m.diffView.Update(msg)
			cmds = append(cmds, cmd)
		}

	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.width = msg.Width - h
		m.height = msg.Height - v

		m.commitList.SetSize(m.width, m.height)
		m.fileList.SetSize(m.width/3, m.height)

		m.diffView.Width = m.width - (m.width / 3) - 6
		m.diffView.Height = m.height - 4 - 3 // leave space for header and body
	}

	return m, tea.Batch(cmds...)
}

func colorizeDiff(diff string) string {
	lines := strings.Split(diff, "\n")
	var out []string

	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7ee787"))
	rmStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffa198"))
	hStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#79c0ff"))

	for _, l := range lines {
		if strings.HasPrefix(l, "+") && !strings.HasPrefix(l, "+++") {
			out = append(out, addStyle.Render(l))
		} else if strings.HasPrefix(l, "-") && !strings.HasPrefix(l, "---") {
			out = append(out, rmStyle.Render(l))
		} else if strings.HasPrefix(l, "@@") {
			out = append(out, hStyle.Render(l))
		} else {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func (m model) View() string {
	if m.state == 0 {
		return docStyle.Render(m.commitList.View())
	}

	left := sidebarStyle.Width(m.width / 3).Height(m.height).Render(m.fileList.View())

	header := headerStyle.Render(m.selectedCommit.Message)
	body := ""
	if m.selectedCommit.Body != "" {
		body = bodyStyle.Render(m.selectedCommit.Body)
	}

	diffContent := lipgloss.JoinVertical(lipgloss.Left, header, body, m.diffView.View())
	right := diffStyle.Width(m.width - (m.width / 3) - 2).Height(m.height).Render(diffContent)

	return docStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, left, right))
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Errore TUI: %v", err)
		os.Exit(1)
	}
}
