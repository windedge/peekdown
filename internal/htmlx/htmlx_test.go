package htmlx

import (
	"reflect"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name  string
		html  string
		check func(*testing.T, []Node)
	}{
		{
			name: "para segs",
			html: `<p>Hello <b>bold</b> <i>italic</i> <u>under</u> <s>gone</s> <code>x</code></p>`,
			check: func(t *testing.T, nodes []Node) {
				want := []Seg{
					{Text: "Hello "},
					{Text: "bold", Bold: true},
					{Text: " "},
					{Text: "italic", Italic: true},
					{Text: " "},
					{Text: "under", Underline: true},
					{Text: " "},
					{Text: "gone", Strike: true},
					{Text: " "},
					{Text: "x", Code: true},
				}
				assertOne(t, nodes, func(n Node) {
					if !reflect.DeepEqual(n.Segs, want) {
						t.Fatalf("Segs = %+v, want %+v", n.Segs, want)
					}
					if n.Text != "Hello bold italic under gone x" {
						t.Fatalf("Text = %q", n.Text)
					}
				})
			},
		},
		{
			name: "heading levels",
			html: `<h1>one</h1><h3>three</h3><h6>six</h6>`,
			check: func(t *testing.T, nodes []Node) {
				if len(nodes) != 3 {
					t.Fatalf("len = %d", len(nodes))
				}
				for i, want := range []int{1, 3, 6} {
					if nodes[i].Level != want {
						t.Errorf("node %d Level = %d, want %d", i, nodes[i].Level, want)
					}
					if nodes[i].Kind != Para {
						t.Errorf("node %d Kind = %v", i, nodes[i].Kind)
					}
				}
			},
		},
		{
			name: "pre keeps newlines",
			html: "<pre>line1\nline2\n\n</pre>",
			check: func(t *testing.T, nodes []Node) {
				assertOne(t, nodes, func(n Node) {
					if n.Kind != Pre || n.Text != "line1\nline2" {
						t.Fatalf("Kind/Text = %v/%q", n.Kind, n.Text)
					}
				})
			},
		},
		{
			name:  "rule",
			html:  `<p>a</p><hr><p>b</p>`,
			check: checkNodes([]Node{literalPara("a"), {Kind: Rule}, literalPara("b")}),
		},
		{
			name: "top-level img",
			html: `<img src="https://example.com/a.png" alt="pic">`,
			check: func(t *testing.T, nodes []Node) {
				assertOne(t, nodes, func(n Node) {
					if n.Kind != Image || n.Src != "https://example.com/a.png" || n.Alt != "pic" || n.Text != "pic" {
						t.Fatalf("node = %+v", n)
					}
				})
			},
		},
		{
			name: "unsafe img falls back to alt para",
			html: `<img src="/relative/a.png" alt="fallback">`,
			check: func(t *testing.T, nodes []Node) {
				assertOne(t, nodes, func(n Node) {
					if n.Kind != Para || n.Text != "fallback" || len(n.Segs) != 1 || n.Segs[0].Text != "fallback" {
						t.Fatalf("node = %+v", n)
					}
				})
			},
		},
		{
			name: "table cells and text",
			html: `<table><thead><tr><th>H1</th><th>H2</th></tr></thead><tbody><tr><td>a</td><td>b</td></tr></tbody></table>`,
			check: func(t *testing.T, nodes []Node) {
				assertOne(t, nodes, func(n Node) {
					wantCells := [][]string{{"H1", "H2"}, {"a", "b"}}
					if n.Kind != Table || !reflect.DeepEqual(n.Cells, wantCells) || n.Text != "H1 H2 a b" {
						t.Fatalf("node = %+v cells %v", n, n.Cells)
					}
				})
			},
		},
		{
			name: "container splits paras",
			html: `<div><p>a</p><section><p>b</p></section>plain<ul><li>c</li></ul></div>`,
			check: func(t *testing.T, nodes []Node) {
				want := []Node{literalPara("a"), literalPara("b"), literalPara("plain"), literalPara("c")}
				if !reflect.DeepEqual(nodes, want) {
					t.Fatalf("nodes = %+v", nodes)
				}
			},
		},
		{
			name:  "script pruned",
			html:  `<script>alert(1)</script>`,
			check: checkNodes(nil),
		},
		{
			name: "script inside div pruned",
			html: `<div>ok<script>alert(1)</script><style>.x{}</style></div>`,
			check: func(t *testing.T, nodes []Node) {
				want := []Node{literalPara("ok")}
				if !reflect.DeepEqual(nodes, want) {
					t.Fatalf("nodes = %+v", nodes)
				}
			},
		},
		{
			name:  "unknown tag transparent",
			html:  `<my-widget><p>inner</p></my-widget>`,
			check: checkNodes([]Node{literalPara("inner")}),
		},
		{
			name: "javascript link drops to transparent",
			html: `<p>see <a href="javascript:alert(1)">here</a> now</p>`,
			check: func(t *testing.T, nodes []Node) {
				want := []Seg{{Text: "see here now"}}
				assertOne(t, nodes, func(n Node) {
					if !reflect.DeepEqual(n.Segs, want) {
						t.Fatalf("Segs = %+v", n.Segs)
					}
				})
			},
		},
		{
			name: "mailto link kept",
			html: `<p><a href="mailto:a@b.com">mail</a></p>`,
			check: func(t *testing.T, nodes []Node) {
				want := []Seg{{Text: "mail", Link: true, Href: "mailto:a@b.com"}}
				assertOne(t, nodes, func(n Node) {
					if !reflect.DeepEqual(n.Segs, want) {
						t.Fatalf("Segs = %+v", n.Segs)
					}
				})
			},
		},
		{
			name:  "data image svg rejected",
			html:  `<img src="data:image/svg+xml;base64,PHN2Zz4=" alt="s">`,
			check: checkNodes([]Node{literalPara("s")}),
		},
		{
			name:  "data image png accepted",
			html:  `<img src="data:image/png;base64,iVBOR" alt="p">`,
			check: checkNodes([]Node{{Kind: Image, Src: "data:image/png;base64,iVBOR", Alt: "p", Text: "p"}}),
		},
		{
			name:  "br makes newline seg",
			html:  `<p>one<br>two</p>`,
			check: checkNodes([]Node{{Kind: Para, Segs: []Seg{{Text: "one\ntwo"}}, Text: "one\ntwo"}}),
		},
		{
			name:  "em and del",
			html:  `<p><em>hi</em><del>bye</del></p>`,
			check: checkNodes([]Node{{Kind: Para, Segs: []Seg{{Text: "hi", Italic: true}, {Text: "bye", Strike: true}}, Text: "hibye"}}),
		},
		{
			name:  "whitespace collapsed",
			html:  "<p>  a\n\t b   c </p>",
			check: checkNodes([]Node{{Kind: Para, Segs: []Seg{{Text: " a b c "}}, Text: " a b c "}}),
		},
		{
			name:  "seg merge same style",
			html:  `<p><b>a</b><strong>b</strong> <b>c</b></p>`,
			check: checkNodes([]Node{{Kind: Para, Segs: []Seg{{Text: "ab", Bold: true}, {Text: " "}, {Text: "c", Bold: true}}, Text: "ab c"}}),
		},
		{
			name: "visible text joins",
			html: `<p>a</p><hr><p>b</p>`,
			check: func(t *testing.T, nodes []Node) {
				if got := VisibleText(nodes); got != "a\nb" {
					t.Fatalf("VisibleText = %q", got)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nodes := Parse(tt.html)
			tt.check(t, nodes)
		})
	}
}

// literalPara builds a plain single-seg Para as tests expect from the parser.
func literalPara(text string) Node {
	return Node{Kind: Para, Segs: []Seg{{Text: text}}, Text: text}
}

func assertOne(t *testing.T, nodes []Node, check func(Node)) {
	t.Helper()
	if len(nodes) != 1 {
		t.Fatalf("len(nodes) = %d, want 1: %+v", len(nodes), nodes)
	}
	check(nodes[0])
}

func checkNodes(want []Node) func(*testing.T, []Node) {
	return func(t *testing.T, nodes []Node) {
		t.Helper()
		if !reflect.DeepEqual(nodes, want) {
			t.Fatalf("nodes = %+v, want %+v", nodes, want)
		}
	}
}
