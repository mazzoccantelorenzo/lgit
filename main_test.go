package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TestCommitPreviewFollowsSelection is the UI flow that keeps the preview and opened detail on the selected commit.
func TestCommitPreviewFollowsSelection(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(t.TempDir())

	runGit := func(args ...string) {
		t.Helper()
		command := exec.Command("git", args...)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, output)
		}
	}
	writeAndCommit := func(path, contents, subject string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
		runGit("add", path)
		runGit("-c", "user.name=Test Author", "-c", "user.email=test@example.com", "commit", "-q", "-m", subject)
	}

	runGit("init", "-q", "-b", "main")
	writeAndCommit("candidate.txt", "Candidate profile\n", "add candidate")
	writeAndCommit("interview.txt", "Interview booked "+strings.Repeat("after candidate screening ", 8)+"\n", "record interview")

	current := initialModel()
	updated, _ := current.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	current = updated.(model)
	firstPreview := fetchCommitPreview(current.selectedCommit.ID, current.previewRequest)().(commitPreviewMsg)
	updated, _ = current.Update(firstPreview)
	current = updated.(model)

	screen := current.View()
	for _, text := range []string{"HEAD -> main", "interview.txt", "Interview booked"} {
		if !strings.Contains(screen, text) {
			t.Errorf("initial split view is missing %q", text)
		}
	}
	if got := lipgloss.Width(screen); got > 160 {
		t.Errorf("split view is %d columns wide, terminal is 160", got)
	}
	if got := lipgloss.Height(screen); got > 40 {
		t.Errorf("split view is %d rows high, terminal is 40", got)
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 40}, {Width: 80, Height: 8}} {
		updated, _ = current.Update(size)
		current = updated.(model)
		view := current.View()
		if width, height := lipgloss.Width(view), lipgloss.Height(view); width > size.Width || height > size.Height {
			t.Errorf("small view is %d×%d, terminal is %d×%d", width, height, size.Width, size.Height)
		}
	}

	updated, _ = current.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	current = updated.(model)
	screen = current.View()
	if !strings.Contains(screen, "interview.txt") || !strings.Contains(screen, "Interview booked") {
		t.Error("compact preview is missing the selected file or its diff")
	}
	if width, height := lipgloss.Width(screen), lipgloss.Height(screen); width > 80 || height > 24 {
		t.Errorf("compact view is %d×%d, terminal is 80×24", width, height)
	}
	updated, _ = current.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	current = updated.(model)

	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyDown})
	current = updated.(model)
	if current.selectedCommit.Message != "add candidate" {
		t.Fatalf("selected commit is %q, want add candidate", current.selectedCommit.Message)
	}
	updated, _ = current.Update(firstPreview)
	current = updated.(model)
	if len(current.fileList.Items()) != 0 {
		t.Fatal("stale preview replaced the newly selected commit")
	}

	secondPreview := fetchCommitPreview(current.selectedCommit.ID, current.previewRequest)().(commitPreviewMsg)
	updated, _ = current.Update(secondPreview)
	current = updated.(model)
	if !strings.Contains(current.View(), "candidate.txt") {
		t.Fatal("preview did not follow the selected commit")
	}

	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyRight})
	current = updated.(model)
	if current.state != 1 || !strings.Contains(current.View(), "candidate.txt") {
		t.Fatal("right arrow did not open the selected commit's preview")
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 40, Height: 24}, {Width: 80, Height: 8}} {
		updated, _ = current.Update(size)
		current = updated.(model)
		view := current.View()
		if width, height := lipgloss.Width(view), lipgloss.Height(view); width > size.Width || height > size.Height {
			t.Errorf("small detail is %d×%d, terminal is %d×%d", width, height, size.Width, size.Height)
		}
	}
	updated, _ = current.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	current = updated.(model)
	var deliverCommand func(tea.Cmd)
	deliverCommand = func(command tea.Cmd) {
		if command == nil {
			return
		}
		message := command()
		if batch, ok := message.(tea.BatchMsg); ok {
			for _, nestedCommand := range batch {
				deliverCommand(nestedCommand)
			}
			return
		}
		updated, nextCommand := current.Update(message)
		current = updated.(model)
		deliverCommand(nextCommand)
	}
	updated, filterCommand := current.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	current = updated.(model)
	deliverCommand(filterCommand)
	if current.fileList.FilterState() != list.Filtering {
		t.Fatal("file list did not enter filtering mode")
	}
	updated, filterCommand = current.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	current = updated.(model)
	deliverCommand(filterCommand)
	if current.commitList.FilterState() != list.Unfiltered || current.state != 1 {
		t.Fatal("file search affected the commit list or closed the detail")
	}
	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyEsc})
	current = updated.(model)
	if current.state != 1 {
		t.Fatal("escape closed detail while editing the file filter")
	}
	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyEsc})
	current = updated.(model)
	if current.state != 0 {
		t.Fatal("escape did not return to the split commit list")
	}
}
