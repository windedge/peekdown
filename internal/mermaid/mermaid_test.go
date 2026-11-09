package mermaid

import (
	"errors"
	"strings"
	"testing"

	gomermaid "github.com/zkrebbekx/go-mermaid"
)

func TestRenderFlowchart(t *testing.T) {
	img, err := Render("graph TD; A-->B", ThemeDefault, 1.0)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if img == nil {
		t.Fatal("Render() image is nil")
	}
	b := img.Bounds()
	if b.Dx() <= 50 || b.Dy() <= 50 {
		t.Fatalf("image too small: %dx%d", b.Dx(), b.Dy())
	}
}

func TestRenderSequence(t *testing.T) {
	img, err := Render("sequenceDiagram\nAlice->>Bob: hi", ThemeDefault, 1.0)
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}
	if img == nil {
		t.Fatal("Render() image is nil")
	}
}

func TestRenderPie(t *testing.T) {
	src := "pie title Pets\n \"Dogs\": 386\n \"Cats\": 85"
	img, err := Render(src, ThemeDefault, 1.0)
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}
	if img == nil {
		t.Fatal("Render() image is nil")
	}
}

func TestRenderDark(t *testing.T) {
	img, err := Render("graph TD; A-->B", ThemeDark, 1.0)
	if err != nil {
		t.Fatalf("Render() error = %v, want nil", err)
	}
	if img == nil {
		t.Fatal("Render() image is nil")
	}
}

func TestRenderEmpty(t *testing.T) {
	for _, src := range []string{"", "   ", "\n\t  \n"} {
		img, err := Render(src, ThemeDefault, 1.0)
		if err == nil {
			t.Errorf("Render(%q) error = nil, want error", src)
		}
		if img != nil {
			t.Errorf("Render(%q) image = %v, want nil", src, img)
		}
	}
}

func TestRenderSyntaxError(t *testing.T) {
	img, err := Render("graph TD; A--", ThemeDefault, 1.0)
	if err == nil {
		t.Error("Render() error = nil, want error")
	}
	if img != nil {
		t.Errorf("Render() image = %v, want nil", img)
	}
}

// TestRenderErrorSinglePrefix keeps the error to one "mermaid:" even
// though the library's own sentinels carry one already.
func TestRenderErrorSinglePrefix(t *testing.T) {
	_, err := Render("graph TD; A--", ThemeDefault, 1.0)
	if err == nil {
		t.Fatal("Render() error = nil, want error")
	}
	if n := strings.Count(err.Error(), "mermaid:"); n != 1 {
		t.Errorf("error %q carries %d prefixes, want 1", err, n)
	}
}

// TestRenderErrorIsParse keeps errors.Is working on the library's
// sentinels through the wrap.
func TestRenderErrorIsParse(t *testing.T) {
	_, err := Render("graph TD; A--", ThemeDefault, 1.0)
	if !errors.Is(err, gomermaid.ErrParse) {
		t.Errorf("errors.Is(err, ErrParse) = false for %v", err)
	}
}
