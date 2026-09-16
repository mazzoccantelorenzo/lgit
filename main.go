package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/loreenzo/lorenzogit/core"
)

// UI Constants
var (
	docStyle     = lipgloss.NewStyle().Margin(2, 2)
	sidebarStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363d")).Padding(1, 2)
	diffStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363d")).Padding(1, 2)
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d1d9")).Bold(true).PaddingBottom(1)
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

	state          int // 0 = Commits, 1 = Commit Files/Diff, 2 = Branch Files/Diff
	width          int
	height         int
	selectedCommit core.Commit

	branches    []string
	branchIndex int
	expanded    bool
	diffFocus   bool
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func initialModel() model {
	branches, _ := core.FetchBranches()
	if len(branches) == 0 {
		branches = []string{"master"}
	}

	commits, _ := core.FetchCommits(branches[0])

	var items []list.Item
	for _, c := range commits {
		items = append(items, commitItem{commit: c})
	}

	m := list.New(items, newCustomDelegate(false), 0, 0)
	m.Title = fmt.Sprintf("Git Log TUI  [ Tab: branch | Space: expand ]  Branch: %s", branches[0])
	m.SetShowStatusBar(true)

	fList := list.New([]list.Item{}, newFileDelegate(), 0, 0)
	fList.Title = "Files Changed"
	fList.SetShowStatusBar(false)

	vp := viewport.New(0, 0)

	return model{
		commitList:  m,
		fileList:    fList,
		diffView:    vp,
		state:       0,
		branches:    branches,
		branchIndex: 0,
	}
}

func (m *model) updateCommits() {
	branch := m.branches[m.branchIndex]
	commits, _ := core.FetchCommits(branch)
	var items []list.Item
	for _, c := range commits {
		items = append(items, commitItem{commit: c})
	}
	m.commitList.SetItems(items)
	m.commitList.Title = fmt.Sprintf("Git Log TUI  [ Tab: branch | Space: expand ]  Branch: %s", branch)
	m.commitList.ResetSelected()
}

func (m model) Init() tea.Cmd {
	return tickCmd()
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tickMsg:
		// Re-fetch branches and commits in background to check for updates
		branches, _ := core.FetchBranches()
		if len(branches) > 0 {
			m.branches = branches
			if m.branchIndex >= len(m.branches) {
				m.branchIndex = 0
			}
			branch := m.branches[m.branchIndex]
			commits, _ := core.FetchCommits(branch)

			if len(commits) > 0 {
				needsUpdate := false
				if len(commits) != len(m.commitList.Items()) {
					needsUpdate = true
				} else if len(m.commitList.Items()) > 0 {
					topExistingID := m.commitList.Items()[0].(commitItem).commit.ID
					topExistingDate := m.commitList.Items()[0].(commitItem).commit.Date
					if commits[0].ID != topExistingID || commits[0].Date != topExistingDate {
						needsUpdate = true
					}
				}

				if needsUpdate {
					var items []list.Item
					for _, c := range commits {
						items = append(items, commitItem{commit: c})
					}
					m.commitList.SetItems(items)
				}
			}
		}
		return m, tickCmd()

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}

		if m.state == 0 {
			if msg.String() == "tab" {
				m.branchIndex = (m.branchIndex + 1) % len(m.branches)
				m.updateCommits()
				return m, nil
			} else if msg.String() == "shift+tab" {
				m.branchIndex = (m.branchIndex - 1 + len(m.branches)) % len(m.branches)
				m.updateCommits()
				return m, nil
			} else if msg.String() == " " || msg.String() == "e" {
				m.expanded = !m.expanded
				m.commitList.SetDelegate(newCustomDelegate(m.expanded))
				return m, nil
			} else if msg.String() == "enter" || msg.String() == "right" || msg.String() == "l" {
				if i, ok := m.commitList.SelectedItem().(commitItem); ok {
					m.selectedCommit = i.commit
					files, _ := core.FetchCommitFiles(m.selectedCommit.ID)

					var fItems []list.Item
					for _, f := range files {
						fItems = append(fItems, fileItem{file: f})
					}
					m.fileList.SetItems(fItems)
					m.fileList.Title = "Commit Files"

					if len(files) > 0 {
						diff, _ := core.FetchFileDiff(m.selectedCommit.ID, files[0].Path)
						m.diffView.SetContent(colorizeDiff(diff))
					} else {
						m.diffView.SetContent("No diff available.")
					}

					m.state = 1
					return m, nil
				}
			} else if msg.String() == "b" {
				branch := m.branches[m.branchIndex]
				files, _ := core.FetchBranchFiles(branch)

				var fItems []list.Item
				for _, f := range files {
					fItems = append(fItems, fileItem{file: f})
				}
				m.fileList.SetItems(fItems)
				m.fileList.Title = "Branch Files (" + branch + ")"

				if len(files) > 0 {
					diff, _ := core.FetchBranchFileDiff(branch, files[0].Path)
					m.diffView.SetContent(colorizeDiff(diff))
				} else {
					m.diffView.SetContent("No diff available (branch is even with master).")
				}
				m.state = 2
				return m, nil
			}

			m.commitList, cmd = m.commitList.Update(msg)
			cmds = append(cmds, cmd)

		} else if m.state == 1 || m.state == 2 {
			if msg.String() == "esc" || msg.String() == "left" || msg.String() == "h" {
				m.state = 0
				return m, nil
			}

			oldIndex := m.fileList.Index()
			m.fileList, cmd = m.fileList.Update(msg)
			cmds = append(cmds, cmd)

			if m.fileList.Index() != oldIndex {
				if f, ok := m.fileList.SelectedItem().(fileItem); ok {
					var diff string
					if m.state == 1 {
						diff, _ = core.FetchFileDiff(m.selectedCommit.ID, f.file.Path)
					} else {
						diff, _ = core.FetchBranchFileDiff(m.branches[m.branchIndex], f.file.Path)
					}
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
		m.height = msg.Height - v - 2

		m.commitList.SetSize(m.width, m.height)
		m.fileList.SetSize(m.width/3, m.height)

		m.diffView.Width = m.width - (m.width / 3) - 6
		m.diffView.Height = m.height - 4 - 2 // leave space for header
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
	if m.width < 50 || m.height < 15 {
		return docStyle.Render("Terminal too small.\nPlease resize the window.")
	}

	if m.state == 0 {
		return docStyle.Render(m.commitList.View())
	}

	fileListWidth := m.width / 3
	if fileListWidth < 20 {
		fileListWidth = 20
	}
	left := ""

	var header string
	if m.state == 1 {
		header = headerStyle.Render(m.selectedCommit.Message)
	} else {
		header = headerStyle.Render("Changes in branch: " + m.branches[m.branchIndex])
	}

	diffContent := lipgloss.JoinVertical(lipgloss.Left, header, m.diffView.View())
	diffWidth := m.width - fileListWidth - 2
	if diffWidth < 10 {
		diffWidth = 10
	}
	right := diffStyle.Width(diffWidth).Height(m.height).Render(diffContent)

	return docStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, left, right))
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Errore TUI: %v", err)
		os.Exit(1)
	}
}
