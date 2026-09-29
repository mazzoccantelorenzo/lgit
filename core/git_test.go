package core

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestFetchCommitsDecorationsAndMessageDelimiters(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Chdir(t.TempDir())

	runGit := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}

	runGit("init", "-q", "-b", "main")
	runGit("-c", "user.name=Test Author", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-q", "-m", "old|subject")
	runGit("-c", "user.name=Test Author", "-c", "user.email=test@example.com", "commit", "--allow-empty", "-q", "-m", "new|subject ---END_COMMIT---", "-m", "body|piece\n---END_COMMIT---")
	runGit("branch", "feature,one")
	runGit("update-ref", "refs/remotes/origin/main", "HEAD")
	runGit("tag", "v1")

	commits, err := FetchCommits("main")
	if err != nil {
		t.Fatal(err)
	}
	if len(commits) != 2 {
		t.Fatalf("got %d commits, want 2", len(commits))
	}
	if commits[0].Message != "new|subject ---END_COMMIT---" || commits[0].Body != "body|piece\n---END_COMMIT---" {
		t.Errorf("new commit message/body parsed incorrectly: %#v", commits[0])
	}
	if commits[1].Message != "old|subject" || commits[1].Decorations != "" {
		t.Errorf("old commit parsed incorrectly: %#v", commits[1])
	}
	for _, ref := range []string{"HEAD -> main", "feature,one", "origin/main", "tag: v1"} {
		if !strings.Contains(commits[0].Decorations, ref) {
			t.Errorf("decorations %q missing %q", commits[0].Decorations, ref)
		}
	}
}
