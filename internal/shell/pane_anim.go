package shell

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// paneSlideDuration is the time a panel takes to reach its full width.
const paneSlideDuration = 200 * time.Millisecond

// paneSlide is the render width of one panel. Visible state is applied at
// once; only this width moves, so the persisted panel width never changes
// while the animation runs.
type paneSlide struct {
	from     float32
	value    float32
	target   float32
	elapsed  time.Duration
	duration time.Duration
	last     time.Time
	wants    bool
	seen     bool
}

func (a *paneSlide) init(v float32, now time.Time) {
	a.from, a.value, a.target = v, v, v
	a.elapsed = paneSlideDuration
	a.duration = paneSlideDuration
	a.last = now
	a.wants = v > 0
	a.seen = true
}

// retarget starts a new leg from the width now shown, so a reversal in
// mid-flight keeps continuity instead of jumping to an endpoint.
func (a *paneSlide) retarget(to float32) {
	if a.target == to {
		return
	}
	a.from, a.target = a.value, to
	a.elapsed = 0
	a.duration = paneSlideDuration
}

func (a *paneSlide) snap(to float32) {
	a.from, a.value, a.target = to, to, to
	a.elapsed = paneSlideDuration
}

func (a *paneSlide) advance(dt time.Duration) {
	if dt <= 0 || a.elapsed >= a.duration {
		return
	}
	a.elapsed = min(a.elapsed+dt, a.duration)
	t := float32(a.elapsed) / float32(a.duration)
	a.value = a.from + (a.target-a.from)*ui.EaseOut(t)
	if a.elapsed >= a.duration {
		a.value = a.target
	}
}

func (a *paneSlide) moving() bool {
	return a.seen && a.elapsed < a.duration
}
