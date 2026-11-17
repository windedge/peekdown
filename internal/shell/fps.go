package shell

import (
	"fmt"
	"math"
	"slices"
	"time"
)

// The readout's constants. fpsWindow is how far back the rate counts: a
// second of frames smooths the jitter out and still shows a stall that
// lasts that long. fpsInterval is how often the text changes, as rebuilding
// the label every frame would cost the frames it measures. fpsStallFactor is
// the multiple of the window's median gap that marks a dropped frame: 2 is
// 33ms on a 60Hz panel and 14ms on a 144Hz one.
const (
	fpsWindow      = time.Second
	fpsInterval    = 500 * time.Millisecond
	fpsStallFactor = 2
	// fpsWidth is the readout's box, wide enough for "144 fps (120ms)", so
	// the zoom label beside it never slides as the text changes.
	fpsWidth = 104
)

// fpsMeter measures what the window really draws: the frame clock's samples
// over the last second, the rate they came at, and the longest gap between
// two of them, which is where a dropped frame shows.
type fpsMeter struct {
	// frames are the window's frame times, oldest first, with the newest one
	// before the window kept as the anchor of the first gap: a stall that
	// outlasts the window still reads as one.
	frames    []time.Time
	published time.Time
	text      string
	fps       int
	worst     time.Duration
	stalled   bool
}

// sample records a frame at now and, every fpsInterval, recomputes the
// readout.
func (m *fpsMeter) sample(now time.Time) {
	// A frame that handled an event builds the view again at the same time:
	// counting it twice would inflate the rate and put zeros in the gaps.
	if n := len(m.frames); n > 0 && !now.After(m.frames[n-1]) {
		return
	}
	m.frames = append(m.frames, now)
	m.trim(now)
	if m.text != "" && now.Sub(m.published) < fpsInterval {
		return
	}
	m.published = now
	m.measure()
}

// trim drops the frame times the window has left behind, keeping the newest
// one before it: the gap that spans the window's edge is the stall the rate
// must still show.
func (m *fpsMeter) trim(now time.Time) {
	cut := now.Add(-fpsWindow)
	i := 0
	for i < len(m.frames) && m.frames[i].Before(cut) {
		i++
	}
	if i > 1 {
		m.frames = append(m.frames[:0], m.frames[i-1:]...)
	}
}

// measure counts the window's frames per second and finds its longest gap.
// The median gap stands in for the display's refresh interval, so the stall
// threshold follows the panel rather than a hard-coded 60Hz.
func (m *fpsMeter) measure() {
	if len(m.frames) < 2 {
		return
	}
	gaps := make([]time.Duration, len(m.frames)-1)
	for i := range gaps {
		gaps[i] = m.frames[i+1].Sub(m.frames[i])
	}
	span := m.frames[len(m.frames)-1].Sub(m.frames[0])
	m.fps = int(math.Round(float64(len(gaps)) / span.Seconds()))
	m.worst = slices.Max(gaps)
	m.stalled = m.worst > fpsStallFactor*medianGap(gaps)
	m.text = fpsLabel(m.fps)
}

// label is the readout's text, empty until the first measurement.
func (m *fpsMeter) label() string { return m.text }

// reset forgets the window as the readout goes: the next time it shows, it
// starts from no history rather than from a gap that was not a stall.
func (m *fpsMeter) reset() {
	if len(m.frames) == 0 && m.text == "" {
		return
	}
	*m = fpsMeter{}
}

// fpsLabel formats the readout: just the rate. The meter keeps the longest
// gap and the stall flag, so a readout can show them again if it has to.
func fpsLabel(fps int) string {
	return fmt.Sprintf("%d fps", fps)
}

// medianGap is the middle of the gaps as sorted.
func medianGap(gaps []time.Duration) time.Duration {
	sorted := slices.Clone(gaps)
	slices.Sort(sorted)
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
