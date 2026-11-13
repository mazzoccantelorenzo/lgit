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
	editorError     string
	localChanges    map[string]bool

	branches        []string
	branchIndex     int
	baseBranch      string
	expanded        bool
	diffFocus       bool
	rawDiff         string
	diffColorized   bool
	selection       diffSelection
	clipboardWriter func(string) error

	showSplash bool
}

type splashDoneMsg struct{}

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
	title := fmt.Sprintf("Git Log TUI  [ Tab: branch | p: PR diff | Space: expand ]  Branch: %s", branch)
	if updateAvailable {
		return "⚠ lgit update available  |  " + title
	}
	return title
}

func newModel(showSplash bool) model {
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
		showSplash:  showSplash,
	}
	result.refreshLocalChanges()
	result.selectCommitPreview()
	return result
}

func initialModel() model {
	return newModel(false)
}

// selectCommitPreview is the selection handler that keeps the right pane on the highlighted commit.
func (m *model) selectCommitPreview() tea.Cmd {
	item, ok := m.commitList.SelectedItem().(commitItem)
	if !ok {
		m.previewRequest++
		m.selectedCommit = core.Commit{}
		m.fileList.ResetFilter()
		m.fileList.SetItems(nil)
		m.setDiffContent("No commit selected.", false)
		m.previewLoading = false
		m.applyLayout()
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
	m.setDiffContent("Loading commit preview...", false)
	m.diffView.GotoTop()
	m.previewRequest++
	m.previewLoading = true
	m.applyLayout()
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
		m.setDiffContent("No file selected.", false)
		m.applyLayout()
		return
	}
	if item.file.Path == previousPath {
		return
	}
	var diff string
	var err error
	if m.state == 2 {
		diff, err = core.FetchBranchFileDiff(m.baseBranch, m.branches[m.branchIndex], item.file.Path)
	} else {
		diff, err = core.FetchFileDiff(m.selectedCommit.ID, item.file.Path)
	}
	if err != nil {
		m.setDiffContent("Could not load file diff: "+err.Error(), false)
	} else {
		m.setDiffContent(diff, true)
	}
	m.diffView.GotoTop()
	m.applyLayout()
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

func splashTimerCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return splashDoneMsg{}
	})
}

