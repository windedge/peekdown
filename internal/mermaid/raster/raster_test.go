// Copied from zkrebbekx/go-mermaid v0.1.6 (MIT License, Copyright (c) 2026 Zac Krebbekx). Rewritten from goconvey to the standard testing package.

package raster_test

import (
	"bytes"
	"image/png"
	"testing"

	"github.com/windedge/peekdown/internal/mermaid/raster"
	mermaid "github.com/zkrebbekx/go-mermaid"
)

func TestPNG(t *testing.T) {
	out, err := raster.PNG("graph TD\nA[Start] --> B[End]", 1)
	if err != nil {
		t.Fatalf("PNG() error = %v", err)
	}
	if !bytes.Equal(out[:4], []byte{0x89, 'P', 'N', 'G'}) {
		t.Fatalf("PNG() magic = %v, want a PNG signature", out[:4])
	}
	img, err := png.Decode(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	if img.Bounds().Dx() <= 0 || img.Bounds().Dy() <= 0 {
		t.Fatalf("image bounds = %v, want positive size", img.Bounds())
	}
}

func TestPNGScale2xLarger(t *testing.T) {
	one, err := raster.PNG("graph TD\nA --> B", 1)
	if err != nil {
		t.Fatalf("PNG(1x) error = %v", err)
	}
	two, err := raster.PNG("graph TD\nA --> B", 2)
	if err != nil {
		t.Fatalf("PNG(2x) error = %v", err)
	}
	i1, err := png.Decode(bytes.NewReader(one))
	if err != nil {
		t.Fatalf("png.Decode(1x) error = %v", err)
	}
	i2, err := png.Decode(bytes.NewReader(two))
	if err != nil {
		t.Fatalf("png.Decode(2x) error = %v", err)
	}
	if i2.Bounds().Dx() <= i1.Bounds().Dx() {
		t.Fatalf("2x width = %d, want larger than 1x width %d", i2.Bounds().Dx(), i1.Bounds().Dx())
	}
}

func TestPNGInvalidSourcePropagatesError(t *testing.T) {
	if _, err := raster.PNG("graph TD\nA[oops", 1); err == nil {
		t.Fatal("PNG() error = nil, want the render error propagated")
	}
}

func TestPNGWithOptions(t *testing.T) {
	out, err := raster.PNG("graph TD\nA --> B", 1, mermaid.WithTheme(mermaid.Dark))
	if err != nil {
		t.Fatalf("PNG() with theme error = %v", err)
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
}
