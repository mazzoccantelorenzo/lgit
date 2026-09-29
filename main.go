package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/loreenzo/lorenzogit/core"
)

// UI Constants
var (
	docStyle     = lipgloss.NewStyle().Margin(0, 1)
	sidebarStyle = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363d")).Padding(1, 2)
	diffStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#30363d")).Padding(1, 2)
	headerStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#c9d1d9")).Bold(true).PaddingBottom(1)
)

const compactPreviewWidth = 72
const minimumSplitWidth = 60
const minimumDetailHeight = 12

const (
	commitListTarget = iota
	fileListTarget
)

type commitItem struct {
	commit core.Commit
}

func (i commitItem) Title() string { return fmt.Sprintf("%s  %s", i.commit.ID, i.commit.Message) }
func (i commitItem) Description() string {
	return fmt.Sprintf("%s • %s", i.commit.Author, i.commit.Date)
}
func (i commitItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s", i.commit.ID, i.commit.Message, i.commit.Author)
}

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

	state           int // 0 = Commits, 1 = Commit Files/Diff, 2 = Branch Files/Diff
	width           int
	height          int
	selectedCommit  core.Commit
	previewRequest  uint64
	previewLoading  bool
	updateAvailable bool

	branches    []string
	branchIndex int
	expanded    bool
	diffFocus   bool
}

type rebaseFinishedMsg struct{ err error }
type tickMsg time.Time

// routedFilterMsg is a list filter result tagged with the pane that requested it.
type routedFilterMsg struct {
	target  int
	matches list.FilterMatchesMsg
}

// routeListCommand is the command wrapper that keeps asynchronous filter results in their list.
func routeListCommand(command tea.Cmd, target int) tea.Cmd {
	if command == nil {
		return nil
	}
	return func() tea.Msg {
		message := command()
		switch result := message.(type) {
		case list.FilterMatchesMsg:
			return routedFilterMsg{target: target, matches: result}
		case tea.BatchMsg:
			routed := make(tea.BatchMsg, len(result))
			for index, nestedCommand := range result {
				routed[index] = routeListCommand(nestedCommand, target)
			}
			return routed
		default:
			return message
		}
	}
}

// commitPreviewMsg is the selected commit's file list and first file diff.
type commitPreviewMsg struct {
	commitID  string
	requestID uint64
	files     []core.FileChange
	diff      string
	err       error
}

