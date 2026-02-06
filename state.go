package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// State holds the application state
type State struct {
	Refs         []string
	Staged       bool
	Hunks        []Hunk
	Scroll       int
	Height       int
	Width        int
	PendingKey   rune
	PendingLabel string // accumulated label chars for multi-char yank
	PendingTime  time.Time
	Screen       tcell.Screen
	Lines        []DisplayLine
	PipeMode     bool
	SideBySide   bool
	LineNumbers  bool
	ContextLines int
	Wrap         bool
	ScrollX      int
	WatchEnabled bool

	Theme UITheme

	SyntaxHighlight bool
	HL              *Highlighter

	SearchMode    bool   // true when typing a search query
	SearchQuery   string // current search text
	SearchMatches []int  // line indices that match
	SearchIdx     int    // current match index (-1 if none)

	TreeOpen    bool
	TreeFiles   []TreeFile
	TreeNodes   []TreeNode // hierarchical tree for display
	TreeFocused bool
	TreeCursor  int
	TreeScroll  int
	FilterFile  string // when set, only show hunks for this file
	DiffX       int    // starting column for diff content (after tree sidebar)
	DiffWidth   int    // available width for diff content
	LabelGutter int    // dynamic gutter width: max label chars + 3 (" │ ")

	DiffBg bool // subtle background tints on added/removed lines

	FullFile     bool   // full-file view mode
	FullFileName string // file being viewed in full-file mode

	FollowMode bool // auto-scroll to new changes on watch reload

	ShowHelp    bool
	FlashMsg    string
	FlashExpiry time.Time
}

// HalfLine represents one side of a side-by-side display
type HalfLine struct {
	Text   string
	Style  LineStyle
	LineNo int
}

// DisplayLine represents a rendered line
type DisplayLine struct {
	Text         string
	Style        LineStyle
	Label        string // hunk label (a, b, c...)
	HunkIdx      int    // -1 if not a hunk line
	OldLineNo    int    // old file line number (0 = none)
	NewLineNo    int    // new file line number (0 = none)
	Continuation bool   // wrapped continuation of previous line
	Left         HalfLine
	Right        HalfLine
}

type LineStyle int

const (
	StyleNormal LineStyle = iota
	StyleFileHeader
	StyleHunkHeader
	StyleAdded
	StyleRemoved
	StyleContext
)

// updateLayout computes DiffX and DiffWidth based on tree state
func (s *State) updateLayout() {
	if s.TreeOpen {
		s.DiffX = treeWidth + 1 // +1 for divider
		s.DiffWidth = s.Width - treeWidth - 1
	} else {
		s.DiffX = 0
		s.DiffWidth = s.Width
	}
	if s.DiffWidth < 1 {
		s.DiffWidth = 1
	}
}

// maxLabelWidth returns the number of characters used by the widest label.
// Returns at least 1 so the gutter is never zero-width.
func (s *State) maxLabelWidth() int {
	maxW := 1
	for _, h := range s.Hunks {
		if w := len([]rune(h.Label)); w > maxW {
			maxW = w
		}
	}
	return maxW
}

// computeLabelGutter sets LabelGutter based on the widest hunk label.
// The gutter consists of the label text plus the " │ " separator (3 chars).
func (s *State) computeLabelGutter() {
	s.LabelGutter = s.maxLabelWidth() + 3 // label + " │ "
}

// BuildLines creates display lines from hunks
func (s *State) BuildLines() {
	s.updateLayout()
	s.computeLabelGutter()
	// Reset all StartLine to prevent stale values when switching views
	for i := range s.Hunks {
		s.Hunks[i].StartLine = -1
	}
	if s.FullFile && s.FullFileName != "" {
		if s.SideBySide {
			s.buildFullFileSideBySideLines()
			if s.Wrap {
				s.wrapSideBySideLines()
			}
		} else {
			s.buildFullFileLines()
			if s.Wrap {
				s.wrapLines()
			}
		}
	} else if s.SideBySide {
		s.buildSideBySideLines()
		if s.Wrap {
			s.wrapSideBySideLines()
		}
	} else {
		s.buildInlineLines()
		if s.Wrap {
			s.wrapLines()
		}
	}
	// Refresh search matches since line indices changed
	if s.SearchQuery != "" {
		UpdateMatches(s)
	}
}

