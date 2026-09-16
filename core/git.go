package core

import (
	"bytes"
	"os/exec"
	"strings"
)

// Commit represents a single Git commit parsed from the history.
type Commit struct {
	ID      string
	Message string
	Body    string
	Author  string
	Date    string
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
	cmd := exec.Command("git", "log", branch, "-n", "50", "--pretty=format:%h|%s|%an|%cr|%b%n---END_COMMIT---")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return nil, err
	}

	var commits []Commit
	blocks := strings.Split(out.String(), "---END_COMMIT---")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		parts := strings.SplitN(block, "|", 5)
		if len(parts) == 5 {
			commits = append(commits, Commit{
				ID:      strings.TrimSpace(parts[0]),
				Message: strings.TrimSpace(parts[1]),
				Author:  strings.TrimSpace(parts[2]),
				Date:    strings.TrimSpace(parts[3]),
				Body:    strings.TrimSpace(parts[4]),
			})
		}
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
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-status", "-r", commitID)
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
	cmd := exec.Command("git", "show", commitID+"^!"+":"+filePath)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		fallback := exec.Command("git", "diff", commitID+"^", commitID, "--", filePath)
		var fbOut bytes.Buffer
		fallback.Stdout = &fbOut
		fbErr := fallback.Run()
		if fbErr != nil {
			return "Diff non disponibile", nil
		}
		return fbOut.String(), nil
	}
	return out.String(), nil
}
