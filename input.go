package main

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Double-click detection state
var (
	lastClickTime time.Time
	lastClickY    int
)

// labelTimeout is the duration to wait before auto-resolving an ambiguous
// single-char label that is also a prefix of longer labels.
const labelTimeout = 500 * time.Millisecond

// labelTimer fires to auto-resolve an ambiguous pending label.
var labelTimer *time.Timer

// HandleKey processes a key event, returns true if should quit
func HandleKey(s *State, ev *tcell.EventKey) bool {
	// Dismiss help overlay on any key
	if s.ShowHelp {
		s.ShowHelp = false
		return false
	}

	// When in search mode, route all keys to search handler
	if s.SearchMode {
		return HandleSearchKey(s, ev)
	}

	// When tree is focused, route keys to tree handler
	if s.TreeFocused {
		return handleTreeKey(s, ev)
	}

	// Handle pending multi-key commands
	if s.PendingKey != 0 {
		return handlePending(s, ev)
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		if len(s.SearchMatches) > 0 {
			ClearSearch(s)
			return false
		}
		return true
	case tcell.KeyTab:
		if s.TreeOpen {
			s.TreeFocused = true
			s.InitTreeCursorFromScroll()
			s.EnsureTreeCursorVisible()
		} else if s.FullFile {
			s.NextFullFile()
		} else {
			s.JumpToNextFile()
		}
	case tcell.KeyBacktab:
		if s.TreeOpen && s.TreeFocused {
			s.TreeFocused = false
		} else if s.FullFile {
			s.PrevFullFile()
		} else {
			s.JumpToPrevFile()
		}
	case tcell.KeyUp:
		s.ScrollBy(-1)
	case tcell.KeyDown:
		s.ScrollBy(1)
	case tcell.KeyLeft:
		if !s.Wrap || s.SideBySide {
			s.ScrollX -= 4
			if s.ScrollX < 0 {
				s.ScrollX = 0
			}
		}
	case tcell.KeyRight:
		if !s.Wrap || s.SideBySide {
			s.ScrollX += 4
		}
	case tcell.KeyCtrlD:
		s.ScrollBy(s.Height / 2)
	case tcell.KeyCtrlU:
		s.ScrollBy(-s.Height / 2)
	case tcell.KeyRune:
		return handleRune(s, ev.Rune())
	}

	return false
}

func handleRune(s *State, r rune) bool {
	switch r {
	case 'q':
		return true
	case 'j':
		s.ScrollBy(1)
	case 'k':
		s.ScrollBy(-1)
	case 'd':
		s.ScrollBy(s.Height / 2)
	case 'u':
		s.ScrollBy(-s.Height / 2)
	case 's':
		s.SideBySide = !s.SideBySide
		s.BuildLines()
		s.ClampScroll()
	case 'n':
		if len(s.SearchMatches) > 0 {
			JumpToNextMatch(s)
		} else {
			s.LineNumbers = !s.LineNumbers
			s.BuildLines()
			s.ClampScroll()
		}
	case 'w':
		s.Wrap = !s.Wrap
		if s.Wrap {
			s.ScrollX = 0
		}
		s.BuildLines()
		s.ClampScroll()
	case 'e':
		s.TreeOpen = !s.TreeOpen
		if !s.TreeOpen {
			s.TreeFocused = false
		}
		s.BuildLines()
		s.ClampScroll()
	case 'h':
		s.SyntaxHighlight = !s.SyntaxHighlight
	case 'b':
		s.DiffBg = !s.DiffBg
	case '+', '=':
		if !s.PipeMode {
			s.ContextLines++
			_ = loadDiff(s)
		}
	case '-':
		if !s.PipeMode && s.ContextLines > 0 {
			s.ContextLines--
			_ = loadDiff(s)
		}
	case 'g':
		s.ScrollTo(0)
	case 'G':
		s.ScrollTo(s.MaxScroll())
	case '/':
		StartSearch(s)
	case 'N':
		JumpToPrevMatch(s)
	case 'o':
		file := s.CurrentFile()
		if file != "" {
			openInEditor(s, file, s.CurrentLineNo())
			if !s.PipeMode {
				reloadDiff(s)
			}
		}
	case 'W':
		if !s.PipeMode {
			s.WatchEnabled = !s.WatchEnabled
			if s.WatchEnabled {
				s.FlashMsg = "Watch mode enabled"
			} else {
				s.FlashMsg = "Watch mode disabled"
			}
			s.FlashExpiry = time.Now().Add(2 * time.Second)
		}
	case 'f':
		s.FullFile = !s.FullFile
		if s.FullFile {
			if s.FilterFile != "" {
				s.FullFileName = s.FilterFile
			} else {
				s.FullFileName = s.CurrentFile()
			}
			if s.FullFileName == "" && len(s.Hunks) > 0 {
				s.FullFileName = s.Hunks[0].File
			}
		}
		s.BuildLines()
		s.ClampScroll()
	case '?':
		s.ShowHelp = true
	case 'F':
		if !s.PipeMode {
			s.FollowMode = !s.FollowMode
			if s.FollowMode {
				s.FlashMsg = "Follow mode enabled"
			} else {
				s.FlashMsg = "Follow mode disabled"
			}
			s.FlashExpiry = time.Now().Add(2 * time.Second)
		}
	case ']', '[', 'y', 'Y', 'p', 'c', 'A':
		s.PendingKey = r
	}
	return false
}