// sideBySideColWidth returns the character width available for each column
// in side-by-side mode (including the op prefix character).
func (s *State) sideBySideColWidth() int {
	lnoExtra := 0
	if s.LineNumbers {
		lnoExtra = lineNoWidth
	}
	colWidth := (s.DiffWidth - s.LabelGutter - 1) / 2
	tw := colWidth - lnoExtra
	if tw < 1 {
		tw = 1
	}
	return tw
}

// wrapSideBySideLines splits long half-lines into continuation DisplayLines
func (s *State) wrapSideBySideLines() {
	tw := s.sideBySideColWidth()
	var wrapped []DisplayLine
	for _, line := range s.Lines {
		if line.Style == StyleNormal || line.Style == StyleFileHeader || line.Style == StyleHunkHeader {
			wrapped = append(wrapped, line)
			continue
		}

		leftRunes := []rune(line.Left.Text)
		rightRunes := []rune(line.Right.Text)

		if len(leftRunes) <= tw && len(rightRunes) <= tw {
			wrapped = append(wrapped, line)
			continue
		}

		// First chunk keeps line numbers
		lEnd := tw
		if lEnd > len(leftRunes) {
			lEnd = len(leftRunes)
		}
		rEnd := tw
		if rEnd > len(rightRunes) {
			rEnd = len(rightRunes)
		}
		wrapped = append(wrapped, DisplayLine{
			Style:   line.Style,
			Label:   line.Label,
			HunkIdx: line.HunkIdx,
			Left:    HalfLine{Text: string(leftRunes[:lEnd]), Style: line.Left.Style, LineNo: line.Left.LineNo},
			Right:   HalfLine{Text: string(rightRunes[:rEnd]), Style: line.Right.Style, LineNo: line.Right.LineNo},
		})
		leftRunes = leftRunes[lEnd:]
		rightRunes = rightRunes[rEnd:]

		// Continuation lines
		for len(leftRunes) > 0 || len(rightRunes) > 0 {
			var lText, rText string
			if len(leftRunes) > 0 {
				end := tw
				if end > len(leftRunes) {
					end = len(leftRunes)
				}
				lText = string(leftRunes[:end])
				leftRunes = leftRunes[end:]
			}
			if len(rightRunes) > 0 {
				end := tw
				if end > len(rightRunes) {
					end = len(rightRunes)
				}
				rText = string(rightRunes[:end])
				rightRunes = rightRunes[end:]
			}
			wrapped = append(wrapped, DisplayLine{
				Style:        line.Style,
				HunkIdx:      line.HunkIdx,
				Continuation: true,
				Left:         HalfLine{Text: lText, Style: line.Left.Style},
				Right:        HalfLine{Text: rText, Style: line.Right.Style},
			})
		}
	}
	s.Lines = wrapped
	// Fix StartLine values after wrapping shifted indices
	for i, line := range s.Lines {
		if line.Style == StyleHunkHeader && line.Label != "" {
			if h := s.HunkByLabel(line.Label); h != nil {
				h.StartLine = i
			}
		}
	}
}

// textWidth returns the available character width for text content in inline mode
func (s *State) textWidth() int {
	w := s.DiffWidth - s.LabelGutter
	if s.LineNumbers {
		w -= lineNoWidth
	}
	if w < 1 {
		w = 1
	}
	return w
}

