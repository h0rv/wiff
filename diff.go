package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/bluekeyes/go-gitdiff/gitdiff"
)

// Hunk represents a diff hunk with a label for quick reference
type Hunk struct {
	Label     string
	File      string
	Header    string // raw @@ header for AsPatch
	Comment   string // function/context from header (clean display)
	OldStart  int    // starting line number in old file
	NewStart  int    // starting line number in new file
	Lines     []Line
	StartLine int
}

// Line represents a single line in a diff hunk
type Line struct {
	Op      rune // '+', '-', ' '
	Content string
}

// AddedLines returns all added lines joined by newlines
func (h *Hunk) AddedLines() string {
	return h.filterLines('+')
}

// RemovedLines returns all removed lines joined by newlines
func (h *Hunk) RemovedLines() string {
	return h.filterLines('-')
}

func (h *Hunk) filterLines(op rune) string {
	var lines []string
	for _, l := range h.Lines {
		if l.Op == op {
			lines = append(lines, l.Content)
		}
	}
	return strings.Join(lines, "\n")
}

// AsPatch formats the hunk as a unified diff patch
func (h *Hunk) AsPatch() string {
	var sb strings.Builder
	sb.WriteString(h.Header)
	sb.WriteByte('\n')
	for _, l := range h.Lines {
		sb.WriteRune(l.Op)
		sb.WriteString(l.Content)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// RunDiff executes git diff and updates state
func RunDiff(s *State) error {
	args := []string{"diff", "--no-color"}
	if s.Staged {
		args = append(args, "--staged")
	}
	args = append(args, s.Refs...)

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return err
		}
	}

	hunks, err := parseDiff(out)
	if err != nil {
		return err
	}

	s.Hunks = hunks
	s.ClampScroll()
	return nil
}

func parseDiff(data []byte) ([]Hunk, error) {
	files, _, err := gitdiff.Parse(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	var hunks []Hunk
	for _, file := range files {
		filename := file.NewName
		if filename == "" || filename == "/dev/null" {
			filename = file.OldName
		}
		for _, frag := range file.TextFragments {
			hunks = append(hunks, Hunk{
				Label:    indexToLabel(len(hunks)),
				File:     filename,
				Header:   formatHeader(frag),
				Comment:  strings.TrimSpace(frag.Comment),
				OldStart: int(frag.OldPosition),
				NewStart: int(frag.NewPosition),
				Lines:    parseLines(frag),
			})
		}
	}
	return hunks, nil
}

// reservedKeys and availableLabels are defined in keys.go

func indexToLabel(idx int) string {
	n := len(availableLabels)
	if idx < n {
		return string(availableLabels[idx])
	}
	// Fallback: two-char labels for very large diffs
	over := idx - n
	first := over / n
	second := over % n
	if first >= n {
		first = n - 1
	}
	return string(availableLabels[first]) + string(availableLabels[second])
}

func formatHeader(frag *gitdiff.TextFragment) string {
	comment := ""
	if frag.Comment != "" {
		comment = " " + frag.Comment
	}
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@%s",
		frag.OldPosition, frag.OldLines,
		frag.NewPosition, frag.NewLines,
		comment)
}

func parseLines(frag *gitdiff.TextFragment) []Line {
	lines := make([]Line, 0, len(frag.Lines))
	for _, l := range frag.Lines {
		op := ' '
		switch l.Op {
		case gitdiff.OpAdd:
			op = '+'
		case gitdiff.OpDelete:
			op = '-'
		}
		lines = append(lines, Line{Op: op, Content: strings.TrimRight(l.Line, "\n")})
	}
	return lines
}
