// Domain types — mirror of internal/domain/types.go

export interface SpacedRepetition {
  next_review: string | null
  interval_days: number
  consecutive_correct: number
}

export interface Concept {
  index: number
  name: string
  branch: string
  description: string
  bloom_current: number
  bloom_target: number
  is_bottleneck: boolean
  prerequisite_indexes: number[]
  unlocks_indexes: number[]
  spaced_repetition: SpacedRepetition
}

export interface ConceptMap {
  track: string
  major_branches: string[]
  generated_at: string
  note?: string
  concepts: Concept[]
}

export interface Track {
  id: string
  parent_id: string
  major_branches: string[]
  children?: Track[]
  created_at: string
}

export interface Session {
  track_id: string
  number: number
  created_at: string
  has_profile: boolean
  has_questions: boolean
  has_responses: boolean
  has_evaluations: boolean
  has_synthesis: boolean
}

// API response shapes
export interface ListTracksResult {
  tracks: Track[]
}

export interface GetTrackResult {
  track: Track
  concept_map: ConceptMap
  sessions: Session[]
}

export interface GetTrackContextResult {
  track_id: string
  files: Record<string, string> // filename → content
}

export interface DuplicateTrackResult {
  new_track_id: string
}

export interface CreateSessionResult {
  session_number: number
  shard_id?: string
}

export interface SplitShard {
  id: string
  status: 'pending' | 'approved'
  token_count: number
  files: string[]
}

export interface SplitPlan {
  status: string
  total_tokens: number
  soft_limit: number
  generated_at: string
  shards: SplitShard[]
}

export interface GetSplitPlanResult {
  split_plan: SplitPlan | null
}

export interface MetaSynthesis {
  track_id: string
  date: string
  generation_id: string
  sessions_aggregated: number[]
  shards_aggregated: string[]
  concept_map_updates: ConceptMapUpdate[]
  learner_summary: string
  applied: boolean
}

export interface GetMetaSynthesisResult {
  track_id: string
  meta_synthesis: MetaSynthesis | null
}

export interface MetaSynthesisReadinessResult {
  missing_shards: string[]
  ready: boolean
}

export interface GetSessionQuestionsResult {
  track_id: string
  session_number: number
  questions: Question[]
}

export interface Response {
  question_id: string
  generation_id: string
  selected_answer: string
  confidence: number   // 1–5
  explanation: string
  time_seconds: number
}

export interface ExplanationSubscores {
  mechanism_accuracy: number
  terminology_precision: number
  edge_case_awareness: number
  generalization_quality: number
}

export interface Evaluation {
  question_id: string
  generation_id: string
  concept_indexes: number[]
  correctness: number          // 0.0–1.0
  explanation_score: number
  explanation_subscores: ExplanationSubscores
  brier_contribution: number
  calibration_flag: string | null
  error_taxonomy: string | null
  misconception_identified: string | null
  time_signal: string
  bloom_level_demonstrated: number
  evaluator_notes: string
  feedback_for_learner: string
}

export interface GetSessionResponsesResult {
  track_id: string
  session_number: number
  responses: Response[]
}

export interface GetSessionEvaluationsResult {
  track_id: string
  session_number: number
  evaluations: Evaluation[]
}

export interface SpacedRepetition {
  next_review: string | null
  interval_days: number
  consecutive_correct: number
}

export interface ConceptMapUpdate {
  concept_index: number
  bloom_current_before: number
  bloom_current_after: number
  spaced_repetition: SpacedRepetition
}

export interface Synthesis {
  session_number: number
  session_date: string
  generation_id: string
  concept_map_updates: ConceptMapUpdate[]
  learner_summary: string
  applied: boolean
}

export interface GetSynthesisResult {
  track_id: string
  session_number: number
  synthesis: Synthesis | null
}

export interface Question {
  id: string
  generation_id: string
  concept_indexes: number[]
  bloom_level: number
  bloom_label: string
  question: string
  format: 'mcq' | 'free_text' | 'scenario_mcq' | 'design'
  options?: Record<string, string>
  correct?: string
  correct_explanation: string
  distractor_explanations?: Record<string, string>
  is_cross_branch: boolean
  difficulty_estimate: number
  spaced_repetition_concept_id: number | null
  expected_time_seconds: number
  requires_explanation: boolean
}
