package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// splashLoadingView renders the brief 3D ASCII loading animation (0.5s) upon opening lgit.
func splashLoadingView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	logoStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#58a6ff"))

	taglineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8b949e"))

	dotsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d29922"))

	art3d := `
    __           _  __ 
   / /____ _    (_)/ /_
  / // __ '/   / // __/
 / // /_/ /   / // /_  
/_/ \__, /   /_/ \__/  
   /____/              `

	content := lipgloss.JoinVertical(
		lipgloss.Center,
		logoStyle.Render(strings.TrimPrefix(art3d, "\n")),
		"",
		taglineStyle.Render("loading git repository..."),
		dotsStyle.Render("• • •"),
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#30363d")).
		Padding(1, 4)

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		boxStyle.Render(content),
	)
}

// helpModalView renders the keyboard shortcuts guide as a centered modal dialog with a dim border.
func helpModalView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#58a6ff"))

	subStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8b949e"))

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#79c0ff"))

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#c9d1d9"))

	hintStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#6e7681"))

	formatRow := func(key, desc string) string {
		k := keyStyle.Width(18).Render(key)
		d := descStyle.Render(desc)
		return lipgloss.JoinHorizontal(lipgloss.Top, k, d)
	}

	guideRows := []string{
		headerStyle.Render("KEYBOARD SHORTCUTS"),
		subStyle.Render("Navigation & Commands"),
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
		formatRow("h / ?", "Toggle this help modal"),
		formatRow("Esc / q", "Close modal / Exit lgit"),
		"",
		hintStyle.Render("Press Esc, Enter, or h to close"),
	}

	boxContent := lipgloss.JoinVertical(lipgloss.Left, guideRows...)

	modalBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#58a6ff")).
		Background(lipgloss.Color("#0d1117")).
		Padding(1, 3).
		Render(boxContent)

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		modalBox,
	)
}

// overlayModal renders the modal content on top of background lines.
func overlayModal(background string, modal string, width, height int) string {
	bgLines := strings.Split(background, "\n")
	modalLines := strings.Split(modal, "\n")

	modalH := len(modalLines)
	if modalH == 0 {
		return background
	}

	modalW := lipgloss.Width(modalLines[0])
	startY := (height - modalH) / 2
	startX := (width - modalW) / 2

	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	for len(bgLines) < height {
		bgLines = append(bgLines, strings.Repeat(" ", width))
	}

	for i, mLine := range modalLines {
		targetY := startY + i
		if targetY >= len(bgLines) {
			break
		}

		origLine := bgLines[targetY]
		origRunes := []rune(origLine)

		if len(origRunes) < width {
			origRunes = append(origRunes, []rune(strings.Repeat(" ", width-len(origRunes)))...)
		}

		mLen := lipgloss.Width(mLine)
		endX := startX + mLen
		if endX > len(origRunes) {
			endX = len(origRunes)
		}

		// Composite line
		var leftPart string
		if startX > 0 && startX <= len(origRunes) {
			leftPart = string(origRunes[:startX])
		}
		var rightPart string
		if endX < len(origRunes) {
			rightPart = string(origRunes[endX:])
		}

		bgLines[targetY] = leftPart + mLine + rightPart
	}

	return strings.Join(bgLines, "\n")
}
