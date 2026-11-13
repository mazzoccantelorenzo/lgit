package main

import (
	"strings"

	"github.com/atotto/clipboard"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const tabDisplayWidth = 4

// diffPosition is the zero-based line and visual column coordinate inside the diff document.
type diffPosition struct {
	line   int
	column int
}

// diffSelection is the mouse selection state that tracks highlighted ranges inside the diff viewport.
type diffSelection struct {
	active   bool
	dragging bool
	anchor   diffPosition
	cursor   diffPosition
}

// hasRange is the selection predicate that reports whether the user dragged across at least one cell.
func (selection diffSelection) hasRange() bool {
	if !selection.active {
		return false
	}
	return selection.anchor.line != selection.cursor.line || selection.anchor.column != selection.cursor.column
}

// orderedBounds is the coordinate normalizer that returns the selection start and end in document order.
func (selection diffSelection) orderedBounds() (diffPosition, diffPosition) {
	if selection.anchor.line < selection.cursor.line {
		return selection.anchor, selection.cursor
	}
	if selection.anchor.line > selection.cursor.line {
		return selection.cursor, selection.anchor
	}
	if selection.anchor.column <= selection.cursor.column {
		return selection.anchor, selection.cursor
	}
	return selection.cursor, selection.anchor
}

// setDiffContent is the diff state updater that stores raw diff text, resets selection, and refreshes the viewport.
func (m *model) setDiffContent(content string, colorized bool) {
	m.rawDiff = content
	m.diffColorized = colorized
	m.selection = diffSelection{}
	m.refreshDiffViewport()
}

// refreshDiffViewport is the renderer that updates the viewport with syntax colors and active mouse selection highlights.
func (m *model) refreshDiffViewport() {
	if m.diffColorized {
		m.diffView.SetContent(renderDiffWithSelection(m.rawDiff, m.selection))
		return
	}
	m.diffView.SetContent(renderPlainTextWithSelection(m.rawDiff, m.selection))
}

// handleMouseEvent is the mouse controller that restricts drag selection and wheel scrolling to the diff viewport.
func (m *model) handleMouseEvent(mouseEvent tea.MouseMsg) {
	left, top, width, height, visible := m.diffViewportBounds()
	if !visible || width <= 0 || height <= 0 {
		return
	}

	insideHorizontal := mouseEvent.X >= left && mouseEvent.X < left+width
	insideVertical := mouseEvent.Y >= top && mouseEvent.Y < top+height
	insideDiff := insideHorizontal && insideVertical

	if mouseEvent.Button == tea.MouseButtonWheelUp && (insideDiff || m.diffFocus) {
		m.diffView.LineUp(3)
		return
	}
	if mouseEvent.Button == tea.MouseButtonWheelDown && (insideDiff || m.diffFocus) {
		m.diffView.LineDown(3)
		return
	}

	switch mouseEvent.Action {
	case tea.MouseActionPress:
		if mouseEvent.Button != tea.MouseButtonLeft {
			return
		}
		if !insideDiff {
			if m.selection.active {
				m.selection = diffSelection{}
				m.refreshDiffViewport()
			}
			return
		}
		if m.state == 1 || m.state == 2 {
			m.diffFocus = true
		}
		position := m.mouseDiffPosition(mouseEvent.X, mouseEvent.Y, left, top, width, height)
		m.selection = diffSelection{
			active:   true,
			dragging: true,
			anchor:   position,
			cursor:   position,
		}
		m.refreshDiffViewport()

	case tea.MouseActionMotion:
		if !m.selection.dragging {
			return
		}
		if mouseEvent.Y < top && m.diffView.YOffset > 0 {
			m.diffView.LineUp(1)
		} else if mouseEvent.Y >= top+height && !m.diffView.AtBottom() {
			m.diffView.LineDown(1)
		}
		position := m.mouseDiffPosition(mouseEvent.X, mouseEvent.Y, left, top, width, height)
		if position != m.selection.cursor {
			m.selection.cursor = position
			m.refreshDiffViewport()
		}

	case tea.MouseActionRelease:
		if !m.selection.dragging {
			return
		}
		m.selection.dragging = false
		if !m.selection.hasRange() {
			m.selection = diffSelection{}
			m.refreshDiffViewport()
			return
		}
		m.copySelectedDiffToClipboard()
	}
}

// copySelectedDiffToClipboard is the clipboard dispatcher that copies the currently highlighted diff text.
func (m *model) copySelectedDiffToClipboard() {
	selectedText := extractSelectedDiffText(m.rawDiff, m.selection, m.diffColorized)
	if selectedText == "" {
		return
	}
	if m.clipboardWriter != nil {
		_ = m.clipboardWriter(selectedText)
		return
	}
	_ = clipboard.WriteAll(selectedText)
}

// diffViewportBounds is the layout geometry resolver that computes the terminal screen coordinates of the diff viewport.
func (m model) diffViewportBounds() (left, top, width, height int, visible bool) {
	if m.width <= 0 || m.height < minimumDetailHeight {
		return 0, 0, 0, 0, false
	}

	detailWidth := m.width
	detailLeft := docStyle.GetMarginLeft()
	preview := m.state == 0

	if preview {
		if m.width < minimumSplitWidth {
			return 0, 0, 0, 0, false
		}
		commitWidth := m.width / 2
		detailWidth = m.width - commitWidth
		detailLeft = docStyle.GetMarginLeft() + commitWidth
	}

	if detailWidth < 16 {
		return 0, 0, 0, 0, false
	}

	contentTop := docStyle.GetMarginTop() + diffStyle.GetBorderTopSize() + diffStyle.GetPaddingTop()
	if (preview && detailWidth < compactPreviewWidth) || (!preview && detailWidth < minimumSplitWidth) {
		left = detailLeft + diffStyle.GetBorderLeftSize() + diffStyle.GetPaddingLeft()
		top = contentTop + 3
		return left, top, m.diffView.Width, m.diffView.Height, true
	}

	fileWidth, _, _ := detailDimensions(detailWidth, m.height)
	sideFrameWidth, _ := sidebarStyle.GetFrameSize()
	left = detailLeft + fileWidth + sideFrameWidth + diffStyle.GetBorderLeftSize() + diffStyle.GetPaddingLeft()
	top = contentTop + 2
	return left, top, m.diffView.Width, m.diffView.Height, true
}

// mouseDiffPosition is the coordinate converter that maps a terminal cell into a clamped diff line and column.
func (m model) mouseDiffPosition(screenX, screenY, left, top, width, height int) diffPosition {
	relativeX := screenX - left
	if relativeX < 0 {
		relativeX = 0
	}
	if relativeX >= width {
		relativeX = width - 1
	}

	relativeY := screenY - top
	if relativeY < 0 {
		relativeY = 0
	}
	if relativeY >= height {
		relativeY = height - 1
	}

	lines := strings.Split(m.rawDiff, "\n")
	lineIndex := m.diffView.YOffset + relativeY
	if lineIndex < 0 {
		lineIndex = 0
	}
	if len(lines) > 0 && lineIndex >= len(lines) {
		lineIndex = len(lines) - 1
	}

	return diffPosition{
		line:   lineIndex,
		column: relativeX,
	}
}

// renderDiffWithSelection is the diff formatter that applies syntax colors and highlights any selected code region.
func renderDiffWithSelection(diff string, selection diffSelection) string {
	lines := strings.Split(diff, "\n")
	renderedLines := make([]string, len(lines))

	additionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7ee787"))
	removalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffa198"))
	hunkHeaderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#79c0ff"))
	contextStyle := lipgloss.NewStyle()

	for lineIndex, line := range lines {
		lineStyle := contextStyle
		hasLineStyle := false
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			lineStyle = additionStyle
			hasLineStyle = true
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			lineStyle = removalStyle
			hasLineStyle = true
		} else if strings.HasPrefix(line, "@@") {
			lineStyle = hunkHeaderStyle
			hasLineStyle = true
		}

		startColumn, endColumn, isSelected := lineSelectionColumnRange(line, lineIndex, selection, true)
		if !isSelected {
			if hasLineStyle {
				renderedLines[lineIndex] = lineStyle.Render(line)
			} else {
				renderedLines[lineIndex] = expandTabsForDisplay(line)
			}
			continue
		}

		renderedLines[lineIndex] = renderHighlightedLine(line, startColumn, endColumn, lineStyle, hasLineStyle)
	}

	return strings.Join(renderedLines, "\n")
}

