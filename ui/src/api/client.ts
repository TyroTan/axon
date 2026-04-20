import type {
  GetTrackResult,
  GetTrackContextResult,
  GetSessionQuestionsResult,
  GetSessionResponsesResult,
  GetSessionEvaluationsResult,
  GetSynthesisResult,
  GetSplitPlanResult,
  GetMetaSynthesisResult,
  MetaSynthesisReadinessResult,
  GetContextTokensResult,
  PromptPreviewResult,
  MetricsSnapshot,
  ServerConfig,
  GetThreadResult,
  ThreadPreviewResult,
  ForkTrackResult,
  CreateTrackResult,
  MergeTracksResult,
  CreateSessionResult,
  ListTracksResult,
  Response,
  Conversation,
  ListConversationsResult,
  GetConversationResult,
  ConversationIndex,
} from './types'

const BASE = '/api'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${path}`)
  return res.json() as Promise<T>
}

async function post<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`, { method: 'POST' })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${path}`)
  return res.json() as Promise<T>
}

async function postJSON(path: string, body: unknown): Promise<void> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${path}`)
}

async function postJSONResult<T>(path: string, body: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${path}`)
  return res.json() as Promise<T>
}

async function put(path: string, body: string, contentType = 'text/plain'): Promise<void> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'PUT',
    headers: { 'Content-Type': contentType },
    body,
  })
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${path}`)
}

