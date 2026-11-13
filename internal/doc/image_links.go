package doc

import (
	"github.com/egoist/mygo/ui/markdown/parser"
)

// linkTarget locates the link that wraps one block's whole content.
type linkTarget struct {
	block int
	dest  string
}

// LinkTargetOf returns the destination of the link that wraps block i's
// whole content, as [![alt](img)](href) wraps one image, or "". The view
// needs it because the inlines' walk marks the image's alt with the
// image's own dest and never sees the outer link: the click on the
// picture opens the outer href.
func (m *Model) LinkTargetOf(i int) string {
	for _, lt := range m.linkTargetsCache() {
		if lt.block == i {
			return lt.dest
		}
	}
	return ""
}

// linkTargetsCache builds the targets once per scan and caches them; scan
// drops the cache, so a new document rebuilds it.
func (m *Model) linkTargetsCache() []linkTarget {
	if m.linkTargets == nil {
		m.linkTargets = m.collectLinkTargets()
	}
	return m.linkTargets
}

// collectLinkTargets walks the top-level blocks and records each one a
// single link wraps: its leaf's click belongs to that link.
func (m *Model) collectLinkTargets() []linkTarget {
	var out []linkTarget
	for i := range m.doc.Children(root) {
		b := m.doc.Block(i)
		if b.Kind != parser.Paragraph && b.Kind != parser.Heading {
			continue
		}
		in := m.doc.Inlines(i)
		if len(in) == 0 || in[0].Kind != parser.Link {
			continue
		}
		if dest := outerLinkDest(in); dest != "" {
			out = append(out, linkTarget{block: i, dest: dest})
		}
	}
	return out
}

// outerLinkDest returns in[0]'s destination when the whole run of content
// is that link, an image or bare text wrapped in it, or "".
func outerLinkDest(in []parser.Inline) string {
	n := &in[0]
	if n.Kind != parser.Link || n.Next <= 0 || n.Next > len(in) {
		return ""
	}
	for k := 1; k < n.Next; k++ {
		switch in[k].Kind {
		case parser.Image, parser.SoftBreak, parser.LineBreak, parser.Text:
		default:
			return ""
		}
	}
	return n.Dest
}