// handleTreeKey handles keys when the tree sidebar is focused.
func handleTreeKey(s *State, ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		// If filter is active, clear the filter first
		if s.FilterFile != "" {
			s.FilterFile = ""
			s.BuildLines()
			s.ClampScroll()
		} else {
			// Otherwise, switch focus back to diff
			s.TreeFocused = false
		}
		return false
	case tcell.KeyTab:
		s.TreeFocused = false
		return false
	case tcell.KeyEnter:
		handleTreeSelect(s)
		return false
	case tcell.KeyUp:
		treeMoveCursor(s, -1)
		return false
	case tcell.KeyDown:
		treeMoveCursor(s, 1)
		return false
	case tcell.KeyRune:
		return handleTreeRune(s, ev.Rune())
	}
	return false
}

func handleTreeRune(s *State, r rune) bool {
	switch r {
	case 'q':
		return true
	case 'j':
		treeMoveCursor(s, 1)
	case 'k':
		treeMoveCursor(s, -1)
	case 'a':
		// "Show all" - clear filter
		if s.FilterFile != "" {
			s.FilterFile = ""
			s.BuildLines()
			s.ClampScroll()
		}
	case 'o':
		// Open selected file in editor
		file := s.TreeCursorPath()
		if file != "" {
			openInEditor(s, file, 0)
			if !s.PipeMode {
				reloadDiff(s)
			}
		}
	case 'e':
		// Close tree
		s.TreeOpen = false
		s.TreeFocused = false
		s.BuildLines()
		s.ClampScroll()
	case 'g':
		// Jump to first file
		s.TreeCursor = 0
		s.EnsureTreeCursorVisible()
	case 'G':
		// Jump to last file
		fileIndices := treeFileNodes(s.TreeNodes)
		if len(fileIndices) > 0 {
			s.TreeCursor = len(fileIndices) - 1
		}
		s.EnsureTreeCursorVisible()
	}
	return false
}

func treeMoveCursor(s *State, delta int) {
	fileIndices := treeFileNodes(s.TreeNodes)
	if len(fileIndices) == 0 {
		return
	}
	s.TreeCursor += delta
	s.ClampTreeCursor()
	s.EnsureTreeCursorVisible()
}

func handleTreeSelect(s *State) {
	path := s.TreeCursorPath()
	if path == "" {
		return
	}

	// Toggle: if already filtered to this file, deselect
	if s.FilterFile == path {
		s.FilterFile = ""
	} else {
		s.FilterFile = path
	}
	if s.FullFile {
		if s.FilterFile != "" {
			s.FullFileName = s.FilterFile
		} else {
			s.FullFileName = path
		}
	}
	s.BuildLines()
	s.Scroll = 0
	s.ClampScroll()
}

