package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

const lineNoWidth = 5 // "1234 " = 4 digits + space

// applyDiffBg adds a subtle background tint based on the line's diff style.
func applyDiffBg(s *State, style tcell.Style, ls LineStyle) tcell.Style {
	switch ls {
	case StyleAdded:
		return style.Background(s.Theme.BgAdded)
	case StyleRemoved:
		return style.Background(s.Theme.BgRemoved)
	default:
		return style
	}
}

// Render draws the screen
func Render(s *State) {
	screen := s.Screen
	screen.Clear()
	s.updateLayout()

	if s.TreeOpen {
		drawTree(s)
	}

	visible := s.Height - 1
	if s.SearchMode {
		visible-- // reserve one row for the search bar above the status bar
	}

	// Compute sticky hunk label: if the first visible line's hunk header
	// has scrolled off the top, show the hunk label on the first visible
	// diff content line so the user always knows which hunk they're in.
	stickyLabel := ""
	stickyHunkIdx := -1
	if len(s.Hunks) > 0 {
		hIdx := s.CurrentHunkIndex()
		if hIdx >= 0 && hIdx < len(s.Hunks) && s.Hunks[hIdx].StartLine < s.Scroll {
			stickyLabel = s.Hunks[hIdx].Label
			stickyHunkIdx = hIdx
		}
	}
	stickyUsed := false

	for i := 0; i < visible && s.Scroll+i < len(s.Lines); i++ {
		line := s.Lines[s.Scroll+i]

		// Apply sticky label to the first eligible diff content line
		// that belongs to the same hunk (don't leak into next hunk)
		if !stickyUsed && stickyLabel != "" && line.Label == "" &&
			line.Style != StyleNormal && line.Style != StyleFileHeader && line.Style != StyleHunkHeader &&
			line.HunkIdx == stickyHunkIdx {
			line.Label = stickyLabel
			stickyUsed = true
		}

		lineIdx := s.Scroll + i
		if s.SideBySide {
			drawSideBySideLine(s, i, line, lineIdx)
		} else {
			drawInlineLine(s, i, line, lineIdx)
		}
	}

	if s.SearchMode {
		drawSearchBar(s)
	}
	drawStatusBar(s)
	if s.ShowHelp {
		drawHelpOverlay(s)
	}
	screen.Show()
}

// drawFileHeader renders ── filename ────────────
func drawFileHeader(s *State, screen tcell.Screen, x, y int, text string, rightEdge int) {
	col := x
	// Leading decoration
	screen.SetContent(col, y, '─', nil, s.Theme.Dim)
	col++
	screen.SetContent(col, y, '─', nil, s.Theme.Dim)
	col++
	screen.SetContent(col, y, ' ', nil, s.Theme.Dim)
	col++
	// Filename
	for _, r := range text {
		if col >= rightEdge-1 {
			break
		}
		screen.SetContent(col, y, r, nil, s.Theme.FileHeader)
		col++
	}
	screen.SetContent(col, y, ' ', nil, s.Theme.Dim)
	col++
	// Trailing decoration
	for col < rightEdge {
		screen.SetContent(col, y, '─', nil, s.Theme.Dim)
		col++
	}
}

// drawGutter draws the label gutter and returns the column position after it.
// maxLabelWidth is the number of characters reserved for the label text.
func drawGutter(s *State, screen tcell.Screen, x, y int, line DisplayLine, maxLabelWidth int) int {
	col := x
	labelLen := len([]rune(line.Label))
	if line.Label != "" {
		for _, r := range line.Label {
			screen.SetContent(col, y, r, nil, s.Theme.Label)
			col++
		}
		// Pad if label is shorter than the widest label
		for i := labelLen; i < maxLabelWidth; i++ {
			screen.SetContent(col, y, ' ', nil, s.Theme.Dim)
			col++
		}
	} else {
		// No label: fill the label area with spaces
		for i := 0; i < maxLabelWidth; i++ {
			screen.SetContent(col, y, ' ', nil, s.Theme.Dim)
			col++
		}
	}
	// " │ " separator
	screen.SetContent(col, y, ' ', nil, s.Theme.Dim)
	col++
	screen.SetContent(col, y, '│', nil, s.Theme.Dim)
	col++
	screen.SetContent(col, y, ' ', nil, s.Theme.Dim)
	col++
	return col
}