// wrapLines splits long lines into continuation DisplayLines
func (s *State) wrapLines() {
	tw := s.textWidth()
	var wrapped []DisplayLine
	for _, line := range s.Lines {
		// Don't wrap non-content lines
		if line.Style == StyleNormal || line.Style == StyleFileHeader || line.Style == StyleHunkHeader {
			wrapped = append(wrapped, line)
			continue
		}
		runes := []rune(line.Text)
		if len(runes) <= tw {
			wrapped = append(wrapped, line)
			continue
		}
		// First chunk keeps line numbers
		wrapped = append(wrapped, DisplayLine{
			Text:      string(runes[:tw]),
			Style:     line.Style,
			HunkIdx:   line.HunkIdx,
			OldLineNo: line.OldLineNo,
			NewLineNo: line.NewLineNo,
		})
		runes = runes[tw:]
		for len(runes) > 0 {
			end := tw
			if end > len(runes) {
				end = len(runes)
			}
			wrapped = append(wrapped, DisplayLine{
				Text:         string(runes[:end]),
				Style:        line.Style,
				HunkIdx:      line.HunkIdx,
				Continuation: true,
			})
			runes = runes[end:]
		}
	}
	s.Lines = wrapped
	// Fix StartLine values after wrapping shifted indices
	for i, line := range s.Lines {
		if line.Style == StyleHunkHeader && line.Label != "" {
			if h := s.HunkByLabel(line.Label); h != nil {
				h.StartLine = i
			}
		}
	}
}

func (s *State) buildInlineLines() {
	var lines []DisplayLine
	var currentFile string

	for i := range s.Hunks {
		h := &s.Hunks[i]

		// Skip hunks not matching the filter
		if s.FilterFile != "" && h.File != s.FilterFile {
			h.StartLine = -1
			continue
		}

		// File header
		if h.File != currentFile {
			if currentFile != "" {
				lines = append(lines, DisplayLine{Style: StyleNormal})
			}
			lines = append(lines, DisplayLine{
				Text:  h.File,
				Style: StyleFileHeader,
			})
			currentFile = h.File
		}

		// Blank line before hunk
		lines = append(lines, DisplayLine{Style: StyleNormal})

		// Record start line for navigation
		h.StartLine = len(lines)

		// Hunk header with label (clean: just the function context)
		lines = append(lines, DisplayLine{
			Text:    h.Comment,
			Style:   StyleHunkHeader,
			Label:   h.Label,
			HunkIdx: i,
		})

		// Diff lines with line number tracking
		oldNo := h.OldStart
		newNo := h.NewStart
		for _, dl := range h.Lines {
			style := StyleContext
			var oln, nln int
			switch dl.Op {
			case '+':
				style = StyleAdded
				nln = newNo
				newNo++
			case '-':
				style = StyleRemoved
				oln = oldNo
				oldNo++
			default:
				oln = oldNo
				nln = newNo
				oldNo++
				newNo++
			}
			lines = append(lines, DisplayLine{
				Text:      string(dl.Op) + dl.Content,
				Style:     style,
				HunkIdx:   i,
				OldLineNo: oln,
				NewLineNo: nln,
			})
		}
	}

	s.Lines = lines
}

