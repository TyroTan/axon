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
}

export interface GetSessionQuestionsResult {
  track_id: string
  session_number: number
  questions: Question[]
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
