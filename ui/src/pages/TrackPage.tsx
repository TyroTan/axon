import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import type { GetTrackResult, Concept, Session } from '@/api/types'
import { buttonVariants } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

function groupByBranch(branches: string[], concepts: Concept[]): [string, Concept[]][] {
  const map = new Map<string, Concept[]>(branches.map(b => [b, []]))
  for (const c of concepts) {
    map.get(c.branch)?.push(c)
  }
  return [...map.entries()]
}

function bloomColor(current: number): string {
  const colors = ['', 'bg-slate-600', 'bg-blue-900', 'bg-cyan-800', 'bg-green-800', 'bg-green-500', 'bg-indigo-500']
  return colors[current] ?? 'bg-slate-600'
}

function SessionSteps({ s }: { s: Session }) {
  const steps = [
    { key: 'P', done: s.has_profile, title: 'Profile' },
    { key: 'Q', done: s.has_questions, title: 'Questions' },
    { key: 'R', done: s.has_responses, title: 'Responses' },
    { key: 'E', done: s.has_evaluations, title: 'Evaluations' },
    { key: 'S', done: s.has_synthesis, title: 'Synthesis' },
  ]
  return (
    <div className="flex gap-1">
      {steps.map(step => (
        <Tooltip key={step.key}>
          <TooltipTrigger>
            <span className={cn(
              'inline-flex items-center justify-center w-5 h-5 rounded text-[10px] font-bold',
              step.done ? 'bg-green-500 text-white' : 'bg-muted text-muted-foreground'
            )}>
              {step.key}
            </span>
          </TooltipTrigger>
          <TooltipContent>{step.title}</TooltipContent>
        </Tooltip>
      ))}
    </div>
  )
}

function ConceptRow({ c }: { c: Concept }) {
  const pct = Math.round((c.bloom_current / c.bloom_target) * 100)
  return (
    <div className="grid grid-cols-[1fr_120px_40px_40px] items-center gap-3 py-1.5 text-sm">
      <span className="truncate flex items-center gap-1.5">
        {c.is_bottleneck && (
          <Tooltip>
            <TooltipTrigger>
              <span className="inline-block w-1.5 h-1.5 rounded-full bg-yellow-400 shrink-0" />
            </TooltipTrigger>
            <TooltipContent>Bottleneck concept</TooltipContent>
          </Tooltip>
        )}
        {c.name}
      </span>
      <div className="h-1.5 w-full rounded-full bg-muted overflow-hidden">
        <div
          className={cn('h-full rounded-full transition-all', bloomColor(c.bloom_current))}
          style={{ width: `${pct}%` }}
        />
      </div>
      <span className="text-xs text-muted-foreground text-right">L{c.bloom_current}</span>
      <span className="text-xs text-muted-foreground text-right">L{c.bloom_target}</span>
    </div>
  )
}

export function TrackPage() {
  const { trackId } = useParams<{ trackId: string }>()
  const navigate = useNavigate()
  const [data, setData] = useState<GetTrackResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [duplicating, setDuplicating] = useState(false)
  const [startingSession, setStartingSession] = useState(false)

  async function startSession() {
    if (!trackId || startingSession) return
    setStartingSession(true)
    try {
      const res = await api.createSession(trackId)
      navigate(`/tracks/${trackId}/sessions/${res.session_number}`)
    } catch (e) {
      setError(String(e))
      setStartingSession(false)
    }
  }

  useEffect(() => {
    if (!trackId) return
    setLoading(true)
    setError(null)
    api.getTrack(trackId)
      .then(setData)
      .catch(e => setError(String(e)))
      .finally(() => setLoading(false))
  }, [trackId])

  if (loading) return (
    <div className="space-y-4 max-w-4xl">
      <Skeleton className="h-8 w-48" />
      <div className="grid grid-cols-[1fr_280px] gap-5">
        <Skeleton className="h-96 rounded-xl" />
        <Skeleton className="h-64 rounded-xl" />
      </div>
    </div>
  )

  if (error) return <p className="text-destructive">{error}</p>
  if (!data) return null

  const { track, concept_map, sessions } = data
  const branchGroups = groupByBranch(concept_map.major_branches ?? [], concept_map.concepts ?? [])

  return (
    <div className="max-w-5xl space-y-6">
      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-xl font-bold font-mono">{track.id}</h1>
          <p className="text-sm text-muted-foreground mt-1">
            {(track.major_branches ?? []).join(' · ')}
          </p>
        </div>
        <div className="flex gap-2">
          <Link
            to={`/tracks/${track.id}/context`}
            className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }))}
          >
            Edit Context
          </Link>
          <button
            onClick={async () => {
              if (!trackId || duplicating) return
              setDuplicating(true)
              try {
                const res = await api.duplicateTrack(trackId)
                navigate(`/tracks/${res.new_track_id}`)
              } catch (e) {
                setError(String(e))
              } finally {
                setDuplicating(false)
              }
            }}
            disabled={duplicating}
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), duplicating && 'opacity-60 cursor-not-allowed')}
          >
            {duplicating ? 'Duplicating…' : 'Duplicate'}
          </button>
          <button
            onClick={startSession}
            disabled={startingSession}
            className={cn(buttonVariants({ size: 'sm' }), startingSession && 'opacity-60 cursor-not-allowed')}
          >
            {startingSession ? 'Starting…' : '+ Start Session'}
          </button>
        </div>
      </div>

      {/* Body */}
      <div className="grid grid-cols-[1fr_300px] gap-5 items-start">

        {/* Concept map heatmap */}
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-2">
              Concept Map
              <Badge variant="secondary">{concept_map.concepts?.length ?? 0} concepts</Badge>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-5">
            {branchGroups.map(([branch, concepts]) => (
              <div key={branch}>
                <p className="text-xs font-semibold uppercase tracking-wider text-primary mb-2">{branch}</p>
                <div className="divide-y divide-border/50">
                  {concepts.map(c => <ConceptRow key={c.index} c={c} />)}
                </div>
              </div>
            ))}
          </CardContent>
        </Card>

        {/* Sessions */}
        <Card className="sticky top-0">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground uppercase tracking-wider">
              Sessions
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2">
            {(!sessions || sessions.length === 0) ? (
              <p className="text-sm text-muted-foreground">No sessions yet.</p>
            ) : (
              sessions.map(s => (
                <Link
                  key={s.number}
                  to={`/tracks/${s.track_id}/sessions/${s.number}`}
                  className="flex items-center gap-3 p-2.5 rounded-lg bg-muted/50 hover:bg-muted transition-colors text-sm no-underline"
                >
                  <span className="font-mono text-xs text-muted-foreground w-6">#{s.number}</span>
                  <SessionSteps s={s} />
                  <span className="text-xs text-muted-foreground ml-auto">
                    {s.created_at ? new Date(s.created_at).toLocaleDateString() : '—'}
                  </span>
                </Link>
              ))
            )}
            <button
              onClick={startSession}
              disabled={startingSession}
              className={cn(buttonVariants({ size: 'sm' }), 'w-full mt-2', startingSession && 'opacity-60 cursor-not-allowed')}
            >
              {startingSession ? 'Starting…' : '+ Start Session'}
            </button>
          </CardContent>
        </Card>

      </div>
    </div>
  )
}
