package config

import "testing"

func TestEffectiveScore(t *testing.T) {
	cases := []struct {
		config, learner, want int
		label                 string
	}{
		// Symmetric neutral
		{0, 0, 0, "both neutral → default"},
		// Simple averages
		{2, 0, 1, "challenge + neutral → medium"},
		{0, -2, -1, "neutral + easy → easy"},
		{5, -3, 1, "extreme + recall → medium (level out)"},
		// Regression scenario: session generated at 0, learner regressed to -1
		{0, -1, -1, "neutral config + regressed learner → easy"},
		// Operator nudges medium to offset regression
		{1, -1, 0, "medium + regressed → back to default"},
		// Operator over-nudges to challenge — delta goes negative
		{2, -1, 1, "challenge + regressed → medium"},
		// Clamp floor
		{-3, -3, -3, "clamp at floor"},
		// Clamp ceiling
		{5, 5, 5, "clamp at ceiling"},
		// Odd sums round away from zero
		{1, 0, 1, "sum=1 rounds up to 1"},
		{-1, 0, -1, "sum=-1 rounds down to -1"},
	}
	for _, c := range cases {
		got := EffectiveScore(c.config, c.learner)
		if got != c.want {
			t.Errorf("%s: EffectiveScore(%d, %d) = %d, want %d",
				c.label, c.config, c.learner, got, c.want)
		}
	}
}

func TestNearestLevel(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{0, "default"},
		{-3, "recall"},
		{-2, "easy"},
		{1, "medium"},
		{2, "challenge"},
		{3, "intense"},
		{5, "extreme"},
		// Between values — nearest wins
		{-1, "easy"},  // equidistant between easy(-2) and default(0) — easy is closer to -1
		{4, "intense"}, // equidistant between intense(3) and extreme(5) — intense is closer to 4
	}
	for _, c := range cases {
		got := NearestLevel(c.score)
		if got.Name != c.want {
			t.Errorf("NearestLevel(%d) = %q, want %q", c.score, got.Name, c.want)
		}
	}
}

func TestDeltaScenarios(t *testing.T) {
	// Scenario A: session generated at effective=0, learner regresses (no nudge)
	// → delta positive → lesser penalty territory
	generationEffective := 0
	currentConfig := 0   // no nudge
	currentLearner := -1 // regressed after fail
	currentEffective := EffectiveScore(currentConfig, currentLearner)
	delta := generationEffective - currentEffective
	if delta <= 0 {
		t.Errorf("scenario A: expected positive delta (session harder than current state), got %d", delta)
	}

	// Scenario B: same snapshot, operator nudges to challenge(+2) before returning
	// → delta negative → harsher penalty territory
	currentConfigNudged := 2 // challenge
	currentEffectiveNudged := EffectiveScore(currentConfigNudged, currentLearner)
	deltaNudged := generationEffective - currentEffectiveNudged
	if deltaNudged >= 0 {
		t.Errorf("scenario B: expected negative delta (session easier than nudged state), got %d", deltaNudged)
	}

	// The nudged scenario must be harsher than the non-nudged one
	if deltaNudged >= delta {
		t.Errorf("nudged delta (%d) should be less than non-nudged delta (%d)", deltaNudged, delta)
	}
}

func TestLevelSpineOrdering(t *testing.T) {
	// Scores must be strictly increasing in LevelOrder
	prev := -999
	for _, name := range LevelOrder {
		l, ok := Levels[name]
		if !ok {
			t.Fatalf("level %q in LevelOrder but not in Levels map", name)
		}
		if l.Score <= prev {
			t.Errorf("level %q score %d not strictly greater than previous %d", name, l.Score, prev)
		}
		prev = l.Score
	}
}
