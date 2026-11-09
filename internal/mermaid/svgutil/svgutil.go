// Copied from zkrebbekx/go-mermaid v0.1.6 (MIT License, Copyright (c) 2026 Zac Krebbekx). Local fork adds SetFonts for CJK font injection.
// Trimmed to the sans TextWidth path used by raster/text.go.

// Package svgutil estimates text width: the metric table the raster
// package shrinks oversized labels against.
package svgutil

import (
	"unicode"
)

// helveticaAdvance holds advance widths (per 1000 em units) for printable
// ASCII 32..126, approximating Helvetica/Arial. Used for label sizing so wide
// glyphs (W, m, @) reserve more room than narrow ones (i, l, .).
var helveticaAdvance = [95]int{
	278, 278, 355, 556, 556, 889, 667, 191, 333, 333, 389, 584, 278, 333, 278, 278, // 32..47
	556, 556, 556, 556, 556, 556, 556, 556, 556, 556, 278, 278, 584, 584, 584, 556, // 48..63
	1015, 667, 667, 722, 722, 667, 611, 778, 722, 278, 500, 667, 556, 833, 722, 778, // 64..79
	667, 778, 722, 667, 611, 722, 667, 944, 667, 667, 611, 278, 278, 278, 469, 556, // 80..95
	333, 556, 556, 500, 556, 556, 278, 556, 556, 222, 222, 500, 222, 833, 556, 556, // 96..111
	556, 556, 333, 500, 278, 556, 500, 722, 500, 500, 500, 334, 260, 334, 584, // 112..126
}

// East Asian wide ranges. A character in one of these occupies a full em,
// not the 0.6 em that a non-ASCII fallback would otherwise charge.
var wideRanges = [][2]rune{
	{0x1100, 0x115F}, {0x2E80, 0x303E}, {0x3041, 0x33FF},
	{0x3400, 0x4DBF}, {0x4E00, 0x9FFF}, {0xA000, 0xA4CF},
	{0xAC00, 0xD7A3}, {0xF900, 0xFAFF}, {0xFE30, 0xFE6F},
	{0xFF00, 0xFF60}, {0xFFE0, 0xFFE6},
	{0x20000, 0x2FFFD}, {0x30000, 0x3FFFD},
}

// Emoji ranges, which render wider than a full em in most fonts.
var emojiRanges = [][2]rune{
	{0x1F000, 0x1F0FF}, {0x1F300, 0x1F5FF}, {0x1F600, 0x1F64F},
	{0x1F680, 0x1F6FF}, {0x1F900, 0x1FAFF},
}

func inRanges(rs [][2]rune, r rune) bool {
	for _, p := range rs {
		if r >= p[0] && r <= p[1] {
			return true
		}
	}
	return false
}

// asciiAdvance returns the advance of a printable ASCII rune in 1/1000 em.
func asciiAdvance(r rune) int {
	return helveticaAdvance[r-32]
}

// runeWidth returns the advance of one rune as a fraction of the font size.
func runeWidth(r rune) float64 {
	if r >= 32 && r < 127 {
		return float64(asciiAdvance(r)) / 1000
	}
	switch {
	case unicode.Is(unicode.Mn, r), unicode.Is(unicode.Me, r), unicode.Is(unicode.Cf, r):
		// Combining marks, enclosing marks and format controls such as the
		// zero-width joiner and the variation selectors take no room of
		// their own; they modify the glyph beside them.
		return 0
	case r >= 0x1F3FB && r <= 0x1F3FF:
		// Skin tone modifiers merge into the emoji they follow.
		return 0
	case inRanges(wideRanges, r):
		return 1.0
	case inRanges(emojiRanges, r):
		return 1.2
	}
	return 0.6
}

// zeroWidthJoiner combines the emoji on either side of it into one glyph.
const zeroWidthJoiner = 0x200D

// TextWidth estimates the rendered width of s at the given font size using
// sans-serif (Helvetica) metrics.
func TextWidth(s string, fontSize float64) float64 {
	var em float64
	joined := false
	for _, r := range s {
		if joined {
			// The previous rune was a zero-width joiner, so this one merges
			// into the glyph before it and adds no width of its own.
			joined = r == zeroWidthJoiner
			continue
		}
		joined = r == zeroWidthJoiner
		em += runeWidth(r)
	}
	return em * fontSize
}
