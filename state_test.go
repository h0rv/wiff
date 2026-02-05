package main

import "testing"

func TestRefDisplayUnstaged(t *testing.T) {
	s := &State{}
	if got := s.RefDisplay(); got != "unstaged" {
		t.Errorf("RefDisplay() = %q, want %q", got, "unstaged")
	}
}

func TestRefDisplaySingleRef(t *testing.T) {
	s := &State{Refs: []string{"HEAD"}}
	if got := s.RefDisplay(); got != "HEAD" {
		t.Errorf("RefDisplay() = %q, want %q", got, "HEAD")
	}
}

func TestRefDisplayTwoRefs(t *testing.T) {
	s := &State{Refs: []string{"main", "feature"}}
	if got := s.RefDisplay(); got != "main..feature" {
		t.Errorf("RefDisplay() = %q, want %q", got, "main..feature")
	}
}

func TestRefDisplayStaged(t *testing.T) {
	s := &State{Staged: true}
	if got := s.RefDisplay(); got != "staged" {
		t.Errorf("RefDisplay() = %q, want %q", got, "staged")
	}
}

func TestRefDisplayStagedWithRef(t *testing.T) {
	s := &State{Staged: true, Refs: []string{"HEAD"}}
	if got := s.RefDisplay(); got != "HEAD (staged)" {
		t.Errorf("RefDisplay() = %q, want %q", got, "HEAD (staged)")
	}
}

func TestTextWidthNoLineNumbers(t *testing.T) {
	s := &State{DiffWidth: 80, LabelGutter: 4, LineNumbers: false}
	if got := s.textWidth(); got != 76 {
		t.Errorf("textWidth() = %d, want 76", got)
	}
}

func TestTextWidthWithLineNumbers(t *testing.T) {
	s := &State{DiffWidth: 80, LabelGutter: 4, LineNumbers: true}
	// 80 - 4 - 5 (lineNoWidth) = 71
	if got := s.textWidth(); got != 71 {
		t.Errorf("textWidth() = %d, want 71", got)
	}
}

func TestTextWidthMinimum(t *testing.T) {
	s := &State{DiffWidth: 1, LabelGutter: 10, LineNumbers: true}
	if got := s.textWidth(); got != 1 {
		t.Errorf("textWidth() = %d, want 1 (minimum)", got)
	}
}

func TestBuildInlineLinesFileHeaders(t *testing.T) {
	s := makeTestState(80, false, false, []Line{
		{Op: '+', Content: "added"},
		{Op: '-', Content: "removed"},
		{Op: ' ', Content: "context"},
	})
	s.BuildLines()

	// Should have a file header line
	found := false
	for _, l := range s.Lines {
		if l.Style == StyleFileHeader && l.Text == "test.go" {
			found = true
		}
	}
	if !found {
		t.Error("expected a StyleFileHeader line with 'test.go'")
	}
}

func TestBuildInlineLinesHunkHeader(t *testing.T) {
	s := makeTestState(80, false, false, []Line{
		{Op: '+', Content: "added"},
	})
	s.BuildLines()

	found := false
	for _, l := range s.Lines {
		if l.Style == StyleHunkHeader && l.Label == "b" {
			found = true
		}
	}
	if !found {
		t.Error("expected a StyleHunkHeader line with label 'b'")
	}
}

