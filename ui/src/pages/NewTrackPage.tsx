import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import type { Track } from '@/api/types'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { X } from 'lucide-react'

// NewTrackPage — creates a new ROOT track (track_2, track_3, …).
//
// Two paths:
//   New topic  — define branch names, concept map starts empty
//   Copy track — pick an existing track, copies its concept map + context as snapshot
//               (independent root; no parent link after copy)
//
// To create a CHILD track (track_1_2) — go to track_1 and click Fork.

export function NewTrackPage() {
  const [mode, setMode] = useState<'blank' | 'clone'>('blank')
  const [branches, setBranches] = useState<string[]>([''])
  const [sourceId, setSourceId] = useState('')
  const [tracks, setTracks] = useState<Track[]>([])
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()

  useEffect(() => {
    api.listTracks().then(r => setTracks(flattenTracks(r.tracks ?? []))).catch(() => {})
  }, [])

  function addBranch() { setBranches(b => [...b, '']) }
  function updateBranch(i: number, val: string) { setBranches(b => b.map((v, idx) => idx === i ? val : v)) }
  function removeBranch(i: number) { setBranches(b => b.filter((_, idx) => idx !== i)) }

  async function handleCreate() {
    setError(null)
    if (mode === 'blank') {
      const valid = branches.map(b => b.trim()).filter(Boolean)
      if (valid.length === 0) { setError('Add at least one branch name.'); return }
      setCreating(true)
      try {
        const res = await api.createTrack(valid)
        navigate(`/tracks/${res.new_track_id}`)
      } catch (e) { setError(String(e)); setCreating(false) }
    } else {
      if (!sourceId) { setError('Select a source track to clone.'); return }
      setCreating(true)
      try {
        const res = await api.cloneTrack(sourceId)
        navigate(`/tracks/${res.new_track_id}`)
      } catch (e) { setError(String(e)); setCreating(false) }
    }
  }

  const canCreate = mode === 'blank'
    ? branches.some(b => b.trim())
    : !!sourceId

  return (
    <div className="max-w-lg space-y-5 mt-8">
      <div>
        <h1 className="text-xl font-bold mb-1">New Track</h1>
        <p className="text-sm text-muted-foreground">
          Creates an independent root track (e.g.{' '}
          <code className="bg-muted px-1 rounded text-xs">track_2</code>).
          To create a child of an existing track, use <strong>Fork</strong> on that track's page.
        </p>
      </div>

      {/* Mode toggle */}
      <div className="flex gap-2">
        <button
          onClick={() => setMode('blank')}
          title="Start from scratch — no source track. You define the branches."
          className={cn(
            buttonVariants({ variant: mode === 'blank' ? 'default' : 'outline', size: 'sm' }),
          )}
        >
          New topic
        </button>
        <button
          onClick={() => setMode('clone')}
          title="Copy concept map + context from an existing track. Independent root — no parent link after copy."
          className={cn(
            buttonVariants({ variant: mode === 'clone' ? 'default' : 'outline', size: 'sm' }),
          )}
        >
          Copy track
        </button>
      </div>

      {/* New topic path */}
      {mode === 'blank' && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Knowledge branches</CardTitle>
            <CardDescription>
              2–5 top-level domains, e.g. "RAG Architecture", "LLM Systems".
              Concept map starts empty — populate it via <code className="text-xs">prompts/00</code> after creation.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {branches.map((b, i) => (
              <div key={i} className="flex gap-2 items-center">
                <input
                  type="text"
                  value={b}
                  onChange={e => updateBranch(i, e.target.value)}
                  onKeyDown={e => e.key === 'Enter' && addBranch()}
                  placeholder={`Branch ${i + 1}`}
                  className="flex-1 text-sm bg-muted border border-border rounded px-3 py-1.5 focus:outline-none focus:ring-1 focus:ring-primary"
                />
                {branches.length > 1 && (
                  <button onClick={() => removeBranch(i)} className="text-muted-foreground hover:text-foreground">
                    <X size={14} />
                  </button>
                )}
              </div>
            ))}
            <button onClick={addBranch} className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'text-xs mt-1')}>
              + Add branch
            </button>
          </CardContent>
        </Card>
      )}

      {/* Copy track path */}
      {mode === 'clone' && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Copy from</CardTitle>
            <CardDescription>
              Copies concept map and context files as a one-time snapshot.
              The result is an independent root — no parent link, no inheritance after copy.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-2">
            {tracks.length === 0 ? (
              <p className="text-sm text-muted-foreground">No tracks yet.</p>
            ) : (
              tracks.map(t => (
                <label key={t.id} className="flex items-center gap-3 cursor-pointer p-2 rounded hover:bg-muted/50">
                  <input
                    type="radio"
                    name="source"
                    value={t.id}
                    checked={sourceId === t.id}
                    onChange={() => setSourceId(t.id)}
                    className="accent-primary"
                  />
                  <span className="font-mono text-sm">{t.id}</span>
                  <span className="text-xs text-muted-foreground ml-auto">
                    {(t.major_branches ?? []).join(' · ')}
                  </span>
                </label>
              ))
            )}
          </CardContent>
        </Card>
      )}

      {error && <p className="text-sm text-red-500">{error}</p>}

      <button
        onClick={handleCreate}
        disabled={creating || !canCreate}
        className={cn(buttonVariants(), (creating || !canCreate) && 'opacity-60 cursor-not-allowed')}
      >
        {creating ? 'Creating…' : mode === 'clone' ? `Copy ${sourceId} → new track` : 'Create topic'}
      </button>

      <Card className="border-dashed">
        <CardContent className="pt-4 pb-4">
          <p className="text-sm font-medium mb-1">Want a child track (Fork)?</p>
          <p className="text-sm text-muted-foreground">
            Open e.g. <code className="bg-muted px-1 rounded text-xs">track_1</code> and click{' '}
            <strong>Fork</strong> to create <code className="bg-muted px-1 rounded text-xs">track_1_2</code>.
            A forked child inherits the parent's context on every session — it stays connected, not a snapshot.
          </p>
        </CardContent>
      </Card>
    </div>
  )
}

// Flatten the track tree into a flat list for the clone picker.
function flattenTracks(tracks: Track[]): Track[] {
  const result: Track[] = []
  function walk(t: Track) {
    result.push(t)
    for (const c of t.children ?? []) walk(c)
  }
  for (const t of tracks) walk(t)
  return result
}
