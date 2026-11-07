package doc

import (
	"reflect"
	"testing"
)

func TestExtractFrontmatter(t *testing.T) {
	src := "---\ntitle: Note\nslug: hello-world\ndraft: true\nweight: 3\nscore: 2.5\nmissing:\ntags:\n  - a\n  - b\nmeta:\n  author: Li\n  year: 2024\n---\n\n# Title\n\nBody text.\n"
	front, rest, ok := ExtractFrontmatter(src)
	if !ok || front == nil {
		t.Fatalf("no frontmatter found in %q", src)
	}
	if front.Format != "yaml" {
		t.Fatalf("Format %q, want yaml", front.Format)
	}
	if front.Broken {
		t.Fatalf("Broken for valid YAML")
	}
	wantOrder := []string{"title", "slug", "draft", "weight", "score", "missing", "tags", "meta"}
	if !reflect.DeepEqual(front.Order, wantOrder) {
		t.Fatalf("Order %v, want %v", front.Order, wantOrder)
	}
	if front.Values["title"] != "Note" {
		t.Fatalf("title %v, want Note", front.Values["title"])
	}
	if front.Values["draft"] != true {
		t.Fatalf("draft %v, want true", front.Values["draft"])
	}
	if front.Values["weight"] != float64(3) {
		t.Fatalf("weight %v, want float64(3)", front.Values["weight"])
	}
	if front.Values["score"] != 2.5 {
		t.Fatalf("score %v, want 2.5", front.Values["score"])
	}
	if v := front.Values["missing"]; v != nil {
		t.Fatalf("missing %v, want nil", v)
	}
	if _, ok := front.Values["missing"]; !ok {
		t.Fatalf("missing not set")
	}
	if !reflect.DeepEqual(front.Values["tags"], []any{"a", "b"}) {
		t.Fatalf("tags %v, want [a b]", front.Values["tags"])
	}
	if !reflect.DeepEqual(front.Values["meta"], map[string]any{"author": "Li", "year": float64(2024)}) {
		t.Fatalf("meta %v, want map", front.Values["meta"])
	}
	if front.LineCount != 14 {
		// 1 open + 12 content + 1 close.
		t.Fatalf("LineCount %d, want 14", front.LineCount)
	}
	if front.Raw == "" {
		t.Fatalf("Raw empty")
	}
	wantRest := "\n# Title\n\nBody text.\n"
	if rest != wantRest {
		t.Fatalf("rest %q, want %q", rest, wantRest)
	}
}

func TestExtractFrontmatterCRLFAndBOM(t *testing.T) {
	src := "\uFEFF---\r\ntitle: Note\r\n---\r\n\r\n# Title\r\n\r\nBody.\r\n"
	front, rest, ok := ExtractFrontmatter(src)
	if !ok || front == nil {
		t.Fatalf("no frontmatter")
	}
	if front.Values["title"] != "Note" {
		t.Fatalf("title %v, want Note", front.Values["title"])
	}
	if front.LineCount != 3 {
		t.Fatalf("LineCount %d, want 3", front.LineCount)
	}
	// The rest keeps its CRLF and the blank line after the close.
	wantRest := "\r\n# Title\r\n\r\nBody.\r\n"
	if rest != wantRest {
		t.Fatalf("rest %q, want %q", rest, wantRest)
	}
}

func TestExtractFrontmatterOnlyAtStart(t *testing.T) {
	// A --- later in the document is a thematic break, untouched.
	src := "# Title\n\nText.\n\n---\n\ntitle: not frontmatter\n\n---\n\nMore.\n"
	front, rest, ok := ExtractFrontmatter(src)
	if ok || front != nil {
		t.Fatalf("frontmatter found mid-document")
	}
	if rest != src {
		t.Fatalf("rest %q, want the source unchanged", rest)
	}
}

func TestExtractFrontmatterUnclosed(t *testing.T) {
	// Only one ---: not frontmatter.
	src := "---\ntitle: Note\n"
	front, rest, ok := ExtractFrontmatter(src)
	if ok || front != nil {
		t.Fatalf("frontmatter found without a close")
	}
	if rest != src {
		t.Fatalf("rest %q, want the source unchanged", rest)
	}
}

func TestExtractFrontmatterDotsCloseNotRecognized(t *testing.T) {
	// The Rust version closes only on ---, not on ...; without a --- close
	// the text stays ordinary Markdown.
	src := "---\ntitle: Note\n...\n\n# Title\n"
	front, rest, ok := ExtractFrontmatter(src)
	if ok || front != nil {
		t.Fatalf("... closed the block")
	}
	if rest != src {
		t.Fatalf("rest %q, want the source unchanged", rest)
	}
}

func TestExtractFrontmatterTOML(t *testing.T) {
	src := "+++\ntitle = \"Note\"\n+++\n\n# Title\n"
	front, rest, ok := ExtractFrontmatter(src)
	if !ok || front == nil {
		t.Fatalf("no frontmatter")
	}
	if front.Format != "toml" {
		t.Fatalf("Format %q, want toml", front.Format)
	}
	if len(front.Order) != 0 {
		t.Fatalf("Order %v, want empty for TOML", front.Order)
	}
	if front.Broken {
		t.Fatalf("Broken for TOML")
	}
	if front.LineCount != 3 {
		t.Fatalf("LineCount %d, want 3", front.LineCount)
	}
	if front.Raw == "" {
		t.Fatalf("Raw empty")
	}
	if rest != "\n# Title\n" {
		t.Fatalf("rest %q", rest)
	}
}

