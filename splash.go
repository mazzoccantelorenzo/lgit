package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// splashLogo is the 3D isometric block ASCII art with a retro-terminal face.
const (
	splashASCII = `
   ___           _   _   
  / (_)         | | | |  
  | |    __ _   | |_| |_ 
  | |   / _' |  |  _  __|
  | |_ | (_| |  | | | |_ 
  \__(_)__,  |  \_| \___|
        |___/            `

	splashFace = `( ˘ ³˘)♥  or  ( •̀ ω •́ )✧`
)

// splashView renders a centered 3D ASCII logo with glowing gradients and a retro Kaomoji face.
func splashView(width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}

	logoStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#58a6ff"))

	taglineStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8b949e")).
		Italic(true)

	faceStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#3fb950"))

	dotsStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#d29922"))

	art3d := `
    __           _  __ 
   / /____ _    (_)/ /_
  / // __ '/   / // __/
 / // /_/ /   / // /_  
/_/ \__, /   /_/ \__/  
   /____/              `

	face := "( •̀ ω •́ )✧  loading repo..."

	boxContent := lipgloss.JoinVertical(
		lipgloss.Center,
		logoStyle.Render(strings.TrimPrefix(art3d, "\n")),
		"",
		faceStyle.Render(face),
		"",
		taglineStyle.Render("blazing fast terminal git history"),
		dotsStyle.Render("• • •"),
	)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#30363d")).
		Padding(1, 4)

	renderedBox := boxStyle.Render(boxContent)

	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		renderedBox,
	)
}
