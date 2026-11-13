package view

import (
	"os"
	"testing"
	"time"
)

// TestFrontmatterCopyButtonShot writes the frontmatter card's copy button
// screenshots for a human to look at; skipped unless PEEKDOWN_SHOT is set,
// so the regular test runs stay headless-clean. The hover pass rests the
// pointer on the button past mygo's tooltip delay, so the icon-only button
// and its tooltip land in the same frame.
func TestFrontmatterCopyButtonShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the frontmatter copy button screenshot")
	}
	_, tt, st := frontCard(t, "---\ntitle: x\n---\n\nbody\n")
	st.CopyFrontmatter = func(string) {}
	tt.Frame()
	r, ok := tt.Find("Copy YAML")
	if !ok {
		t.Fatalf("texts %v: no copy button", texts(tt))
	}
	tt.Move(r.X+r.W/2, r.Y+r.H/2)
	tt.Advance(650 * time.Millisecond)
	tt.Frame()
	writeShotPng(t, tt, "C:/tmp/fm-copy-full.png")

	// Crop around the button and blow it up 3x nearest-neighbor, so the
	// icon's shape is easy to judge against the code block's button.
	writeImagePng(t, scale3x(shotCrop(tt.Image(), r)), "C:/tmp/fm-copy-icon.png")
}

// TestFrontmatterFoldShot writes the card open and folded, for a human
// to compare: folded must be the one header row, no leftover rows, the
// buttons in place. Skipped unless PEEKDOWN_SHOT is set.
func TestFrontmatterFoldShot(t *testing.T) {
	if os.Getenv("PEEKDOWN_SHOT") == "" {
		t.Skip("set PEEKDOWN_SHOT=1 to write the frontmatter fold screenshots")
	}
	_, tt, st := frontCard(t, "---\ntitle: Note\ndraft: true\ntags:\n  - a\n  - b\n---\n\nbody\n")
	st.CopyFrontmatter = func(string) {}
	tt.Frame()
	writeShotPng(t, tt, "C:/tmp/fm-expanded.png")

	if err := tt.Click("Toggle frontmatter"); err != nil {
		t.Fatalf("no fold button: %v", err)
	}
	tt.Frame()
	writeShotPng(t, tt, "C:/tmp/fm-collapsed.png")
}