func TestExtractFrontmatterEmpty(t *testing.T) {
	// An empty frontmatter is still a frontmatter.
	src := "---\n---\n\n# Title\n"
	front, rest, ok := ExtractFrontmatter(src)
	if !ok || front == nil {
		t.Fatalf("no frontmatter")
	}
	if len(front.Order) != 0 {
		t.Fatalf("Order %v, want empty", front.Order)
	}
	if front.Broken {
		t.Fatalf("Broken for an empty block")
	}
	if front.LineCount != 2 {
		t.Fatalf("LineCount %d, want 2", front.LineCount)
	}
	if rest != "\n# Title\n" {
		t.Fatalf("rest %q", rest)
	}
}

func TestExtractFrontmatterBrokenYAML(t *testing.T) {
	// Broken YAML: still a frontmatter, Raw kept for the view to show it
	// as a code block.
	src := "---\n: :\n---\n\n# Title\n"
	front, rest, ok := ExtractFrontmatter(src)
	if !ok || front == nil {
		t.Fatalf("no frontmatter")
	}
	if !front.Broken {
		t.Fatalf("Broken false for %q", ": :")
	}
	if front.Format != "yaml" {
		t.Fatalf("Format %q, want yaml", front.Format)
	}
	if front.Raw == "" {
		t.Fatalf("Raw empty for broken YAML")
	}
	if rest != "\n# Title\n" {
		t.Fatalf("rest %q", rest)
	}
}

func TestYamlAliasResolves(t *testing.T) {
	// An alias shows the value its anchor holds, not the anchor's name.
	src := "---\nbase: &b\n  x: 1\nuse: *b\nlabel: &l ABC\ncopy: *l\n---\n\n# Title\n"
	front, _, ok := ExtractFrontmatter(src)
	if !ok || front == nil || front.Broken {
		t.Fatalf("front %+v", front)
	}
	want := []string{"base", "use", "label", "copy"}
	if !reflect.DeepEqual(front.Order, want) {
		t.Fatalf("Order %v, want %v", front.Order, want)
	}
	if !reflect.DeepEqual(front.Values["use"], map[string]any{"x": float64(1)}) {
		t.Fatalf("use %v, want the anchored map", front.Values["use"])
	}
	if front.Values["copy"] != "ABC" {
		t.Fatalf("copy %v, want ABC", front.Values["copy"])
	}
}

func TestYamlMergeKey(t *testing.T) {
	// A merge key is not a key: the aliased map's pairs expand in place,
	// keys the parent already holds winning, as YAML merges them.
	src := "---\nbase: &b\n  x: 1\n  shared: from-base\nmerged:\n  <<: *b\n  shared: from-merged\n  z: 3\n---\n\n# Title\n"
	front, _, ok := ExtractFrontmatter(src)
	if !ok || front == nil || front.Broken {
		t.Fatalf("front %+v", front)
	}
	m, mapOK := front.Values["merged"].(map[string]any)
	if !mapOK {
		t.Fatalf("merged %v, want a map", front.Values["merged"])
	}
	if m["x"] != float64(1) {
		t.Fatalf("merged x %v, want 1", m["x"])
	}
	if m["shared"] != "from-merged" {
		t.Fatalf("merged shared %v, want from-merged", m["shared"])
	}
	if m["z"] != float64(3) {
		t.Fatalf("merged z %v, want 3", m["z"])
	}
	for _, key := range front.Order {
		if key == "<<" {
			t.Fatalf("Order %v: the merge key leaked into it", front.Order)
		}
	}
}

func TestYamlMergeKeySequence(t *testing.T) {
	// A merge to a sequence of maps expands them in order.
	src := "---\na: &a\n  x: 1\nb: &b\n  y: 2\nmerged:\n  <<: [*a, *b]\n---\n\n# Title\n"
	front, _, ok := ExtractFrontmatter(src)
	if !ok || front == nil || front.Broken {
		t.Fatalf("front %+v", front)
	}
	m, mapOK := front.Values["merged"].(map[string]any)
	if !mapOK || m["x"] != float64(1) || m["y"] != float64(2) {
		t.Fatalf("merged %v, want x and y", front.Values["merged"])
	}
}

func TestYamlDuplicateKeyFallsBack(t *testing.T) {
	// A repeated key hides which value the pair carries: fall back to
	// the code block, Raw kept.
	src := "---\na: 1\na: 2\n---\n\n# Title\n"
	front, rest, ok := ExtractFrontmatter(src)
	if !ok || front == nil {
		t.Fatalf("no frontmatter")
	}
	if !front.Broken {
		t.Fatalf("Broken false for a duplicate key")
	}
	if front.Raw != "a: 1\na: 2\n" {
		t.Fatalf("Raw %q, want the raw lines", front.Raw)
	}
	if rest != "\n# Title\n" {
		t.Fatalf("rest %q", rest)
	}
}

func TestYamlMergeRefusesFallback(t *testing.T) {
	// A merge key at the top level targets a scalar: nothing sane to
	// expand, the fallback shows the raw text instead.
	src := "---\nbase: &b plain\n<<: *b\n---\n\n# Title\n"
	front, _, ok := ExtractFrontmatter(src)
	if !ok || front == nil {
		t.Fatalf("no frontmatter")
	}
	if !front.Broken {
		t.Fatalf("Broken false for a scalar merge")
	}
}

func TestExtractFrontmatterAbsent(t *testing.T) {
	front, rest, ok := ExtractFrontmatter("no fence here\nplain text\n")
	if ok {
		t.Fatalf("frontmatter found")
	}
	if front != nil {
		t.Fatalf("front %v, want nil", front)
	}
	if rest != "no fence here\nplain text\n" {
		t.Fatalf("rest %q", rest)
	}
}
