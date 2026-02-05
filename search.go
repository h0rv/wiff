package main

import (
	"strings"

	"github.com/gdamore/tcell/v2"
)

// StartSearch enters search mode.
func StartSearch(s *State) {
	s.SearchMode = true
	s.SearchQuery = ""
	s.SearchMatches = nil
	s.SearchIdx = -1
}

// EndSearch exits search mode but keeps matches highlighted.
func EndSearch(s *State) {
	s.SearchMode = false
}

// ClearSearch clears search entirely.
func ClearSearch(s *State) {
	s.SearchMode = false
	s.SearchQuery = ""
	s.SearchMatches = nil
	s.SearchIdx = -1
}

// HandleSearchKey handles key input during search mode.
// Returns true if the main loop should quit (never, for search).
func HandleSearchKey(s *State, ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		ClearSearch(s)
		return false
	case tcell.KeyEnter:
		UpdateMatches(s)
		if len(s.SearchMatches) > 0 {
			s.SearchIdx = 0
			s.ScrollTo(s.SearchMatches[0])
		}
		EndSearch(s)
		return false
	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if len(s.SearchQuery) > 0 {
			s.SearchQuery = s.SearchQuery[:len(s.SearchQuery)-1]
			UpdateMatches(s)
		}
		return false
	case tcell.KeyRune:
		s.SearchQuery += string(ev.Rune())
		UpdateMatches(s)
		return false
	}
	return false
}

// UpdateMatches scans s.Lines for SearchQuery matches (case-insensitive).
func UpdateMatches(s *State) {
	s.SearchMatches = nil
	s.SearchIdx = -1

	if s.SearchQuery == "" {
		return
	}

	query := strings.ToLower(s.SearchQuery)
	for i, line := range s.Lines {
		if strings.Contains(strings.ToLower(line.Text), query) {
			s.SearchMatches = append(s.SearchMatches, i)
		} else if line.Left.Text != "" && strings.Contains(strings.ToLower(line.Left.Text), query) {
			s.SearchMatches = append(s.SearchMatches, i)
		} else if line.Right.Text != "" && strings.Contains(strings.ToLower(line.Right.Text), query) {
			s.SearchMatches = append(s.SearchMatches, i)
		}
	}
}

// JumpToNextMatch scrolls to the next search match.
func JumpToNextMatch(s *State) {
	if len(s.SearchMatches) == 0 {
		return
	}
	s.SearchIdx++
	if s.SearchIdx >= len(s.SearchMatches) {
		s.SearchIdx = 0
	}
	s.ScrollTo(s.SearchMatches[s.SearchIdx])
}

// JumpToPrevMatch scrolls to the previous search match.
func JumpToPrevMatch(s *State) {
	if len(s.SearchMatches) == 0 {
		return
	}
	s.SearchIdx--
	if s.SearchIdx < 0 {
		s.SearchIdx = len(s.SearchMatches) - 1
	}
	s.ScrollTo(s.SearchMatches[s.SearchIdx])
}

// IsSearchMatch returns whether a given line index is in SearchMatches.
func IsSearchMatch(s *State, lineIdx int) bool {
	for _, idx := range s.SearchMatches {
		if idx == lineIdx {
			return true
		}
	}
	return false
}

// drawSearchBar draws the search input bar at the bottom of the screen,
// on the row just above the status bar.
func drawSearchBar(s *State) {
	y := s.Height - 2
	if y < 0 {
		y = 0
	}

	screen := s.Screen
	col := 0
	barStyle := s.Theme.FileHeader // white + bold

	// Draw "/" prefix
	screen.SetContent(col, y, '/', nil, barStyle)
	col++

	// Draw query text
	for _, r := range s.SearchQuery {
		if col >= s.Width-1 {
			break
		}
		screen.SetContent(col, y, r, nil, barStyle)
		col++
	}

	// Draw cursor indicator
	if col < s.Width {
		screen.SetContent(col, y, ' ', nil, tcell.StyleDefault.Reverse(true))
		col++
	}

	// Clear rest of line
	for col < s.Width {
		screen.SetContent(col, y, ' ', nil, s.Theme.Default)
		col++
	}
}
