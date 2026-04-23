package domain

import "time"

// ─── Track ───────────────────────────────────────────────────────────────────

// Track represents one learning track folder (e.g. "track_1", "track_1_2").
type Track struct {
	ID          string    `json:"id"`                    // e.g. "track_1"
	ParentID    string    `json:"parent_id"`             // "" if root track
	Branches    []string  `json:"major_branches"`
	Children    []Track   `json:"children,omitempty"`    // populated by ListTracks query
	CreatedAt   time.Time `json:"created_at"`
	// Composite track fields — set when track_meta.json exists.
	IsComposite bool     `json:"is_composite,omitempty"` // true if merged from multiple sources
	SourceIDs   []string `json:"source_ids,omitempty"`   // source track IDs used at merge time
}

// TrackMeta is persisted as track_meta.json for tracks that carry extra provenance.
// It is always optional — tracks without it are treated as plain lineage tracks.
type TrackMeta struct {
	IsComposite bool     `json:"is_composite,omitempty"`
	SourceIDs   []string `json:"source_ids,omitempty"`
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

	// v2 dual state model — exploration state (never used in scoring).
	// ExplorationUnlocked: concept is accessible for question selection even if
	// prerequisites are not yet demonstrated (provisional unlock).
	// AspirationCount: number of times learner has engaged above their evidence
	// floor on this concept. Reaches threshold (default 2) → auto-unlock.
	ExplorationUnlocked bool `json:"exploration_unlocked,omitempty"`
	AspirationCount     int  `json:"aspiration_count,omitempty"`

	// v2 inquiry quality — E9.
	// InquiryPrecision: session-averaged score (0.0–1.0) of how well the learner's
	// own questions in tutoring threads targeted mechanism over symptom.
	// A learner who answers well but questions poorly is distinguishable from one
	// who does both. Updated by ApplySynthesis when Inquiry Patterns section exists.
	InquiryPrecision float64 `json:"inquiry_precision,omitempty"`
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

	// v2 composite learner state — written by DetectLearnerStateCommand.
	CompositeState      string  `json:"composite_state,omitempty"`       // e.g. "Flow", "Plateau"
	StateConfidence     float64 `json:"state_confidence,omitempty"`      // 0.0–1.0
	PerceivedTrustProxy float64 `json:"perceived_trust_proxy,omitempty"` // 0.0–1.0
	NudgeSuggestion     string  `json:"nudge_suggestion,omitempty"`      // pre-session one-liner
	NudgeOverride       bool    `json:"nudge_override,omitempty"`        // true if learner dismissed

	// Path-aware scoring — written by ApplySynthesisCommand.
	// LearnerSignal: signal derived from this session's outcomes (-3..+3).
	// DeltaMultiplier: ratio of (generation effective score) vs (answer-time effective score).
	// A session answered harder than it was generated for → multiplier > 1.0 → extra bloom reward.
	LearnerSignal   int     `json:"learner_signal,omitempty"`
	DeltaMultiplier float64 `json:"delta_multiplier,omitempty"`
}

// ─── Track state ─────────────────────────────────────────────────────────────

// TrackState is persisted as track_state.json at the track root.
// Tracks consecutive failure counts for adaptive signal decay.
// ConsecutiveFails: sessions with avg correctness below failThreshold in a row.
// ConsecutiveFailsEasyDelta: subset where session was easier than current state
// (delta < 0) — this is a stronger signal of genuine struggle.
// Both reset to 0 on a passing session.
type TrackState struct {
	ConsecutiveFails          int `json:"consecutive_fails"`
	ConsecutiveFailsEasyDelta int `json:"consecutive_fails_easy_delta"`
}

// ─── Path tracking ───────────────────────────────────────────────────────────

// StateSnapshot captures the two-sided difficulty state at a point in time.
// Written once at question-generation time; never mutated afterward.
// AxonConfigScore: numeric position of AXON_LEVEL_OVERRIDE on the difficulty spine.
// LearnerSignal: derived from the most recent synthesis composite_state (-3..+3).
// EffectiveScore: combined value used for prompt injection and delta computation.
type StateSnapshot struct {
	AxonConfigScore int    `json:"axon_config_score"`
	LearnerSignal   int    `json:"learner_signal"`
	EffectiveScore  int    `json:"effective_score"`
	LevelName       string `json:"level_name"` // human label of the resolved level
}