export const api = {
  listTracks: () => get<ListTracksResult>('/tracks'),
  getTrack: (id: string) => get<GetTrackResult>(`/tracks/${id}`),
  createTrack: (branches: string[]) => postJSONResult<CreateTrackResult>('/tracks', { branches }),
  cloneTrack: (sourceId: string) => postJSONResult<CreateTrackResult>('/tracks', { source_id: sourceId }),
  importJob: async (trackId: string, roleLabel: string, jobText: string, onChunk: (text: string) => void): Promise<string> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/context/import-job`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ role_label: roleLabel, job_text: jobText }),
    })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    let filename = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string; filename?: string }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'import job failed')
        if (ev.type === 'done') filename = ev.filename ?? ''
      }
    }
    return filename
  },
  distillThreads: async (trackId: string, onChunk: (text: string) => void): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/distill-threads`, { method: 'POST' })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'distillation failed')
        if (ev.type === 'done') return
      }
    }
  },
  forkTrack: (id: string) => post<ForkTrackResult>(`/tracks/${id}/fork`),
  mergeTracks: (sourceIds: string[], parentId: string) =>
    fetch(`${BASE}/tracks/merge`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ source_ids: sourceIds, parent_id: parentId }),
    }).then(async res => {
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      return res.json() as Promise<MergeTracksResult>
    }),
  getTrackContext: (id: string) => get<GetTrackContextResult>(`/tracks/${id}/context`),
  updateContextFile: (id: string, filename: string, content: string) =>
    put(`/tracks/${id}/context/${encodeURIComponent(filename)}`, content),
  compactContextFile: async (trackId: string, filename: string, onChunk: (text: string) => void): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/context/${encodeURIComponent(filename)}/compact`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({}),
    })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'compaction failed')
        if (ev.type === 'done') return
      }
    }
  },
  getSplitPlan: (trackId: string) => get<GetSplitPlanResult>(`/tracks/${trackId}/split-plan`),
  createSession: (trackId: string, shardId?: string) => {
    if (shardId) {
      return fetch(`${BASE}/tracks/${trackId}/sessions`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ shard_id: shardId }),
      }).then(async res => {
        if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
        return res.json() as Promise<CreateSessionResult>
      })
    }
    return post<CreateSessionResult>(`/tracks/${trackId}/sessions`)
  },
  getSessionQuestions: (trackId: string, num: number) =>
    get<GetSessionQuestionsResult>(`/tracks/${trackId}/sessions/${num}/questions`),
  getSessionResponses: (trackId: string, num: number) =>
    get<GetSessionResponsesResult>(`/tracks/${trackId}/sessions/${num}/responses`),
  getSessionEvaluations: (trackId: string, num: number) =>
    get<GetSessionEvaluationsResult>(`/tracks/${trackId}/sessions/${num}/evaluations`),
  submitResponses: (trackId: string, num: number, responses: Response[]) =>
    postJSON(`/tracks/${trackId}/sessions/${num}/responses`, responses),
  evaluateResponses: async (trackId: string, num: number, onChunk: (text: string) => void): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/sessions/${num}/evaluate`, { method: 'POST' })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'evaluation failed')
        if (ev.type === 'done') return
      }
    }
  },
  // Streams question generation via POST+SSE.
  generateQuestions: async (
    trackId: string,
    num: number,
    onChunk: (text: string) => void,
  ): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/sessions/${num}/questions/generate`, {
      method: 'POST',
    })
    if (!res.ok || !res.body) {
      throw new Error(`${res.status} ${res.statusText}`)
    }
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'generation failed')
        if (ev.type === 'done') return
      }
    }
  },
  getSynthesis: (trackId: string, num: number) =>
    get<GetSynthesisResult>(`/tracks/${trackId}/sessions/${num}/synthesis`),
  generateSynthesis: async (trackId: string, num: number, onChunk: (text: string) => void): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/sessions/${num}/synthesize`, { method: 'POST' })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'synthesis failed')
        if (ev.type === 'done') return
      }
    }
  },
  getMetaSynthesis: (trackId: string) => get<GetMetaSynthesisResult>(`/tracks/${trackId}/meta-synthesis`),
  getMetaSynthesisReadiness: (trackId: string) => get<MetaSynthesisReadinessResult>(`/tracks/${trackId}/meta-synthesis/readiness`),
  generateMetaSynthesis: async (trackId: string, onChunk: (text: string) => void): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/meta-synthesis/generate`, { method: 'POST' })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'meta-synthesis failed')
        if (ev.type === 'done') return
      }
    }
  },
  applyMetaSynthesis: async (trackId: string): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/meta-synthesis/apply`, { method: 'POST' })
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  },
  applySynthesis: async (trackId: string, num: number): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/sessions/${num}/apply-synthesis`, { method: 'POST' })
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  },
  getConfig: () => get<ServerConfig>('/config'),
  getMetrics: () => get<MetricsSnapshot>('/metrics'),
  getContextTokens: (trackId: string) => get<GetContextTokensResult>(`/tracks/${trackId}/context-tokens`),
  getPromptPreview: (trackId: string, num: number) =>
    get<PromptPreviewResult>(`/tracks/${trackId}/sessions/${num}/prompt-preview`),
  getThread: (trackId: string, num: number, questionId: string) =>
    get<GetThreadResult>(`/tracks/${trackId}/sessions/${num}/threads/${questionId}`),
  getThreadPreview: (trackId: string, num: number, questionId: string) =>
    get<ThreadPreviewResult>(`/tracks/${trackId}/sessions/${num}/threads/${questionId}/preview`),
  threadTurn: async (
    trackId: string,
    num: number,
    questionId: string,
    message: string,
    onChunk: (text: string) => void,
  ): Promise<{ sessionId: string; inputTokens: number }> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/sessions/${num}/threads/${questionId}`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message }),
    })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    let sessionId = ''
    let inputTokens = 0
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string; session_id?: string; input_tokens?: number }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'thread turn failed')
        if (ev.type === 'done') {
          sessionId = ev.session_id ?? ''
          inputTokens = ev.input_tokens ?? 0
        }
      }
    }
    return { sessionId, inputTokens }
  },

  // ─── Conversations ────────────────────────────────────────────────────────
  listConversations: () => get<ListConversationsResult>('/conversations'),
  getConversation: (id: string) => get<GetConversationResult>(`/conversations/${id}`),
  createConversation: (trackIds: string[], title?: string) =>
    fetch(`${BASE}/conversations`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ track_ids: trackIds, title: title ?? '' }),
    }).then(async res => {
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      return res.json() as Promise<Conversation>
    }),
  addConversationContext: (id: string, trackIds: string[]) =>
    fetch(`${BASE}/conversations/${id}/context`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ track_ids: trackIds }),
    }).then(async res => {
      if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
      return res.json() as Promise<Conversation>
    }),
  conversationTurn: async (
    id: string,
    message: string,
    adhocText: string,
    onChunk: (text: string) => void,
  ): Promise<{ callMode: string; inputTokens: number; contextChunks: number; claudeSessionId: string }> => {
    const res = await fetch(`${BASE}/conversations/${id}/turn`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ message, adhoc_text: adhocText }),
    })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    let callMode = 'fresh'
    let inputTokens = 0
    let contextChunks = 0
    let claudeSessionId = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as {
          type: string; text?: string; message?: string
          call_mode?: string; input_tokens?: number; context_chunks?: number; claude_session_id?: string
        }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'conversation turn failed')
        if (ev.type === 'done') {
          callMode = ev.call_mode ?? 'fresh'
          inputTokens = ev.input_tokens ?? 0
          contextChunks = ev.context_chunks ?? 0
          claudeSessionId = ev.claude_session_id ?? ''
        }
      }
    }
    return { callMode, inputTokens, contextChunks, claudeSessionId }
  },
  getConversationIndex: (id: string) => get<ConversationIndex>(`/conversations/${id}/index`),
  indexConversation: async (id: string, onChunk: (text: string) => void): Promise<ConversationIndex> => {
    const res = await fetch(`${BASE}/conversations/${id}/index`, { method: 'POST' })
    if (!res.ok || !res.body) throw new Error(`${res.status} ${res.statusText}`)
    const reader = res.body.getReader()
    const decoder = new TextDecoder()
    let buf = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buf += decoder.decode(value, { stream: true })
      const lines = buf.split('\n')
      buf = lines.pop() ?? ''
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue
        const ev = JSON.parse(line.slice(6)) as { type: string; text?: string; message?: string }
        if (ev.type === 'chunk' && ev.text) onChunk(ev.text)
        if (ev.type === 'error') throw new Error(ev.message ?? 'indexing failed')
        if (ev.type === 'done') { /* index written server-side */ }
      }
    }
    // Fetch the final index document.
    return api.getConversationIndex(id)
  },
}