func (s *State) buildFullFileLines() {
	// Read the NEW version of the file from disk
	root, err := gitRoot()
	if err != nil {
		return
	}
	path := filepath.Join(root, s.FullFileName)
	content, err := os.ReadFile(path)
	if err != nil {
		// File might be deleted, try git show
		content, _ = exec.Command("git", "show", "HEAD:"+s.FullFileName).Output()
		if content == nil {
			return
		}
	}

	fileLines := strings.Split(strings.TrimRight(string(content), "\n"), "\n")

	// Collect hunks for this file and find the first hunk index (for syntax highlighting)
	type indexedHunk struct {
		hunk      *Hunk
		globalIdx int
	}
	var fileHunks []indexedHunk
	firstHunkIdx := -1
	for i := range s.Hunks {
		if s.Hunks[i].File == s.FullFileName {
			if firstHunkIdx < 0 {
				firstHunkIdx = i
			}
			fileHunks = append(fileHunks, indexedHunk{hunk: &s.Hunks[i], globalIdx: i})
		}
	}

	// For context lines between hunks, use firstHunkIdx so syntax highlighting
	// can determine the language from the file name. If no hunks, use -1.
	contextHunkIdx := firstHunkIdx

	var lines []DisplayLine

	// File header
	lines = append(lines, DisplayLine{
		Text:  s.FullFileName,
		Style: StyleFileHeader,
	})

	newLineNo := 1 // current position in the new file (1-based)
	oldLineNo := 1 // tracking old file line numbers

	for _, fh := range fileHunks {
		h := fh.hunk
		hIdx := fh.globalIdx

		// Emit context lines from current position up to this hunk
		for newLineNo < h.NewStart && newLineNo-1 < len(fileLines) {
			lines = append(lines, DisplayLine{
				Text:      " " + fileLines[newLineNo-1],
				Style:     StyleContext,
				HunkIdx:   contextHunkIdx,
				OldLineNo: oldLineNo,
				NewLineNo: newLineNo,
			})
			newLineNo++
			oldLineNo++
		}

		// Blank line before hunk
		lines = append(lines, DisplayLine{Style: StyleNormal})

		// Record start line for navigation
		h.StartLine = len(lines)

		// Hunk header
		lines = append(lines, DisplayLine{
			Text:    h.Comment,
			Style:   StyleHunkHeader,
			Label:   h.Label,
			HunkIdx: hIdx,
		})

		// Walk hunk lines
		hunkOldNo := h.OldStart
		hunkNewNo := h.NewStart
		for _, dl := range h.Lines {
			switch dl.Op {
			case ' ':
				lines = append(lines, DisplayLine{
					Text:      " " + dl.Content,
					Style:     StyleContext,
					HunkIdx:   hIdx,
					OldLineNo: hunkOldNo,
					NewLineNo: hunkNewNo,
				})
				hunkOldNo++
				hunkNewNo++
			case '+':
				lines = append(lines, DisplayLine{
					Text:      "+" + dl.Content,
					Style:     StyleAdded,
					HunkIdx:   hIdx,
					NewLineNo: hunkNewNo,
				})
				hunkNewNo++
			case '-':
				lines = append(lines, DisplayLine{
					Text:      "-" + dl.Content,
					Style:     StyleRemoved,
					HunkIdx:   hIdx,
					OldLineNo: hunkOldNo,
				})
				hunkOldNo++
			}
		}

		// Blank line after hunk
		lines = append(lines, DisplayLine{Style: StyleNormal})

		// Update position tracking
		newLineNo = hunkNewNo
		oldLineNo = hunkOldNo
	}

	// Emit remaining file lines after the last hunk
	for newLineNo-1 < len(fileLines) {
		lines = append(lines, DisplayLine{
			Text:      " " + fileLines[newLineNo-1],
			Style:     StyleContext,
			HunkIdx:   contextHunkIdx,
			OldLineNo: oldLineNo,
			NewLineNo: newLineNo,
		})
		newLineNo++
		oldLineNo++
	}

	s.Lines = lines
}

// reconstructOldFile derives the old file content from the new file and diff hunks.
// This is reliable because we always have the new file and the hunk data.
func (s *State) reconstructOldFile(filename string, newLines []string) []string {
	var fileHunks []*Hunk
	for i := range s.Hunks {
		if s.Hunks[i].File == filename {
			fileHunks = append(fileHunks, &s.Hunks[i])
		}
	}
	if len(fileHunks) == 0 {
		// No changes to this file: old == new
		return append([]string{}, newLines...)
	}

	var old []string
	newPos := 1 // 1-based position in new file

	for _, h := range fileHunks {
		// Context before hunk: same in both files
		for newPos < h.NewStart && newPos-1 < len(newLines) {
			old = append(old, newLines[newPos-1])
			newPos++
		}
		// Walk hunk lines
		for _, dl := range h.Lines {
			switch dl.Op {
			case ' ':
				old = append(old, dl.Content)
				newPos++
			case '-':
				old = append(old, dl.Content)
			case '+':
				newPos++
			}
		}
	}

	// Remaining lines after last hunk
	for newPos-1 < len(newLines) {
		old = append(old, newLines[newPos-1])
		newPos++
	}
	return old
}

// readNewFile returns the new version of a file as lines.
func (s *State) readNewFile(filename string) []string {
	if s.Staged {
		out, _ := exec.Command("git", "show", ":"+filename).Output()
		if out == nil {
			return nil
		}
		return strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	}
	if len(s.Refs) >= 2 {
		out, _ := exec.Command("git", "show", s.Refs[1]+":"+filename).Output()
		if out == nil {
			return nil
		}
		return strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	}
	// working tree
	root, err := gitRoot()
	if err != nil {
		return nil
	}
	content, err := os.ReadFile(filepath.Join(root, filename))
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimRight(string(content), "\n"), "\n")
}

