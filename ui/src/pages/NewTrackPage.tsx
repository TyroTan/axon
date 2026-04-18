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
//   Blank  — define branch names, concept map starts empty
//   Clone  — pick an existing track, copies its concept map + context as snapshot
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
        <h1 className="text-xl font-bold mb-1">New Root Track</h1>
        <p className="text-sm text-muted-foreground">
          A root track is a new independent topic cluster (e.g.{' '}
          <code className="bg-muted px-1 rounded text-xs">track_2</code>).
          Choose how to initialise it:
        </p>
      </div>

      {/* Mode toggle */}
      <div className="flex gap-2">
        <button
          onClick={() => setMode('blank')}
          className={cn(
            buttonVariants({ variant: mode === 'blank' ? 'default' : 'outline', size: 'sm' }),
          )}
        >
          Blank track
        </button>
        <button
          onClick={() => setMode('clone')}
          className={cn(
            buttonVariants({ variant: mode === 'clone' ? 'default' : 'outline', size: 'sm' }),
          )}
        >
          Clone existing track
        </button>
      </div>

      {/* Blank path */}
      {mode === 'blank' && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Major Branches</CardTitle>
            <CardDescription>
              2–5 bodies of knowledge, e.g. "RAG Architecture", "LLM Systems".
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

      {/* Clone path */}
      {mode === 'clone' && (
        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="text-base">Clone from</CardTitle>
            <CardDescription>
              Copies the concept map (bloom_current preserved) and all context files as a snapshot.
              The new track is a root — it has no parent and context is not inherited via cascade.
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
        {creating ? 'Creating…' : mode === 'clone' ? `Clone ${sourceId} → new track` : 'Create blank track'}
      </button>

      <Card className="border-dashed">
        <CardContent className="pt-4 pb-4">
          <p className="text-sm font-medium mb-1">Want a child track instead?</p>
          <p className="text-sm text-muted-foreground">
            Open e.g. <code className="bg-muted px-1 rounded text-xs">track_1</code> and click{' '}
            <strong>Fork</strong> to create <code className="bg-muted px-1 rounded text-xs">track_1_2</code>.
            Child tracks inherit context from the parent at question-generation time (not a physical copy).
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
