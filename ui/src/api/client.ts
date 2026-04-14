import type {
  GetTrackResult,
  GetTrackContextResult,
  DuplicateTrackResult,
  ListTracksResult,
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

async function put(path: string, body: string): Promise<void> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/plain' },
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
}