// PathEntry is one append-only record in learner_path.jsonl.
// Event: "generated" | "answered" | "evaluated".
type PathEntry struct {
	TrackID         string    `json:"track_id"`
	SessionNum      int       `json:"session_num"`
	Event           string    `json:"event"`
	VisitedAt       time.Time `json:"visited_at"`
	AxonConfigScore int       `json:"axon_config_score"`
	LearnerSignal   int       `json:"learner_signal"`
	EffectiveScore  int       `json:"effective_score"`
}

// ─── Session Metadata ─────────────────────────────────────────────────────────

// SessionMetadata is written to 00_metadata.json at session creation.
// ShardID is empty for sessions that use the full (unsplit) context.
// ClaudeSessionID is set after the first LLM call when --resume is wired (U2B).
// AccumulatedInputTokens tracks the running total of tokens sent across all LLM
// calls in this session (used by prompt-preview to show "tokens consumed so far").
// StateSnapshot is written at question-generation time and never mutated.
type SessionMetadata struct {
	ShardID                string        `json:"shard_id"`
	ClaudeSessionID        string        `json:"claude_session_id,omitempty"`
	AccumulatedInputTokens int           `json:"accumulated_input_tokens,omitempty"`
	StateSnapshot          *StateSnapshot `json:"state_snapshot,omitempty"`
}

// ─── Thread ───────────────────────────────────────────────────────────────────

// ThreadMessageMeta records observability info for an assistant turn:
// what call mode was used, how many tokens were sent, how many RAG chunks
// were injected. This lets the user verify per-message context management.
type ThreadMessageMeta struct {
	CallMode          string `json:"call_mode"`           // "fresh" | "resumed" | "seeded"
	InputTokensSent   int    `json:"input_tokens_sent"`   // 0 for seeded, actual for LLM calls
	ContextChunksUsed int    `json:"context_chunks_used"` // RAG chunks injected (fresh calls)
	ClaudeSessionID   string `json:"claude_session_id,omitempty"` // session ID returned by CLI
}

// ThreadMessage is one turn in a per-question follow-up conversation.
type ThreadMessage struct {
	Role      string             `json:"role"`       // "user" | "assistant"
	Content   string             `json:"content"`
	Timestamp time.Time          `json:"ts"`
	Meta      *ThreadMessageMeta `json:"meta,omitempty"` // only on assistant turns
}

// Thread is the full conversation history for one question.
// Written to sessions/session_NNN/threads/q_ID.json.
type Thread struct {
	QuestionID             string          `json:"question_id"`
	ClaudeSessionID        string          `json:"claude_session_id,omitempty"`
	AccumulatedInputTokens int             `json:"accumulated_input_tokens,omitempty"`
	Messages               []ThreadMessage `json:"messages"`
}

// ─── Meta-Synthesis ───────────────────────────────────────────────────────────

// MetaSynthesis is the track-level result written after all shards have been
// evaluated. It aggregates bloom updates across every shard session.
// Written to track_N/meta_synthesis.json.
type MetaSynthesis struct {
	TrackID           string             `json:"track_id"`
	Date              string             `json:"date"`
	GenerationID      string             `json:"generation_id"`
	SessionsAggregated []int             `json:"sessions_aggregated"` // session numbers included
	ShardsAggregated  []string           `json:"shards_aggregated"`   // shard IDs included
	ConceptMapUpdates []ConceptMapUpdate `json:"concept_map_updates"`
	LearnerSummary    string             `json:"learner_summary"`
	Applied           bool               `json:"applied"`
}

// ─── Track Origin ─────────────────────────────────────────────────────────────

// TrackOriginConcept is a point-in-time snapshot of one concept's bloom state.
// Index is always relative to THIS track's concept map (post-remap for merges).
// BloomCurrent is the effective cascaded floor at copy time (GetEffectiveConceptMap output).
type TrackOriginConcept struct {
	Index        int    `json:"index"`
	Name         string `json:"name"`
	BloomCurrent int    `json:"bloom_current"`
	BloomTarget  int    `json:"bloom_target"`
}

// TrackOriginSourceFloor is the pre-remap concept bloom state for one source track.
// Used in merge events to preserve per-source attribution before index remapping.
// Index is the concept's original index in the source track's concept map.
type TrackOriginSourceFloor struct {
	SourceID string               `json:"source_id"`
	Concepts []TrackOriginConcept `json:"concepts"` // original indexes, not remapped
}

