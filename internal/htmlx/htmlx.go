// Package htmlx turns an embedded HTML block into the semantic content a
// reader sees: paragraphs, tables, images, rules and preformatted text.
package htmlx

import (
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type Kind int

const (
	Para Kind = iota
	Pre
	Rule
	Image
	Table
)

// Seg is one run of inline text with its formatting.
type Seg struct {
	Text      string
	Bold      bool
	Italic    bool
	Strike    bool
	Underline bool
	Code      bool
	Link      bool
	Href      string
}

// Node is one semantic block extracted from an HTML fragment.
type Node struct {
	Kind  Kind
	Level int        // heading level for Para
	Segs  []Seg      // inline content of a Para
	Text  string     // visible text of the node ("" for Rule; alt for Image; cells joined by spaces for Table)
	Src   string     // Image
	Alt   string     // Image
	Cells [][]string // Table rows, header first
}

// Parse parses s as an HTML fragment in a body context and returns the
// semantic nodes of its top-level blocks.
func Parse(s string) []Node {
	ctx := &html.Node{Type: html.ElementNode, DataAtom: atom.Body, Data: "body"}
	frag, _ := html.ParseFragment(strings.NewReader(s), ctx)
	p := &parser{}
	for _, n := range frag {
		p.block(n)
		p.flush()
	}
	return p.nodes
}

type parser struct {
	nodes []Node
	segs  []Seg
}

// block dispatches one element: render, recurse as a container, or prune.
func (p *parser) block(n *html.Node) {
	switch {
	case n.Type == html.TextNode:
		p.appendText(collapse(n.Data), false)
		return
	case n.Type != html.ElementNode:
		return
	}
	switch n.DataAtom {
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6:
		p.inlineWalk(n, false)
		segs := p.takeSegs()
		if hasVisibleText(segs) {
			p.nodes = append(p.nodes, Node{
				Kind:  Para,
				Level: headingLevel(n.DataAtom),
				Segs:  segs,
				Text:  segsText(segs),
			})
		}
	case atom.P:
		p.inlineWalk(n, false)
		p.flush()
	case atom.Pre:
		var b strings.Builder
		collectText(n, &b)
		if t := strings.Trim(b.String(), "\n"); t != "" {
			p.nodes = append(p.nodes, Node{Kind: Pre, Text: t})
		}
	case atom.Hr:
		p.nodes = append(p.nodes, Node{Kind: Rule})
	case atom.Img:
		alt := attr(n, "alt")
		if src := attr(n, "src"); src != "" && srcSafe(src) {
			p.nodes = append(p.nodes, Node{Kind: Image, Src: src, Alt: alt, Text: alt})
		} else if alt != "" {
			p.nodes = append(p.nodes, Node{Kind: Para, Segs: []Seg{{Text: alt}}, Text: alt})
		}
	case atom.Table:
		p.table(n)
	case atom.Div, atom.Section, atom.Article, atom.Aside, atom.Header, atom.Footer,
		atom.Main, atom.Nav, atom.Figure, atom.Figcaption, atom.Blockquote,
		atom.Ul, atom.Ol, atom.Li, atom.Dl, atom.Dt, atom.Dd,
		atom.Details, atom.Summary, atom.Center, atom.Address, atom.Caption:
		p.blockChildren(n)
	default:
		// Known pruned tags drop the subtree; unknown tags are transparent.
		if pruned(n.DataAtom) {
			return
		}
		p.blockChildren(n)
	}
}

// blockChildren recurses into children as blocks, flushing after each.
func (p *parser) blockChildren(n *html.Node) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		p.block(c)
		p.flush()
	}
}

func (p *parser) table(n *html.Node) {
	var rows [][]string
	var findRows func(n *html.Node)
	findRows = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Tr:
				var row []string
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode && (c.DataAtom == atom.Td || c.DataAtom == atom.Th) {
						row = append(row, cellText(c))
					}
				}
				if len(row) > 0 {
					rows = append(rows, row)
				}
				return
			case atom.Table:
				return
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findRows(c)
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		findRows(c)
	}
	if len(rows) == 0 {
		return
	}
	var b strings.Builder
	for i, row := range rows {
		if i > 0 {
			b.WriteString(" ")
		}
		b.WriteString(strings.Join(row, " "))
	}
	p.nodes = append(p.nodes, Node{Kind: Table, Cells: rows, Text: b.String()})
}

