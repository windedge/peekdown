package doc

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui/markdown/parser"
)

// The run table is the one authority on what text a block contributes to
// the document, in reading order. Selections, search and copy address it;
// the renderer's spans must agree with it rune for rune.

func TestRunsOfASample(t *testing.T) {
	m := New("# Title\n\nAlpha beta.\n\n```go\nx := 1\ny := 2\n```\n\n> quoted\n\n- item one\n- item two\n")
	if m.Len() != 5 {
		t.Fatalf("%d blocks, want 5 (heading, paragraph, code, quote, list)", m.Len())
	}
	var got []string
	for _, r := range m.Runs() {
		got = append(got, r.Text)
	}
	want := []string{
		"Title",
		"Alpha beta.",
		"x := 1\ny := 2",
		"quoted",
		"item one\nitem two",
	}
	if len(got) != len(want) {
		t.Fatalf("%d runs %q, want %d %q", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("run %d: %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunTextMatchesPlainForParagraphs(t *testing.T) {
	m := New("Plain words here.\n")
	for _, r := range m.Runs() {
		if r.Text != m.Plain(r.Block) {
			t.Fatalf("run %q != Plain %q", r.Text, m.Plain(r.Block))
		}
	}
}

func TestRunsOfTableCells(t *testing.T) {
	m := New("| a | b |\n|---|---|\n| 1 | 2 |\n")
	table := m.Blocks()[0]
	var got []string
	for _, r := range m.Runs() {
		got = append(got, r.Text)
	}
	// The table is one run: cells in reading order, separated by spaces.
	want := "a b 1 2"
	if strings.Join(got, "\n") != want {
		t.Fatalf("table runs %q, want %q", got, want)
	}
	_ = table
}

func TestRunBlockIsTheLeaf(t *testing.T) {
	m := New("- one\n- two\n")
	runs := m.Runs()
	if len(runs) != 1 {
		t.Fatalf("%d runs, want 1 (the list is one run)", len(runs))
	}
	// The run's Block is the top-level block, not the nested items.
	if runs[0].Block != m.Blocks()[0].Index {
		t.Fatalf("run block %d, want the list %d", runs[0].Block, m.Blocks()[0].Index)
	}
}

func TestRunTableSurvivesSet(t *testing.T) {
	m := New("before\n")
	if m.Runs()[0].Text != "before" {
		t.Fatalf("initial run %q", m.Runs()[0].Text)
	}
	m.Set("after\n")
	if m.Runs()[0].Text != "after" {
		t.Fatalf("run after Set %q", m.Runs()[0].Text)
	}
}

func TestEmptyDocumentHasNoRuns(t *testing.T) {
	m := New("\n\n")
	if len(m.Runs()) != 0 {
		t.Fatalf("runs %v, want none", m.Runs())
	}
}

func TestTextOfRun(t *testing.T) {
	m := New("Alpha beta.\n\nGamma delta.\n")
	runs := m.Runs()
	if m.RunText(runs[0], 6, 10) != "beta" {
		t.Fatalf("RunText: %q", m.RunText(runs[0], 6, 10))
	}
	if m.RunText(runs[0], 0, 100) != "Alpha beta." {
		t.Fatalf("RunText clamped: %q", m.RunText(runs[0], 0, 100))
	}
	_ = parser.Default
}
