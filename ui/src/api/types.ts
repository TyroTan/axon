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