// drawLineNo draws a line number (or blank) and returns the column position
func drawLineNo(s *State, screen tcell.Screen, col, y, num int) int {
	if num > 0 {
		str := fmt.Sprintf("%4d ", num)
		for _, r := range str {
			screen.SetContent(col, y, r, nil, s.Theme.LineNo)
			col++
		}
	} else {
		for i := 0; i < lineNoWidth; i++ {
			screen.SetContent(col, y, ' ', nil, s.Theme.Default)
			col++
		}
	}
	return col
}

// clearToEnd fills the rest of the line with spaces
func clearToEnd(s *State, screen tcell.Screen, col, y, width int) {
	for col < width {
		screen.SetContent(col, y, ' ', nil, s.Theme.Default)
		col++
	}
}

// searchHighlightStyle returns the highlight style for a search match.
// isCurrent indicates whether the line is the currently-focused match.
func searchHighlightStyle(s *State, baseStyle tcell.Style, isCurrent bool) tcell.Style {
	if isCurrent {
		return s.Theme.SearchCur
	}
	return baseStyle.Reverse(true)
}

// isCurrentMatchLine returns true if lineIdx is the line of the current search match.
func isCurrentMatchLine(s *State, lineIdx int) bool {
	if s.SearchIdx < 0 || s.SearchIdx >= len(s.SearchMatches) {
		return false
	}
	return s.SearchMatches[s.SearchIdx] == lineIdx
}

// drawTextWithHighlight draws text, highlighting search query matches.
// If there is no active search, it draws normally with baseStyle.
func drawTextWithHighlight(s *State, screen tcell.Screen, col, y int, text string, baseStyle tcell.Style, maxCol int, lineIdx int) int {
	if s.SearchQuery == "" || len(s.SearchMatches) == 0 {
		return drawText(screen, col, y, text, baseStyle, maxCol)
	}

	isCurrent := isCurrentMatchLine(s, lineIdx)
	hlStyle := searchHighlightStyle(s, baseStyle, isCurrent)

	runes := []rune(text)
	lowerRunes := []rune(strings.ToLower(text))
	queryRunes := []rune(strings.ToLower(s.SearchQuery))
	qRuneLen := len(queryRunes)

	i := 0
	for i < len(runes) {
		if col >= maxCol {
			break
		}
		// Check if a match starts at position i
		if i+qRuneLen <= len(lowerRunes) && string(lowerRunes[i:i+qRuneLen]) == string(queryRunes) {
			for j := 0; j < qRuneLen && col < maxCol; j++ {
				screen.SetContent(col, y, runes[i+j], nil, hlStyle)
				col++
			}
			i += qRuneLen
		} else {
			screen.SetContent(col, y, runes[i], nil, baseStyle)
			col++
			i++
		}
	}
	return col
}

// drawText draws text starting at col, returns final column
func drawText(screen tcell.Screen, col, y int, text string, style tcell.Style, maxCol int) int {
	for _, r := range text {
		if col >= maxCol {
			break
		}
		screen.SetContent(col, y, r, nil, style)
		col++
	}
	return col
}

// drawSyntaxText draws syntax-highlighted diff content.
// For non-continuation lines, the first character (op prefix) is drawn with diffStyle.
// The remaining code content is tokenized and colored by the highlighter.
// lineIdx is the index in s.Lines for search highlight overlay.
func drawSyntaxText(s *State, screen tcell.Screen, col, y int, text string, diffStyle tcell.Style, maxCol int, line DisplayLine, lineIdx int) int {
	if line.HunkIdx < 0 || line.HunkIdx >= len(s.Hunks) {
		return drawTextWithHighlight(s, screen, col, y, text, diffStyle, maxCol, lineIdx)
	}

	filename := s.Hunks[line.HunkIdx].File
	content := text

	// For non-continuation lines, first char is the op prefix (+/-/space)
	opVisible := !line.Continuation && (s.Wrap || s.ScrollX == 0)
	if opVisible && len(text) > 0 {
		runes := []rune(text)
		screen.SetContent(col, y, runes[0], nil, diffStyle)
		col++
		content = string(runes[1:])
	}

	// Build search highlight mask over the full text (rune positions)
	hlMask := buildSearchMask(s, text)

	// Compute the rune offset where content starts within text
	contentOffset := len([]rune(text)) - len([]rune(content))

	isCurrent := isCurrentMatchLine(s, lineIdx)
	dimmed := line.Style == StyleRemoved && !s.DiffBg
	spans := s.HL.Highlight(filename, content)
	runePos := contentOffset
	for _, span := range spans {
		style := span.Style
		if dimmed {
			style = style.Dim(true)
		}
		if s.DiffBg {
			style = applyDiffBg(s, style, line.Style)
		}
		for _, r := range span.Text {
			if col >= maxCol {
				return col
			}
			drawStyle := style
			if runePos < len(hlMask) && hlMask[runePos] {
				drawStyle = searchHighlightStyle(s, style, isCurrent)
			}
			screen.SetContent(col, y, r, nil, drawStyle)
			col++
			runePos++
		}
	}
	return col
}