// renderPlainTextWithSelection is the plain-text formatter that highlights selected regions in non-diff messages.
func renderPlainTextWithSelection(content string, selection diffSelection) string {
	lines := strings.Split(content, "\n")
	renderedLines := make([]string, len(lines))
	plainStyle := lipgloss.NewStyle()

	for lineIndex, line := range lines {
		startColumn, endColumn, isSelected := lineSelectionColumnRange(line, lineIndex, selection, false)
		if !isSelected {
			renderedLines[lineIndex] = expandTabsForDisplay(line)
			continue
		}
		renderedLines[lineIndex] = renderHighlightedLine(line, startColumn, endColumn, plainStyle, false)
	}

	return strings.Join(renderedLines, "\n")
}

// renderHighlightedLine is the line renderer that styles the selected column span with a high-contrast background.
func renderHighlightedLine(line string, startColumn, endColumn int, baseStyle lipgloss.Style, hasBaseStyle bool) string {
	beforeDisplay, selectedDisplay, afterDisplay := sliceDisplayLineByColumns(line, startColumn, endColumn)
	highlightStyle := baseStyle.Background(lipgloss.Color("#264f78"))
	if !hasBaseStyle {
		highlightStyle = highlightStyle.Foreground(lipgloss.Color("#ffffff"))
	}

	var builder strings.Builder
	if beforeDisplay != "" {
		if hasBaseStyle {
			builder.WriteString(baseStyle.Render(beforeDisplay))
		} else {
			builder.WriteString(beforeDisplay)
		}
	}
	if selectedDisplay != "" {
		builder.WriteString(highlightStyle.Render(selectedDisplay))
	}
	if afterDisplay != "" {
		if hasBaseStyle {
			builder.WriteString(baseStyle.Render(afterDisplay))
		} else {
			builder.WriteString(afterDisplay)
		}
	}
	return builder.String()
}

