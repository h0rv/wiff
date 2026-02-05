package main

import "testing"

func TestUpdateMatchesFindsCorrectLines(t *testing.T) {
	s := &State{
		Lines: []DisplayLine{
			{Text: "+\tHost:  \"0.0.0.0\",", Style: StyleAdded},
			{Text: " \treturn &Config{", Style: StyleContext},
			{Text: "-\tHost:  \"localhost\",", Style: StyleRemoved},
			{Text: "+\tDebug: true,", Style: StyleAdded},
			{Text: "func LoadConfig() *Config {", Style: StyleHunkHeader},
		},
	}

	// Search for "host" (case-insensitive) should match lines 0 and 2
	s.SearchQuery = "host"
	UpdateMatches(s)

	if len(s.SearchMatches) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(s.SearchMatches))
	}
	if s.SearchMatches[0] != 0 {
		t.Errorf("expected first match at index 0, got %d", s.SearchMatches[0])
	}
	if s.SearchMatches[1] != 2 {
		t.Errorf("expected second match at index 2, got %d", s.SearchMatches[1])
	}
}

func TestUpdateMatchesCaseInsensitive(t *testing.T) {
	s := &State{
		Lines: []DisplayLine{
			{Text: "Hello World", Style: StyleContext},
			{Text: "hello world", Style: StyleContext},
			{Text: "HELLO WORLD", Style: StyleContext},
		},
	}

	s.SearchQuery = "HELLO"
	UpdateMatches(s)

	if len(s.SearchMatches) != 3 {
		t.Fatalf("expected 3 matches for case-insensitive search, got %d", len(s.SearchMatches))
	}
}

func TestUpdateMatchesEmptyQuery(t *testing.T) {
	s := &State{
		Lines: []DisplayLine{
			{Text: "some text", Style: StyleContext},
		},
	}

	s.SearchQuery = ""
	UpdateMatches(s)

	if len(s.SearchMatches) != 0 {
		t.Errorf("expected 0 matches for empty query, got %d", len(s.SearchMatches))
	}
}

func TestUpdateMatchesNoResults(t *testing.T) {
	s := &State{
		Lines: []DisplayLine{
			{Text: "alpha", Style: StyleContext},
			{Text: "beta", Style: StyleContext},
		},
	}

	s.SearchQuery = "gamma"
	UpdateMatches(s)

	if len(s.SearchMatches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(s.SearchMatches))
	}
	if s.SearchIdx != -1 {
		t.Errorf("expected SearchIdx -1, got %d", s.SearchIdx)
	}
}

func TestIsSearchMatch(t *testing.T) {
	s := &State{
		SearchMatches: []int{1, 5, 10},
	}

	if !IsSearchMatch(s, 1) {
		t.Error("expected line 1 to be a match")
	}
	if !IsSearchMatch(s, 5) {
		t.Error("expected line 5 to be a match")
	}
	if IsSearchMatch(s, 3) {
		t.Error("expected line 3 to NOT be a match")
	}
}

func TestJumpToNextMatchWraps(t *testing.T) {
	s := &State{
		Lines:         make([]DisplayLine, 20),
		Height:        10,
		SearchMatches: []int{3, 8, 15},
		SearchIdx:     -1,
	}

	JumpToNextMatch(s)
	if s.SearchIdx != 0 {
		t.Errorf("expected SearchIdx 0 after first next, got %d", s.SearchIdx)
	}

	JumpToNextMatch(s)
	if s.SearchIdx != 1 {
		t.Errorf("expected SearchIdx 1 after second next, got %d", s.SearchIdx)
	}

	// Jump past last match should wrap to 0
	s.SearchIdx = 2
	JumpToNextMatch(s)
	if s.SearchIdx != 0 {
		t.Errorf("expected SearchIdx 0 after wrap, got %d", s.SearchIdx)
	}
}

func TestJumpToPrevMatchWraps(t *testing.T) {
	s := &State{
		Lines:         make([]DisplayLine, 20),
		Height:        10,
		SearchMatches: []int{3, 8, 15},
		SearchIdx:     0,
	}

	JumpToPrevMatch(s)
	if s.SearchIdx != 2 {
		t.Errorf("expected SearchIdx 2 after wrap backward, got %d", s.SearchIdx)
	}

	JumpToPrevMatch(s)
	if s.SearchIdx != 1 {
		t.Errorf("expected SearchIdx 1 after prev, got %d", s.SearchIdx)
	}
}

func TestStartAndClearSearch(t *testing.T) {
	s := &State{}

	StartSearch(s)
	if !s.SearchMode {
		t.Error("expected SearchMode true after StartSearch")
	}
	if s.SearchIdx != -1 {
		t.Errorf("expected SearchIdx -1 after StartSearch, got %d", s.SearchIdx)
	}

	s.SearchQuery = "test"
	s.SearchMatches = []int{1, 2}
	s.SearchIdx = 0

	ClearSearch(s)
	if s.SearchMode {
		t.Error("expected SearchMode false after ClearSearch")
	}
	if s.SearchQuery != "" {
		t.Errorf("expected empty SearchQuery after ClearSearch, got %q", s.SearchQuery)
	}
	if len(s.SearchMatches) != 0 {
		t.Errorf("expected no SearchMatches after ClearSearch, got %d", len(s.SearchMatches))
	}
	if s.SearchIdx != -1 {
		t.Errorf("expected SearchIdx -1 after ClearSearch, got %d", s.SearchIdx)
	}
}

func TestEndSearchKeepsMatches(t *testing.T) {
	s := &State{
		SearchMode:    true,
		SearchQuery:   "test",
		SearchMatches: []int{1, 5},
		SearchIdx:     0,
	}

	EndSearch(s)
	if s.SearchMode {
		t.Error("expected SearchMode false after EndSearch")
	}
	if s.SearchQuery != "test" {
		t.Errorf("expected SearchQuery preserved after EndSearch, got %q", s.SearchQuery)
	}
	if len(s.SearchMatches) != 2 {
		t.Errorf("expected SearchMatches preserved after EndSearch, got %d", len(s.SearchMatches))
	}
}