// buildSearchMask returns a boolean slice where true indicates the rune at
// that position in text is part of a case-insensitive search match.
func buildSearchMask(s *State, text string) []bool {
	if s.SearchQuery == "" || len(s.SearchMatches) == 0 {
		return nil
	}
	runes := []rune(strings.ToLower(text))
	queryRunes := []rune(strings.ToLower(s.SearchQuery))
	qLen := len(queryRunes)
	if qLen == 0 {
		return nil
	}
	mask := make([]bool, len(runes))
	for i := 0; i <= len(runes)-qLen; i++ {
		if string(runes[i:i+qLen]) == string(queryRunes) {
			for j := 0; j < qLen; j++ {
				mask[i+j] = true
			}
		}
	}
	return mask
}

func drawInlineLine(s *State, y int, line DisplayLine, lineIdx int) {
	screen := s.Screen
	rightEdge := s.DiffX + s.DiffWidth

	// File header: decorative line
	if line.Style == StyleFileHeader {
		drawFileHeader(s, screen, s.DiffX, y, line.Text, rightEdge)
		return
	}

	// Normal blank lines
	if line.Style == StyleNormal {
		clearToEnd(s, screen, s.DiffX, y, rightEdge)
		return
	}

	// Hunk header and diff content get the gutter
	col := drawGutter(s, screen, s.DiffX, y, line, s.maxLabelWidth())

	// Line numbers (if enabled, for diff content lines only, blank for continuations)
	if s.LineNumbers && line.Style != StyleHunkHeader {
		lineNo := line.NewLineNo
		if line.Style == StyleRemoved {
			lineNo = line.OldLineNo
		}
		col = drawLineNo(s, screen, col, y, lineNo)
	}

	// Text content (apply horizontal scroll when not wrapping)
	text := line.Text
	if !s.Wrap && s.ScrollX > 0 && line.Style != StyleHunkHeader {
		runes := []rune(text)
		if s.ScrollX < len(runes) {
			text = string(runes[s.ScrollX:])
		} else {
			text = ""
		}
	}
	style := getStyle(s, line.Style)
	if s.DiffBg {
		style = applyDiffBg(s, style, line.Style)
	}
	if s.SyntaxHighlight && s.HL != nil && line.Style != StyleHunkHeader {
		col = drawSyntaxText(s, screen, col, y, text, style, rightEdge, line, lineIdx)
	} else {
		col = drawTextWithHighlight(s, screen, col, y, text, style, rightEdge, lineIdx)
	}
	if s.DiffBg {
		bgStyle := applyDiffBg(s, s.Theme.Default, line.Style)
		for col < rightEdge {
			screen.SetContent(col, y, ' ', nil, bgStyle)
			col++
		}
	} else {
		clearToEnd(s, screen, col, y, rightEdge)
	}
}

