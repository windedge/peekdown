package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// The window's view builds without a window, as the native template does.
func TestEmptyView(t *testing.T) {
	a := newTestApp(t)
	tt := ui.NewTester(a.view, 640, 400)
	if !tt.HasText("Welcome to Peekdown") {
		t.Fatalf("texts %q: want the welcome title", tt.Texts())
	}
	if !tt.HasText("Or drag and drop files \u2022 Ctrl+O to open") {
		t.Fatalf("texts %q: want the drop hint", tt.Texts())
	}
}
