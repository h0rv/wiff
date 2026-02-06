package main

import (
	"strings"
	"testing"
)

// fakeDiff builds a realistic unified diff with multiple files and edge cases.
// Line counts in @@ headers are carefully matched to actual content lines.
//
// Hunks produced:
//
//	[0] app/config.go  - mixed adds/removes with context (3 add, 2 del, 2 ctx)
//	[1] app/config.go  - single line change with context  (1 add, 1 del, 2 ctx)
//	[2] docs/notes.txt - pure adds (new file, 4 adds)
//	[3] old/cleanup.go - pure removes (deleted file, 5 dels)
func fakeDiff() []byte {
	return []byte("diff --git a/app/config.go b/app/config.go\n" +
		"index 1a2b3c4..5d6e7f8 100644\n" +
		"--- a/app/config.go\n" +
		"+++ b/app/config.go\n" +
		"@@ -10,4 +10,5 @@ func LoadConfig() *Config {\n" +
		" \treturn &Config{\n" +
		"-\t\tHost:  \"localhost\",\n" +
		"-\t\tPort:  8080,\n" +
		"+\t\tHost:  \"0.0.0.0\",\n" +
		"+\t\tPort:  9090,\n" +
		"+\t\tDebug: true,\n" +
		" \t}\n" +
		"@@ -25,3 +26,3 @@ func validate(c *Config) error {\n" +
		" \tif c.Port == 0 {\n" +
		"-\t\treturn fmt.Errorf(\"port required\")\n" +
		"+\t\treturn fmt.Errorf(\"port must be > 0\")\n" +
		" \t}\n" +
		"diff --git a/docs/notes.txt b/docs/notes.txt\n" +
		"new file mode 100644\n" +
		"index 0000000..abcdef1\n" +
		"--- /dev/null\n" +
		"+++ b/docs/notes.txt\n" +
		"@@ -0,0 +1,4 @@\n" +
		"+First line of notes\n" +
		"+\tindented with tab\n" +
		"+line with trailing space \n" +
		"+last line\n" +
		"diff --git a/old/cleanup.go b/old/cleanup.go\n" +
		"index 9f8e7d6..0000000 100644\n" +
		"--- a/old/cleanup.go\n" +
		"+++ /dev/null\n" +
		"@@ -1,5 +0,0 @@\n" +
		"-package old\n" +
		"-\n" +
		"-func Cleanup() {\n" +
		"-\t// TODO: remove this\twith tab\n" +
		"-}\n")
}

// helperParseFakeDiff parses the fakeDiff and fails the test on error.
func helperParseFakeDiff(t *testing.T) []Hunk {
	t.Helper()
	hunks, err := parseDiff(fakeDiff())
	if err != nil {
		t.Fatalf("parseDiff returned error: %v", err)
	}
	return hunks
}

func TestParseDiffHunkCount(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	// File 1 has 2 hunks, file 2 has 1 hunk, file 3 has 1 hunk = 4 total
	if got := len(hunks); got != 4 {
		t.Fatalf("expected 4 hunks, got %d", got)
	}
}

func TestParseDiffFileNames(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	// parseDiff uses file.NewName, falling back to file.OldName if empty.
	// For deleted files (old/cleanup.go -> /dev/null), go-gitdiff may
	// report NewName as "" so OldName is used, or NewName as the raw value.
	// We capture whichever the parser actually returns.
	for i, h := range hunks {
		if h.File == "" {
			t.Errorf("hunk %d: File is empty", i)
		}
	}

	// First two hunks should be in the same file
	if hunks[0].File != hunks[1].File {
		t.Errorf("hunks 0 and 1 should share a file, got %q and %q", hunks[0].File, hunks[1].File)
	}

	// Hunk 2 should be docs/notes.txt
	if hunks[2].File != "docs/notes.txt" {
		t.Errorf("hunk 2: file = %q, want %q", hunks[2].File, "docs/notes.txt")
	}
}

func TestParseDiffLabels(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	// Labels should be sequential from the availableLabels list.
	for i, h := range hunks {
		want := indexToLabel(i)
		if h.Label != want {
			t.Errorf("hunk %d: label = %q, want %q", i, h.Label, want)
		}
	}
}