func handlePending(s *State, ev *tcell.EventKey) bool {
	pending := s.PendingKey

	if ev.Key() == tcell.KeyEscape {
		s.PendingKey = 0
		s.PendingLabel = ""
		cancelLabelTimer()
		return false // cancel
	}

	if ev.Key() != tcell.KeyRune {
		s.PendingKey = 0
		s.PendingLabel = ""
		cancelLabelTimer()
		return false
	}

	r := ev.Rune()

	switch pending {
	case ']':
		s.PendingKey = 0
		switch r {
		case 'c':
			s.JumpToNextHunk()
		case 'f':
			if s.FullFile {
				s.NextFullFile()
			} else {
				s.JumpToNextFile()
			}
		}
	case '[':
		s.PendingKey = 0
		switch r {
		case 'c':
			s.JumpToPrevHunk()
		case 'f':
			if s.FullFile {
				s.PrevFullFile()
			} else {
				s.JumpToPrevFile()
			}
		}
	case 'y', 'Y', 'p', 'c':
		candidate := s.PendingLabel + string(r)
		// Exact match with no longer labels — yank immediately
		if h := s.HunkByLabel(candidate); h != nil && !s.hasLabelPrefix(candidate) {
			s.PendingKey = 0
			s.PendingLabel = ""
			cancelLabelTimer()
			handleYankHunk(s, pending, h)
			return false
		}
		// Exact match AND prefix of longer labels — accumulate, start timeout
		// so the user can still yank the single-char label by waiting
		if s.hasLabelPrefix(candidate) || s.HunkByLabel(candidate) != nil {
			s.PendingLabel = candidate
			s.PendingTime = time.Now()
			startLabelTimer(s)
			return false
		}
		// No match and not a prefix — if we had accumulated chars, try them alone
		if s.PendingLabel != "" {
			if h := s.HunkByLabel(s.PendingLabel); h != nil {
				s.PendingKey = 0
				s.PendingLabel = ""
				cancelLabelTimer()
				handleYankHunk(s, pending, h)
				return false
			}
		}
		s.PendingKey = 0
		s.PendingLabel = ""
		cancelLabelTimer()
	case 'A':
		candidate := s.PendingLabel + string(r)
		if h := s.HunkByLabel(candidate); h != nil && !s.hasLabelPrefix(candidate) {
			s.PendingKey = 0
			s.PendingLabel = ""
			cancelLabelTimer()
			handleStageHunk(s, h)
			return false
		}
		if s.hasLabelPrefix(candidate) || s.HunkByLabel(candidate) != nil {
			s.PendingLabel = candidate
			s.PendingTime = time.Now()
			startLabelTimer(s)
			return false
		}
		if s.PendingLabel != "" {
			if h := s.HunkByLabel(s.PendingLabel); h != nil {
				s.PendingKey = 0
				s.PendingLabel = ""
				cancelLabelTimer()
				handleStageHunk(s, h)
				return false
			}
		}
		s.PendingKey = 0
		s.PendingLabel = ""
		cancelLabelTimer()
	}

	return false
}

func handleTreeClick(s *State, y int) {
	// Tree header is row 0, separator row 1, nodes start at row 2
	nodeIdx := s.TreeScroll + (y - 2)
	if nodeIdx < 0 || nodeIdx >= len(s.TreeNodes) {
		return
	}
	node := s.TreeNodes[nodeIdx]
	if node.IsDir {
		return
	}
	// Find the file cursor index for this node
	fileIndices := treeFileNodes(s.TreeNodes)
	for ci, ni := range fileIndices {
		if ni == nodeIdx {
			s.TreeCursor = ci
			handleTreeSelect(s)
			return
		}
	}
}

// HandleDiffClick handles a click on the diff area. Returns true if it was
// a double-click that triggered a copy action.
func HandleDiffClick(s *State, x, y int) bool {
	now := time.Now()
	isDouble := now.Sub(lastClickTime) < 400*time.Millisecond && y == lastClickY
	lastClickTime = now
	lastClickY = y

	if !isDouble {
		return false
	}

	return copyClickedChunk(s, x, y)
}

// HandleDiffRightClick copies the chunk at the clicked line.
func HandleDiffRightClick(s *State, x, y int) bool {
	return copyClickedChunk(s, x, y)
}

