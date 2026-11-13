package core

import (
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

// TestFetchLocalChanges is the Git integration check for working tree and index changes from a subdirectory.
func TestFetchLocalChanges(t *testing.T) {
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
	if err := os.Mkdir("nested", 0o700); err != nil {
		t.Fatal(err)
	}
	paths := []string{"clean.txt", "unstaged.txt", "staged.txt", "reverted.txt", "deleted.txt", "renamed.txt", "nested/spaces and è.txt", "nested/tab\tand\nnewline.txt"}
	for _, path := range paths {
		writeFile(path, "original\n")
	}
	writeFile(".gitignore", "ignored.txt\n")
	runGit("add", ".")
	runGit("-c", "user.name=Test Author", "-c", "user.email=test@example.com", "commit", "-q", "-m", "initial files")

	for _, path := range paths[1:] {
		writeFile(path, "local changes\n")
	}
	runGit("add", "staged.txt", "reverted.txt")
	writeFile("reverted.txt", "original\n")
	runGit("rm", "-f", "deleted.txt")
	runGit("mv", "renamed.txt", "nested/renamed.txt")
	writeFile("nested/untracked.txt", "new file\n")
	writeFile("ignored.txt", "ignored contents\n")
	t.Chdir("nested")

	changes, err := FetchLocalChanges()
	if err != nil {
		t.Fatal(err)
	}
	expected := make(map[string]bool)
	for _, path := range append(paths[1:], "nested/renamed.txt", "nested/untracked.txt") {
		expected[path] = true
	}
	if !reflect.DeepEqual(changes, expected) {
		t.Fatalf("local changes = %#v, want %#v", changes, expected)
	}

	runGit("add", "-A")
	runGit("-c", "user.name=Test Author", "-c", "user.email=test@example.com", "commit", "-q", "-m", "commit local changes")
	changes, err = FetchLocalChanges()
	if err != nil || len(changes) != 0 {
		t.Fatalf("committed repository has local changes: %#v, %v", changes, err)
	}
}

// TestFetchLocalChangesOutsideRepository is the error check for an unavailable Git repository.
func TestFetchLocalChangesOutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := FetchLocalChanges(); err == nil {
		t.Fatal("expected a status error outside a repository")
	}
}
