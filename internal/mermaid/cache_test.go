package mermaid

import (
	"image"
	"testing"
)

func TestCacheHit(t *testing.T) {
	c := NewCache(8)
	first, err := c.Render("graph TD; A-->B", ThemeDefault, 1.0)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if first == nil {
		t.Fatal("Render() image is nil")
	}
	// A fresh render decodes into a new image instance, so the second
	// call returning the very same instance proves it was a cache hit
	// and did not render again.
	second, err := c.Render("graph TD; A-->B", ThemeDefault, 1.0)
	if err != nil {
		t.Fatalf("Render() again error = %v", err)
	}
	if second != first {
		t.Fatal("the second call rendered again instead of returning the cached image")
	}
}

func TestCacheErrorCached(t *testing.T) {
	c := NewCache(8)
	_, err1 := c.Render("graph TD; A--", ThemeDefault, 1.0)
	if err1 == nil {
		t.Fatal("Render() error = nil, want error")
	}
	// The second call was a hit: the same error instance came back
	// without a second render, as a syntax error is deterministic.
	_, err2 := c.Render("graph TD; A--", ThemeDefault, 1.0)
	if err2 == nil {
		t.Fatal("Render() again error = nil, want error")
	}
	if err1 != err2 {
		t.Fatal("the second call rendered again instead of returning the cached error")
	}
}

func TestCacheEviction(t *testing.T) {
	c := NewCache(2)
	srcs := []string{
		"graph TD; A-->B",
		"graph TD; A-->C",
		"graph TD; A-->D",
	}
	imgs := make([]image.Image, len(srcs))
	for i, src := range srcs {
		img, err := c.Render(src, ThemeDefault, 1.0)
		if err != nil {
			t.Fatalf("Render(%q) error = %v", src, err)
		}
		imgs[i] = img
	}
	if len(c.items) != 2 {
		t.Fatalf("cache size = %d, want 2", len(c.items))
	}
	// The first source was evicted; re-rendering must be a miss, so a
	// fresh instance comes back, not the evicted one.
	again, err := c.Render(srcs[0], ThemeDefault, 1.0)
	if err != nil {
		t.Fatalf("Render(%q) error = %v", srcs[0], err)
	}
	if again == imgs[0] {
		t.Fatal("the evicted entry was still served")
	}
	// The most recent entry survives and is served as-is.
	recent, err := c.Render(srcs[2], ThemeDefault, 1.0)
	if err != nil {
		t.Fatalf("Render(%q) error = %v", srcs[2], err)
	}
	if recent != imgs[2] {
		t.Fatal("most recent entry was evicted")
	}
}

func TestCacheKeyDistinct(t *testing.T) {
	c := NewCache(8)
	src := "graph TD; A-->B"
	img1, err := c.Render(src, ThemeDefault, 1.0)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	img2, err := c.Render(src, ThemeDark, 1.0)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	img3, err := c.Render(src, ThemeDark, 2.0)
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	// Distinct theme/scale are distinct keys: each call missed and
	// rendered, so the instances differ.
	if img1 == img2 || img2 == img3 || img1 == img3 {
		t.Fatal("distinct theme/scale shared a cache entry")
	}
	if got := len(c.items); got != 3 {
		t.Fatalf("cache size = %d, want 3", got)
	}
}

func TestCacheKeyHash(t *testing.T) {
	k1 := Key("graph TD", ThemeDefault, 1.0)
	k2 := Key("graph TD", ThemeDefault, 1.0)
	k3 := Key("graph TD ", ThemeDefault, 1.0)
	if k1 != k2 {
		t.Fatal("identical inputs produced different keys")
	}
	if k1 == k3 {
		t.Fatal("different src produced the same key")
	}
}

// TestKeyLengthPrefixed guards the length prefixes: no split of the
// inputs into src, theme and scale reads as another.
func TestKeyLengthPrefixed(t *testing.T) {
	if Key("abcd", "ef", 0.0) == Key("abc", "def", 0.0) {
		t.Fatal("a shifted split of the inputs produced the same key")
	}
}