func (s *State) buildFullFileSideBySideLines() {
	newLines := s.readNewFile(s.FullFileName)
	if newLines == nil {
		return
	}
	oldLines := s.reconstructOldFile(s.FullFileName, newLines)

	// Collect hunks for this file
	type indexedHunk struct {
		hunk      *Hunk
		globalIdx int
	}
	var fileHunks []indexedHunk
	firstHunkIdx := -1
	for i := range s.Hunks {
		if s.Hunks[i].File == s.FullFileName {
			if firstHunkIdx < 0 {
				firstHunkIdx = i
			}
			fileHunks = append(fileHunks, indexedHunk{hunk: &s.Hunks[i], globalIdx: i})
		}
	}
	contextHunkIdx := firstHunkIdx

	var lines []DisplayLine

	// File header
	lines = append(lines, DisplayLine{
		Text:  s.FullFileName,
		Style: StyleFileHeader,
	})

	oldLineNo := 1
	newLineNo := 1

	for _, fh := range fileHunks {
		h := fh.hunk
		hIdx := fh.globalIdx

		// Context lines before this hunk - pair old and new
		for newLineNo < h.NewStart {
			var left, right HalfLine
			if oldLineNo-1 < len(oldLines) {
				left = HalfLine{Text: " " + oldLines[oldLineNo-1], Style: StyleContext, LineNo: oldLineNo}
			}
			if newLineNo-1 < len(newLines) {
				right = HalfLine{Text: " " + newLines[newLineNo-1], Style: StyleContext, LineNo: newLineNo}
			}
			lines = append(lines, DisplayLine{
				Style:   StyleContext,
				HunkIdx: contextHunkIdx,
				Left:    left,
				Right:   right,
			})
			oldLineNo++
			newLineNo++
		}

		// Blank line + hunk header
		lines = append(lines, DisplayLine{Style: StyleNormal})
		h.StartLine = len(lines)
		lines = append(lines, DisplayLine{
			Text:    h.Comment,
			Style:   StyleHunkHeader,
			Label:   h.Label,
			HunkIdx: hIdx,
		})

		// Walk hunk lines with remove/add pairing
		hunkOldNo := h.OldStart
		hunkNewNo := h.NewStart
		j := 0
		for j < len(h.Lines) {
			dl := h.Lines[j]

			if dl.Op == ' ' {
				lines = append(lines, DisplayLine{
					Style:   StyleContext,
					HunkIdx: hIdx,
					Left:    HalfLine{Text: " " + dl.Content, Style: StyleContext, LineNo: hunkOldNo},
					Right:   HalfLine{Text: " " + dl.Content, Style: StyleContext, LineNo: hunkNewNo},
				})
				hunkOldNo++
				hunkNewNo++
				j++
				continue
			}

			// Collect consecutive removes
			var removes []Line
			var removeNos []int
			for j < len(h.Lines) && h.Lines[j].Op == '-' {
				removes = append(removes, h.Lines[j])
				removeNos = append(removeNos, hunkOldNo)
				hunkOldNo++
				j++
			}
			// Collect consecutive adds
			var adds []Line
			var addNos []int
			for j < len(h.Lines) && h.Lines[j].Op == '+' {
				adds = append(adds, h.Lines[j])
				addNos = append(addNos, hunkNewNo)
				hunkNewNo++
				j++
			}

			// Pair removes and adds
			maxLen := len(removes)
			if len(adds) > maxLen {
				maxLen = len(adds)
			}
			for k := 0; k < maxLen; k++ {
				var left, right HalfLine
				if k < len(removes) {
					left = HalfLine{Text: "-" + removes[k].Content, Style: StyleRemoved, LineNo: removeNos[k]}
				}
				if k < len(adds) {
					right = HalfLine{Text: "+" + adds[k].Content, Style: StyleAdded, LineNo: addNos[k]}
				}
				lineStyle := StyleContext
				if left.Text != "" {
					lineStyle = StyleRemoved
				} else if right.Text != "" {
					lineStyle = StyleAdded
				}
				lines = append(lines, DisplayLine{
					Style:   lineStyle,
					HunkIdx: hIdx,
					Left:    left,
					Right:   right,
				})
			}
		}

		// Blank after hunk
		lines = append(lines, DisplayLine{Style: StyleNormal})
		oldLineNo = hunkOldNo
		newLineNo = hunkNewNo
	}

	// Remaining lines after last hunk
	for oldLineNo-1 < len(oldLines) || newLineNo-1 < len(newLines) {
		var left, right HalfLine
		if oldLineNo-1 < len(oldLines) {
			left = HalfLine{Text: " " + oldLines[oldLineNo-1], Style: StyleContext, LineNo: oldLineNo}
			oldLineNo++
		}
		if newLineNo-1 < len(newLines) {
			right = HalfLine{Text: " " + newLines[newLineNo-1], Style: StyleContext, LineNo: newLineNo}
			newLineNo++
		}
		lines = append(lines, DisplayLine{
			Style:   StyleContext,
			HunkIdx: contextHunkIdx,
			Left:    left,
			Right:   right,
		})
	}

	s.Lines = lines
}

