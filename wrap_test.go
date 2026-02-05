package main

import (
	"strings"
	"testing"
)

// makeTestState creates a State with the given parameters and a single hunk
// containing the provided diff lines.
func makeTestState(width int, wrap bool, lineNumbers bool, lines []Line) *State {
	s := &State{
		Width:       width,
		Wrap:        wrap,
		LineNumbers: lineNumbers,
		Hunks: []Hunk{
			{
				Label:    "b",
				File:     "test.go",
				Comment:  "func test()",
				OldStart: 10,
				NewStart: 10,
				Lines:    lines,
			},
		},
	}
	return s
}

// contentDisplayLines returns only the display lines that are diff content
// (StyleAdded, StyleRemoved, StyleContext), filtering out headers and blanks.
func contentDisplayLines(lines []DisplayLine) []DisplayLine {
	var out []DisplayLine
	for _, l := range lines {
		if l.Style == StyleAdded || l.Style == StyleRemoved || l.Style == StyleContext {
			out = append(out, l)
		}
	}
	return out
}

func TestWrapLinesBasic(t *testing.T) {
	// Width=40, labelGutter=4, so textWidth = 40 - 4 = 36
	// Create a line that is 50 runes long (including the '+' prefix).
	// That means the content portion is 49 chars, but Text = "+" + content = 50 runes.
	// 50 > 36, so it should be wrapped into two display lines:
	//   first chunk: 36 runes
	//   second chunk: 14 runes
	longContent := strings.Repeat("x", 49) // 49 chars of content
	s := makeTestState(40, true, false, []Line{
		{Op: '+', Content: longContent},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	if len(content) < 2 {
		t.Fatalf("expected at least 2 content display lines after wrapping, got %d", len(content))
	}

	// First line should NOT be a continuation
	if content[0].Continuation {
		t.Error("first chunk should not be a Continuation")
	}
	// First line should have line numbers
	if content[0].NewLineNo == 0 {
		t.Error("first chunk should have NewLineNo set")
	}

	// Second line should be a continuation
	if !content[1].Continuation {
		t.Error("second chunk should be a Continuation")
	}
	// Continuation should have same style
	if content[1].Style != content[0].Style {
		t.Errorf("continuation Style = %d, want %d", content[1].Style, content[0].Style)
	}
	// Continuation should have zero line numbers
	if content[1].OldLineNo != 0 || content[1].NewLineNo != 0 {
		t.Errorf("continuation should have zero line numbers, got old=%d new=%d",
			content[1].OldLineNo, content[1].NewLineNo)
	}

	// Concatenated text should equal the original
	var allText string
	for _, dl := range content {
		allText += dl.Text
	}
	originalText := "+" + longContent
	if allText != originalText {
		t.Errorf("concatenated text mismatch:\ngot:  %q\nwant: %q", allText, originalText)
	}
}

func TestWrapLinesWithLineNumbers(t *testing.T) {
	// Width=40, labelGutter=4, lineNoWidth=5, so textWidth = 40 - 4 - 2*5 = 26
	longContent := strings.Repeat("y", 40) // Text = "-" + 40 chars = 41 runes
	s := makeTestState(40, true, true, []Line{
		{Op: '-', Content: longContent},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	// 41 runes / 26 per chunk = 2 chunks (26 + 15)
	if len(content) < 2 {
		t.Fatalf("expected at least 2 content display lines with line numbers, got %d", len(content))
	}

	// First chunk has line number, continuation does not
	if content[0].OldLineNo == 0 {
		t.Error("first chunk should have OldLineNo set")
	}
	if !content[1].Continuation {
		t.Error("second chunk should be a Continuation")
	}
	if content[1].OldLineNo != 0 {
		t.Error("continuation should have OldLineNo = 0")
	}

	// Text reconstruction
	var allText string
	for _, dl := range content {
		allText += dl.Text
	}
	originalText := "-" + longContent
	if allText != originalText {
		t.Errorf("concatenated text mismatch:\ngot:  %q\nwant: %q", allText, originalText)
	}
}

func TestWrapLinesExactBoundary(t *testing.T) {
	// Width=40, no line numbers -> textWidth = 36
	// Line that is exactly 36 runes should NOT be wrapped.
	content36 := strings.Repeat("a", 35) // Text = "+" + 35 chars = 36 runes
	s := makeTestState(40, true, false, []Line{
		{Op: '+', Content: content36},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	if len(content) != 1 {
		t.Fatalf("line exactly at boundary should produce 1 display line, got %d", len(content))
	}
	if content[0].Continuation {
		t.Error("line at exact boundary should not be a continuation")
	}
}

func TestWrapLinesOneCharOver(t *testing.T) {
	// Width=40, no line numbers -> textWidth = 36
	// Line that is 37 runes should be wrapped into 2 display lines (36 + 1).
	content36 := strings.Repeat("b", 36) // Text = "+" + 36 chars = 37 runes
	s := makeTestState(40, true, false, []Line{
		{Op: '+', Content: content36},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	if len(content) != 2 {
		t.Fatalf("line 1 char over boundary should produce 2 display lines, got %d", len(content))
	}
	if content[0].Continuation {
		t.Error("first chunk should not be a continuation")
	}
	if !content[1].Continuation {
		t.Error("second chunk should be a continuation")
	}
	// First chunk should be exactly textWidth runes
	if len([]rune(content[0].Text)) != 36 {
		t.Errorf("first chunk should be 36 runes, got %d", len([]rune(content[0].Text)))
	}
	// Second chunk should be 1 rune
	if len([]rune(content[1].Text)) != 1 {
		t.Errorf("second chunk should be 1 rune, got %d", len([]rune(content[1].Text)))
	}
}

func TestWrapLinesVeryShortTerminal(t *testing.T) {
	// Width=6, labelGutter=4, so textWidth = max(6-4, 1) = 2
	// A 5-rune line "+abcd" should split into: 2 + 2 + 1
	s := makeTestState(6, true, false, []Line{
		{Op: '+', Content: "abcd"},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	// "+abcd" = 5 runes, textWidth = 2, so 3 display lines (2 + 2 + 1)
	if len(content) != 3 {
		t.Fatalf("very short terminal should produce 3 display lines, got %d", len(content))
	}

	// Verify reconstruction
	var allText string
	for _, dl := range content {
		allText += dl.Text
	}
	if allText != "+abcd" {
		t.Errorf("concatenated text = %q, want %q", allText, "+abcd")
	}

	// First line not continuation, rest are
	if content[0].Continuation {
		t.Error("first chunk should not be continuation")
	}
	for i := 1; i < len(content); i++ {
		if !content[i].Continuation {
			t.Errorf("chunk %d should be continuation", i)
		}
	}
}

func TestWrapLinesMinimalWidth(t *testing.T) {
	// Width=1, labelGutter=4, so textWidth = max(1-4, 1) = 1
	// Every rune gets its own display line.
	s := makeTestState(1, true, false, []Line{
		{Op: ' ', Content: "hi"},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	// " hi" = 3 runes, textWidth = 1, so 3 display lines
	if len(content) != 3 {
		t.Fatalf("minimal width should produce 3 display lines, got %d", len(content))
	}

	var allText string
	for _, dl := range content {
		allText += dl.Text
	}
	if allText != " hi" {
		t.Errorf("concatenated text = %q, want %q", allText, " hi")
	}
}

func TestWrapLinesDoesNotWrapHeaders(t *testing.T) {
	// Headers (StyleNormal, StyleFileHeader, StyleHunkHeader) should not be wrapped
	longContent := strings.Repeat("z", 100)
	s := makeTestState(40, true, false, []Line{
		{Op: '+', Content: longContent},
	})
	s.BuildLines()

	// Count non-content lines -- they should NOT have been split
	for _, line := range s.Lines {
		if line.Style == StyleFileHeader || line.Style == StyleHunkHeader || line.Style == StyleNormal {
			if line.Continuation {
				t.Errorf("non-content line (style=%d) should not be continuation", line.Style)
			}
		}
	}
}

func TestWrapLinesMultipleContentLines(t *testing.T) {
	// Test that wrapping handles multiple lines correctly and preserves order.
	// Width=20, textWidth=16
	s := makeTestState(20, true, false, []Line{
		{Op: '+', Content: strings.Repeat("A", 20)}, // "+AAA..." = 21 runes -> 16 + 5
		{Op: '-', Content: strings.Repeat("B", 20)}, // "-BBB..." = 21 runes -> 16 + 5
		{Op: ' ', Content: "short"},                 // " short" = 6 runes -> no wrap
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	// Line 1: 2 display lines (16 + 5)
	// Line 2: 2 display lines (16 + 5)
	// Line 3: 1 display line (6 < 16)
	// Total: 5
	if len(content) != 5 {
		t.Fatalf("expected 5 content display lines, got %d", len(content))
	}

	// First added line chunk
	if content[0].Style != StyleAdded {
		t.Errorf("content[0] style = %d, want StyleAdded (%d)", content[0].Style, StyleAdded)
	}
	if content[0].Continuation {
		t.Error("content[0] should not be continuation")
	}

	// Continuation of added line
	if content[1].Style != StyleAdded {
		t.Errorf("content[1] style = %d, want StyleAdded", content[1].Style)
	}
	if !content[1].Continuation {
		t.Error("content[1] should be continuation")
	}

	// First removed line chunk
	if content[2].Style != StyleRemoved {
		t.Errorf("content[2] style = %d, want StyleRemoved (%d)", content[2].Style, StyleRemoved)
	}
	if content[2].Continuation {
		t.Error("content[2] should not be continuation")
	}

	// Continuation of removed line
	if content[3].Style != StyleRemoved {
		t.Errorf("content[3] style = %d, want StyleRemoved", content[3].Style)
	}
	if !content[3].Continuation {
		t.Error("content[3] should be continuation")
	}

	// Short context line
	if content[4].Style != StyleContext {
		t.Errorf("content[4] style = %d, want StyleContext", content[4].Style)
	}
	if content[4].Continuation {
		t.Error("content[4] should not be continuation")
	}
}

func TestWrapLinesPreservesHunkStartLine(t *testing.T) {
	// After wrapping, hunk StartLine should still point to the hunk header
	longContent := strings.Repeat("x", 100)
	s := makeTestState(40, true, false, []Line{
		{Op: '+', Content: longContent},
	})
	s.BuildLines()

	hunk := s.Hunks[0]
	if hunk.StartLine < 0 || hunk.StartLine >= len(s.Lines) {
		t.Fatalf("StartLine %d out of range [0, %d)", hunk.StartLine, len(s.Lines))
	}
	if s.Lines[hunk.StartLine].Style != StyleHunkHeader {
		t.Errorf("Lines[StartLine=%d].Style = %d, want StyleHunkHeader (%d)",
			hunk.StartLine, s.Lines[hunk.StartLine].Style, StyleHunkHeader)
	}
	if s.Lines[hunk.StartLine].Label != "b" {
		t.Errorf("Lines[StartLine].Label = %q, want %q", s.Lines[hunk.StartLine].Label, "b")
	}
}

func TestWrapDisabled(t *testing.T) {
	// When Wrap=false, no wrapping should occur
	longContent := strings.Repeat("x", 100)
	s := makeTestState(40, false, false, []Line{
		{Op: '+', Content: longContent},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	if len(content) != 1 {
		t.Fatalf("with wrap disabled, expected 1 content display line, got %d", len(content))
	}
	if content[0].Text != "+"+longContent {
		t.Error("text should be unchanged when wrap is disabled")
	}
}

func TestWrapLinesContextLine(t *testing.T) {
	// Context lines (op=' ') should also be wrapped
	longContent := strings.Repeat("c", 50)
	s := makeTestState(40, true, false, []Line{
		{Op: ' ', Content: longContent},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	// " ccc..." = 51 runes, textWidth=36, so 2 chunks (36 + 15)
	if len(content) < 2 {
		t.Fatalf("context lines should also be wrapped, got %d display lines", len(content))
	}
	if content[0].Style != StyleContext {
		t.Errorf("first chunk style = %d, want StyleContext", content[0].Style)
	}
	if content[1].Style != StyleContext {
		t.Errorf("continuation style = %d, want StyleContext", content[1].Style)
	}
}

func TestWrapLinesUnicode(t *testing.T) {
	// Test wrapping with multi-byte Unicode characters.
	// Each Chinese character is 1 rune but multiple bytes.
	// Width=10, textWidth=6
	unicodeContent := strings.Repeat("\u4e16", 10) // 10 Chinese chars
	s := makeTestState(10, true, false, []Line{
		{Op: '+', Content: unicodeContent},
	})
	s.BuildLines()

	content := contentDisplayLines(s.Lines)
	// "+" + 10 chars = 11 runes, textWidth = 6, so 2 chunks (6 + 5)
	if len(content) < 2 {
		t.Fatalf("unicode line should be wrapped, got %d display lines", len(content))
	}

	// Verify reconstruction
	var allText string
	for _, dl := range content {
		allText += dl.Text
	}
	if allText != "+"+unicodeContent {
		t.Errorf("unicode concatenated text mismatch:\ngot:  %q\nwant: %q", allText, "+"+unicodeContent)
	}
}