func drawSideBySideLine(s *State, y int, line DisplayLine, lineIdx int) {
	screen := s.Screen
	rightEdge := s.DiffX + s.DiffWidth

	// File header: decorative line
	if line.Style == StyleFileHeader {
		drawFileHeader(s, screen, s.DiffX, y, line.Text, rightEdge)
		return
	}

	// Normal lines: same as inline
	if line.Style == StyleNormal {
		drawInlineLine(s, y, line, lineIdx)
		return
	}

	// Hunk header: show function context on both columns
	if line.Style == StyleHunkHeader {
		lnoExtra := 0
		if s.LineNumbers {
			lnoExtra = lineNoWidth
		}
		colWidth := (s.DiffWidth - s.LabelGutter - 1) / 2
		contentWidth := colWidth - lnoExtra

		col := drawGutter(s, screen, s.DiffX, y, line, s.maxLabelWidth())

		// Left half
		if s.LineNumbers {
			col = drawLineNo(s, screen, col, y, 0)
		}
		col = drawText(screen, col, y, line.Text, s.Theme.HunkHeader, s.DiffX+s.LabelGutter+lnoExtra+contentWidth)
		leftEnd := s.DiffX + s.LabelGutter + lnoExtra + contentWidth
		for col < leftEnd {
			screen.SetContent(col, y, ' ', nil, s.Theme.Default)
			col++
		}

		// Center divider
		screen.SetContent(col, y, '│', nil, s.Theme.Dim)
		col++

		// Right half
		if s.LineNumbers {
			col = drawLineNo(s, screen, col, y, 0)
		}
		col = drawText(screen, col, y, line.Text, s.Theme.HunkHeader, rightEdge)
		clearToEnd(s, screen, col, y, rightEdge)
		return
	}

	// Diff content: split into two columns
	lnoExtra := 0
	if s.LineNumbers {
		lnoExtra = lineNoWidth
	}
	colWidth := (s.DiffWidth - s.LabelGutter - 1) / 2 // 1 for center divider
	contentWidth := colWidth - lnoExtra

	col := drawGutter(s, screen, s.DiffX, y, line, s.maxLabelWidth())

	// Apply horizontal scroll to text
	leftText := line.Left.Text
	rightText := line.Right.Text
	if s.ScrollX > 0 {
		if lr := []rune(leftText); s.ScrollX < len(lr) {
			leftText = string(lr[s.ScrollX:])
		} else {
			leftText = ""
		}
		if rr := []rune(rightText); s.ScrollX < len(rr) {
			rightText = string(rr[s.ScrollX:])
		} else {
			rightText = ""
		}
	}

	// Left half: line number + content
	if s.LineNumbers {
		col = drawLineNo(s, screen, col, y, line.Left.LineNo)
	}
	leftStyle := getStyle(s, line.Left.Style)
	col = drawHalfContent(s, screen, col, y, leftText, leftStyle, contentWidth, line, true, lineIdx)
	leftEnd := s.DiffX + s.LabelGutter + lnoExtra + contentWidth
	leftBgStyle := s.Theme.Default
	if s.DiffBg {
		leftBgStyle = applyDiffBg(s, leftBgStyle, line.Left.Style)
	}
	for col < leftEnd {
		screen.SetContent(col, y, ' ', nil, leftBgStyle)
		col++
	}

	// Center divider
	screen.SetContent(col, y, '│', nil, s.Theme.Dim)
	col++

	// Right half: line number + content
	if s.LineNumbers {
		col = drawLineNo(s, screen, col, y, line.Right.LineNo)
	}
	rightStyle := getStyle(s, line.Right.Style)
	col = drawHalfContent(s, screen, col, y, rightText, rightStyle, contentWidth, line, false, lineIdx)
	rightBgStyle := s.Theme.Default
	if s.DiffBg {
		rightBgStyle = applyDiffBg(s, rightBgStyle, line.Right.Style)
	}
	for col < rightEdge {
		screen.SetContent(col, y, ' ', nil, rightBgStyle)
		col++
	}
}

// drawHalfContent draws one half of a side-by-side line, with optional syntax highlighting.
func drawHalfContent(s *State, screen tcell.Screen, col, y int, text string, diffStyle tcell.Style, maxChars int, line DisplayLine, isLeft bool, lineIdx int) int {
	// Build search highlight mask for the half text.
	// Side-by-side search matching uses the half-line text, but
	// SearchMatches are indexed by s.Lines which store full DisplayLines.
	// The match was found on line.Text (inline) but for side-by-side, the
	// left/right text is what we display. We highlight the query in whatever
	// text we're drawing.
	hlMask := buildSearchMask(s, text)
	isCurrent := isCurrentMatchLine(s, lineIdx)

	if s.SyntaxHighlight && s.HL != nil && line.HunkIdx >= 0 && line.HunkIdx < len(s.Hunks) && text != "" {
		filename := s.Hunks[line.HunkIdx].File
		dimmed := !s.DiffBg && ((isLeft && line.Left.Style == StyleRemoved) || (!isLeft && line.Right.Style == StyleRemoved))

		runes := []rune(text)
		chars := 0
		content := text

		// First char is op prefix (+/-/space) for non-continuation lines
		if !line.Continuation && len(runes) > 0 {
			opStyle := diffStyle
			if s.DiffBg {
				halfStyle := line.Left.Style
				if !isLeft {
					halfStyle = line.Right.Style
				}
				opStyle = applyDiffBg(s, opStyle, halfStyle)
			}
			screen.SetContent(col, y, runes[0], nil, opStyle)
			col++
			chars = 1
			content = string(runes[1:])
		}

		contentOffset := len(runes) - len([]rune(content))
		spans := s.HL.Highlight(filename, content)
		runePos := contentOffset
		for _, span := range spans {
			style := span.Style
			if dimmed {
				style = style.Dim(true)
			}
			if s.DiffBg {
				halfStyle := line.Left.Style
				if !isLeft {
					halfStyle = line.Right.Style
				}
				style = applyDiffBg(s, style, halfStyle)
			}
			for _, r := range span.Text {
				if chars >= maxChars {
					return col
				}
				drawStyle := style
				if runePos < len(hlMask) && hlMask[runePos] {
					drawStyle = searchHighlightStyle(s, style, isCurrent)
				}
				screen.SetContent(col, y, r, nil, drawStyle)
				col++
				chars++
				runePos++
			}
		}
		return col
	}

	// Non-syntax path: draw with search highlights
	baseStyle := diffStyle
	if s.DiffBg {
		halfStyle := line.Left.Style
		if !isLeft {
			halfStyle = line.Right.Style
		}
		baseStyle = applyDiffBg(s, baseStyle, halfStyle)
	}
	runes := []rune(text)
	chars := 0
	for i, r := range runes {
		if chars >= maxChars {
			break
		}
		drawStyle := baseStyle
		if i < len(hlMask) && hlMask[i] {
			drawStyle = searchHighlightStyle(s, baseStyle, isCurrent)
		}
		screen.SetContent(col, y, r, nil, drawStyle)
		col++
		chars++
	}
	return col
}