func TestParseDiffHeaders(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	for i, h := range hunks {
		if !strings.HasPrefix(h.Header, "@@ ") {
			t.Errorf("hunk %d: Header should start with '@@ ', got %q", i, h.Header)
		}
		if !strings.Contains(h.Header, "@@") {
			t.Errorf("hunk %d: Header missing closing @@, got %q", i, h.Header)
		}
	}

	// Hunk 0 should have the function comment from the @@ line
	if !strings.Contains(hunks[0].Header, "func LoadConfig") {
		t.Errorf("hunk 0 header missing function context: %q", hunks[0].Header)
	}
	if hunks[0].Comment != "func LoadConfig() *Config {" {
		t.Errorf("hunk 0 Comment = %q, want %q", hunks[0].Comment, "func LoadConfig() *Config {")
	}
}

func TestParseDiffLineNumberTracking(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	// Hunk 0: OldStart=10, NewStart=10
	if hunks[0].OldStart != 10 {
		t.Errorf("hunk 0: OldStart = %d, want 10", hunks[0].OldStart)
	}
	if hunks[0].NewStart != 10 {
		t.Errorf("hunk 0: NewStart = %d, want 10", hunks[0].NewStart)
	}

	// Hunk 1: OldStart=25, NewStart=26
	if hunks[1].OldStart != 25 {
		t.Errorf("hunk 1: OldStart = %d, want 25", hunks[1].OldStart)
	}
	if hunks[1].NewStart != 26 {
		t.Errorf("hunk 1: NewStart = %d, want 26", hunks[1].NewStart)
	}
}

// TestMixedAddRemoveHunk tests a hunk with both adds and removes (hunk 0).
func TestMixedAddRemoveHunk(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[0]

	// Count ops
	var adds, dels, ctx int
	for _, l := range h.Lines {
		switch l.Op {
		case '+':
			adds++
		case '-':
			dels++
		case ' ':
			ctx++
		}
	}
	if adds != 3 {
		t.Errorf("hunk 0: expected 3 adds, got %d", adds)
	}
	if dels != 2 {
		t.Errorf("hunk 0: expected 2 deletes, got %d", dels)
	}
	if ctx != 2 {
		t.Errorf("hunk 0: expected 2 context lines, got %d", ctx)
	}

	// Verify AddedLines contains the right content (each Content may have trailing \n from parser)
	added := h.AddedLines()
	if !strings.Contains(added, "Host") || !strings.Contains(added, "0.0.0.0") {
		t.Errorf("AddedLines missing expected Host content: %q", added)
	}
	if !strings.Contains(added, "Port") || !strings.Contains(added, "9090") {
		t.Errorf("AddedLines missing expected Port content: %q", added)
	}
	if !strings.Contains(added, "Debug") {
		t.Errorf("AddedLines missing expected Debug content: %q", added)
	}

	// Verify RemovedLines
	removed := h.RemovedLines()
	if !strings.Contains(removed, "localhost") {
		t.Errorf("RemovedLines missing 'localhost': %q", removed)
	}
	if !strings.Contains(removed, "8080") {
		t.Errorf("RemovedLines missing '8080': %q", removed)
	}

	// Removed should NOT contain added content
	if strings.Contains(removed, "0.0.0.0") {
		t.Errorf("RemovedLines should not contain '0.0.0.0': %q", removed)
	}
	// Added should NOT contain removed content
	if strings.Contains(added, "localhost") {
		t.Errorf("AddedLines should not contain 'localhost': %q", added)
	}
}

// TestSingleLineChangeHunk tests hunk 1 with 1 remove + 1 add.
func TestSingleLineChangeHunk(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[1]

	added := h.AddedLines()
	if !strings.Contains(added, "port must be > 0") {
		t.Errorf("AddedLines missing expected content: %q", added)
	}

	removed := h.RemovedLines()
	if !strings.Contains(removed, "port required") {
		t.Errorf("RemovedLines missing expected content: %q", removed)
	}

	// Ensure the old and new are distinct
	if strings.Contains(added, "port required") {
		t.Errorf("AddedLines should not contain old text: %q", added)
	}
	if strings.Contains(removed, "port must be > 0") {
		t.Errorf("RemovedLines should not contain new text: %q", removed)
	}
}

