package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The window's view builds without a window, as the native template does.
func TestEmptyView(t *testing.T) {
	a := &app{}
	tt := ui.NewTester(a.view, 640, 400)
	if !tt.HasText("Peekdown") {
		t.Fatalf("texts %q: want the app name", tt.Texts())
	}
	if !tt.HasText("Markdown viewer") {
		t.Fatalf("texts %q: want the placeholder line", tt.Texts())
	}
}