func getStyle(s *State, ls LineStyle) tcell.Style {
	switch ls {
	case StyleFileHeader:
		return s.Theme.FileHeader
	case StyleHunkHeader:
		return s.Theme.HunkHeader
	case StyleAdded:
		return s.Theme.DiffAdded
	case StyleRemoved:
		return s.Theme.DiffRemoved
	default:
		return s.Theme.Default
	}
}

func drawStatusBar(s *State) {
	if s.FlashMsg != "" && time.Now().Before(s.FlashExpiry) {
		y := s.Height - 1
		msg := " " + s.FlashMsg + " "
		col := 0
		for _, r := range msg {
			if col >= s.Width {
				break
			}
			s.Screen.SetContent(col, y, r, nil, s.Theme.Flash)
			col++
		}
		for col < s.Width {
			s.Screen.SetContent(col, y, ' ', nil, s.Theme.Flash)
			col++
		}
		return
	}
	s.FlashMsg = ""

	var status string
	if s.PipeMode {
		status = fmt.Sprintf(" wiff (pipe) • %d hunks", len(s.Hunks))
	} else {
		status = fmt.Sprintf(" wiff %s • %d files • %d hunks",
			s.RefDisplay(), s.UniqueFiles(), len(s.Hunks))
	}

	if s.FilterFile != "" && !s.FullFile {
		status += fmt.Sprintf(" • viewing: %s", s.FilterFile)
	}

	if s.TreeFocused {
		status += " [TREE]"
	}

	if !s.PipeMode && !s.WatchEnabled {
		status += " [watch off]"
	}

	if len(s.SearchMatches) > 0 && s.SearchQuery != "" {
		if s.SearchIdx >= 0 && s.SearchIdx < len(s.SearchMatches) {
			status += fmt.Sprintf(" \u2022 \"%s\" [%d/%d]", s.SearchQuery, s.SearchIdx+1, len(s.SearchMatches))
		} else {
			status += fmt.Sprintf(" \u2022 \"%s\" [%d matches]", s.SearchQuery, len(s.SearchMatches))
		}
	}

	if pd := s.PendingDisplay(); pd != "" {
		status += fmt.Sprintf(" [%s…]", pd)
	}

	// Right-aligned help
	help := "(s)plit (n)ums (w)rap (e)xpl (h)l (/)search (+/-)ctx (q)uit"
	if s.TreeFocused {
		help = "j/k:nav enter:select a:all tab:diff esc:back q:quit"
	}
	pad := s.Width - len(status) - len(help) - 1
	if pad > 0 {
		for i := 0; i < pad; i++ {
			status += " "
		}
		status += help
	}

	y := s.Height - 1
	col := 0
	for _, r := range status {
		if col >= s.Width {
			break
		}
		s.Screen.SetContent(col, y, r, nil, s.Theme.StatusBar)
		col++
	}
	for col < s.Width {
		s.Screen.SetContent(col, y, ' ', nil, s.Theme.StatusBar)
		col++
	}
}

