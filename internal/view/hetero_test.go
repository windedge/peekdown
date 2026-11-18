package view

import (
	"fmt"
	"strings"

	"github.com/windedge/peekdown/internal/doc"
)

// heteroDoc mixes rows of very different heights: 60 short paragraphs, 20
// tall code blocks, then 60 short paragraphs again, so the list's height
// estimates of the rows it has not measured are far off theirs.
func heteroDoc() *doc.Model {
	var b strings.Builder
	for i := range 60 {
		fmt.Fprintf(&b, "Short paragraph %03d.\n\n", i)
	}
	for i := range 20 {
		b.WriteString("```go\n")
		for j := range 30 {
			fmt.Fprintf(&b, "func deepFunc%02d_%02d(a int) int { return a + %d }\n", i, j, j)
		}
		b.WriteString("```\n\n")
	}
	for i := range 60 {
		fmt.Fprintf(&b, "Short paragraph tail %03d.\n\n", i)
	}
	return doc.New(b.String())
}