// cellText renders one table cell to plain text: inline text with collapsed
// whitespace and br as spaces.
func cellText(n *html.Node) string {
	var b strings.Builder
	var walk func(n *html.Node)
	walk = func(n *html.Node) {
		switch {
		case n.Type == html.TextNode:
			b.WriteString(collapse(n.Data))
		case n.Type != html.ElementNode:
			return
		case n.DataAtom == atom.Br:
			b.WriteString(" ")
			return
		case pruned(n.DataAtom):
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Trim(b.String(), " ")
}

// inlineWalk walks inline content, appending formatted segs to the buffer.
func (p *parser) inlineWalk(n *html.Node, strike bool) {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		switch {
		case c.Type == html.TextNode:
			p.appendText(collapse(c.Data), strike)
		case c.Type != html.ElementNode:
		case pruned(c.DataAtom):
		case c.DataAtom == atom.Br:
			p.appendSeg(Seg{Text: "\n", Strike: strike})
		case c.DataAtom == atom.Img:
			if alt := attr(c, "alt"); alt != "" {
				p.appendText("["+alt+"]", strike)
			}
		case c.DataAtom == atom.A:
			if href := attr(c, "href"); href != "" && hrefSafe(href) {
				saved := p.takeSegs()
				p.inlineWalk(c, strike)
				inner := p.takeSegs()
				if !hasVisibleText(inner) {
					inner = []Seg{{Text: "", Link: true, Href: href, Strike: strike}}
				} else {
					for i := range inner {
						inner[i].Link = true
						inner[i].Href = href
					}
				}
				p.segs = saved
				for _, seg := range inner {
					p.appendSeg(seg)
				}
			} else {
				p.inlineWalk(c, strike)
			}
		case c.DataAtom == atom.Del || c.DataAtom == atom.S || c.DataAtom == atom.Strike:
			p.inlineWalk(c, true)
		default:
			bold := c.DataAtom == atom.B || c.DataAtom == atom.Strong
			italic := c.DataAtom == atom.I || c.DataAtom == atom.Em
			underline := c.DataAtom == atom.U || c.DataAtom == atom.Ins
			code := c.DataAtom == atom.Code || c.DataAtom == atom.Kbd || c.DataAtom == atom.Samp || c.DataAtom == atom.Tt
			if bold || italic || underline || code {
				saved := p.takeSegs()
				p.inlineWalk(c, strike)
				inner := p.takeSegs()
				for i := range inner {
					if bold {
						inner[i].Bold = true
					}
					if italic {
						inner[i].Italic = true
					}
					if underline {
						inner[i].Underline = true
					}
					if code {
						inner[i].Code = true
					}
				}
				p.segs = saved
				for _, seg := range inner {
					p.appendSeg(seg)
				}
			} else {
				p.inlineWalk(c, strike)
			}
		}
	}
}

// appendText appends s as a seg, merging with the previous one when the
// styles match.
func (p *parser) appendText(s string, strike bool) {
	if s == "" {
		return
	}
	p.appendSeg(Seg{Text: s, Strike: strike})
}

func (p *parser) appendSeg(seg Seg) {
	// An empty <a href> contributes nothing; br's "\n" stays.
	if seg.Text == "" {
		return
	}
	if n := len(p.segs); n > 0 {
		last := &p.segs[n-1]
		if sameStyle(*last, seg) {
			last.Text += seg.Text
			return
		}
	}
	p.segs = append(p.segs, seg)
}

func (p *parser) takeSegs() []Seg {
	segs := p.segs
	p.segs = nil
	return segs
}

// flush turns the inline buffer into a Para when it holds visible text.
func (p *parser) flush() {
	segs := p.takeSegs()
	if hasVisibleText(segs) {
		p.nodes = append(p.nodes, Node{Kind: Para, Segs: segs, Text: segsText(segs)})
	}
}

func sameStyle(a, b Seg) bool {
	return a.Bold == b.Bold && a.Italic == b.Italic && a.Strike == b.Strike &&
		a.Underline == b.Underline && a.Code == b.Code &&
		a.Link == b.Link && a.Href == b.Href
}

func hasVisibleText(segs []Seg) bool {
	for _, seg := range segs {
		if strings.ContainsFunc(seg.Text, func(r rune) bool { return !isSpace(r) }) {
			return true
		}
	}
	return false
}

func isSpace(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\v', '\f', '\r', 0x85, 0xA0:
		return true
	}
	return false
}

// attr returns the value of the attribute with the given key.
func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func headingLevel(a atom.Atom) int {
	switch a {
	case atom.H1:
		return 1
	case atom.H2:
		return 2
	case atom.H3:
		return 3
	case atom.H4:
		return 4
	case atom.H5:
		return 5
	case atom.H6:
		return 6
	}
	return 0
}

func hrefSafe(href string) bool {
	l := strings.ToLower(href)
	return strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") ||
		strings.HasPrefix(l, "mailto:") || strings.HasPrefix(l, "#")
}

func srcSafe(src string) bool {
	l := strings.ToLower(src)
	if strings.HasPrefix(l, "http://") || strings.HasPrefix(l, "https://") {
		return true
	}
	return strings.HasPrefix(l, "data:image/") && !strings.Contains(l, "svg")
}

// collapse folds whitespace runs to a single space, keeping edge spaces so
// words stay separated across inline boundaries.
func collapse(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for _, r := range s {
		if isSpace(r) {
			space = true
			continue
		}
		if space {
			b.WriteByte(' ')
			space = false
		}
		b.WriteRune(r)
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

func collectText(n *html.Node, b *strings.Builder) {
	if n.Type == html.TextNode {
		b.WriteString(n.Data)
		return
	}
	if n.Type != html.ElementNode || !pruned(n.DataAtom) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collectText(c, b)
		}
	}
}

func pruned(a atom.Atom) bool {
	switch a {
	case atom.Script, atom.Style, atom.Head, atom.Meta, atom.Link, atom.Title,
		atom.Iframe, atom.Object, atom.Embed, atom.Form, atom.Video, atom.Audio,
		atom.Canvas, atom.Svg, atom.Noscript, atom.Template, atom.Base,
		atom.Colgroup, atom.Col, atom.Param, atom.Source, atom.Track,
		atom.Map, atom.Area:
		return true
	}
	return false
}

// RunText parses s and returns its visible text, the text the run table
// holds for an HTML block, as the view's overlays address it.
func RunText(s string) string { return VisibleText(Parse(s)) }

// VisibleText joins the text of non-empty nodes with "\n".
func VisibleText(nodes []Node) string {
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if n.Text != "" {
			parts = append(parts, n.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func segsText(segs []Seg) string {
	var b strings.Builder
	for _, seg := range segs {
		b.WriteString(seg.Text)
	}
	return b.String()
}
