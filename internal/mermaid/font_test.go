package mermaid

import (
	"runtime"
	"testing"
)

func TestCJKFontLoadedOrFallback(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("CJK font injection is Windows-only for now")
	}
	if !cjkActive() {
		t.Fatal("cjkActive() = false on Windows; msyh.ttc/msyhbd.ttc should be present")
	}
}

func TestRenderCJKDiagram(t *testing.T) {
	// Glyph pixels are not asserted: a tofu-box render also succeeds. The
	// visual check is a real-device screenshot; here we log whether the
	// CJK face was active for this render.
	img, err := Render("graph LR; 开始-->处理-->结束", ThemeDefault, 1.0)
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
	if cjkActive() {
		t.Logf("CJK face injected; rendered %dx%d", b.Dx(), b.Dy())
	} else {
		t.Logf("CJK face NOT injected (fallback to Go fonts); rendered %dx%d", b.Dx(), b.Dy())
	}
}
