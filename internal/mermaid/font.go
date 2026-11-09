package mermaid

import (
	"os"
	"runtime"
	"sync"

	"github.com/windedge/peekdown/internal/mermaid/raster"
	"golang.org/x/image/font/opentype"
)

var (
	cjkOnce sync.Once
	cjkOK   bool
)

// ensureCJKFonts injects system CJK faces into the raster package once,
// before the first render. The go-mermaid defaults (Go fonts) carry no CJK
// glyphs, so CJK labels would draw as tofu boxes; Microsoft YaHei covers
// them, and go-mermaid's width shrinking matches its wide-rune layout
// metrics, so no layout change is needed.
func ensureCJKFonts() {
	cjkOnce.Do(func() {
		regular, bold, ok := loadCJKFonts()
		if ok {
			raster.SetFonts(regular, bold)
		}
		cjkOK = ok
	})
}

// cjkActive reports whether system CJK faces were injected. False means the
// raster package still uses the Go defaults: English renders, CJK shows tofu
// boxes.
func cjkActive() bool {
	ensureCJKFonts()
	return cjkOK
}

// loadCJKFonts loads the Microsoft YaHei regular and bold faces. If either
// is missing or unparsable it returns ok=false and nothing is injected,
// silently keeping the go-mermaid defaults.
// TODO: macOS PingFang lives in /System/Library/Fonts/ (also .ttc), Linux
// needs fontconfig; both are out of scope for v0.3.0, which is Windows-first.
func loadCJKFonts() (regular, bold *opentype.Font, ok bool) {
	if runtime.GOOS != "windows" {
		return nil, nil, false
	}
	regular = loadTTCFace(`C:/Windows/Fonts/msyh.ttc`)
	bold = loadTTCFace(`C:/Windows/Fonts/msyhbd.ttc`)
	return regular, bold, regular != nil && bold != nil
}

// loadTTCFace parses the first face of a TrueType collection; errors return
// nil and the caller falls back.
func loadTTCFace(path string) *opentype.Font {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	col, err := opentype.ParseCollection(data)
	if err != nil {
		return nil
	}
	fnt, err := col.Font(0)
	if err != nil {
		return nil
	}
	return fnt
}
