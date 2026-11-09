// Package mermaid renders Mermaid diagrams to images for the viewer, with
// an LRU cache. It has no dependency on the UI layer: on error the view
// falls back to showing the raw source block.
package mermaid

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"

	"github.com/windedge/peekdown/internal/mermaid/raster"
	mermaid "github.com/zkrebbekx/go-mermaid"
)

// Theme selects the diagram color palette. The light app theme maps to
// ThemeDefault, the dark app theme to ThemeDark.
type Theme string

const (
	ThemeDefault Theme = "default"
	ThemeDark    Theme = "dark"
)

// prefixError wraps err with a single "mermaid: ": the library's errors
// carry that prefix already, so its message is trimmed before wrapping,
// and Unwrap keeps errors.Is matching the go-mermaid sentinels.
type prefixError struct{ err error }

func (e *prefixError) Error() string {
	return "mermaid: " + strings.TrimPrefix(e.err.Error(), "mermaid: ")
}

func (e *prefixError) Unwrap() error { return e.err }

// Render rasterizes a mermaid diagram to an image. scale 1.0 is the SVG's
// intrinsic size; callers pass at least 2.0 on high-DPI displays, capped
// by MaxScale. Library errors (syntax, unsupported diagram) keep matching
// via errors.Is on the go-mermaid sentinels (ErrParse, ErrUnsupported,
// ...) and carry exactly one "mermaid: " prefix.
func Render(src string, theme Theme, scale float64) (image.Image, error) {
	if strings.TrimSpace(src) == "" {
		return nil, errors.New("mermaid: empty diagram")
	}

	ensureCJKFonts()

	data, err := raster.PNG(src, scale, mermaid.WithTheme(mermaid.Theme(theme)))
	if err != nil {
		return nil, &prefixError{err}
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("mermaid: decode png: %w", err)
	}
	return img, nil
}
