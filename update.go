package main

import (
	"encoding/hex"
	"encoding/json"
	"net/http"
	"runtime/debug"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const githubCompareEndpoint = "https://api.github.com/repos/mazzoccantelorenzo/lgit/compare/"

// updateAvailableMsg is the notification that the installed commit is behind the published branch.
type updateAvailableMsg struct{}

// installedCommitRevision is the Git revision embedded by Go when lgit is built from a clone.
func installedCommitRevision() string {
	buildInformation, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	for _, setting := range buildInformation.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value
		}
	}
	return ""
}

// checkForUpdates is the background command that checks for newer commits without delaying the UI.
func checkForUpdates() tea.Cmd {
	revision := installedCommitRevision()
	if len(revision) != 40 {
		return nil
	}
	if _, err := hex.DecodeString(revision); err != nil {
		return nil
	}

	return func() tea.Msg {
		client := &http.Client{Timeout: 4 * time.Second}
		if newerCommitAvailable(client, githubCompareEndpoint, revision) {
			return updateAvailableMsg{}
		}
		return nil
	}
}

// newerCommitAvailable is the GitHub comparison that ignores unavailable or inconclusive responses.
func newerCommitAvailable(client *http.Client, endpoint, revision string) bool {
	request, err := http.NewRequest(http.MethodGet, endpoint+revision+"...HEAD?per_page=1", nil)
	if err != nil {
		return false
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "lgit")

	response, err := client.Do(request)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return false
	}

	var comparison struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(response.Body).Decode(&comparison); err != nil {
		return false
	}
	return comparison.Status == "ahead"
}
