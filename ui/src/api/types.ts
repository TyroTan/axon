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
  is_composite?: boolean
  source_ids?: string[]
}

export interface MergeTracksResult {
  new_track_id: string
  file_count: number
  total_concepts: number
}

// ─── Conversations ────────────────────────────────────────────────────────────

export interface ConversationMessageMeta {
  call_mode: 'fresh' | 'resumed'
  input_tokens_sent: number
  context_chunks_used: number
  claude_session_id?: string
  track_ids_loaded?: string[]
}

export interface ConversationMessage {
  role: 'user' | 'assistant'
  content: string
  ts: string
  meta?: ConversationMessageMeta
}

export interface Conversation {
  id: string
  title?: string
  track_ids: string[]
  adhoc_text?: string
  created_at: string
  updated_at: string
  claude_session_id?: string
  accumulated_input_tokens: number
  messages: ConversationMessage[]
}

export interface ListConversationsResult {
  conversations: Conversation[]
}

export interface GetConversationResult {
  conversation: Conversation
  index?: ConversationIndex
}

// ─── Conversation Index ───────────────────────────────────────────────────────

export interface ConversationPlyIndex {
  turn: number
  role: 'user' | 'assistant'
  summary: string
  topics: string[]
  key_points?: string[]
  tokens: number
}

export interface ConversationIndex {
  conversation_id: string
  track_ids: string[]
  title?: string
  generation_id: string
  indexed_at: string
  turn_count: number
  summary: string
  topics: string[]
  key_decisions: string[]
  open_questions: string[]
  concept_indexes_referenced: number[]
  ply_index: ConversationPlyIndex[]
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

export interface CreateTrackResult {
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

export interface ContextFileTokens {
  filename: string
  tokens: number
  source_track_id: string
}

export interface GetContextTokensResult {
  track_id: string
  files: ContextFileTokens[]
  total_tokens: number
  file_count: number
}

export interface PromptPreviewResult {
  track_id: string
  session_number: number
  shard_id: string
  // Context-management / resume awareness
  call_mode: 'fresh' | 'resumed'
  claude_session_id: string
  accumulated_input_tokens: number
  tokens_to_send: number
  tokens_saved_by_resume: number
  // Prompt content
  system_prompt: string
  user_prompt: string
  system_prompt_tokens: number
  user_prompt_tokens: number
  context_tokens: number
  total_tokens: number
  context_files_included: string[]
  // Limits
  soft_limit: number
  hard_limit: number
  context_limit: number
}

export interface ThreadMessageMeta {
  call_mode: 'fresh' | 'resumed' | 'seeded'
  input_tokens_sent: number
  context_chunks_used: number
  claude_session_id?: string
}

export interface ThreadMessage {
  role: 'user' | 'assistant'
  content: string
  ts: string
  meta?: ThreadMessageMeta
}

export interface Thread {
  question_id: string
  claude_session_id?: string
  accumulated_input_tokens?: number
  messages: ThreadMessage[]
}

export interface ThreadPreviewChunk {
  file: string
  heading: string
  tokens: number
  score: number
  preview: string
}

export interface ThreadPreviewResult {
  question_id: string
  call_mode: 'fresh' | 'resumed'
  claude_session_id: string
  accumulated_tokens: number
  system_prompt: string
  user_prompt: string
  system_tokens: number
  user_tokens: number
  tokens_to_send: number
  tokens_saved_by_resume: number
  rag_chunks: ThreadPreviewChunk[]
}

export interface GetThreadResult {
  track_id: string
  session_number: number
  thread: Thread | null
}

export interface ServerConfig {
  context_token_limit: number
  soft_token_limit: number
  hard_token_limit: number
}

export interface MetricsSnapshot {
  uptime_seconds: number
  counts: Record<string, number>
  recent: Array<{
    ts: string
    event: string
    track_id?: string
    session_num?: number
    tokens?: number
    extra?: Record<string, unknown>
  }>
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
