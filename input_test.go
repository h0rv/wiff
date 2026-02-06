package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// makeKeyEvent creates a tcell key event for a rune.
func makeKeyEvent(r rune) *tcell.EventKey {
	return tcell.NewEventKey(tcell.KeyRune, r, tcell.ModNone)
}

func TestYankSingleCharLabel(t *testing.T) {
	hunks := []Hunk{
		{Label: "c", File: "test.go", Lines: []Line{{Op: '+', Content: "added"}}},
		{Label: "i", File: "test.go", Lines: []Line{{Op: '+', Content: "other"}}},
	}
	s := &State{Hunks: hunks, Width: 80, Height: 40}

	// Press 'y' then 'c' — should yank hunk "c" immediately (no two-char labels exist)
	HandleKey(s, makeKeyEvent('y'))
	HandleKey(s, makeKeyEvent('c'))

	if !strings.Contains(s.FlashMsg, "hunk c") {
		t.Errorf("expected FlashMsg to reference hunk c, got %q", s.FlashMsg)
	}
	if s.PendingKey != 0 {
		t.Errorf("PendingKey = %q, want 0", s.PendingKey)
	}
}

func TestYankTwoCharLabel(t *testing.T) {
	n := len(availableLabels)
	// Create n+1 hunks so the last one gets a two-char label
	hunks := make([]Hunk, n+1)
	for i := range hunks {
		hunks[i] = Hunk{
			Label: indexToLabel(i),
			File:  "test.go",
			Lines: []Line{{Op: '+', Content: "line from hunk"}},
		}
	}

	lastLabel := hunks[n].Label
	if len(lastLabel) != 2 {
		t.Fatalf("expected last label to be 2 chars, got %q", lastLabel)
	}

	s := &State{Hunks: hunks, Width: 80, Height: 40}

	// Simulate: press 'y' (enters pending), then first char, then second char
	HandleKey(s, makeKeyEvent('y'))
	HandleKey(s, makeKeyEvent(rune(lastLabel[0])))
	HandleKey(s, makeKeyEvent(rune(lastLabel[1])))

	// The flash message must reference the two-char label, not a single-char one.
	if !strings.Contains(s.FlashMsg, lastLabel) {
		t.Errorf("expected FlashMsg to reference two-char label %q, got %q", lastLabel, s.FlashMsg)
	}
}

func TestPendingDisplayEmpty(t *testing.T) {
	s := &State{}
	if got := s.PendingDisplay(); got != "" {
		t.Errorf("PendingDisplay() = %q, want empty", got)
	}
}

func TestPendingDisplaySingleKey(t *testing.T) {
	s := &State{PendingKey: 'y'}
	if got := s.PendingDisplay(); got != "y" {
		t.Errorf("PendingDisplay() = %q, want %q", got, "y")
	}
}

func TestPendingDisplayWithLabel(t *testing.T) {
	s := &State{PendingKey: 'y', PendingLabel: "c"}
	if got := s.PendingDisplay(); got != "y c" {
		t.Errorf("PendingDisplay() = %q, want %q", got, "y c")
	}
}

func TestPendingDisplayBracket(t *testing.T) {
	s := &State{PendingKey: ']'}
	if got := s.PendingDisplay(); got != "]" {
		t.Errorf("PendingDisplay() = %q, want %q", got, "]")
	}
}

func TestPendingDisplayShowsDuringYank(t *testing.T) {
	n := len(availableLabels)
	hunks := make([]Hunk, n+1)
	for i := range hunks {
		hunks[i] = Hunk{
			Label: indexToLabel(i),
			File:  "test.go",
			Lines: []Line{{Op: '+', Content: "line"}},
		}
	}

	s := &State{Hunks: hunks, Width: 80, Height: 40}

	// Press 'y' — should show pending display
	HandleKey(s, makeKeyEvent('y'))
	if got := s.PendingDisplay(); got != "y" {
		t.Errorf("after 'y': PendingDisplay() = %q, want %q", got, "y")
	}

	// Press first char of two-char label — should accumulate
	firstChar := rune(hunks[n].Label[0])
	HandleKey(s, makeKeyEvent(firstChar))
	want := "y " + string(firstChar)
	if got := s.PendingDisplay(); got != want {
		t.Errorf("after first label char: PendingDisplay() = %q, want %q", got, want)
	}
}