// copyClickedChunk finds the hunk at screen row y and copies the appropriate
// lines (added or removed) to the clipboard. In side-by-side mode, x position
// determines whether the left (removed) or right (added) side is copied.
func copyClickedChunk(s *State, x, y int) bool {
	lineIdx := s.Scroll + y
	if lineIdx < 0 || lineIdx >= len(s.Lines) {
		return false
	}
	line := s.Lines[lineIdx]

	// Only act on diff content lines
	if line.Style != StyleAdded && line.Style != StyleRemoved && line.Style != StyleContext {
		return false
	}
	// Skip pure context lines (no diff content)
	if line.Style == StyleContext && line.Left.Style != StyleAdded && line.Left.Style != StyleRemoved &&
		line.Right.Style != StyleAdded && line.Right.Style != StyleRemoved {
		return false
	}

	hunkIdx := line.HunkIdx
	if hunkIdx < 0 || hunkIdx >= len(s.Hunks) {
		return false
	}
	hunk := &s.Hunks[hunkIdx]

	// Determine whether to copy added or removed lines
	wantAdded := line.Style == StyleAdded

	// In side-by-side mode, use x position to determine left (removed) vs right (added)
	if s.SideBySide {
		lnoExtra := 0
		if s.LineNumbers {
			lnoExtra = 5 // lineNoWidth
		}
		colWidth := (s.DiffWidth - s.LabelGutter - 1) / 2
		midpoint := s.DiffX + s.LabelGutter + lnoExtra + colWidth
		wantAdded = x >= midpoint
	}

	var text, kind string
	if wantAdded {
		text = hunk.AddedLines()
		kind = "added"
	} else {
		text = hunk.RemovedLines()
		kind = "removed"
	}

	if text == "" {
		return false
	}

	if copyToClipboard(text) {
		s.FlashMsg = fmt.Sprintf("Copied %s lines from hunk %s", kind, hunk.Label)
	} else {
		s.FlashMsg = "Copy failed: could not write to terminal"
	}
	s.FlashExpiry = time.Now().Add(2 * time.Second)
	return true
}

// EventLabelTimeout is posted by the label timer to auto-resolve ambiguous labels.
type EventLabelTimeout struct {
	t time.Time
}

func (e *EventLabelTimeout) When() time.Time { return e.t }

// ResolvePendingLabel auto-resolves an ambiguous pending label on timeout.
func ResolvePendingLabel(s *State) {
	if s.PendingKey == 0 || s.PendingLabel == "" {
		return
	}
	cmd := s.PendingKey
	if h := s.HunkByLabel(s.PendingLabel); h != nil {
		if cmd == 'A' {
			handleStageHunk(s, h)
		} else {
			handleYankHunk(s, cmd, h)
		}
	}
	s.PendingKey = 0
	s.PendingLabel = ""
}

// cancelLabelTimer stops any pending label timeout.
func cancelLabelTimer() {
	if labelTimer != nil {
		labelTimer.Stop()
		labelTimer = nil
	}
}

// startLabelTimer starts a timer that posts EventLabelTimeout after labelTimeout.
func startLabelTimer(s *State) {
	cancelLabelTimer()
	labelTimer = time.AfterFunc(labelTimeout, func() {
		if s.Screen != nil {
			_ = s.Screen.PostEvent(&EventLabelTimeout{t: time.Now()})
		}
	})
}

func handleYankHunk(s *State, cmd rune, hunk *Hunk) {
	var text string
	switch cmd {
	case 'y':
		text = hunk.AddedLines()
	case 'Y':
		text = hunk.RemovedLines()
	case 'p':
		text = hunk.AsPatch()
	case 'c':
		text = hunk.ResultLines()
	}

	if text != "" {
		if copyToClipboard(text) {
			switch cmd {
			case 'y':
				s.FlashMsg = fmt.Sprintf("Yanked added lines from hunk %s", hunk.Label)
			case 'Y':
				s.FlashMsg = fmt.Sprintf("Yanked removed lines from hunk %s", hunk.Label)
			case 'p':
				s.FlashMsg = fmt.Sprintf("Yanked patch from hunk %s", hunk.Label)
			case 'c':
				s.FlashMsg = fmt.Sprintf("Copied result from hunk %s", hunk.Label)
			}
		} else {
			s.FlashMsg = fmt.Sprintf("Yank failed for hunk %s: could not write to terminal", hunk.Label)
		}
		s.FlashExpiry = time.Now().Add(2 * time.Second)
	}
}

func handleStageHunk(s *State, hunk *Hunk) {
	patch := hunk.AsFullPatch()
	args := []string{"apply", "--cached"}
	if hunk.Staged {
		args = append(args, "-R") // reverse to unstage
	}
	cmd := exec.Command("git", args...)
	cmd.Stdin = strings.NewReader(patch)
	if err := cmd.Run(); err != nil {
		action := "Stage"
		if hunk.Staged {
			action = "Unstage"
		}
		s.FlashMsg = fmt.Sprintf("%s failed for hunk %s: %v", action, hunk.Label, err)
		s.FlashExpiry = time.Now().Add(2 * time.Second)
		return
	}
	hunk.Staged = !hunk.Staged
	if hunk.Staged {
		s.FlashMsg = fmt.Sprintf("Staged hunk %s", hunk.Label)
	} else {
		s.FlashMsg = fmt.Sprintf("Unstaged hunk %s", hunk.Label)
	}
	s.FlashExpiry = time.Now().Add(2 * time.Second)
}
