package commands

import "testing"

func TestDeltaMultiplier(t *testing.T) {
	cases := []struct {
		delta int
		want  float64
		label string
	}{
		{3, 1.5, "delta>=3: max extra reward"},
		{2, 1.3, "delta=2: strong extra reward"},
		{1, 1.15, "delta=1: slight extra reward — session harder than current state"},
		{0, 1.0, "delta=0: no adjustment"},
		{-1, 0.9, "delta=-1: slight reduction — session easier than current state"},
		{-2, 0.8, "delta<=-2: max reduction"},
		{-99, 0.8, "extreme negative: clamps to max reduction"},
		{99, 1.5, "extreme positive: clamps to max reward"},
	}
	for _, c := range cases {
		got := deltaMultiplier(c.delta)
		if got != c.want {
			t.Errorf("%s: deltaMultiplier(%d) = %.2f, want %.2f", c.label, c.delta, got, c.want)
		}
	}
}

func TestDeltaMultiplierFailurePath(t *testing.T) {
	// When a session was generated at a higher effective score than current state
	// (positive delta), the multiplier should be > 1 — rewarding harder success
	// or withholding amplified SR penalty on failure.
	//
	// Scenario: session generated at effective=0, learner regressed to -1 (no nudge).
	// delta = 0 - (-1) = +1 → multiplier = 1.15 — lesser penalty path.
	generationEffective := 0
	currentEffective := -1
	delta := generationEffective - currentEffective // +1
	m := deltaMultiplier(delta)
	if m <= 1.0 {
		t.Errorf("no-nudge regression: expected multiplier > 1.0 (lesser penalty), got %.2f", m)
	}

	// Scenario: operator nudges to challenge(+2), learner still at -1.
	// effective = (2 + -1) / 2 = +1. delta = 0 - 1 = -1 → multiplier = 0.9 — harsher penalty path.
	currentEffectiveNudged := 1
	deltaNudged := generationEffective - currentEffectiveNudged // -1
	mNudged := deltaMultiplier(deltaNudged)
	if mNudged >= 1.0 {
		t.Errorf("nudged scenario: expected multiplier < 1.0 (harsher penalty), got %.2f", mNudged)
	}

	// Nudged penalty must be strictly harsher than non-nudged.
	if mNudged >= m {
		t.Errorf("nudged multiplier (%.2f) should be less than no-nudge multiplier (%.2f)", mNudged, m)
	}
}
