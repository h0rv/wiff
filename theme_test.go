package main

import (
	"testing"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/gdamore/tcell/v2"
)

func TestClamp32(t *testing.T) {
	tests := []struct {
		in, want int32
	}{
		{-10, 0},
		{0, 0},
		{128, 128},
		{255, 255},
		{300, 255},
	}
	for _, tt := range tests {
		if got := clamp32(tt.in); got != tt.want {
			t.Errorf("clamp32(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestChromaColorKnownToken(t *testing.T) {
	cs := styles.Get("monokai")
	if cs == nil {
		t.Fatal("monokai style not found")
	}

	got := chromaColor(cs, chroma.Keyword, tcell.ColorAqua)
	if got == tcell.ColorAqua {
		t.Error("expected chromaColor to return a theme color for Keyword, got the fallback")
	}
}

func TestChromaColorFallback(t *testing.T) {
	cs := styles.Get("monokai")
	if cs == nil {
		t.Fatal("monokai style not found")
	}

	// Use an obscure token type that likely has no color set
	fallback := tcell.ColorFuchsia
	got := chromaColor(cs, chroma.TokenType(9999), fallback)
	// Should get either the fallback or a real color (chroma may inherit)
	// The key thing is it doesn't panic
	_ = got
}

func TestComputeDiffBgDarkTheme(t *testing.T) {
	cs := styles.Get("monokai")
	if cs == nil {
		t.Fatal("monokai style not found")
	}

	bgAdded, bgRemoved := computeDiffBg(cs)
	if bgAdded == bgRemoved {
		t.Error("expected bgAdded and bgRemoved to be distinct colors")
	}
}

func TestComputeDiffBgNilBackground(t *testing.T) {
	// Create a minimal style with no background set
	cs := styles.Get("bw") // black-and-white style, may not have BG
	if cs == nil {
		t.Skip("bw style not available")
	}

	// Should not panic regardless of whether background is set
	bgAdded, bgRemoved := computeDiffBg(cs)
	_ = bgAdded
	_ = bgRemoved
}

func TestNewUIThemeMonokai(t *testing.T) {
	theme := NewUITheme("monokai")

	if theme.Added == 0 {
		t.Error("expected Added color to be non-zero")
	}
	if theme.Removed == 0 {
		t.Error("expected Removed color to be non-zero")
	}
	if theme.Accent == 0 {
		t.Error("expected Accent color to be non-zero")
	}
	if theme.Highlight == 0 {
		t.Error("expected Highlight color to be non-zero")
	}
}

func TestNewUIThemeFallback(t *testing.T) {
	theme := NewUITheme("nonexistent-theme-name-12345")
	monokai := NewUITheme("monokai")

	// Fallback should produce the same accent as monokai
	if theme.Accent != monokai.Accent {
		t.Errorf("expected fallback theme accent %v to match monokai %v", theme.Accent, monokai.Accent)
	}
	if theme.Highlight != monokai.Highlight {
		t.Errorf("expected fallback theme highlight %v to match monokai %v", theme.Highlight, monokai.Highlight)
	}
}

func TestNewUIThemeLightTheme(t *testing.T) {
	// Verify a light theme doesn't panic and produces valid values
	theme := NewUITheme("github")

	if theme.BgAdded == theme.BgRemoved {
		t.Error("expected light theme bgAdded and bgRemoved to differ")
	}
}