func (s *State) buildSideBySideLines() {
	var lines []DisplayLine
	var currentFile string

	for i := range s.Hunks {
		h := &s.Hunks[i]

		// Skip hunks not matching the filter
		if s.FilterFile != "" && h.File != s.FilterFile {
			h.StartLine = -1
			continue
		}

		// File header (spans full width)
		if h.File != currentFile {
			if currentFile != "" {
				lines = append(lines, DisplayLine{Style: StyleNormal})
			}
			lines = append(lines, DisplayLine{
				Text:  h.File,
				Style: StyleFileHeader,
			})
			currentFile = h.File
		}

		// Blank line before hunk
		lines = append(lines, DisplayLine{Style: StyleNormal})

		// Record start line for navigation
		h.StartLine = len(lines)

		// Hunk header with label (spans full width, clean)
		lines = append(lines, DisplayLine{
			Text:    h.Comment,
			Style:   StyleHunkHeader,
			Label:   h.Label,
			HunkIdx: i,
		})

		// Group consecutive removes and adds, emit paired lines
		oldNo := h.OldStart
		newNo := h.NewStart
		j := 0
		for j < len(h.Lines) {
			dl := h.Lines[j]

			if dl.Op == ' ' {
				// Context: same text on both sides
				lines = append(lines, DisplayLine{
					Style:   StyleContext,
					HunkIdx: i,
					Left:    HalfLine{Text: " " + dl.Content, Style: StyleContext, LineNo: oldNo},
					Right:   HalfLine{Text: " " + dl.Content, Style: StyleContext, LineNo: newNo},
				})
				oldNo++
				newNo++
				j++
				continue
			}

			// Collect consecutive removes
			var removes []Line
			var removeNos []int
			for j < len(h.Lines) && h.Lines[j].Op == '-' {
				removes = append(removes, h.Lines[j])
				removeNos = append(removeNos, oldNo)
				oldNo++
				j++
			}
			// Collect consecutive adds
			var adds []Line
			var addNos []int
			for j < len(h.Lines) && h.Lines[j].Op == '+' {
				adds = append(adds, h.Lines[j])
				addNos = append(addNos, newNo)
				newNo++
				j++
			}

			// Pair up removes and adds, pad shorter side
			maxLen := len(removes)
			if len(adds) > maxLen {
				maxLen = len(adds)
			}
			for k := 0; k < maxLen; k++ {
				var left, right HalfLine
				if k < len(removes) {
					left = HalfLine{Text: "-" + removes[k].Content, Style: StyleRemoved, LineNo: removeNos[k]}
				}
				if k < len(adds) {
					right = HalfLine{Text: "+" + adds[k].Content, Style: StyleAdded, LineNo: addNos[k]}
				}
				lineStyle := StyleContext
				if left.Text != "" {
					lineStyle = StyleRemoved
				} else if right.Text != "" {
					lineStyle = StyleAdded
				}
				lines = append(lines, DisplayLine{
					Style:   lineStyle,
					HunkIdx: i,
					Left:    left,
					Right:   right,
				})
			}
		}
	}

	s.Lines = lines
}