// TestPureAddHunk tests a hunk with only additions (new file).
func TestPureAddHunk(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[2] // docs/notes.txt

	// All lines should be adds
	for i, l := range h.Lines {
		if l.Op != '+' {
			t.Errorf("hunk 2, line %d: expected op '+', got %q", i, string(l.Op))
		}
	}
	if len(h.Lines) != 4 {
		t.Errorf("hunk 2: expected 4 lines, got %d", len(h.Lines))
	}

	removed := h.RemovedLines()
	if removed != "" {
		t.Errorf("expected no removed lines, got %q", removed)
	}

	added := h.AddedLines()
	if !strings.Contains(added, "First line of notes") {
		t.Errorf("AddedLines missing 'First line of notes': %q", added)
	}
	if !strings.Contains(added, "last line") {
		t.Errorf("AddedLines missing 'last line': %q", added)
	}
}

// TestPureRemoveHunk tests a hunk with only removals (deleted file).
func TestPureRemoveHunk(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[3] // old/cleanup.go

	// All lines should be removes
	for i, l := range h.Lines {
		if l.Op != '-' {
			t.Errorf("hunk 3, line %d: expected op '-', got %q", i, string(l.Op))
		}
	}
	if len(h.Lines) != 5 {
		t.Errorf("hunk 3: expected 5 lines, got %d", len(h.Lines))
	}

	added := h.AddedLines()
	if added != "" {
		t.Errorf("expected no added lines, got %q", added)
	}

	removed := h.RemovedLines()
	if !strings.Contains(removed, "package old") {
		t.Errorf("RemovedLines missing 'package old': %q", removed)
	}
	if !strings.Contains(removed, "func Cleanup()") {
		t.Errorf("RemovedLines missing 'func Cleanup()': %q", removed)
	}
}

// TestFilterLinesExcludesOtherOps verifies AddedLines has no removed/context content
// and RemovedLines has no added/context content.
func TestFilterLinesExcludesOtherOps(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[0] // mixed hunk

	added := h.AddedLines()
	removed := h.RemovedLines()

	// Collect actual content by op
	var addContents, delContents []string
	for _, l := range h.Lines {
		switch l.Op {
		case '+':
			addContents = append(addContents, l.Content)
		case '-':
			delContents = append(delContents, l.Content)
		}
	}

	// AddedLines should consist of exactly the '+' line contents, joined by \n
	wantAdded := strings.Join(addContents, "\n")
	if added != wantAdded {
		t.Errorf("AddedLines mismatch:\ngot:  %q\nwant: %q", added, wantAdded)
	}

	// RemovedLines should consist of exactly the '-' line contents, joined by \n
	wantRemoved := strings.Join(delContents, "\n")
	if removed != wantRemoved {
		t.Errorf("RemovedLines mismatch:\ngot:  %q\nwant: %q", removed, wantRemoved)
	}
}

// TestAsPatchFormat validates that AsPatch produces valid unified diff patch text.
func TestAsPatchFormat(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	for i, h := range hunks {
		patch := h.AsPatch()

		if patch == "" {
			t.Errorf("hunk %d: AsPatch returned empty string", i)
			continue
		}

		// Patch must start with @@
		if !strings.HasPrefix(patch, "@@") {
			t.Errorf("hunk %d: patch should start with @@, got prefix %q", i, patch[:20])
		}

		// Patch must end with \n
		if !strings.HasSuffix(patch, "\n") {
			t.Errorf("hunk %d: patch should end with newline", i)
		}
	}
}

// TestAsPatchStartsWithHeader ensures AsPatch starts with the exact hunk header.
func TestAsPatchStartsWithHeader(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	for i, h := range hunks {
		patch := h.AsPatch()
		if !strings.HasPrefix(patch, h.Header+"\n") {
			firstLine := strings.SplitN(patch, "\n", 2)[0]
			t.Errorf("hunk %d: AsPatch first line = %q, Header = %q", i, firstLine, h.Header)
		}
	}
}

// TestAsPatchContentPreservation verifies that AsPatch faithfully includes
// every line from h.Lines with the correct op prefix.
func TestAsPatchContentPreservation(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	for i, h := range hunks {
		patch := h.AsPatch()

		// Reconstruct what AsPatch should produce from the Hunk data
		var expected strings.Builder
		expected.WriteString(h.Header)
		expected.WriteByte('\n')
		for _, l := range h.Lines {
			expected.WriteRune(l.Op)
			expected.WriteString(l.Content)
			expected.WriteByte('\n')
		}

		if patch != expected.String() {
			t.Errorf("hunk %d: AsPatch output does not match expected reconstruction\ngot:\n%s\nwant:\n%s",
				i, patch, expected.String())
		}
	}
}