func drawHelpOverlay(s *State) {
	const boxW = 60
	const boxH = 27

	screen := s.Screen
	styleBorder := s.Theme.Dim
	styleTitle := s.Theme.Default.Bold(true)
	styleBody := s.Theme.Default

	// Center the box
	x0 := (s.Width - boxW) / 2
	y0 := (s.Height - boxH) / 2
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}

	// Fill interior with spaces
	for row := y0; row < y0+boxH && row < s.Height; row++ {
		for col := x0; col < x0+boxW && col < s.Width; col++ {
			screen.SetContent(col, row, ' ', nil, styleBody)
		}
	}

	// Draw border
	// Top edge
	screen.SetContent(x0, y0, '┌', nil, styleBorder)
	for col := x0 + 1; col < x0+boxW-1 && col < s.Width; col++ {
		screen.SetContent(col, y0, '─', nil, styleBorder)
	}
	if x0+boxW-1 < s.Width {
		screen.SetContent(x0+boxW-1, y0, '┐', nil, styleBorder)
	}
	// Bottom edge
	if y0+boxH-1 < s.Height {
		screen.SetContent(x0, y0+boxH-1, '└', nil, styleBorder)
		for col := x0 + 1; col < x0+boxW-1 && col < s.Width; col++ {
			screen.SetContent(col, y0+boxH-1, '─', nil, styleBorder)
		}
		if x0+boxW-1 < s.Width {
			screen.SetContent(x0+boxW-1, y0+boxH-1, '┘', nil, styleBorder)
		}
	}
	// Left and right edges
	for row := y0 + 1; row < y0+boxH-1 && row < s.Height; row++ {
		screen.SetContent(x0, row, '│', nil, styleBorder)
		if x0+boxW-1 < s.Width {
			screen.SetContent(x0+boxW-1, row, '│', nil, styleBorder)
		}
	}

	// Content area starts at (x0+2, y0+1), max width boxW-4
	cx := x0 + 2
	contentW := boxW - 4

	// Helper to draw a line of text at a given row within the box
	drawLine := func(row int, text string, style tcell.Style) {
		y := y0 + row
		if y >= s.Height || y < 0 {
			return
		}
		col := cx
		for _, r := range text {
			if col >= cx+contentW || col >= s.Width {
				break
			}
			screen.SetContent(col, y, r, nil, style)
			col++
		}
	}

	// Title (centered)
	title := "wiff - keyboard shortcuts"
	titleX := x0 + (boxW-len(title))/2
	if titleRow := y0 + 1; titleRow < s.Height {
		col := titleX
		for _, r := range title {
			if col >= x0+boxW-1 || col >= s.Width {
				break
			}
			screen.SetContent(col, titleRow, r, nil, styleTitle)
			col++
		}
	}

	// Help content lines (row offset from y0)
	// Box is 24 rows: row 0 border, row 1 title, rows 3-20 content,
	// row 21 blank, row 22 hint, row 23 border. Max 18 content lines.
	lines := []string{
		"Navigation                    Modes & Display",
		"j/k     scroll up/down        s   side-by-side",
		"d/u     half page down/up     n   line numbers",
		"g/G     top/bottom            w   wrap",
		"^D/^U   half page             e   file explorer",
		"Tab     next file             h   syntax highlight",
		"S-Tab   prev file             b   diff background",
		"                              f   full file view",
		"Hunks & Files                 W   watch mode",
		"]c/[c   next/prev hunk",
		"]f/[f   next/prev file        Search",
		"+/-     more/less context     /   start search",
		"mouse   scroll + tree click   n   next match",
		"dbl-clk copy chunk            N   prev match",
		"right-clk copy chunk          Esc clear search",
		"Yank (copies to clipboard)",
		"y+label yank added lines      File Tree",
		"Y+label yank removed lines    Tab focus tree",
		"p+label yank as patch         Enter select file",
		"o       open in $EDITOR       a   show all files",
		"?       help  q/Esc   quit",
	}

	startRow := 3
	for i, line := range lines {
		drawLine(startRow+i, line, styleBody)
	}

	// Dismiss hint at the bottom
	hint := "press any key to close"
	hintX := x0 + (boxW-len(hint))/2
	if hintRow := y0 + boxH - 2; hintRow < s.Height {
		col := hintX
		for _, r := range hint {
			if col >= x0+boxW-1 || col >= s.Width {
				break
			}
			screen.SetContent(col, hintRow, r, nil, s.Theme.Dim)
			col++
		}
	}
}
