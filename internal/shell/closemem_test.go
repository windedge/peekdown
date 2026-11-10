package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"github.com/windedge/peekdown/internal/doc"
)

func closeMemDoc(blocks int) string {
	var b strings.Builder
	b.WriteString("| c1 | c2 |\n|---|---|\n")
	for i := 0; i < 1281; i++ {
		b.WriteString("| r")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("| some cell text ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(" |\n")
	}
	para := strings.Repeat("Lorem ipsum dolor sit amet, consectetur adipiscing elit, sed do eiusmod. ", 8)
	for i := 0; i < blocks; i++ {
		b.WriteString("\n## Heading ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n\n")
		b.WriteString(para)
		b.WriteString("\n")
	}
	return b.String()
}

// closeMemShell opens the synthetic big document in a rendered shell and
// returns it with the tester.
func closeMemShell(t *testing.T) (*Shell, *ui.Tester) {
	t.Helper()
	s, dir := newShell(t)
	big := filepath.Join(dir, "big.md")
	if err := os.WriteFile(big, []byte(closeMemDoc(2000)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.OpenFile(big); err != nil {
		t.Fatal(err)
	}
	tt := s.renderSize(t, 1024, 768)
	// Frames build the view's caches and the engine's layout cache, the
	// way a document the user actually looked at has.
	for k := 0; k < 5; k++ {
		tt.Frame()
	}
	s.Tabs[0].State.List.ScrollToEnd()
	tt.Frame()
	s.Tabs[0].State.List.ScrollTo(0, ui.Start)
	tt.Frame()
	return s, tt
}

// heapAfterGC returns the live heap over a full sweep: runtime.GC
// leaves the sweep lazy, so a HeapAlloc read straight after it still
// counts spans of the last test in the same process (a repeat run fails
// the budget on leftovers, not on the closed tab).
func heapAfterGC() uint64 {
	debug.FreeOSMemory()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m.HeapAlloc
}

// TestClosingBigTabReleasesMemory proves the closed document leaves the
// heap: the baseline over a shell holding the rendered document, the
// same after the close. The tail slot the shift leaves behind must not
// hold the tab.
func TestClosingBigTabReleasesMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("memory regression: run on the full suite only")
	}
	s, _ := closeMemShell(t)
	baseline := heapAfterGC()
	t.Logf("baseline HeapAlloc=%d MB", baseline>>20)
	if baseline < 4<<20 {
		t.Fatalf("baseline %d MB too small to measure against", baseline>>20)
	}

	// The proof is the delta: the close must free at least the
	// document's own footprint (the source and its parse, the hold
	// modelBytes guarantees) from the heap. A relative budget over
	// HeapAlloc breaks in a process that ran the suite before: the ui
	// package's global shaped-text caches stay live across runs and
	// inflate the baseline with heap the tab never held.
	doc := len(closeMemDoc(2000))
	s.CloseTab(0)
	after := heapAfterGC()
	t.Logf("after close HeapAlloc=%d MB, released %d MB", after>>20, (baseline-after)>>20)
	if released := baseline - after; released < uint64(2*doc) {
		t.Fatalf("closing the tab freed %d KB, under the document's own %d KB: the tab still holds references",
			released>>10, 2*doc>>10)
	}
}

// TestCloseAllReleasesMemory is the same proof over CloseAll.
func TestCloseAllReleasesMemory(t *testing.T) {
	if testing.Short() {
		t.Skip("memory regression: run on the full suite only")
	}
	s, _ := closeMemShell(t)
	baseline := heapAfterGC()

	doc := len(closeMemDoc(2000))
	s.CloseAll()
	after := heapAfterGC()
	t.Logf("after close all HeapAlloc=%d MB, released %d MB", after>>20, (baseline-after)>>20)
	if released := baseline - after; released < uint64(2*doc) {
		t.Fatalf("close all freed %d KB, under the document's own %d KB: the tabs still hold references",
			released>>10, 2*doc>>10)
	}
}

// TestModelBytesThreshold checks the gate that decides whether a close
// hands the freed pages to the OS: a small document stays under the
// threshold, a big one goes over it.
func TestModelBytesThreshold(t *testing.T) {
	small := &Tab{Model: doc.New(strings.Repeat("# t", 10))}
	if modelBytes(small) >= freeHeapThreshold {
		t.Fatalf("modelBytes of a small doc = %d, under the threshold", modelBytes(small))
	}
	big := &Tab{Model: doc.New(closeMemDoc(2000))}
	if modelBytes(big) < freeHeapThreshold {
		t.Fatalf("modelBytes of the big doc = %d, want over the threshold", modelBytes(big))
	}
}

// TestReleaseTabsReportsTheScavenger covers the release step itself: a
// big closed tab asks for the scavenger once, a small one does not.
func TestReleaseTabsReportsTheScavenger(t *testing.T) {
	s, _ := newShell(t)

	// releaseTabs zeroes the slots the slice itself holds, so a slice
	// over dereferenced copies would leave its originals untouched.
	batch := []Tab{{Model: doc.New(closeMemDoc(2000))}}
	if !s.releaseTabs(batch...) {
		t.Fatal("big document: no scavenger run")
	}
	if batch[0].Model != nil || batch[0].Path != "" || batch[0].State.LoadMermaid != nil {
		t.Fatal("the released tab still holds references")
	}

	if s.releaseTabs(Tab{Model: doc.New("# t")}) {
		t.Fatal("small document: scavenger ran, want none")
	}

	// A batch of one big and one small sweeps once, for the whole batch.
	if !s.releaseTabs(Tab{Model: doc.New("# t")}, Tab{Model: doc.New(closeMemDoc(2000))}) {
		t.Fatal("batch with a big tab: no scavenger run")
	}
}