// TestAsPatchEveryContentLineHasOpPrefix checks that after the header,
// every non-empty line in the patch starts with +, -, or space.
func TestAsPatchEveryContentLineHasOpPrefix(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	for i, h := range hunks {
		patch := h.AsPatch()
		lines := strings.Split(patch, "\n")

		for j, line := range lines {
			if j == 0 {
				// Header line, skip
				continue
			}
			if line == "" {
				// Trailing empty from final \n, or blank content line
				continue
			}
			prefix := line[0]
			if prefix != '+' && prefix != '-' && prefix != ' ' {
				t.Errorf("hunk %d, line %d: invalid prefix %q in %q", i, j, string(prefix), line)
			}
		}
	}
}

// TestSpecialCharactersTabs validates that tab characters survive parsing.
func TestSpecialCharactersTabs(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	// Hunk 2 (docs/notes.txt) has a line with a leading tab
	h := hunks[2]
	foundTab := false
	for _, l := range h.Lines {
		if strings.Contains(l.Content, "\t") && strings.Contains(l.Content, "indented with tab") {
			foundTab = true
		}
	}
	if !foundTab {
		t.Error("hunk 2: did not find line with tab character")
	}

	// Hunk 3 (old/cleanup.go) has a line with an embedded tab in the middle
	h3 := hunks[3]
	foundEmbeddedTab := false
	for _, l := range h3.Lines {
		if strings.Contains(l.Content, "remove this") && strings.Contains(l.Content, "with tab") {
			foundEmbeddedTab = true
			if !strings.Contains(l.Content, "\t") {
				t.Error("hunk 3: tab between 'remove this' and 'with tab' was lost")
			}
		}
	}
	if !foundEmbeddedTab {
		t.Error("hunk 3: did not find line with embedded tab")
	}
}

// TestSpecialCharactersTrailingSpace validates that trailing spaces survive.
func TestSpecialCharactersTrailingSpace(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	// Hunk 2 (docs/notes.txt) has "+line with trailing space "
	h := hunks[2]
	foundTrailingSpace := false
	for _, l := range h.Lines {
		// The content might have a trailing \n from the parser, so check
		// for trailing space before any potential \n
		content := strings.TrimRight(l.Content, "\n")
		if strings.Contains(content, "trailing space") && strings.HasSuffix(content, " ") {
			foundTrailingSpace = true
		}
	}
	if !foundTrailingSpace {
		t.Error("hunk 2: did not find line with trailing space preserved")
		for j, l := range h.Lines {
			t.Logf("  line %d: op=%q content=%q", j, string(l.Op), l.Content)
		}
	}
}

// TestEmptyDiff verifies that an empty diff produces zero hunks.
func TestEmptyDiff(t *testing.T) {
	hunks, err := parseDiff([]byte{})
	if err != nil {
		t.Fatalf("parseDiff on empty input returned error: %v", err)
	}
	if len(hunks) != 0 {
		t.Errorf("expected 0 hunks from empty diff, got %d", len(hunks))
	}
}

// TestContextLinesPresent verifies that context lines (space prefix) are parsed.
func TestContextLinesPresent(t *testing.T) {
	hunks := helperParseFakeDiff(t)

	// Hunk 0 (first hunk of config.go) should have context lines
	h := hunks[0]
	contextCount := 0
	for _, l := range h.Lines {
		if l.Op == ' ' {
			contextCount++
		}
	}
	if contextCount == 0 {
		t.Error("hunk 0 should have context lines (op=' '), found none")
	}

	// Verify counts match: filterLines only returns matching ops
	addedCount := 0
	removedCount := 0
	for _, l := range h.Lines {
		switch l.Op {
		case '+':
			addedCount++
		case '-':
			removedCount++
		}
	}

	// AddedLines split by \n should give exactly addedCount elements
	added := h.AddedLines()
	if addedCount > 0 {
		gotAddedCount := len(strings.Split(added, "\n"))
		if gotAddedCount != addedCount {
			t.Errorf("AddedLines has %d parts (split by \\n), but hunk has %d '+' lines",
				gotAddedCount, addedCount)
		}
	}

	removed := h.RemovedLines()
	if removedCount > 0 {
		gotRemovedCount := len(strings.Split(removed, "\n"))
		if gotRemovedCount != removedCount {
			t.Errorf("RemovedLines has %d parts (split by \\n), but hunk has %d '-' lines",
				gotRemovedCount, removedCount)
		}
	}
}