func TestResolvePendingLabel(t *testing.T) {
	n := len(availableLabels)
	hunks := make([]Hunk, n+1)
	for i := range hunks {
		hunks[i] = Hunk{
			Label: indexToLabel(i),
			File:  "test.go",
			Lines: []Line{{Op: '+', Content: "line"}},
		}
	}

	s := &State{Hunks: hunks, Width: 80, Height: 40}

	// Set up ambiguous state: pressed 'y' then first char of two-char label.
	// The first char is also a valid single-char label.
	firstChar := string(hunks[n].Label[0])
	singleLabel := firstChar
	s.PendingKey = 'y'
	s.PendingLabel = singleLabel

	// ResolvePendingLabel should yank the single-char label
	ResolvePendingLabel(s)

	if !strings.Contains(s.FlashMsg, "hunk "+singleLabel) {
		t.Errorf("expected FlashMsg to reference single-char hunk %q, got %q", singleLabel, s.FlashMsg)
	}
	if s.PendingKey != 0 {
		t.Errorf("PendingKey = %q after resolve, want 0", s.PendingKey)
	}
	if s.PendingLabel != "" {
		t.Errorf("PendingLabel = %q after resolve, want empty", s.PendingLabel)
	}
}

func TestResolvePendingLabelNoop(t *testing.T) {
	s := &State{}
	// Nothing pending — should be a no-op
	ResolvePendingLabel(s)
	if s.FlashMsg != "" {
		t.Errorf("expected no FlashMsg, got %q", s.FlashMsg)
	}
}

func TestCopyResultYank(t *testing.T) {
	hunks := []Hunk{{
		Label: indexToLabel(0),
		File:  "test.go",
		Lines: []Line{
			{Op: ' ', Content: "context"},
			{Op: '-', Content: "old"},
			{Op: '+', Content: "new"},
		},
	}}
	s := &State{Hunks: hunks, Width: 80, Height: 40}
	label := hunks[0].Label

	// Press 'c' then the label
	HandleKey(s, makeKeyEvent('c'))
	HandleKey(s, makeKeyEvent(rune(label[0])))

	if !strings.Contains(s.FlashMsg, "result") && !strings.Contains(s.FlashMsg, "hunk "+label) {
		// Either "Copied result from hunk X" or "Yank failed for hunk X"
		t.Errorf("expected copy result flash for hunk %s, got %q", label, s.FlashMsg)
	}
}

func TestStagePendingKey(t *testing.T) {
	hunks := []Hunk{{
		Label: indexToLabel(0),
		File:  "test.go",
		Lines: []Line{{Op: '+', Content: "added"}},
	}}
	s := &State{Hunks: hunks, Width: 80, Height: 40}

	// Press 'A' — should enter pending mode
	HandleKey(s, makeKeyEvent('A'))
	if s.PendingKey != 'A' {
		t.Errorf("after 'A', PendingKey = %q, want 'A'", s.PendingKey)
	}
}

func TestFollowModeToggle(t *testing.T) {
	s := &State{Width: 80, Height: 40}

	if s.FollowMode {
		t.Error("FollowMode should default to false")
	}

	HandleKey(s, makeKeyEvent('F'))
	if !s.FollowMode {
		t.Error("FollowMode should be true after pressing F")
	}
	if !strings.Contains(s.FlashMsg, "Follow mode enabled") {
		t.Errorf("expected follow mode enabled flash, got %q", s.FlashMsg)
	}

	HandleKey(s, makeKeyEvent('F'))
	if s.FollowMode {
		t.Error("FollowMode should be false after pressing F again")
	}
	if !strings.Contains(s.FlashMsg, "Follow mode disabled") {
		t.Errorf("expected follow mode disabled flash, got %q", s.FlashMsg)
	}
}

func TestFollowModeDisabledInPipeMode(t *testing.T) {
	s := &State{Width: 80, Height: 40, PipeMode: true}

	HandleKey(s, makeKeyEvent('F'))
	if s.FollowMode {
		t.Error("FollowMode should not toggle in pipe mode")
	}
}

func TestHunkStagedDefault(t *testing.T) {
	h := Hunk{Label: "a", File: "test.go"}
	if h.Staged {
		t.Error("Hunk.Staged should default to false")
	}
}