func TestBuildInlineLinesLineNumbers(t *testing.T) {
	s := makeTestState(80, false, false, []Line{
		{Op: ' ', Content: "context"},
		{Op: '+', Content: "added"},
		{Op: '-', Content: "removed"},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	if len(content) != 3 {
		t.Fatalf("expected 3 content lines, got %d", len(content))
	}

	// Context line: both old and new line numbers set
	if content[0].OldLineNo == 0 || content[0].NewLineNo == 0 {
		t.Error("context line should have both OldLineNo and NewLineNo")
	}

	// Added line: only NewLineNo set
	if content[1].NewLineNo == 0 {
		t.Error("added line should have NewLineNo set")
	}
	if content[1].OldLineNo != 0 {
		t.Error("added line should NOT have OldLineNo set")
	}

	// Removed line: only OldLineNo set
	if content[2].OldLineNo == 0 {
		t.Error("removed line should have OldLineNo set")
	}
	if content[2].NewLineNo != 0 {
		t.Error("removed line should NOT have NewLineNo set")
	}
}

func TestUniqueFiles(t *testing.T) {
	s := &State{
		Hunks: []Hunk{
			{File: "a.go"},
			{File: "a.go"},
			{File: "b.go"},
		},
	}
	if got := s.UniqueFiles(); got != 2 {
		t.Errorf("UniqueFiles() = %d, want 2", got)
	}
}

func TestMaxScroll(t *testing.T) {
	s := &State{Height: 10}
	s.Lines = make([]DisplayLine, 20)
	// MaxScroll = 20 - 9 = 11
	if got := s.MaxScroll(); got != 11 {
		t.Errorf("MaxScroll() = %d, want 11", got)
	}
}

func TestMaxScrollShortContent(t *testing.T) {
	s := &State{Height: 20}
	s.Lines = make([]DisplayLine, 5)
	if got := s.MaxScroll(); got != 0 {
		t.Errorf("MaxScroll() = %d, want 0 (content fits in window)", got)
	}
}

func TestScrollByClamps(t *testing.T) {
	s := &State{Height: 10}
	s.Lines = make([]DisplayLine, 20)
	s.ScrollBy(-100)
	if s.Scroll != 0 {
		t.Errorf("Scroll = %d after scrolling far up, want 0", s.Scroll)
	}
	s.ScrollBy(1000)
	if s.Scroll != s.MaxScroll() {
		t.Errorf("Scroll = %d after scrolling far down, want %d", s.Scroll, s.MaxScroll())
	}
}

func TestFullFileState(t *testing.T) {
	s := &State{}
	if s.FullFile {
		t.Error("FullFile should default to false")
	}
	if s.FullFileName != "" {
		t.Error("FullFileName should default to empty")
	}
}

func TestReconstructOldFileNoChanges(t *testing.T) {
	newLines := []string{"a", "b", "c"}
	s := &State{Hunks: []Hunk{{File: "other.go"}}}
	old := s.reconstructOldFile("test.go", newLines)
	if len(old) != 3 || old[0] != "a" || old[1] != "b" || old[2] != "c" {
		t.Errorf("no-change reconstruction = %v, want [a b c]", old)
	}
}

func TestReconstructOldFileWithHunks(t *testing.T) {
	// New file has line "added" at position 2 that wasn't in old,
	// and old file had "removed" that's not in new.
	newLines := []string{"context1", "added", "context2"}
	s := &State{
		Hunks: []Hunk{{
			File:     "test.go",
			OldStart: 2,
			NewStart: 2,
			Lines: []Line{
				{Op: '-', Content: "removed"},
				{Op: '+', Content: "added"},
			},
		}},
	}
	old := s.reconstructOldFile("test.go", newLines)
	// Old file should be: context1, removed, context2
	if len(old) != 3 {
		t.Fatalf("reconstructOldFile len = %d, want 3, got %v", len(old), old)
	}
	if old[0] != "context1" || old[1] != "removed" || old[2] != "context2" {
		t.Errorf("reconstructOldFile = %v, want [context1 removed context2]", old)
	}
}

func TestReconstructOldFileAddOnly(t *testing.T) {
	// New file has an extra line that was added
	newLines := []string{"a", "b", "new", "c"}
	s := &State{
		Hunks: []Hunk{{
			File:     "test.go",
			OldStart: 3,
			NewStart: 3,
			Lines: []Line{
				{Op: '+', Content: "new"},
			},
		}},
	}
	old := s.reconstructOldFile("test.go", newLines)
	// Old file should be: a, b, c
	if len(old) != 3 {
		t.Fatalf("reconstructOldFile len = %d, want 3, got %v", len(old), old)
	}
	if old[0] != "a" || old[1] != "b" || old[2] != "c" {
		t.Errorf("reconstructOldFile = %v, want [a b c]", old)
	}
}

func TestWatchEnabledState(t *testing.T) {
	s := &State{WatchEnabled: true}
	if !s.WatchEnabled {
		t.Error("WatchEnabled should be true")
	}
}
