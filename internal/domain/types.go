package domain

import "time"

// ─── Track ───────────────────────────────────────────────────────────────────

// Track represents one learning track folder (e.g. "track_1", "track_1_2").
type Track struct {
	ID       string   `json:"id"`        // e.g. "track_1"
	ParentID string   `json:"parent_id"` // "" if root track
	Branches []string `json:"major_branches"`
	Children []Track  `json:"children,omitempty"` // populated by ListTracks query
	CreatedAt time.Time `json:"created_at"`
}

// ─── Concept ──────────────────────────────────────────────────────────────────

type SpacedRepetition struct {
	NextReview        *time.Time `json:"next_review"`
	IntervalDays      int        `json:"interval_days"`
	ConsecutiveCorrect int       `json:"consecutive_correct"`
}

type Concept struct {
	Index              int              `json:"index"`
	Name               string           `json:"name"`
	Branch             string           `json:"branch"`
	Description        string           `json:"description"`
	BloomCurrent       int              `json:"bloom_current"`
	BloomTarget        int              `json:"bloom_target"`
	IsBottleneck       bool             `json:"is_bottleneck"`
	PrerequisiteIndexes []int           `json:"prerequisite_indexes"`
	UnlocksIndexes     []int            `json:"unlocks_indexes"`
	SpacedRepetition   SpacedRepetition `json:"spaced_repetition"`
}

// ─── Concept Map ─────────────────────────────────────────────────────────────

type ConceptMap struct {
	Track         string    `json:"track"`
	MajorBranches []string  `json:"major_branches"`
	GeneratedAt   string    `json:"generated_at"`
	Note          string    `json:"note,omitempty"`
	Concepts      []Concept `json:"concepts"`
}

// ─── Session ─────────────────────────────────────────────────────────────────

type Session struct {
	TrackID    string    `json:"track_id"`
	Number     int       `json:"number"`
	CreatedAt  time.Time `json:"created_at"`
	HasProfile bool      `json:"has_profile"`
	HasQuestions bool    `json:"has_questions"`
	HasResponses bool    `json:"has_responses"`
	HasEvaluations bool  `json:"has_evaluations"`
	HasSynthesis bool    `json:"has_synthesis"`
}

// ─── Question ─────────────────────────────────────────────────────────────────

type QuestionFormat string

const (
	FormatMCQ         QuestionFormat = "mcq"
	FormatFreeText    QuestionFormat = "free_text"
	FormatScenarioMCQ QuestionFormat = "scenario_mcq"
	FormatDesign      QuestionFormat = "design"
)

type Question struct {
	ID                      string            `json:"id"`
	GenerationID            string            `json:"generation_id"`
	ConceptIndexes          []int             `json:"concept_indexes"`
	BloomLevel              int               `json:"bloom_level"`
	BloomLabel              string            `json:"bloom_label"`
	Question                string            `json:"question"`
	Format                  QuestionFormat    `json:"format"`
	Options                 map[string]string `json:"options,omitempty"`
	Correct                 string            `json:"correct,omitempty"`
	CorrectExplanation      string            `json:"correct_explanation"`
	DistractorExplanations  map[string]string `json:"distractor_explanations,omitempty"`
	IsCrossBranch           bool              `json:"is_cross_branch"`
	DifficultyEstimate      float64           `json:"difficulty_estimate"`
	SpacedRepetitionConceptID *int            `json:"spaced_repetition_concept_id"`
	ExpectedTimeSeconds     int               `json:"expected_time_seconds"`
	RequiresExplanation     bool              `json:"requires_explanation"`
}

// ─── Response ─────────────────────────────────────────────────────────────────

type Response struct {
	QuestionID     string `json:"question_id"`
	GenerationID   string `json:"generation_id"`
	SelectedAnswer string `json:"selected_answer"`
	Confidence     int    `json:"confidence"` // 1–5
	Explanation    string `json:"explanation"`
	TimeSeconds    int    `json:"time_seconds"`
}

// ─── Evaluation ───────────────────────────────────────────────────────────────

type ExplanationSubscores struct {
	MechanismAccuracy    int `json:"mechanism_accuracy"`
	TerminologyPrecision int `json:"terminology_precision"`
	EdgeCaseAwareness    int `json:"edge_case_awareness"`
	GeneralizationQuality int `json:"generalization_quality"`
}

type Evaluation struct {
	QuestionID            string               `json:"question_id"`
	GenerationID          string               `json:"generation_id"`
	ConceptIndexes        []int                `json:"concept_indexes"`
	Correctness           float64              `json:"correctness"`
	ExplanationScore      float64              `json:"explanation_score"`
	ExplanationSubscores  ExplanationSubscores `json:"explanation_subscores"`
	BrierContribution     float64              `json:"brier_contribution"`
	CalibrationFlag       *string              `json:"calibration_flag"`
	ErrorTaxonomy         *string              `json:"error_taxonomy"`
	MisconceptionIdentified *string            `json:"misconception_identified"`
	TimeSignal            string               `json:"time_signal"`
	BloomLevelDemonstrated int                 `json:"bloom_level_demonstrated"`
	EvaluatorNotes        string               `json:"evaluator_notes"`
	FeedbackForLearner    string               `json:"feedback_for_learner"`
}

// ─── Concept Map Update (from synthesis) ──────────────────────────────────────

type ConceptMapUpdate struct {
	ConceptIndex      int              `json:"concept_index"`
	BloomCurrentBefore int             `json:"bloom_current_before"`
	BloomCurrentAfter  int             `json:"bloom_current_after"`
	SpacedRepetition  SpacedRepetition `json:"spaced_repetition"`
}

// ─── Synthesis ────────────────────────────────────────────────────────────────

type Synthesis struct {
	SessionNumber    int                `json:"session_number"`
	SessionDate      string             `json:"session_date"`
	GenerationID     string             `json:"generation_id"`
	ConceptMapUpdates []ConceptMapUpdate `json:"concept_map_updates"`
	LearnerSummary   string             `json:"learner_summary"`
	Applied          bool               `json:"applied"` // true after ApplySynthesisCommand
}

// ─── Steer Intent ─────────────────────────────────────────────────────────────

type SteerDirection string

const (
	SteerSlightlyHarder      SteerDirection = "slightly_harder"
	SteerSignificantlyHarder SteerDirection = "significantly_harder"
	SteerSlightlyEasier      SteerDirection = "slightly_easier"
	SteerSignificantlyEasier SteerDirection = "significantly_easier"
	SteerFocusConcept        SteerDirection = "focus_concept"
	SteerFewerBranch         SteerDirection = "fewer_branch"
)

type SteerIntent struct {
	Direction    SteerDirection `json:"direction"`
	ConceptIndex *int           `json:"concept_index,omitempty"` // for focus_concept
	Branch       *string        `json:"branch,omitempty"`        // for fewer_branch
	Note         string         `json:"note,omitempty"`
}