// ClampScroll ensures scroll position is within valid bounds
func (s *State) ClampScroll() {
	if s.Scroll < 0 {
		s.Scroll = 0
	}
	if max := s.MaxScroll(); s.Scroll > max {
		s.Scroll = max
	}
}

// MaxScroll returns the maximum valid scroll position
func (s *State) MaxScroll() int {
	visible := s.Height - 1
	if len(s.Lines) <= visible {
		return 0
	}
	return len(s.Lines) - visible
}

// ScrollBy adjusts scroll by delta and clamps
func (s *State) ScrollBy(delta int) {
	s.Scroll += delta
	s.ClampScroll()
}

// ScrollTo sets absolute scroll position and clamps
func (s *State) ScrollTo(pos int) {
	s.Scroll = pos
	s.ClampScroll()
}

// HunkByLabel finds a hunk by its label
func (s *State) HunkByLabel(label string) *Hunk {
	for i := range s.Hunks {
		if s.Hunks[i].Label == label {
			return &s.Hunks[i]
		}
	}
	return nil
}

// hasLabelPrefix returns true if any hunk has a label starting with prefix
// that is longer than prefix itself.
func (s *State) hasLabelPrefix(prefix string) bool {
	for i := range s.Hunks {
		l := s.Hunks[i].Label
		if len(l) > len(prefix) && l[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// PendingDisplay returns the current pending key sequence for the status bar.
// Returns "" when nothing is pending.
func (s *State) PendingDisplay() string {
	if s.PendingKey == 0 {
		return ""
	}
	if s.PendingLabel != "" {
		return string(s.PendingKey) + " " + s.PendingLabel
	}
	return string(s.PendingKey)
}

// CurrentFile returns the file path at the current scroll position by walking
// backward through display lines to find the nearest file header. This is more
// accurate than hunk-based detection when the scroll is on a file header or
// blank line before the first hunk.
func (s *State) CurrentFile() string {
	for i := s.Scroll; i >= 0 && i < len(s.Lines); i-- {
		if s.Lines[i].Style == StyleFileHeader {
			return s.Lines[i].Text
		}
	}
	// Fall back to hunk-based detection
	if len(s.Hunks) > 0 {
		return s.Hunks[s.CurrentHunkIndex()].File
	}
	return ""
}

// CurrentLineNo returns the new-file line number near the current scroll
// position, useful for opening an editor at the right line.
func (s *State) CurrentLineNo() int {
	for i := s.Scroll; i < len(s.Lines) && i < s.Scroll+5; i++ {
		if s.Lines[i].NewLineNo > 0 {
			return s.Lines[i].NewLineNo
		}
		// Side-by-side: line numbers are in Right.LineNo
		if s.Lines[i].Right.LineNo > 0 {
			return s.Lines[i].Right.LineNo
		}
	}
	idx := s.CurrentHunkIndex()
	if idx >= 0 && idx < len(s.Hunks) {
		return s.Hunks[idx].NewStart
	}
	return 1
}

// CurrentHunkIndex returns the index of the hunk at current scroll position.
// Hunks with StartLine == -1 (filtered out) are skipped.
func (s *State) CurrentHunkIndex() int {
	for i := len(s.Hunks) - 1; i >= 0; i-- {
		if s.Hunks[i].StartLine >= 0 && s.Hunks[i].StartLine <= s.Scroll {
			return i
		}
	}
	// Fall back to first visible hunk
	for i := range s.Hunks {
		if s.Hunks[i].StartLine >= 0 {
			return i
		}
	}
	return 0
}

// JumpToNextHunk navigates to the next visible hunk
func (s *State) JumpToNextHunk() {
	idx := s.CurrentHunkIndex()
	for i := idx + 1; i < len(s.Hunks); i++ {
		if s.Hunks[i].StartLine >= 0 {
			s.ScrollTo(s.Hunks[i].StartLine)
			return
		}
	}
}

// JumpToPrevHunk navigates to the previous visible hunk
func (s *State) JumpToPrevHunk() {
	if len(s.Hunks) == 0 {
		return
	}
	idx := s.CurrentHunkIndex()
	for i := idx - 1; i >= 0; i-- {
		if s.Hunks[i].StartLine >= 0 {
			s.ScrollTo(s.Hunks[i].StartLine)
			return
		}
	}
	// Stay at current
	if idx >= 0 && idx < len(s.Hunks) && s.Hunks[idx].StartLine >= 0 {
		s.ScrollTo(s.Hunks[idx].StartLine)
	}
}

// JumpToNextFile navigates to the first hunk of the next file
func (s *State) JumpToNextFile() {
	if len(s.Hunks) == 0 {
		return
	}
	currentFile := s.Hunks[s.CurrentHunkIndex()].File
	for i := s.CurrentHunkIndex() + 1; i < len(s.Hunks); i++ {
		if s.Hunks[i].File != currentFile && s.Hunks[i].StartLine >= 0 {
			s.ScrollTo(s.Hunks[i].StartLine)
			return
		}
	}
}

// JumpToPrevFile navigates to the first hunk of the previous file
func (s *State) JumpToPrevFile() {
	if len(s.Hunks) == 0 {
		return
	}
	currentFile := s.Hunks[s.CurrentHunkIndex()].File

	var targetFile string
	for i := s.CurrentHunkIndex() - 1; i >= 0; i-- {
		if s.Hunks[i].File != currentFile && s.Hunks[i].StartLine >= 0 {
			targetFile = s.Hunks[i].File
			break
		}
	}

	if targetFile == "" {
		idx := s.CurrentHunkIndex()
		if idx >= 0 && idx < len(s.Hunks) && s.Hunks[idx].StartLine >= 0 {
			s.ScrollTo(s.Hunks[idx].StartLine)
		}
		return
	}

	for i, h := range s.Hunks {
		if h.File == targetFile && h.StartLine >= 0 {
			s.ScrollTo(s.Hunks[i].StartLine)
			return
		}
	}
}

// UniqueFiles returns the count of unique files in the diff
func (s *State) UniqueFiles() int {
	seen := make(map[string]struct{})
	for _, h := range s.Hunks {
		seen[h.File] = struct{}{}
	}
	return len(seen)
}

// orderedFiles returns file names in the order they appear in hunks.
func (s *State) orderedFiles() []string {
	var files []string
	seen := make(map[string]bool)
	for _, h := range s.Hunks {
		if !seen[h.File] {
			seen[h.File] = true
			files = append(files, h.File)
		}
	}
	return files
}

// SwitchFullFile changes the full-file view to a different file and rebuilds.
func (s *State) SwitchFullFile(filename string) {
	s.FullFileName = filename
	s.FilterFile = filename
	s.Scroll = 0
	s.BuildLines()
	s.ClampScroll()
}

// NextFullFile switches to the next file in full-file mode.
func (s *State) NextFullFile() {
	files := s.orderedFiles()
	for i, f := range files {
		if f == s.FullFileName && i+1 < len(files) {
			s.SwitchFullFile(files[i+1])
			return
		}
	}
}

// PrevFullFile switches to the previous file in full-file mode.
func (s *State) PrevFullFile() {
	files := s.orderedFiles()
	for i, f := range files {
		if f == s.FullFileName && i > 0 {
			s.SwitchFullFile(files[i-1])
			return
		}
	}
}

// DiffStats returns the total number of added and removed lines across all hunks.
func (s *State) DiffStats() (int, int) {
	var added, removed int
	for _, h := range s.Hunks {
		for _, l := range h.Lines {
			switch l.Op {
			case '+':
				added++
			case '-':
				removed++
			}
		}
	}
	return added, removed
}

// RefDisplay returns a display-friendly version of the ref
func (s *State) RefDisplay() string {
	if s.Staged {
		if len(s.Refs) > 0 {
			return strings.Join(s.Refs, "..") + " (staged)"
		}
		return "staged"
	}
	if len(s.Refs) == 0 {
		return "unstaged"
	}
	return strings.Join(s.Refs, "..")
}
