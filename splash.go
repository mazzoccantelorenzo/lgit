package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// splashView renders a clean, professional 3D ASCII logo with an interactive keybinding guide.
// It contains zero emojis and can be reopened anytime by pressing 'h' or '?' from the commit log.
func splashView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	logoStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#58a6ff"))

	taglineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8b949e"))

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#c9d1d9"))

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#79c0ff"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8b949e"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681"))

	art3d := `
    __           _  __ 
   / /____ _    (_)/ /_
  / // __ '/   / // __/
 / // /_/ /   / // /_  
/_/ \__, /   /_/ \__/  
   /____/              `

	formatRow := func(key, desc string) string {
		k := keyStyle.Width(16).Render(key)
		d := descStyle.Render(desc)
		return lipgloss.JoinHorizontal(lipgloss.Top, k, d)
	}

	guideRows := []string{
		titleStyle.Render("KEYBOARD SHORTCUTS"),
		"",
		formatRow("Enter / l / ->", "Open commit files & diff preview"),
		formatRow("Tab / Shift+Tab", "Switch git branches"),
		formatRow("p / b", "Cumulative PR diff vs base branch"),
		formatRow("Space", "Toggle full commit message details"),
		formatRow("Shift+Up / Down", "Scroll file preview from commit list"),
		formatRow("Enter (in files)", "Open selected file directly in Neovim"),
		formatRow("e", "Interactive git history reword"),
		formatRow("f", "Reword commit with 72-char auto-wrap"),
		formatRow("/", "Fuzzy search commits or files"),
		formatRow("h / ?", "Show this keybindings guide"),
		formatRow("q / Esc", "Close guide / Exit lgit"),
	}

	guideBox := lipgloss.JoinVertical(lipgloss.Left, guideRows...)

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		logoStyle.Render(strings.TrimPrefix(art3d, "\n")),
		taglineStyle.Render("terminal ui for git history and diffs"),
		"",
		guideBox,
		"",
		hintStyle.Render("press any key or Enter to continue..."),
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#30363d")).
		Padding(1, 4)

	renderedBox := boxStyle.Render(content)

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		renderedBox,
	)
}
