import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import type { Track } from '@/api/types'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'
import { Link } from 'react-router-dom'

export function NewTrackPage() {
  const [tracks, setTracks] = useState<Track[]>([])
  const [duplicating, setDuplicating] = useState<string | null>(null)
  const navigate = useNavigate()

  useEffect(() => {
    api.listTracks().then(r => setTracks(r.tracks)).catch(() => {})
  }, [])

  async function duplicate(id: string) {
    setDuplicating(id)
    try {
      const res = await api.duplicateTrack(id)
      navigate(`/tracks/${res.new_track_id}`)
    } catch {
      setDuplicating(null)
    }
  }

  return (
    <div className="max-w-lg space-y-6 mt-8">
      <div>
        <h1 className="text-xl font-bold mb-1">New Track</h1>
        <p className="text-sm text-muted-foreground">
          Tracks are created by duplicating or merging existing ones — there is no blank-slate creation.
          Every track inherits its parent's context files and concept map as a starting point.
        </p>
      </div>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Duplicate an existing track</CardTitle>
          <CardDescription>
            Creates a child track that inherits all context files and the current bloom_current
            state from the source. Add new context files afterward to specialize the child.
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {tracks.length === 0 && (
            <p className="text-sm text-muted-foreground">No tracks yet.</p>
          )}
          {tracks.map(t => (
            <div key={t.id} className="flex items-center justify-between gap-3">
              <div>
                <span className="text-sm font-mono">{t.id}</span>
                {t.is_composite && (
                  <span className="ml-2 text-[10px] text-amber-500">composite</span>
                )}
              </div>
              <button
                onClick={() => duplicate(t.id)}
                disabled={duplicating !== null}
                className={cn(buttonVariants({ size: 'sm', variant: 'outline' }), 'text-xs')}
              >
                {duplicating === t.id ? 'Duplicating…' : 'Duplicate →'}
              </button>
            </div>
          ))}
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="pb-2">
          <CardTitle className="text-base">Merge tracks into a composite</CardTitle>
          <CardDescription>
            Combines the compiled contexts of two or more tracks. Open any track page and use
            the <strong>Merge…</strong> button to choose sources.
          </CardDescription>
        </CardHeader>
        <CardContent>
          {tracks.length === 0 ? (
            <p className="text-sm text-muted-foreground">No tracks yet.</p>
          ) : (
            <Link
              to={`/tracks/${tracks[0]?.id}`}
              className={cn(buttonVariants({ size: 'sm', variant: 'outline' }), 'text-xs')}
            >
              Open {tracks[0]?.id} → Merge…
            </Link>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