func (m model) Init() tea.Cmd {
	var initialCmds []tea.Cmd
	if m.showSplash {
		initialCmds = append(initialCmds, splashTimerCmd())
	}
	initialCmds = append(initialCmds, tickCmd(), checkForUpdates())
	if m.selectedCommit.ID != "" {
		initialCmds = append(initialCmds, fetchCommitPreview(m.selectedCommit.ID, m.previewRequest))
	}
	return tea.Batch(initialCmds...)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case splashDoneMsg:
		m.showSplash = false
		m.applyLayout()
		return m, nil
	case editorFinishedMsg:
		if msg.err != nil {
			m.editorError = "Could not open nvim: " + msg.err.Error()
		}
		m.refreshLocalChanges()
		return m, nil
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
			m.setDiffContent("Could not load commit preview: "+msg.err.Error(), false)
			m.applyLayout()
			return m, nil
		}
		var items []list.Item
		for _, file := range msg.files {
			items = append(items, fileItem{file: file})
		}
		filterCommand := m.fileList.SetItems(items)
		m.fileList.Select(0)
		m.setDiffContent(msg.diff, len(msg.files) > 0)
		m.diffView.GotoTop()
		m.applyLayout()
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
		m.refreshLocalChanges()
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
		m.editorError = ""
		isFiltering := m.commitList.FilterState() == list.Filtering
		if m.state != 0 {
			isFiltering = m.fileList.FilterState() == list.Filtering
		}

		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if m.showSplash {
			m.showSplash = false
			m.applyLayout()
			return m, nil
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
				} else if msg.String() == "p" || msg.String() == "b" {
					m.previewRequest++
					currentBranch := m.branches[m.branchIndex]
					baseBranch, detectionError := core.DetectBaseBranch(currentBranch)
					if detectionError != nil || baseBranch == "" || baseBranch == currentBranch {
						m.baseBranch = baseBranch
						m.fileList.ResetFilter()
						m.fileList.SetItems(nil)
						m.fileList.Title = fmt.Sprintf("PR Files (%s)", currentBranch)
						if baseBranch == currentBranch || currentBranch == "main" || currentBranch == "master" {
							m.setDiffContent(fmt.Sprintf("Current branch '%s' is already the default or base branch.\n\nUse Tab to switch to a feature branch to inspect its cumulative PR diff.", currentBranch), false)
						} else {
							m.setDiffContent(fmt.Sprintf("Could not detect base branch for '%s'.\n\nEnsure a remote tracking branch or standard base branch (main/master) exists.", currentBranch), false)
						}
						m.state = 2
						m.applyLayout()
						return m, nil
					}

					m.baseBranch = baseBranch
					branchFiles, fetchError := core.FetchBranchFiles(baseBranch, currentBranch)
					if fetchError != nil {
						m.fileList.ResetFilter()
						m.fileList.SetItems(nil)
						m.setDiffContent(fmt.Sprintf("Error retrieving branch diff: %v", fetchError), false)
						m.state = 2
						m.applyLayout()
						return m, nil
					}

					var fileItems []list.Item
					for _, file := range branchFiles {
						fileItems = append(fileItems, fileItem{file: file})
					}
					m.fileList.ResetFilter()
					m.fileList.SetItems(fileItems)
					m.fileList.Title = fmt.Sprintf("PR Files (%s...%s)", baseBranch, currentBranch)

					if len(branchFiles) > 0 {
						initialDiff, _ := core.FetchBranchFileDiff(baseBranch, currentBranch, branchFiles[0].Path)
						m.setDiffContent(initialDiff, true)
						m.diffView.GotoTop()
					} else {
						m.setDiffContent(fmt.Sprintf("No diff available.\n\nBranch '%s' has no changes compared to '%s'.", currentBranch, baseBranch), false)
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
			if msg.String() == "enter" {
				if filePath := selectedFilePath(m.fileList); filePath != "" {
					return m, openFileInEditor(filePath)
				}
				return m, nil
			}
			if msg.String() == "p" || msg.String() == "b" {
				if m.state == 2 {
					m.state = 0
					m.diffFocus = false
					m.applyLayout()
					m.selectedCommit = core.Commit{}
					return m, m.selectCommitPreview()
				}
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

	case tea.MouseMsg:
		m.handleMouseEvent(msg)
		return m, nil

	case tea.WindowSizeMsg:
		docW, docH := docStyle.GetFrameSize()
		m.width = msg.Width - docW
		m.height = msg.Height - docH - 1 // 1 line safety
		m.applyLayout()
	}

	return m, tea.Batch(cmds...)
}

func colorizeDiff(diff string) string {
	return renderDiffWithSelection(diff, diffSelection{})
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
	if warning := m.renderLocalChangesWarning(m.diffView.Width, m.diffView.Height-1); warning != "" {
		m.diffView.Height -= lipgloss.Height(warning)
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
		if m.baseBranch != "" {
			headerText = fmt.Sprintf("PR Diff: %s ➔ %s (%d files)", m.baseBranch, m.branches[m.branchIndex], len(m.fileList.Items()))
		} else {
			headerText = "PR Diff: " + m.branches[m.branchIndex]
		}
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
	footerText := " [p] PR Diff  [Shift↑/↓] Files  [→] Open "
	if m.state != 0 {
		footerText = " [Enter] nvim  [↑/↓] Files  [←] Back "
		if m.state == 2 {
			footerText = " [Enter] nvim  [p/Esc] Commits  [↑/↓] Files  [←] Back "
		}
	}
	if m.editorError != "" {
		footerText = m.editorError
	}
	footer := lipgloss.NewStyle().Foreground(lipgloss.Color("#8b949e")).Render(ansi.Truncate(footerText, contentWidth, ""))
	lines := []string{header, fileLine, clipLines(m.diffView.View(), contentWidth)}
	if warning := m.renderLocalChangesWarning(contentWidth, innerHeight-5); warning != "" {
		lines = append(lines, warning)
	}
	lines = append(lines, footer)
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)
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
		if m.baseBranch != "" {
			headerText = fmt.Sprintf("PR Diff: %s ➔ %s (%d files)", m.baseBranch, m.branches[m.branchIndex], len(m.fileList.Items()))
		} else {
			headerText = "PR Diff: " + m.branches[m.branchIndex]
		}
	}
	if headerText == "" {
		headerText = "Commit preview"
	}
	header := headerStyle.Render(ansi.Truncate(headerText, diffWidth, "…"))

	footerText := " [p] PR Diff  [Shift↑/↓] Files  [→] Open "
	if !preview {
		footerText = " [Enter] nvim  [←] Sidebar  [→] Scroll Diff "
		if m.state == 2 {
			footerText = " [Enter] nvim  [p/Esc] Commits  [←] Sidebar  [→] Scroll Diff "
		}
		if m.diffFocus {
			footerText = " [Enter] nvim  [↑/↓] Scroll  [←] Back to Sidebar "
		}
	}
	if m.editorError != "" {
		footerText = m.editorError
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

	lines := []string{header, clipLines(m.diffView.View(), diffWidth)}
	if warning := m.renderLocalChangesWarning(diffWidth, innerHeight-4); warning != "" {
		lines = append(lines, warning)
	}
	lines = append(lines, footer)
	diffContent := lipgloss.JoinVertical(lipgloss.Left, lines...)
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
	if m.showSplash {
		return splashView(m.width, m.height)
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
	p := tea.NewProgram(newModel(true), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Errore TUI: %v", err)
		os.Exit(1)
	}
}
