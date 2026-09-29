package core

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// Commit represents a single Git commit parsed from the history.
type Commit struct {
	ID          string
	Message     string
	Body        string
	Author      string
	Date        string
	Decorations string
}

// FileChange represents a file that was modified in a commit.
type FileChange struct {
	Status string
	Path   string
}

// FetchCommits is the function that handles fetching recent commits from the repository.
func FetchBranches() ([]string, error) {
	cmd := exec.Command("git", "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return []string{"master"}, err
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return []string{"master"}, nil
	}
	return lines, nil
}

func FetchCommits(branch string) ([]Commit, error) {
	// Git separates records and fields with NUL bytes. Commit messages can contain
	// the printable delimiters used in ordinary log output, and ref names can
	// contain commas, so keep %D in Git's display format rather than splitting it.
	cmd := exec.Command("git", "log", branch, "-n", "1000", "--decorate=short", "-z", "--pretty=tformat:%h%x00%s%x00%an%x00%cr%x00%D%x00%b")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	if out.Len() == 0 {
		return nil, nil
	}
	fields := bytes.Split(out.Bytes(), []byte{0})
	if len(fields[len(fields)-1]) != 0 || (len(fields)-1)%6 != 0 {
		return nil, fmt.Errorf("unexpected git log output: incomplete commit record")
	}
	fields = fields[:len(fields)-1] // The final NUL terminates the last record.

	var commits []Commit
	for i := 0; i < len(fields); i += 6 {
		commits = append(commits, Commit{
			ID:          strings.TrimSpace(string(fields[i])),
			Message:     strings.TrimSpace(string(fields[i+1])),
			Author:      strings.TrimSpace(string(fields[i+2])),
			Date:        strings.TrimSpace(string(fields[i+3])),
			Decorations: strings.TrimSpace(string(fields[i+4])),
			Body:        strings.TrimSpace(string(fields[i+5])),
		})
	}
	return commits, nil
}

// FetchCommitFiles is the function that retrieves the list of files modified in a given commit.
func FetchBranchFiles(branch string) ([]FileChange, error) {
	// usually diffing against master or main
	baseBranch := "master"
	if branch == "main" || branch == "master" {
		return []FileChange{}, nil
	}
	cmd := exec.Command("git", "diff", "--name-status", baseBranch+"..."+branch)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	var files []FileChange
	lines := strings.Split(out.String(), "\n")
	for _, l := range lines {
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "\t", 2)
		if len(parts) == 2 {
			files = append(files, FileChange{Status: strings.TrimSpace(parts[0]), Path: strings.TrimSpace(parts[1])})
		}
	}
	return files, nil
}

func FetchBranchFileDiff(branch, filePath string) (string, error) {
	baseBranch := "master"
	cmd := exec.Command("git", "diff", baseBranch+"..."+branch, "--", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "No diff available", nil
	}
	return out.String(), nil
}

func FetchCommitFiles(commitID string) ([]FileChange, error) {
	cmd := exec.Command("git", "diff-tree", "--root", "--no-commit-id", "--name-status", "-r", commitID)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	var files []FileChange
	lines := strings.Split(out.String(), "\n")
	for _, l := range lines {
		if l == "" {
			continue
		}
		parts := strings.SplitN(l, "\t", 2)
		if len(parts) == 2 {
			files = append(files, FileChange{
				Status: strings.TrimSpace(parts[0]),
				Path:   strings.TrimSpace(parts[1]),
			})
		}
	}
	return files, nil
}

// FetchFileDiff is the function that extracts the diff content for a specific file in a commit.
func FetchFileDiff(commitID, filePath string) (string, error) {
	cmd := exec.Command("git", "show", "--format=", "--no-ext-diff", commitID, "--", filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func FetchCurrentBranch() string {
	cmd := exec.Command("git", "branch", "--show-current")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out.String())
}
