package view

import (
	"fmt"
	"os"
	"sync"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/selection"
)

// Three rounds of headless guesses did not reproduce the user's dead
// clicks, so a field build needs one stderr trace to find the broken link.
var debugClickEnabled = sync.OnceValue(func() bool {
	return os.Getenv("PEEKDOWN_DEBUG_CLICK") == "1"
})

func debugClick(format string, args ...any) {
	if !debugClickEnabled() {
		return
	}
	fmt.Fprintf(os.Stderr, "[click] "+format+"\n", args...)
}

func debugPoint(p selection.Point, ok bool) (selection.Point, bool) {
	if debugClickEnabled() {
		if ok {
			debugClick("pointat=true block=%d offset=%d", p.Block, p.Offset)
		} else {
			debugClick("pointat=false block=-1 offset=-1")
		}
	}
	return p, ok
}

func debugLink(dest string) string {
	if debugClickEnabled() {
		debugClick("linkat=%q", dest)
	}
	return dest
}

func debugListRect(st *State) ui.Rect {
	if st.ctx == nil {
		return ui.Rect{}
	}
	return st.List.Handle.Bounds(st.ctx)
}
