package main

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestNewHighlighterDefaults(t *testing.T) {
	h := NewHighlighter()
	if got := h.ThemeName(); got != "monokai" {
		t.Errorf("expected default theme 'monokai', got %q", got)
	}
}

func TestSetThemeValid(t *testing.T) {
	h := NewHighlighter()
	h.SetTheme("dracula")
	if got := h.ThemeName(); got != "dracula" {
		t.Errorf("expected theme 'dracula', got %q", got)
	}
}

func TestSetThemeInvalid(t *testing.T) {
	h := NewHighlighter()
	h.SetTheme("nonexistent-theme-12345")
	if got := h.ThemeName(); got != "monokai" {
		t.Errorf("expected theme to remain 'monokai' after invalid SetTheme, got %q", got)
	}
}

func TestHighlightGoCode(t *testing.T) {
	h := NewHighlighter()
	spans := h.Highlight("test.go", "func main() {}")

	if len(spans) < 2 {
		t.Fatalf("expected at least 2 spans for Go code, got %d", len(spans))
	}

	// Verify text reconstruction
	var sb strings.Builder
	for _, s := range spans {
		sb.WriteString(s.Text)
	}
	if got := sb.String(); got != "func main() {}" {
		t.Errorf("span reconstruction = %q, want %q", got, "func main() {}")
	}
}

func TestHighlightEmpty(t *testing.T) {
	h := NewHighlighter()
	spans := h.Highlight("test.go", "")
	if spans != nil {
		t.Errorf("expected nil for empty input, got %d spans", len(spans))
	}
}

func TestHighlightUnknownExtension(t *testing.T) {
	h := NewHighlighter()
	input := "hello world"
	spans := h.Highlight("file.xyz123unknown", input)

	if len(spans) != 1 {
		t.Fatalf("expected 1 fallback span, got %d", len(spans))
	}
	if spans[0].Text != input {
		t.Errorf("expected span text %q, got %q", input, spans[0].Text)
	}
	if spans[0].Style != tcell.StyleDefault {
		t.Error("expected default style for unknown extension")
	}
}

func TestHighlightKeywordStyled(t *testing.T) {
	h := NewHighlighter()
	spans := h.Highlight("test.go", "func")

	// "func" is a Go keyword and should have a non-default style
	hasStyledSpan := false
	for _, s := range spans {
		if s.Style != tcell.StyleDefault {
			hasStyledSpan = true
			break
		}
	}
	if !hasStyledSpan {
		t.Error("expected at least one non-default styled span for Go keyword 'func'")
	}
}

func TestHighlightTextReconstruction(t *testing.T) {
	h := NewHighlighter()
	inputs := []struct {
		file, text string
	}{
		{"main.go", "package main"},
		{"style.css", "body { color: red; }"},
		{"app.js", "const x = 42;"},
		{"data.json", `{"key": "value"}`},
	}

	for _, tt := range inputs {
		spans := h.Highlight(tt.file, tt.text)
		var sb strings.Builder
		for _, s := range spans {
			sb.WriteString(s.Text)
		}
		if got := sb.String(); got != tt.text {
			t.Errorf("Highlight(%q, %q): reconstructed %q", tt.file, tt.text, got)
		}
	}
}

func TestHighlightCachesLexer(t *testing.T) {
	h := NewHighlighter()

	// First call populates cache
	spans1 := h.Highlight("a.go", "func foo() {}")
	// Second call should use cached lexer
	spans2 := h.Highlight("b.go", "func foo() {}")

	// Both should produce non-nil results
	if spans1 == nil {
		t.Error("expected non-nil spans from first call")
	}
	if spans2 == nil {
		t.Error("expected non-nil spans from second call")
	}
}
