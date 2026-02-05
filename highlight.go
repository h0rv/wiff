package main

import (
	"path/filepath"
	"strings"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/gdamore/tcell/v2"
)

// StyledSpan is a run of text with a tcell style applied.
type StyledSpan struct {
	Text  string
	Style tcell.Style
}

// Highlighter tokenizes source lines and maps tokens to tcell styles.
// It caches lexer lookups by file extension and uses a chroma style
// (theme) to determine colors.
type Highlighter struct {
	mu        sync.RWMutex
	lexers    map[string]chroma.Lexer // keyed by extension (e.g. ".go")
	style     *chroma.Style
	themeName string
}

// NewHighlighter returns a ready-to-use Highlighter with the "monokai" theme.
func NewHighlighter() *Highlighter {
	return &Highlighter{
		lexers:    make(map[string]chroma.Lexer),
		style:     styles.Get("monokai"),
		themeName: "monokai",
	}
}

// SetTheme switches to the named chroma theme. If the name is not
// recognised the current theme is kept.
func (h *Highlighter) SetTheme(name string) {
	if !knownStyle(name) {
		return
	}
	if s := styles.Get(name); s != nil {
		h.style = s
		h.themeName = name
	}
}

// ThemeName returns the name of the active theme.
func (h *Highlighter) ThemeName() string {
	return h.themeName
}

// Highlight tokenizes a single line of text and returns styled spans.
// The filename is used only for lexer detection (cached by extension).
// If no lexer is found the whole line is returned as a single default-styled span.
func (h *Highlighter) Highlight(filename, text string) []StyledSpan {
	if text == "" {
		return nil
	}

	lex := h.lexerFor(filename)
	if lex == nil {
		return []StyledSpan{{Text: text, Style: tcell.StyleDefault}}
	}

	iter, err := lex.Tokenise(nil, text)
	if err != nil {
		return []StyledSpan{{Text: text, Style: tcell.StyleDefault}}
	}

	var spans []StyledSpan
	for _, tok := range iter.Tokens() {
		if tok.Value == "" {
			continue
		}
		// Strip any trailing newline that chroma may append to the last token.
		val := strings.TrimRight(tok.Value, "\n")
		if val == "" {
			continue
		}
		spans = append(spans, StyledSpan{
			Text:  val,
			Style: h.tokenStyle(tok.Type),
		})
	}
	return spans
}

// lexerFor returns a (possibly cached) lexer for the given filename.
// Returns nil when no lexer matches.
func (h *Highlighter) lexerFor(filename string) chroma.Lexer {
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = filepath.Base(filename) // handle Makefile, Dockerfile, etc.
	}

	h.mu.RLock()
	lex, ok := h.lexers[ext]
	h.mu.RUnlock()
	if ok {
		return lex // may be nil (negative cache)
	}

	// Lookup and cache.
	lex = lexers.Match(filename)
	if lex != nil {
		lex = chroma.Coalesce(lex)
	}

	h.mu.Lock()
	h.lexers[ext] = lex // cache nil too to avoid repeated misses
	h.mu.Unlock()

	return lex
}

// tokenStyle converts a chroma token type to a tcell style using the active theme.
// Only foreground color is applied (no background) so diff coloring is preserved.
func (h *Highlighter) tokenStyle(t chroma.TokenType) tcell.Style {
	entry := h.style.Get(t)
	style := tcell.StyleDefault

	if entry.Colour.IsSet() {
		style = style.Foreground(tcell.NewRGBColor(
			int32(entry.Colour.Red()),
			int32(entry.Colour.Green()),
			int32(entry.Colour.Blue()),
		))
	}

	// Only foreground - no background (would clash with diff colors).

	if entry.Bold == chroma.Yes {
		style = style.Bold(true)
	}
	if entry.Italic == chroma.Yes {
		style = style.Italic(true)
	}
	if entry.Underline == chroma.Yes {
		style = style.Underline(true)
	}

	return style
}