// TrackOrigin is written as _track_origin.json at the track root after every
// fork or merge. The underscore prefix keeps it invisible to the question generator.
//
// ConceptFloor — concepts in THIS track's index space (post-remap).
// PerSourceFloors — for merge only: each source's concept bloom state at its
//   original indexes before remapping. Enables synergy detection by comparing
//   bloom levels for the same semantic concept across sources.
type TrackOrigin struct {
	EventType       string                   `json:"event_type"`                  // "fork" | "merge"
	SourceIDs       []string                 `json:"source_ids"`                  // one for fork, many for merge
	CreatedAt       string                   `json:"created_at"`                  // RFC3339
	ConceptFloor    []TrackOriginConcept     `json:"concept_floor"`               // merged-map index space
	PerSourceFloors []TrackOriginSourceFloor `json:"per_source_floors,omitempty"` // merge only
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

// ─── Conversation ─────────────────────────────────────────────────────────────

// ConversationMessageMeta captures provenance for one assistant reply.
type ConversationMessageMeta struct {
	CallMode        string   `json:"call_mode"`                   // "fresh" | "resumed"
	InputTokensSent int      `json:"input_tokens_sent"`
	ContextChunks   int      `json:"context_chunks_used"`
	ClaudeSessionID string   `json:"claude_session_id,omitempty"`
	TrackIDsLoaded  []string `json:"track_ids_loaded,omitempty"`  // which tracks' chunks were searched
}

// ConversationMessage is one turn (user or assistant).
type ConversationMessage struct {
	Role    string                   `json:"role"` // "user" | "assistant"
	Content string                   `json:"content"`
	Ts      time.Time                `json:"ts"`
	Meta    *ConversationMessageMeta `json:"meta,omitempty"`
}

// Conversation is a free-form, multi-track conversation stored as a flat document.
// It lives at experimentsDir/conversations/{ID}.json — not under any single track.
type Conversation struct {
	ID                     string                `json:"id"`
	Title                  string                `json:"title,omitempty"`
	TrackIDs               []string              `json:"track_ids"`                // context sources (ordered; first is primary)
	AdHocText              string                `json:"adhoc_text,omitempty"`     // extra inline context injected mid-conversation
	CreatedAt              time.Time             `json:"created_at"`
	UpdatedAt              time.Time             `json:"updated_at"`
	ClaudeSessionID        string                `json:"claude_session_id,omitempty"`
	AccumulatedInputTokens int                   `json:"accumulated_input_tokens"`
	Messages               []ConversationMessage `json:"messages"`
}

// ─── Conversation Index ───────────────────────────────────────────────────────

// ConversationPlyIndex is a per-message entry in the structured index.
type ConversationPlyIndex struct {
	Turn      int      `json:"turn"`       // 0-based message index
	Role      string   `json:"role"`
	Summary   string   `json:"summary"`    // LLM-generated summary of this message
	Topics    []string `json:"topics"`
	KeyPoints []string `json:"key_points,omitempty"` // assistant turns only
	Tokens    int      `json:"tokens"`     // naive token estimate of message content
}

// ConversationIndex is the structured, queryable artifact produced by IndexConversation.
// Stored as experimentsDir/conversations/{ID}.index.json.
// Designed as a self-contained "document" — readable standalone like a MongoDB document.
type ConversationIndex struct {
	ConversationID          string                 `json:"conversation_id"`
	TrackIDs                []string               `json:"track_ids"`
	Title                   string                 `json:"title,omitempty"`
	GenerationID            string                 `json:"generation_id"`
	IndexedAt               time.Time              `json:"indexed_at"`
	TurnCount               int                    `json:"turn_count"`
	Summary                 string                 `json:"summary"`              // whole-conversation summary
	Topics                  []string               `json:"topics"`               // top-level topics discussed
	KeyDecisions            []string               `json:"key_decisions"`        // conclusions or commitments made
	OpenQuestions           []string               `json:"open_questions"`       // unresolved threads
	ConceptIndexesReferenced []int                 `json:"concept_indexes_referenced"` // concept map indexes mentioned
	PlyIndex                []ConversationPlyIndex `json:"ply_index"`            // per-message detail
}
