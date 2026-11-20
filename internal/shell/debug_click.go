package shell

import (
	"fmt"
	"os"
	"sync"
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
