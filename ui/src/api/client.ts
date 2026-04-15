import type {
  GetTrackResult,
  GetTrackContextResult,
  GetSessionQuestionsResult,
  GetSessionResponsesResult,
  GetSessionEvaluationsResult,
  GetSynthesisResult,
  GetSplitPlanResult,
  DuplicateTrackResult,
  CreateSessionResult,
  ListTracksResult,
  Response,
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
  duplicateTrack: (id: string) => post<DuplicateTrackResult>(`/tracks/${id}/duplicate`),
  getTrackContext: (id: string) => get<GetTrackContextResult>(`/tracks/${id}/context`),
  updateContextFile: (id: string, filename: string, content: string) =>
    put(`/tracks/${id}/context/${encodeURIComponent(filename)}`, content),
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
  applySynthesis: async (trackId: string, num: number): Promise<void> => {
    const res = await fetch(`${BASE}/tracks/${trackId}/sessions/${num}/apply-synthesis`, { method: 'POST' })
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`)
  },
}
