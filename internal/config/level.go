package config

import "os"

// LevelConfig defines the generation constraints for a named difficulty level.
// BloomDelta is added to each concept's bloom_current when targeting questions.
// DifficultyFloor is the minimum difficulty_estimate the LLM must assign.
// nil pointer fields mean "use adaptive default — do not constrain".
type LevelConfig struct {
	Name              string
	BloomDelta        int
	DifficultyFloor   float64
	CrossBranchWeight *float64 // nil = adaptive
	FormatBias        string   // "", "mcq_only", "free_text_heavy", "design_heavy"
	TimeMultiplier    float64  // 1.0 = no change
}

func ptr(f float64) *float64 { return &f }

// Levels is the canonical difficulty matrix.
// Operator sets AXON_LEVEL_OVERRIDE=<name>; unset defaults to "default".
var Levels = map[string]LevelConfig{
	"recall": {
		Name:              "recall",
		BloomDelta:        -1,
		DifficultyFloor:   0.0,
		CrossBranchWeight: ptr(0.0),
		FormatBias:        "mcq_only",
		TimeMultiplier:    1.5,
	},
	"easy": {
		Name:            "easy",
		BloomDelta:      0,
		DifficultyFloor: 0.15,
		TimeMultiplier:  1.2,
	},
	"default": {
		Name:            "default",
		BloomDelta:      0,
		DifficultyFloor: 0.4,
		TimeMultiplier:  1.0,
	},
	"medium": {
		Name:            "medium",
		BloomDelta:      0,
		DifficultyFloor: 0.65,
		TimeMultiplier:  1.0,
	},
	"challenge": {
		Name:              "challenge",
		BloomDelta:        1,
		DifficultyFloor:   0.5,
		CrossBranchWeight: ptr(0.4),
		FormatBias:        "free_text_heavy",
		TimeMultiplier:    0.9,
	},
	"intense": {
		Name:              "intense",
		BloomDelta:        1,
		DifficultyFloor:   0.75,
		CrossBranchWeight: ptr(0.6),
		FormatBias:        "free_text_heavy",
		TimeMultiplier:    0.8,
	},
	"extreme": {
		Name:              "extreme",
		BloomDelta:        2,
		DifficultyFloor:   0.85,
		CrossBranchWeight: ptr(1.0),
		FormatBias:        "design_heavy",
		TimeMultiplier:    0.7,
	},
}

// LevelOrder is the canonical display order (easiest → hardest).
var LevelOrder = []string{"recall", "easy", "default", "medium", "challenge", "intense", "extreme"}

// ActiveLevel reads AXON_LEVEL_OVERRIDE and returns the matching config.
// Falls back to "default" for unset or unknown values.
func ActiveLevel() LevelConfig {
	name := os.Getenv("AXON_LEVEL_OVERRIDE")
	if l, ok := Levels[name]; ok {
		return l
	}
	return Levels["default"]
}
