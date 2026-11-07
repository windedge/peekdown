package doc

import (
	"strings"

	"gopkg.in/yaml.v3"
)

// Frontmatter is the key-value header a document may open with: a --- or
// +++ fence, key-value pairs, a closing fence. The doc layer strips it
// before parsing the Markdown, keeps it aside for the view to show as a
// bordered key-value table (YAML) or a code block (TOML or broken YAML).
type Frontmatter struct {
	// Format is "yaml" or "toml".
	Format string
	// Order lists the top-level keys in source order; Values holds each
	// key's value, a string, a float64, a bool, nil, a []any or a
	// map[string]any. Empty for TOML, which the doc layer does not parse.
	Order  []string
	Values map[string]any
	// Raw is the source text between the fences, for the copy button.
	Raw string
	// LineCount counts the fence lines in: the body's line 1 is the
	// source's line LineCount+1 (see the contract on Model).
	LineCount int
	// Broken marks YAML that failed to parse: the view shows Raw as a code
	// block instead of a table.
	Broken bool
}

// ExtractFrontmatter takes the frontmatter off the head of src: it
// returns the frontmatter and the body that follows it, found false when
// there is none. The body is what the Markdown parser sees; the
// frontmatter never is.
//
// Only the head of the file opens one: line 1 must be --- (YAML) or +++
// (TOML), after an optional BOM, with an optional \r before the \n. The
// close is the next line that is exactly the opening fence; ...
// does not close, matching the Rust implementation, so an unclosed
// fence is ordinary Markdown. An empty block is a frontmatter still.
func ExtractFrontmatter(src string) (*Frontmatter, string, bool) {
	head := src
	// A UTF-8 BOM before the fence line is tolerated.
	if strings.HasPrefix(head, "\ufeff") {
		head = head[3:]
	}
	// The fence line, with its trailing newline cut.
	line := head
	if i := strings.IndexByte(head, '\n'); i >= 0 {
		line = head[:i]
	}
	line = strings.TrimSuffix(line, "\r")
	if line != "---" && line != "+++" {
		return nil, src, false
	}
	// The close is the next line that is exactly the fence. Scanning
	// lines of head keeps the body's bytes as they were, BOM included.
	lines := strings.SplitAfter(head, "\n")
	closeLine := -1
	for i := 1; i < len(lines); i++ {
		l := strings.TrimSuffix(lines[i], "\n")
		l = strings.TrimSuffix(l, "\r")
		if l == line {
			closeLine = i
			break
		}
	}
	if closeLine < 0 {
		return nil, src, false
	}
	fm := &Frontmatter{Format: "yaml", Raw: strings.Join(lines[1:closeLine], "")}
	if line == "+++" {
		fm.Format = "toml"
	}
	fm.LineCount = closeLine + 1
	if fm.Format == "yaml" {
		parseYAML(fm)
	}
	// The body starts after the close's newline; when the close ends the
	// file without one, the body is empty.
	after := closeLine + 1
	if after > len(lines)-1 {
		return fm, "", true
	}
	return fm, strings.Join(lines[after:], ""), true
}

// parseYAML fills Order and Values from Raw: the top level must be a
// mapping, keys in source order, scalars by tag, lists and maps
// recursively. A parse error or another top level leaves the frontmatter
// broken, Raw kept for the view. A repeated key falls back too: the
// header promises one key-value pair per key in source order, and a
// duplicate hides which value the pair carries.
func parseYAML(fm *Frontmatter) {
	var node yaml.Node
	if err := yaml.Unmarshal([]byte(fm.Raw), &node); err != nil {
		fm.Broken = true
		return
	}
	if len(node.Content) == 0 {
		// No content: an empty block parses fine and stays empty.
		return
	}
	root := node.Content[0]
	if root.Tag != "!!map" {
		// A scalar or a sequence at the top level is not a key-value
		// header.
		fm.Broken = true
		return
	}
	seen := map[string]bool{}
	var order []string
	values := map[string]any{}
	for k := 0; k+1 < len(root.Content); k += 2 {
		key := root.Content[k]
		val := root.Content[k+1]
		name := key.Value
		if seen[name] {
			fm.Broken = true
			return
		}
		seen[name] = true
		if key.Tag == "!!merge" {
			// A merge key: the aliased map's pairs expand in place,
			// keys it does not collide with first (YAML's rule). A
			// merge the expansion cannot represent falls back whole.
			if !mergeNode(val, values) {
				fm.Broken = true
				return
			}
			continue
		}
		order = append(order, name)
		values[name] = valueOf(val)
	}
	fm.Order = order
	fm.Values = values
}

// mergeNode folds the mapping a << merge value points at into m, keys
// m already holds winning (YAML's rule); a sequence of mappings folds
// in order. It answers false when the merge cannot be expressed as
// key-value pairs (a scalar target, a nested merge that fails), the
// caller's cue to fall back.
func mergeNode(alias *yaml.Node, m map[string]any) bool {
	if alias.Kind == yaml.AliasNode && alias.Alias != nil {
		alias = alias.Alias
	}
	switch alias.Tag {
	case "!!map":
		for k := 0; k+1 < len(alias.Content); k += 2 {
			key, val := alias.Content[k], alias.Content[k+1]
			if key.Tag == "!!merge" {
				if !mergeNode(val, m) {
					return false
				}
				continue
			}
			if _, ok := m[key.Value]; !ok {
				m[key.Value] = valueOf(val)
			}
		}
		return true
	case "!!seq":
		for _, item := range alias.Content {
			if !mergeNode(item, m) {
				return false
			}
		}
		return true
	}
	return false
}

// valueOf converts a YAML node to the value types the frontmatter keeps:
// strings, floats (ints too, the view formats), bools, null, lists and
// maps, the maps in source order recursively. An alias node reads the
// anchor it points at, so the row shows the anchored value.
func valueOf(node *yaml.Node) any {
	if node.Kind == yaml.AliasNode && node.Alias != nil {
		return valueOf(node.Alias)
	}
	switch node.Tag {
	case "!!str":
		return node.Value
	case "!!int", "!!float":
		var f float64
		if err := node.Decode(&f); err != nil {
			return node.Value
		}
		return f
	case "!!bool":
		var b bool
		if node.Decode(&b) != nil {
			return node.Value
		}
		return b
	case "!!null":
		return nil
	case "!!seq":
		list := make([]any, 0, len(node.Content))
		for _, item := range node.Content {
			list = append(list, valueOf(item))
		}
		return list
	case "!!map":
		m := make(map[string]any, len(node.Content)/2)
		for k := 0; k+1 < len(node.Content); k += 2 {
			key, val := node.Content[k], node.Content[k+1]
			if key.Tag == "!!merge" {
				// Nested merges fold the same way; one that cannot be
				// expressed keeps its key rather than showing wrong
				// pairs. The view collapses a nested map to {...}
				// anyway.
				if !mergeNode(val, m) {
					m["<<"] = valueOf(val)
				}
				continue
			}
			m[key.Value] = valueOf(val)
		}
		return m
	}
	return node.Value
}
