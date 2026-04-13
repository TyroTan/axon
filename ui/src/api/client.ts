import type { GetTrackResult, ListTracksResult } from './types'

const BASE = '/api'

async function get<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`)
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}: ${path}`)
  return res.json() as Promise<T>
}

export const api = {
  listTracks: () => get<ListTracksResult>('/tracks'),
  getTrack: (id: string) => get<GetTrackResult>(`/tracks/${id}`),
}