// TestLineOpValues ensures every parsed line has a valid op character.
func TestLineOpValues(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	for i, h := range hunks {
		for j, l := range h.Lines {
			if l.Op != '+' && l.Op != '-' && l.Op != ' ' {
				t.Errorf("hunk %d, line %d: invalid Op %q", i, j, string(l.Op))
			}
		}
	}
}

// TestBlankDeletedLine verifies that a blank removed line (just "-\n" in the diff)
// is parsed as Op='-' with some content (possibly just "\n" or "").
func TestBlankDeletedLine(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[3] // old/cleanup.go has "-\n" (blank removed line)

	foundBlank := false
	for _, l := range h.Lines {
		if l.Op == '-' {
			trimmed := strings.TrimRight(l.Content, "\n")
			if trimmed == "" {
				foundBlank = true
			}
		}
	}
	if !foundBlank {
		t.Error("hunk 3: did not find blank removed line (the empty line between 'package old' and 'func Cleanup')")
		for j, l := range h.Lines {
			t.Logf("  line %d: op=%q content=%q", j, string(l.Op), l.Content)
		}
	}
}

// TestResultLinesMixed tests ResultLines on a hunk with adds, removes, and context.
func TestResultLinesMixed(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[0] // mixed hunk: context, removes, adds, context

	result := h.ResultLines()

	// Result should contain context lines and added lines, but NOT removed lines
	if !strings.Contains(result, "return &Config{") {
		t.Errorf("ResultLines missing context line: %q", result)
	}
	if !strings.Contains(result, "0.0.0.0") {
		t.Errorf("ResultLines missing added content: %q", result)
	}
	if !strings.Contains(result, "Debug") {
		t.Errorf("ResultLines missing added line 'Debug': %q", result)
	}
	// Should NOT contain removed content
	if strings.Contains(result, "localhost") {
		t.Errorf("ResultLines should not contain removed 'localhost': %q", result)
	}
	if strings.Contains(result, "8080") {
		t.Errorf("ResultLines should not contain removed '8080': %q", result)
	}
}

// TestResultLinesPureAdd tests ResultLines on an add-only hunk.
func TestResultLinesPureAdd(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[2] // pure adds

	result := h.ResultLines()
	added := h.AddedLines()

	// For a pure-add hunk, ResultLines == AddedLines
	if result != added {
		t.Errorf("pure add: ResultLines != AddedLines\nresult: %q\nadded:  %q", result, added)
	}
}

// TestResultLinesPureRemove tests ResultLines on a remove-only hunk.
func TestResultLinesPureRemove(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[3] // pure removes

	result := h.ResultLines()

	// For a pure-remove hunk, ResultLines should be empty (no context, no adds)
	if result != "" {
		t.Errorf("pure remove: ResultLines should be empty, got %q", result)
	}
}

// TestAsFullPatchFormat validates AsFullPatch includes file headers.
func TestAsFullPatchFormat(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	h := hunks[0]

	patch := h.AsFullPatch()

	if !strings.HasPrefix(patch, "diff --git a/") {
		t.Errorf("AsFullPatch missing diff header: %q", patch[:50])
	}
	if !strings.Contains(patch, "--- a/"+h.File) {
		t.Errorf("AsFullPatch missing --- header")
	}
	if !strings.Contains(patch, "+++ b/"+h.File) {
		t.Errorf("AsFullPatch missing +++ header")
	}
	if !strings.Contains(patch, h.Header) {
		t.Errorf("AsFullPatch missing hunk header")
	}
}

// TestAsPatchRoundTripStability ensures that AsPatch output is deterministic.
func TestAsPatchRoundTripStability(t *testing.T) {
	hunks := helperParseFakeDiff(t)
	for i, h := range hunks {
		first := h.AsPatch()
		second := h.AsPatch()
		if first != second {
			t.Errorf("hunk %d: AsPatch is not deterministic", i)
		}
	}
}
