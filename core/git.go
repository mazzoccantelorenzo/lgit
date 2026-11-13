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

// FetchLocalChanges is the status query that finds uncommitted files relative to the repository root.
func FetchLocalChanges() (map[string]bool, error) {
	command := exec.Command("git", "--no-optional-locks", "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames", "--ignore-submodules=none")
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("fetch local changes: %w", err)
	}

	changes := make(map[string]bool)
	for _, record := range bytes.Split(output, []byte{0}) {
		if len(record) > 3 {
			changes[string(record[3:])] = true
		}
	}
	return changes, nil
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

// DetectBaseBranch is the function that identifies the base branch used as the comparison source for pull request diffs.
func DetectBaseBranch(currentBranch string) (string, error) {
	// First, we inspect if Git has an explicit default remote head defined for the origin remote repository.
	symbolicReferenceCommand := exec.Command("git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	var symbolicReferenceOutput bytes.Buffer
	symbolicReferenceCommand.Stdout = &symbolicReferenceOutput
	if executionError := symbolicReferenceCommand.Run(); executionError == nil {
		remoteHeadReference := strings.TrimSpace(symbolicReferenceOutput.String())
		trimmedBranchName := strings.TrimPrefix(remoteHeadReference, "origin/")
		if trimmedBranchName != "" && trimmedBranchName != currentBranch {
			mergeBaseCommand := exec.Command("git", "merge-base", remoteHeadReference, currentBranch)
			if mergeBaseError := mergeBaseCommand.Run(); mergeBaseError == nil {
				return remoteHeadReference, nil
			}
		}
	}

	// When the symbolic reference is unavailable or identical to the active branch, we evaluate common standard base branch names in hierarchical order.
	standardCandidateBranches := []string{"main", "master", "develop", "stage", "origin/main", "origin/master"}
	for _, candidateBranch := range standardCandidateBranches {
		if candidateBranch == currentBranch {
			continue
		}
		verificationCommand := exec.Command("git", "rev-parse", "--verify", "--quiet", candidateBranch)
		if verificationError := verificationCommand.Run(); verificationError != nil {
			continue
		}
		mergeBaseCommand := exec.Command("git", "merge-base", candidateBranch, currentBranch)
		if mergeBaseError := mergeBaseCommand.Run(); mergeBaseError == nil {
			return candidateBranch, nil
		}
	}

	// As a broader fallback, we scan all remaining local branches discovered in the repository to locate any branch sharing an ancestor.
	discoveredBranches, discoveryError := FetchBranches()
	if discoveryError == nil {
		for _, discoveredBranch := range discoveredBranches {
			if discoveredBranch == currentBranch {
				continue
			}
			mergeBaseCommand := exec.Command("git", "merge-base", discoveredBranch, currentBranch)
			if mergeBaseError := mergeBaseCommand.Run(); mergeBaseError == nil {
				return discoveredBranch, nil
			}
		}
	}

	return "", fmt.Errorf("no suitable base branch found to compare against %s", currentBranch)
}

// FetchBranchFiles is the function that retrieves all modified files across the current branch compared to the base branch.
func FetchBranchFiles(baseBranch, currentBranch string) ([]FileChange, error) {
	if baseBranch == "" || currentBranch == "" || baseBranch == currentBranch {
		return []FileChange{}, nil
	}

	// We invoke git diff using the triple-dot comparison notation to evaluate changes originating from the merge base.
	diffCommand := exec.Command("git", "diff", "--no-renames", "--name-status", baseBranch+"..."+currentBranch)
	var commandOutput bytes.Buffer
	diffCommand.Stdout = &commandOutput
	if executionError := diffCommand.Run(); executionError != nil {
		return nil, executionError
	}

	var modifiedFiles []FileChange
	outputLines := strings.Split(commandOutput.String(), "\n")
	for _, rawLine := range outputLines {
		trimmedLine := strings.TrimSpace(rawLine)
		if trimmedLine == "" {
			continue
		}
		lineColumns := strings.SplitN(trimmedLine, "\t", 2)
		if len(lineColumns) == 2 {
			modifiedFiles = append(modifiedFiles, FileChange{
				Status: strings.TrimSpace(lineColumns[0]),
				Path:   strings.TrimSpace(lineColumns[1]),
			})
		}
	}
	return modifiedFiles, nil
}

// FetchBranchFileDiff is the function that loads the cumulative unified diff of a specific file for the pull request view.
func FetchBranchFileDiff(baseBranch, currentBranch, filePath string) (string, error) {
	if baseBranch == "" || currentBranch == "" {
		return "No diff available", nil
	}

	// The diff command targets the exact file path relative to the common ancestor using triple-dot syntax.
	diffCommand := exec.Command("git", "diff", baseBranch+"..."+currentBranch, "--", filePath)
	var commandOutput bytes.Buffer
	diffCommand.Stdout = &commandOutput
	if executionError := diffCommand.Run(); executionError != nil {
		return "No diff available", nil
	}
	return commandOutput.String(), nil
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
