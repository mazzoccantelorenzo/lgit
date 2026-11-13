package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TestLocalChangesWarningFollowsSelection is the UI regression check for dirty files across preview, commit, and PR views.
func TestLocalChangesWarningFollowsSelection(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(t.TempDir())
	runGit := func(arguments ...string) {
		t.Helper()
		if output, err := exec.Command("git", arguments...).CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(arguments, " "), err, output)
		}
	}
	writeFile := func(path, content string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runGit("init", "-q", "-b", "main")
	runGit("-c", "user.name=Test Author", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-q", "-m", "base")
	runGit("checkout", "-q", "-b", "feature/local-changes")
	writeFile("clean.txt", "clean contents\n")
	writeFile("dirty.txt", "committed contents\n")
	runGit("add", ".")
	runGit("-c", "user.name=Test Author", "-c", "user.email=test@example.com", "commit", "-q", "-m", "add files")
	writeFile("dirty.txt", "uncommitted contents\n")

	current := initialModel()
	preview := fetchCommitPreview(current.selectedCommit.ID, current.previewRequest)().(commitPreviewMsg)
	updated, _ := current.Update(preview)
	current = updated.(model)
	if current.renderLocalChangesWarning(80, 10) != "" {
		t.Fatal("a clean selected file showed a local changes warning")
	}
	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyShiftDown})
	current = updated.(model)
	if selectedFilePath(current.fileList) != "dirty.txt" {
		t.Fatal("dirty file was not selected")
	}
	for _, state := range []int{0, 1, 2} {
		current.state = state
		if state == 2 {
			current.baseBranch = "main"
		}
		for _, size := range []tea.WindowSizeMsg{{Width: 240, Height: 40}, {Width: 100, Height: 24}, {Width: 80, Height: 13}, {Width: 40, Height: 24}, {Width: 20, Height: 24}, {Width: 20, Height: 13}} {
			updated, _ = current.Update(size)
			current = updated.(model)
			screen := current.View()
			if width, height := lipgloss.Width(screen), lipgloss.Height(screen); width > size.Width || height > size.Height {
				t.Errorf("state %d view is %d×%d, terminal is %d×%d", state, width, height, size.Width, size.Height)
			}
			if state == 0 && size.Width < minimumSplitWidth {
				continue
			}
			expectedWords := strings.Fields(localChangesWarning)
			if size.Width == 20 && size.Height == 13 {
				expectedWords = []string{"local", "changes", "found,"}
			}
			for _, text := range expectedWords {
				if !strings.Contains(screen, text) {
					t.Errorf("state %d at %d×%d is missing warning text %q", state, size.Width, size.Height, text)
				}
			}
		}
	}

	current.state = 1
	updated, _ = current.Update(tea.WindowSizeMsg{Width: 160, Height: 40})
	current = updated.(model)
	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyUp})
	current = updated.(model)
	if strings.Contains(current.View(), "local changes found") {
		t.Fatal("warning stayed visible after selecting a clean file")
	}
	updated, _ = current.Update(tea.KeyMsg{Type: tea.KeyDown})
	current = updated.(model)
	runGit("add", "dirty.txt")
	updated, _ = current.Update(tickMsg{})
	current = updated.(model)
	if !strings.Contains(current.View(), "local changes found") {
		t.Fatal("staging a file hid its uncommitted changes warning")
	}
	runGit("restore", "--source=HEAD", "--staged", "--worktree", "dirty.txt")
	updated, _ = current.Update(tickMsg{})
	current = updated.(model)
	if strings.Contains(current.View(), "local changes found") {
		t.Fatal("warning stayed visible after restoring the file")
	}
}
