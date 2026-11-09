package mermaid

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"math"
)

// MaxScale caps the raster scale. Pixels grow by the square of the
// scale, so a zoom of 3 already costs 9x the intrinsic area for a
// sharpness the eye can barely tell from 2.5x; past this the view
// clamps and the display stretches the last stretch, mildly soft.
const MaxScale = 2.5

// Key hashes src, theme and scale into the cache key: a sha256 over
// length-prefixed parts, so no split of the inputs reads as another.
// Cache and the view's bitmap cache share it.
func Key(src string, theme Theme, scale float64) string {
	h := sha256.New()
	var n [8]byte
	binary.LittleEndian.PutUint64(n[:], uint64(len(src)))
	h.Write(n[:])
	h.Write([]byte(src))
	binary.LittleEndian.PutUint64(n[:], uint64(len(theme)))
	h.Write(n[:])
	h.Write([]byte(theme))
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(scale))
	h.Write(b[:])
	return hex.EncodeToString(h.Sum(nil))
}