// extractSelectedDiffText is the text extractor that builds clean clipboard text from the selected diff lines.
func extractSelectedDiffText(diff string, selection diffSelection, colorized bool) string {
	if !selection.hasRange() || diff == "" {
		return ""
	}

	lines := strings.Split(diff, "\n")
	selectionStart, selectionEnd := selection.orderedBounds()
	if selectionStart.line >= len(lines) {
		return ""
	}
	if selectionEnd.line >= len(lines) {
		selectionEnd.line = len(lines) - 1
	}

	var extractedLines []string
	for lineIndex := selectionStart.line; lineIndex <= selectionEnd.line; lineIndex++ {
		line := lines[lineIndex]
		startColumn, endColumn, isSelected := lineSelectionColumnRange(line, lineIndex, selection, colorized)
		if !isSelected {
			if selectionStart.line != selectionEnd.line {
				extractedLines = append(extractedLines, "")
			}
			continue
		}
		extractedLines = append(extractedLines, sliceRawLineByColumns(line, startColumn, endColumn))
	}

	return strings.Join(extractedLines, "\n")
}

// lineSelectionColumnRange is the span calculator that resolves the selected visual column bounds for a single line.
func lineSelectionColumnRange(line string, lineIndex int, selection diffSelection, colorized bool) (startColumn, endColumn int, selected bool) {
	if !selection.hasRange() {
		return 0, 0, false
	}

	selectionStart, selectionEnd := selection.orderedBounds()
	if lineIndex < selectionStart.line || lineIndex > selectionEnd.line {
		return 0, 0, false
	}

	minimumColumn := 0
	if colorized && isHunkContentLine(line) {
		minimumColumn = 1
	}

	lineWidth := visualLineWidth(line)
	if selectionStart.line == selectionEnd.line {
		startColumn = selectionStart.column
		endColumn = selectionEnd.column + 1
	} else if lineIndex == selectionStart.line {
		startColumn = selectionStart.column
		endColumn = lineWidth
	} else if lineIndex == selectionEnd.line {
		startColumn = minimumColumn
		endColumn = selectionEnd.column + 1
	} else {
		startColumn = minimumColumn
		endColumn = lineWidth
	}

	if startColumn < minimumColumn {
		startColumn = minimumColumn
	}
	if endColumn > lineWidth {
		endColumn = lineWidth
	}
	if startColumn >= endColumn {
		return 0, 0, false
	}
	return startColumn, endColumn, true
}

// isHunkContentLine is the diff classifier that identifies added, removed, and context source lines inside a hunk.
func isHunkContentLine(line string) bool {
	if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
		return true
	}
	if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
		return true
	}
	return strings.HasPrefix(line, " ")
}

// visualLineWidth is the width calculator that measures a line in terminal cells with four-space tab expansion.
func visualLineWidth(line string) int {
	totalWidth := 0
	for _, character := range line {
		totalWidth += runeVisualWidth(character)
	}
	return totalWidth
}

// runeVisualWidth is the cell width calculator for a single rune inside the diff viewport.
func runeVisualWidth(character rune) int {
	if character == '\t' {
		return tabDisplayWidth
	}
	width := lipgloss.Width(string(character))
	if width < 1 {
		return 1
	}
	return width
}

// expandTabsForDisplay is the string normalizer that replaces tabs with spaces so display columns stay aligned.
func expandTabsForDisplay(line string) string {
	return strings.ReplaceAll(line, "\t", strings.Repeat(" ", tabDisplayWidth))
}

// sliceDisplayLineByColumns is the display slicer that splits a line into before, selected, and after visual segments.
func sliceDisplayLineByColumns(line string, startColumn, endColumn int) (before, selected, after string) {
	displayLine := expandTabsForDisplay(line)
	var beforeBuilder strings.Builder
	var selectedBuilder strings.Builder
	var afterBuilder strings.Builder

	currentColumn := 0
	for _, character := range displayLine {
		characterWidth := runeVisualWidth(character)
		nextColumn := currentColumn + characterWidth
		if nextColumn <= startColumn {
			beforeBuilder.WriteRune(character)
		} else if currentColumn >= endColumn {
			afterBuilder.WriteRune(character)
		} else {
			selectedBuilder.WriteRune(character)
		}
		currentColumn = nextColumn
	}

	return beforeBuilder.String(), selectedBuilder.String(), afterBuilder.String()
}

// sliceRawLineByColumns is the clipboard slicer that extracts raw characters overlapping the selected visual columns.
func sliceRawLineByColumns(line string, startColumn, endColumn int) string {
	var selectedBuilder strings.Builder
	currentColumn := 0
	for _, character := range line {
		characterWidth := runeVisualWidth(character)
		nextColumn := currentColumn + characterWidth
		if nextColumn > startColumn && currentColumn < endColumn {
			selectedBuilder.WriteRune(character)
		}
		currentColumn = nextColumn
	}
	return selectedBuilder.String()
}