// fetchCommitPreview is the command that loads detail content without blocking navigation.
func fetchCommitPreview(commitID string, requestID uint64) tea.Cmd {
	return func() tea.Msg {
		files, err := core.FetchCommitFiles(commitID)
		if err != nil {
			return commitPreviewMsg{commitID: commitID, requestID: requestID, err: err}
		}
		if len(files) == 0 {
			return commitPreviewMsg{commitID: commitID, requestID: requestID, files: files, diff: "No diff available."}
		}
		diff, err := core.FetchFileDiff(commitID, files[0].Path)
		return commitPreviewMsg{commitID: commitID, requestID: requestID, files: files, diff: diff, err: err}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// commitListTitle is the list heading that keeps update warnings visible across refreshes.
func commitListTitle(branch string, updateAvailable bool) string {
	title := fmt.Sprintf("Git Log TUI  [ Tab: branch | Space: expand ]  Branch: %s", branch)
	if updateAvailable {
		return "⚠ lgit update available  |  " + title
	}
	return title
}

func initialModel() model {
	branches, _ := core.FetchBranches()
	if len(branches) == 0 {
		branches = []string{"master"}
	}

	currentBranch := core.FetchCurrentBranch()
	branchIndex := 0
	for i, b := range branches {
		if b == currentBranch {
			branchIndex = i
			break
		}
	}

	commits, _ := core.FetchCommits(branches[branchIndex])

	var items []list.Item
	for _, c := range commits {
		items = append(items, commitItem{commit: c})
	}

	m := list.New(items, newCustomDelegate(false), 0, 0)
	m.Title = commitListTitle(branches[branchIndex], false)
	m.SetShowStatusBar(true)

	fList := list.New([]list.Item{}, newFileDelegate(), 0, 0)
	fList.Title = "Files Changed"
	fList.SetShowStatusBar(false)

	vp := viewport.New(0, 0)

	result := model{
		commitList:  m,
		fileList:    fList,
		diffView:    vp,
		state:       0,
		branches:    branches,
		branchIndex: branchIndex,
	}
	result.selectCommitPreview()
	return result
}

// selectCommitPreview is the selection handler that keeps the right pane on the highlighted commit.
func (m *model) selectCommitPreview() tea.Cmd {
	item, ok := m.commitList.SelectedItem().(commitItem)
	if !ok {
		m.previewRequest++
		m.selectedCommit = core.Commit{}
		m.fileList.ResetFilter()
		m.fileList.SetItems(nil)
		m.diffView.SetContent("No commit selected.")
		m.previewLoading = false
		return nil
	}
	if item.commit.ID == m.selectedCommit.ID {
		m.selectedCommit = item.commit
		return nil
	}

	m.selectedCommit = item.commit
	m.fileList.ResetFilter()
	m.fileList.SetItems(nil)
	m.fileList.Title = "Commit Files"
	m.diffView.SetContent("Loading commit preview...")
	m.diffView.GotoTop()
	m.previewRequest++
	m.previewLoading = true
	return fetchCommitPreview(item.commit.ID, m.previewRequest)
}

// replaceCommits is the list refresh that keeps the selected commit when new history arrives.
func (m *model) replaceCommits(branch string, commits []core.Commit) tea.Cmd {
	selectedID := ""
	if item, ok := m.commitList.SelectedItem().(commitItem); ok {
		selectedID = item.commit.ID
	}
	m.commitList.ResetFilter()
	selectedIndex := m.commitList.Index()
	var items []list.Item
	for index, commit := range commits {
		items = append(items, commitItem{commit: commit})
		if commit.ID == selectedID {
			selectedIndex = index
		}
	}
	filterCommand := routeListCommand(m.commitList.SetItems(items), commitListTarget)
	m.commitList.Title = commitListTitle(branch, m.updateAvailable)
	if len(items) > 0 {
		if selectedIndex < 0 {
			selectedIndex = 0
		}
		if selectedIndex >= len(items) {
			selectedIndex = len(items) - 1
		}
		m.commitList.Select(selectedIndex)
	}
	if m.state == 0 {
		return tea.Batch(filterCommand, m.selectCommitPreview())
	}
	return filterCommand
}

// updateCommits is the refresh handler for the branch selected in the commit list.
func (m *model) updateCommits() tea.Cmd {
	branch := m.branches[m.branchIndex]
	commits, _ := core.FetchCommits(branch)
	return m.replaceCommits(branch, commits)
}

// selectedFilePath is the file identity used to notice selection changes after list filtering.
func selectedFilePath(fileList list.Model) string {
	if item, ok := fileList.SelectedItem().(fileItem); ok {
		return item.file.Path
	}
	return ""
}

// updateSelectedFileDiff is the detail refresh when the file selection changes.
func (m *model) updateSelectedFileDiff(previousPath string) {
	item, ok := m.fileList.SelectedItem().(fileItem)
	if !ok {
		m.diffView.SetContent("No file selected.")
		return
	}
	if item.file.Path == previousPath {
		return
	}
	var diff string
	var err error
	if m.state == 2 {
		diff, err = core.FetchBranchFileDiff(m.branches[m.branchIndex], item.file.Path)
	} else {
		diff, err = core.FetchFileDiff(m.selectedCommit.ID, item.file.Path)
	}
	if err != nil {
		m.diffView.SetContent("Could not load file diff: " + err.Error())
	} else {
		m.diffView.SetContent(colorizeDiff(diff))
	}
	m.diffView.GotoTop()
}

// selectPreviewFile is the file navigation used while the commit list keeps focus.
func (m *model) selectPreviewFile(direction int) {
	if len(m.fileList.Items()) == 0 {
		return
	}
	previousPath := selectedFilePath(m.fileList)
	nextIndex := m.fileList.Index() + direction
	if nextIndex < 0 {
		nextIndex = 0
	}
	if nextIndex >= len(m.fileList.Items()) {
		nextIndex = len(m.fileList.Items()) - 1
	}
	m.fileList.Select(nextIndex)
	m.updateSelectedFileDiff(previousPath)
}

func (m model) Init() tea.Cmd {
	if m.selectedCommit.ID == "" {
		return tea.Batch(tickCmd(), checkForUpdates())
	}
	return tea.Batch(tickCmd(), checkForUpdates(), fetchCommitPreview(m.selectedCommit.ID, m.previewRequest))
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case rebaseFinishedMsg:
		return m, m.updateCommits()
	case updateAvailableMsg:
		m.updateAvailable = true
		m.commitList.Title = commitListTitle(m.branches[m.branchIndex], true)
		return m, nil
	case commitPreviewMsg:
		if msg.commitID != m.selectedCommit.ID || msg.requestID != m.previewRequest || m.state == 2 {
			return m, nil
		}
		m.previewLoading = false
		if msg.err != nil {
			m.fileList.SetItems(nil)
			m.diffView.SetContent("Could not load commit preview: " + msg.err.Error())
			return m, nil
		}
		var items []list.Item
		for _, file := range msg.files {
			items = append(items, fileItem{file: file})
		}
		filterCommand := m.fileList.SetItems(items)
		m.fileList.Select(0)
		m.diffView.SetContent(colorizeDiff(msg.diff))
		m.diffView.GotoTop()
		return m, routeListCommand(filterCommand, fileListTarget)
	case routedFilterMsg:
		if msg.target == commitListTarget {
			m.commitList, cmd = m.commitList.Update(msg.matches)
			if m.state == 0 {
				return m, tea.Batch(routeListCommand(cmd, commitListTarget), m.selectCommitPreview())
			}
			return m, routeListCommand(cmd, commitListTarget)
		}
		previousFile := selectedFilePath(m.fileList)
		m.fileList, cmd = m.fileList.Update(msg.matches)
		m.updateSelectedFileDiff(previousFile)
		return m, routeListCommand(cmd, fileListTarget)
	case list.FilterMatchesMsg:
		// A direct result can come from a list command issued before a pane switch.
		target := commitListTarget
		if m.state != 0 {
			target = fileListTarget
		}
		return m.Update(routedFilterMsg{target: target, matches: msg})
	case tickMsg:
		// Re-fetch branches and commits in background to check for updates
		branches, _ := core.FetchBranches()
		if len(branches) > 0 {
			selectedBranch := m.branches[m.branchIndex]
			m.branches = branches
			m.branchIndex = 0
			for index, branch := range branches {
				if branch == selectedBranch {
					m.branchIndex = index
					break
				}
			}
			branch := m.branches[m.branchIndex]
			commits, _ := core.FetchCommits(branch)

			if m.commitList.FilterState() == list.Unfiltered && !sameCommits(m.commitList.Items(), commits) {
				cmds = append(cmds, m.replaceCommits(branch, commits))
			}
			m.commitList.Title = commitListTitle(branch, m.updateAvailable)
		}
		return m, tea.Batch(append(cmds, tickCmd())...)

	case tea.KeyMsg:
		isFiltering := m.commitList.FilterState() == list.Filtering
		if m.state != 0 {
			isFiltering = m.fileList.FilterState() == list.Filtering
		}

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if !isFiltering && msg.String() == "q" {
			return m, tea.Quit
		}

		if m.state == 0 {
			if !isFiltering {
				if msg.String() == "shift+down" {
					m.selectPreviewFile(1)
					return m, nil
				} else if msg.String() == "shift+up" {
					m.selectPreviewFile(-1)
					return m, nil
				} else if msg.String() == "tab" {
					m.branchIndex = (m.branchIndex + 1) % len(m.branches)
					return m, m.updateCommits()
				} else if msg.String() == "shift+tab" {
					m.branchIndex = (m.branchIndex - 1 + len(m.branches)) % len(m.branches)
					return m, m.updateCommits()
				} else if msg.String() == " " {
					m.expanded = !m.expanded
					m.commitList.SetDelegate(newCustomDelegate(m.expanded))
					return m, nil
				} else if msg.String() == "e" {
					if i, ok := m.commitList.SelectedItem().(commitItem); ok {
						cmd := exec.Command("git", "history", "reword", i.commit.ID)
						return m, tea.ExecProcess(cmd, func(err error) tea.Msg {
							return rebaseFinishedMsg{err}
						})
					}
				} else if msg.String() == "f" {
					if i, ok := m.commitList.SelectedItem().(commitItem); ok {
						cmd := exec.Command("git", "history", "reword", i.commit.ID)
						nvimCmd := "nvim --headless -c 'set ft=gitcommit textwidth=72' -c 'g/^#/d' -c 'normal! gg0gqG' -c 'wq'"
						cmd.Env = append(os.Environ(), "GIT_EDITOR="+nvimCmd, "EDITOR="+nvimCmd, "VISUAL="+nvimCmd)
						return m, func() tea.Msg {
							err := cmd.Run()
							return rebaseFinishedMsg{err}
						}
					}
				} else if msg.String() == "enter" || msg.String() == "right" || msg.String() == "l" {
					if _, ok := m.commitList.SelectedItem().(commitItem); ok {
						m.state = 1
						m.applyLayout()
						return m, nil
					}
				} else if msg.String() == "b" {
					m.previewRequest++
					branch := m.branches[m.branchIndex]
					files, _ := core.FetchBranchFiles(branch)

					var fItems []list.Item
					for _, f := range files {
						fItems = append(fItems, fileItem{file: f})
					}
					m.fileList.ResetFilter()
					m.fileList.SetItems(fItems)
					m.fileList.Title = "Branch Files (" + branch + ")"

					if len(files) > 0 {
						diff, _ := core.FetchBranchFileDiff(branch, files[0].Path)
						m.diffView.SetContent(colorizeDiff(diff))
					} else {
						m.diffView.SetContent("No diff available (branch is even with master).")
					}
					m.state = 2
					m.applyLayout()
					return m, nil
				}
			}

			m.commitList, cmd = m.commitList.Update(msg)
			cmds = append(cmds, routeListCommand(cmd, commitListTarget))
			cmds = append(cmds, m.selectCommitPreview())

		} else if m.state == 1 || m.state == 2 {
			if m.fileList.FilterState() == list.Filtering {
				previousFile := selectedFilePath(m.fileList)
				m.fileList, cmd = m.fileList.Update(msg)
				m.updateSelectedFileDiff(previousFile)
				return m, routeListCommand(cmd, fileListTarget)
			}
			if msg.String() == "esc" {
				wasBranchDetails := m.state == 2
				m.state = 0
				m.diffFocus = false
				m.applyLayout()
				if wasBranchDetails {
					m.selectedCommit = core.Commit{}
				}
				return m, m.selectCommitPreview()
			}
			if msg.String() == "left" || msg.String() == "h" {
				if m.diffFocus {
					m.diffFocus = false
					return m, nil
				}
				wasBranchDetails := m.state == 2
				m.state = 0
				m.applyLayout()
				if wasBranchDetails {
					m.selectedCommit = core.Commit{}
				}
				return m, m.selectCommitPreview()
			}
			if msg.String() == "right" || msg.String() == "l" {
				m.diffFocus = true
				return m, nil
			}

			if !m.diffFocus {
				previousFile := selectedFilePath(m.fileList)
				m.fileList, cmd = m.fileList.Update(msg)
				cmds = append(cmds, routeListCommand(cmd, fileListTarget))
				m.updateSelectedFileDiff(previousFile)
			} else {
				m.diffView, cmd = m.diffView.Update(msg)
				cmds = append(cmds, cmd)
			}
		}

	case tea.WindowSizeMsg:
		docW, docH := docStyle.GetFrameSize()
		m.width = msg.Width - docW
		m.height = msg.Height - docH - 1 // 1 line safety
		m.applyLayout()
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

// clipLines is the terminal-safe renderer that keeps long diff lines inside their pane.
func clipLines(content string, width int) string {
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		lines[index] = ansi.Truncate(line, width, "")
	}
	return strings.Join(lines, "\n")
}

// clipRows is the terminal-safe renderer that avoids drawing below a short screen.
func clipRows(content string, height int) string {
	if height <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	return strings.Join(lines, "\n")
}

// sameCommits is the comparison that detects changes to commits and their Git ref labels.
func sameCommits(items []list.Item, commits []core.Commit) bool {
	if len(items) != len(commits) {
		return false
	}
	for index, item := range items {
		current, ok := item.(commitItem)
		if !ok || current.commit != commits[index] {
			return false
		}
	}
	return true
}

// detailDimensions is the layout calculation shared by the preview and full detail view.
func detailDimensions(width, height int) (fileWidth, diffWidth, innerHeight int) {
	sideFrameWidth, sideFrameHeight := sidebarStyle.GetFrameSize()
	diffFrameWidth, _ := diffStyle.GetFrameSize()
	contentWidth := width - sideFrameWidth - diffFrameWidth
	if contentWidth < 2 {
		contentWidth = 2
	}
	fileWidth = contentWidth / 3
	if fileWidth < 1 {
		fileWidth = 1
	}
	diffWidth = contentWidth - fileWidth
	innerHeight = height - sideFrameHeight
	if innerHeight < 1 {
		innerHeight = 1
	}
	return
}

// panelStyle is the size adapter that accounts for Lip Gloss padding inside Width and Height.
func panelStyle(style lipgloss.Style, contentWidth, contentHeight int) lipgloss.Style {
	return style.
		Width(contentWidth + style.GetPaddingLeft() + style.GetPaddingRight()).
		Height(contentHeight + style.GetPaddingTop() + style.GetPaddingBottom())
}

// applyLayout is the sizing handler that gives the log and detail panes their available space.
func (m *model) applyLayout() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	commitWidth := m.width
	detailWidth := m.width
	if m.state == 0 && m.width >= minimumSplitWidth && m.height >= minimumDetailHeight {
		commitWidth = m.width / 2
		detailWidth = m.width - commitWidth
	}
	m.commitList.SetShowHelp(m.height >= 28 && m.width >= minimumSplitWidth)
	m.commitList.SetShowStatusBar(m.height >= 28 && m.width >= minimumSplitWidth)
	m.fileList.SetShowHelp(m.height >= 28 && m.width >= minimumSplitWidth)
	m.commitList.SetSize(commitWidth, m.height)
	fileWidth, diffWidth, innerHeight := detailDimensions(detailWidth, m.height)
	m.fileList.SetSize(fileWidth, innerHeight)
	m.diffView.Width = diffWidth
	m.diffView.Height = innerHeight - 3 // One header line, its spacer, and the footer.
	if (m.state == 0 && detailWidth < compactPreviewWidth) || (m.state != 0 && m.width < minimumSplitWidth) {
		frameWidth, _ := diffStyle.GetFrameSize()
		m.diffView.Width = detailWidth - frameWidth
		m.diffView.Height = innerHeight - 4 // Header, file name, spacer, and footer.
	}
	if m.diffView.Width < 1 {
		m.diffView.Width = 1
	}
	if m.diffView.Height < 1 {
		m.diffView.Height = 1
	}
}

// renderCompactPreview is the narrow pane that shows the selected file and its diff at readable width.
func (m model) renderCompactPreview(width int) string {
	frameWidth, _ := diffStyle.GetFrameSize()
	contentWidth := width - frameWidth
	_, _, innerHeight := detailDimensions(width, m.height)

	headerText := m.selectedCommit.Message
	if m.state == 2 {
		headerText = "Changes in branch: " + m.branches[m.branchIndex]
	}
	if headerText == "" {
		headerText = "Commit preview"
	}
	header := headerStyle.Render(ansi.Truncate(headerText, contentWidth, "…"))

	fileText := "No files changed"
	if item, ok := m.fileList.SelectedItem().(fileItem); ok {
		fileText = item.file.Path
		if count := len(m.fileList.Items()); count > 1 {
			fileText = fmt.Sprintf("%s (%d files)", fileText, count)
		}
	} else if m.previewLoading {
		fileText = "Loading files..."
	}
	fileLine := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e")).Render(ansi.Truncate(fileText, contentWidth, "…"))
	footerText := " [Shift↑/↓] Files  [→] Open "
	if m.state != 0 {
		footerText = " [↑/↓] Files  [←] Back "
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e")).Render(ansi.Truncate(footerText, contentWidth, ""))
	content := lipgloss.JoinVertical(lipgloss.Left, header, fileLine, clipLines(m.diffView.View(), contentWidth), footer)
	return panelStyle(diffStyle, contentWidth, innerHeight).Render(content)
}

// renderDetails is the shared file-list and diff view for previews and opened commits.
func (m model) renderDetails(width int, preview bool) string {
	fileWidth, diffWidth, innerHeight := detailDimensions(width, m.height)
	if width < 16 {
		return lipgloss.NewStyle().Width(width).Height(m.height).Render("Preview")
	}
	if (preview && width < compactPreviewWidth) || (!preview && width < minimumSplitWidth) {
		return m.renderCompactPreview(width)
	}

	headerText := m.selectedCommit.Message
	if m.state == 2 && !preview {
		headerText = "Changes in branch: " + m.branches[m.branchIndex]
	}
	if headerText == "" {
		headerText = "Commit preview"
	}
	header := headerStyle.Render(ansi.Truncate(headerText, diffWidth, "…"))

	footerText := " [Shift↑/↓] Files  [→] Open "
	if !preview {
		footerText = " [←] Sidebar  [→] Scroll Diff "
		if m.diffFocus {
			footerText = " [↑/↓] Scroll  [←] Back to Sidebar "
		}
	}
	footerRight := fmt.Sprintf(" %3d%% ", int(m.diffView.ScrollPercent()*100))
	if diffWidth < lipgloss.Width(footerRight) {
		footerRight = ansi.Truncate(footerRight, diffWidth, "")
	}
	footerText = ansi.Truncate(footerText, diffWidth-lipgloss.Width(footerRight), "")
	footerGap := diffWidth - lipgloss.Width(footerText) - lipgloss.Width(footerRight)

	footerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e")).Background(lipgloss.Color("#161b22"))
	if !preview && m.diffFocus {
		footerStyle = footerStyle.Foreground(lipgloss.Color("#c9d1d9")).Background(lipgloss.Color("#1f6feb"))
	}
	footer := lipgloss.JoinHorizontal(lipgloss.Top,
		footerStyle.Render(footerText),
		strings.Repeat(" ", footerGap),
		footerStyle.Render(footerRight),
	)

	diffContent := lipgloss.JoinVertical(lipgloss.Left, header, clipLines(m.diffView.View(), diffWidth), footer)
	rightStyle := diffStyle.Copy()
	leftStyle := sidebarStyle.Copy()
	if !preview {
		if m.diffFocus {
			rightStyle = rightStyle.BorderForeground(lipgloss.Color("#58a6ff"))
		} else {
			leftStyle = leftStyle.BorderForeground(lipgloss.Color("#58a6ff"))
		}
	}

	right := panelStyle(rightStyle, diffWidth, innerHeight).Render(diffContent)
	left := panelStyle(leftStyle, fileWidth, innerHeight).Render(clipLines(m.fileList.View(), fileWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return ""
	}
	if m.state == 0 {
		if m.width < minimumSplitWidth || m.height < minimumDetailHeight {
			content := clipRows(clipLines(m.commitList.View(), m.width), m.height)
			return docStyle.Render(lipgloss.NewStyle().Width(m.width).Height(m.height).Render(content))
		}
		commitWidth := m.width / 2
		commitPane := lipgloss.NewStyle().Width(commitWidth).Height(m.height).Render(m.commitList.View())
		previewPane := m.renderDetails(m.width-commitWidth, true)
		return docStyle.Render(lipgloss.JoinHorizontal(lipgloss.Top, commitPane, previewPane))
	}
	if m.height < minimumDetailHeight {
		return docStyle.Render(ansi.Truncate("Widen terminal to view commit", m.width, ""))
	}
	return docStyle.Render(m.renderDetails(m.width, false))
}

func main() {
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Errore TUI: %v", err)
		os.Exit(1)
	}
}
