package main

import (
	"fmt"
	"os"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/gdamore/tcell/v2"
)

// UITheme holds all colors and styles derived from a chroma theme.
type UITheme struct {
	// Raw colors (needed for dynamic composition in tree.go)
	Accent    tcell.Color // from Keyword token
	Highlight tcell.Color // from String token
	Added     tcell.Color // semantic green (kept)
	Removed   tcell.Color // semantic red (kept)

	// Pre-built styles
	Default     tcell.Style
	Dim         tcell.Style
	FileHeader  tcell.Style
	HunkHeader  tcell.Style
	DiffAdded   tcell.Style
	DiffRemoved tcell.Style
	Label       tcell.Style
	LineNo      tcell.Style
	StatusBar   tcell.Style
	SearchCur   tcell.Style
	Flash       tcell.Style

	// Diff bg tints (computed from theme background)
	BgAdded   tcell.Color
	BgRemoved tcell.Color
}

// knownStyle returns true if name is a registered chroma style.
func knownStyle(name string) bool {
	for _, n := range styles.Names() {
		if n == name {
			return true
		}
	}
	return false
}

// NewUITheme builds a UITheme from the named chroma style.
func NewUITheme(name string) UITheme {
	cs := styles.Get(name)
	if !knownStyle(name) {
		cs = styles.Get("monokai")
	}

	accent := chromaColor(cs, chroma.Keyword, tcell.ColorAqua)
	highlight := chromaColor(cs, chroma.LiteralString, tcell.ColorYellow)
	comment := chromaColor(cs, chroma.Comment, tcell.ColorAqua)
	fg := chromaColor(cs, chroma.Background, tcell.ColorWhite) // default text foreground

	added := tcell.ColorGreen
	removed := tcell.ColorRed

	base := tcell.StyleDefault
	bgAdded, bgRemoved := computeDiffBg(cs)

	return UITheme{
		Accent:    accent,
		Highlight: highlight,
		Added:     added,
		Removed:   removed,

		Default:     base,
		Dim:         base.Dim(true),
		FileHeader:  base.Bold(true).Foreground(fg),
		HunkHeader:  base.Foreground(comment),
		DiffAdded:   base.Foreground(added),
		DiffRemoved: base.Foreground(removed),
		Label:       base.Foreground(highlight).Bold(true),
		LineNo:      base.Dim(true),
		StatusBar:   base.Background(accent).Foreground(contrastFg(accent)),
		SearchCur:   base.Background(highlight).Foreground(tcell.ColorBlack).Bold(true),
		Flash:       base.Foreground(added).Bold(true).Reverse(true),

		BgAdded:   bgAdded,
		BgRemoved: bgRemoved,
	}
}

// chromaColor extracts the foreground color for a token type from a chroma style.
func chromaColor(s *chroma.Style, t chroma.TokenType, fallback tcell.Color) tcell.Color {
	entry := s.Get(t)
	if entry.Colour.IsSet() {
		return tcell.NewRGBColor(
			int32(entry.Colour.Red()),
			int32(entry.Colour.Green()),
			int32(entry.Colour.Blue()),
		)
	}
	return fallback
}

// computeDiffBg calculates subtle background tint colors for added/removed lines
// by shifting the theme's background color toward green/red.
func computeDiffBg(cs *chroma.Style) (bgAdded, bgRemoved tcell.Color) {
	bgEntry := cs.Get(chroma.Background)
	if !bgEntry.Background.IsSet() {
		return tcell.NewRGBColor(0x1a, 0x3a, 0x1a), tcell.NewRGBColor(0x3a, 0x1a, 0x1a)
	}

	r := int32(bgEntry.Background.Red())
	g := int32(bgEntry.Background.Green())
	b := int32(bgEntry.Background.Blue())

	if bgEntry.Background.Brightness() < 0.5 {
		// Dark theme: shift toward green/red
		bgAdded = tcell.NewRGBColor(r, clamp32(g+32), b)
		bgRemoved = tcell.NewRGBColor(clamp32(r+32), g, b)
	} else {
		// Light theme: shift toward green/red
		bgAdded = tcell.NewRGBColor(clamp32(r-20), g, clamp32(b-20))
		bgRemoved = tcell.NewRGBColor(r, clamp32(g-20), clamp32(b-20))
	}
	return
}

// contrastFg returns black or white depending on which contrasts better with bg.
func contrastFg(bg tcell.Color) tcell.Color {
	r, g, b := bg.RGB()
	// Perceived luminance (ITU-R BT.601)
	lum := 0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)
	if lum > 128 {
		return tcell.ColorBlack
	}
	return tcell.ColorWhite
}

func clamp32(v int32) int32 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

// ListThemes prints all available chroma theme names and exits.
func ListThemes() {
	for _, name := range styles.Names() {
		fmt.Println(name)
	}
	os.Exit(0)
}
