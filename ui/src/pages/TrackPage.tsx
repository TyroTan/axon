import { useEffect, useState } from 'react'
import { useParams, Link, useNavigate } from 'react-router-dom'
import { api } from '@/api/client'
import type { GetTrackResult, Concept, Session, SplitPlan, MetaSynthesis, Track } from '@/api/types'
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
  const [splitPlan, setSplitPlan] = useState<SplitPlan | null>(null)
  const [metaSynth, setMetaSynth] = useState<MetaSynthesis | null>(null)
  const [metaReady, setMetaReady] = useState<boolean>(false)
  const [metaMissing, setMetaMissing] = useState<string[]>([])
  const [generatingMeta, setGeneratingMeta] = useState(false)
  const [metaStream, setMetaStream] = useState('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [duplicating, setDuplicating] = useState(false)
  const [startingSession, setStartingSession] = useState(false)
  const [shardPickerOpen, setShardPickerOpen] = useState(false)
  const [mergeOpen, setMergeOpen] = useState(false)
  const [allTracks, setAllTracks] = useState<Track[]>([])
  const [mergeSelected, setMergeSelected] = useState<string[]>([])
  const [merging, setMerging] = useState(false)
  const [mergeParentId, setMergeParentId] = useState('')

  async function startSession(shardId?: string) {
    if (!trackId || startingSession) return

    // If a split plan with approved shards exists and no shard chosen yet, open picker.
    const approvedShards = splitPlan?.shards.filter(s => s.status === 'approved') ?? []
    if (approvedShards.length > 0 && !shardId) {
      setShardPickerOpen(true)
      return
    }

    setStartingSession(true)
    setShardPickerOpen(false)
    try {
      const res = await api.createSession(trackId, shardId)
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
    Promise.all([
      api.getTrack(trackId),
      api.getSplitPlan(trackId),
      api.getMetaSynthesis(trackId),
      api.getMetaSynthesisReadiness(trackId).catch(() => ({ ready: false, missing_shards: [] })),
    ])
      .then(([trackData, planData, msData, readiness]) => {
        setData(trackData)
        setSplitPlan(planData.split_plan)
        setMetaSynth(msData.meta_synthesis)
        setMetaReady(readiness.ready)
        setMetaMissing(readiness.missing_shards ?? [])
      })
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
  const approvedShards = splitPlan?.shards.filter(s => s.status === 'approved') ?? []
  const pendingShards = splitPlan?.shards.filter(s => s.status === 'pending') ?? []

  return (
    <div className="max-w-5xl space-y-6">
      {/* Split plan notice */}
      {splitPlan && (
        <div className="rounded-lg border border-yellow-500/40 bg-yellow-500/10 px-4 py-3 text-sm space-y-1">
          <p className="font-semibold text-yellow-300">
            Context split plan — {(splitPlan.total_tokens / 1000).toFixed(0)}k tokens total
          </p>
          <p className="text-muted-foreground">
            {approvedShards.length} shard{approvedShards.length !== 1 ? 's' : ''} approved
            {pendingShards.length > 0 && `, ${pendingShards.length} pending`}.
            {approvedShards.length === 0 && ' Open Context Editor to approve shards before starting a session.'}
          </p>
        </div>
      )}

      {/* Meta-synthesis panel — only shown when a split plan exists */}
      {splitPlan && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-muted-foreground uppercase tracking-wider flex items-center gap-2">
              Meta-Synthesis
              {metaSynth?.applied && <Badge variant="secondary">Applied</Badge>}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            {metaSynth ? (
              <>
                <p className="text-sm text-muted-foreground">{metaSynth.learner_summary}</p>
                <p className="text-xs text-muted-foreground">
                  Sessions: {metaSynth.sessions_aggregated.join(', ')} ·
                  Shards: {metaSynth.shards_aggregated.join(', ')} ·
                  {metaSynth.concept_map_updates.length} concept updates
                </p>
                <div className="flex gap-2">
                  <button
                    onClick={async () => {
                      if (!trackId || generatingMeta) return
                      setGeneratingMeta(true)
                      setMetaStream('')
                      try {
                        await api.generateMetaSynthesis(trackId, t => setMetaStream(p => p + t))
                        const ms = await api.getMetaSynthesis(trackId)
                        setMetaSynth(ms.meta_synthesis)
                      } catch (e) { setError(String(e)) }
                      finally { setGeneratingMeta(false) }
                    }}
                    disabled={generatingMeta || !metaReady}
                    className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), (!metaReady || generatingMeta) && 'opacity-60 cursor-not-allowed')}
                  >
                    {generatingMeta ? 'Regenerating…' : 'Re-synthesise'}
                  </button>
                  {!metaSynth.applied && (
                    <button
                      onClick={async () => {
                        if (!trackId) return
                        try {
                          await api.applyMetaSynthesis(trackId)
                          const ms = await api.getMetaSynthesis(trackId)
                          setMetaSynth(ms.meta_synthesis)
                        } catch (e) { setError(String(e)) }
                      }}
                      className={cn(buttonVariants({ size: 'sm' }))}
                    >
                      Apply to concept map
                    </button>
                  )}
                </div>
              </>
            ) : (
              <div className="space-y-2">
                {metaReady ? (
                  <p className="text-sm text-muted-foreground">All shards evaluated. Ready to generate.</p>
                ) : (
                  <p className="text-sm text-muted-foreground">
                    Waiting for evaluations on: <span className="font-mono">{metaMissing.join(', ') || '—'}</span>
                  </p>
                )}
                <button
                  onClick={async () => {
                    if (!trackId || generatingMeta || !metaReady) return
                    setGeneratingMeta(true)
                    setMetaStream('')
                    try {
                      await api.generateMetaSynthesis(trackId, t => setMetaStream(p => p + t))
                      const ms = await api.getMetaSynthesis(trackId)
                      setMetaSynth(ms.meta_synthesis)
                    } catch (e) { setError(String(e)) }
                    finally { setGeneratingMeta(false) }
                  }}
                  disabled={generatingMeta || !metaReady}
                  className={cn(buttonVariants({ size: 'sm' }), (!metaReady || generatingMeta) && 'opacity-60 cursor-not-allowed')}
                >
                  {generatingMeta ? 'Generating…' : 'Generate meta-synthesis'}
                </button>
                {metaStream && (
                  <pre className="text-xs text-muted-foreground bg-muted rounded p-2 max-h-32 overflow-auto whitespace-pre-wrap">{metaStream}</pre>
                )}
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Shard picker modal */}
      {shardPickerOpen && approvedShards.length > 0 && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60">
          <div className="bg-background border rounded-xl shadow-xl p-6 w-full max-w-md space-y-4">
            <h2 className="font-semibold text-base">Pick a context shard</h2>
            <p className="text-sm text-muted-foreground">
              This track's context exceeds the soft limit. Choose which shard of files to study this session.
            </p>
            <div className="space-y-2">
              {approvedShards.map(sh => (
                <button
                  key={sh.id}
                  onClick={() => startSession(sh.id)}
                  disabled={startingSession}
                  className={cn(
                    buttonVariants({ variant: 'outline' }),
                    'w-full justify-start gap-3',
                    startingSession && 'opacity-60 cursor-not-allowed',
                  )}
                >
                  <span className="font-mono text-xs">{sh.id}</span>
                  <span className="text-muted-foreground text-xs">
                    {(sh.token_count / 1000).toFixed(0)}k tokens · {sh.files.length} file{sh.files.length !== 1 ? 's' : ''}
                  </span>
                </button>
              ))}
            </div>
            <button
              onClick={() => setShardPickerOpen(false)}
              className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'w-full')}
            >
              Cancel
            </button>
          </div>
        </div>
      )}

      {/* Header */}
      <div className="flex items-start justify-between">
        <div>
          <h1 className="text-xl font-bold font-mono flex items-center gap-2">
            {track.id}
            {track.is_composite && (
              <Badge variant="outline" className="text-[10px] border-amber-500/50 text-amber-400 font-normal">composite</Badge>
            )}
          </h1>
          <p className="text-sm text-muted-foreground mt-1">
            {(track.major_branches ?? []).join(' · ')}
          </p>
          {track.is_composite && track.source_ids && (
            <p className="text-xs text-muted-foreground mt-0.5">
              Merged from: {track.source_ids.map(id => (
                <Link key={id} to={`/tracks/${id}`} className="font-mono hover:text-foreground mx-0.5 underline underline-offset-2">{id}</Link>
              ))}
            </p>
          )}
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
            onClick={async () => {
              if (mergeOpen) { setMergeOpen(false); return }
              const res = await api.listTracks()
              const flat: Track[] = []
              const walk = (ts: Track[]) => ts.forEach(t => { flat.push(t); if (t.children) walk(t.children) })
              walk(res.tracks ?? [])
              setAllTracks(flat.filter(t => t.id !== trackId))
              setMergeSelected([trackId!])
              setMergeParentId(track.parent_id ?? '')
              setMergeOpen(true)
            }}
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
          >
            Merge…
          </button>
          <button
            onClick={() => startSession()}
            disabled={startingSession}
            className={cn(buttonVariants({ size: 'sm' }), startingSession && 'opacity-60 cursor-not-allowed')}
          >
            {startingSession ? 'Starting…' : '+ Start Session'}
          </button>
        </div>
      </div>

      {/* Merge panel */}
      {mergeOpen && (
        <Card className="border-amber-500/40 bg-amber-950/10">
          <CardHeader className="pb-2">
            <CardTitle className="text-sm font-semibold text-amber-400 flex items-center justify-between">
              <span>Merge Tracks → new composite track</span>
              <button onClick={() => setMergeOpen(false)} className="text-muted-foreground hover:text-foreground text-xs">✕</button>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <p className="text-xs text-muted-foreground">
              Select additional tracks to merge with <span className="font-mono text-foreground">{trackId}</span>.
              The new track gets a union of all inherited context files and concept maps (re-indexed).
              Source tracks are untouched.
            </p>

            {/* Track checkboxes */}
            <div className="space-y-1 max-h-48 overflow-y-auto">
              {/* Current track — always included, shown as locked */}
              <label className="flex items-center gap-2 text-sm opacity-60 cursor-default select-none">
                <input type="checkbox" checked readOnly className="accent-amber-400" />
                <span className="font-mono">{trackId}</span>
                <span className="text-xs text-muted-foreground">(this track)</span>
              </label>
              {allTracks.map(t => (
                <label key={t.id} className="flex items-center gap-2 text-sm cursor-pointer">
                  <input
                    type="checkbox"
                    className="accent-amber-400"
                    checked={mergeSelected.includes(t.id)}
                    onChange={e => setMergeSelected(prev =>
                      e.target.checked ? [...prev, t.id] : prev.filter(x => x !== t.id)
                    )}
                  />
                  <span className="font-mono">{t.id}</span>
                  {t.is_composite && <Badge variant="outline" className="text-[10px] py-0 border-amber-500/50 text-amber-400">composite</Badge>}
                  {t.major_branches && <span className="text-xs text-muted-foreground truncate">{t.major_branches.slice(0, 2).join(' · ')}</span>}
                </label>
              ))}
            </div>

            {/* Parent ID for new track */}
            <div className="flex items-center gap-2 text-sm">
              <label className="text-muted-foreground shrink-0">Parent ID of new track:</label>
              <input
                type="text"
                value={mergeParentId}
                onChange={e => setMergeParentId(e.target.value)}
                placeholder="(leave blank for root)"
                className="flex-1 bg-background border border-border rounded px-2 py-1 text-xs font-mono"
              />
            </div>

            <div className="flex items-center gap-3">
              <button
                disabled={mergeSelected.length < 2 || merging}
                onClick={async () => {
                  if (mergeSelected.length < 2 || merging) return
                  setMerging(true)
                  try {
                    const res = await api.mergeTracks(mergeSelected, mergeParentId)
                    navigate(`/tracks/${res.new_track_id}`)
                  } catch (e) {
                    setError(String(e))
                    setMerging(false)
                  }
                }}
                className={cn(buttonVariants({ size: 'sm' }), (mergeSelected.length < 2 || merging) && 'opacity-50 cursor-not-allowed')}
              >
                {merging ? 'Merging…' : `Merge ${mergeSelected.length} tracks`}
              </button>
              <span className="text-xs text-muted-foreground">
                {mergeSelected.length < 2 ? 'Select at least 2 tracks' : `→ new child of "${mergeParentId || '(root)'}" will be created`}
              </span>
            </div>
          </CardContent>
        </Card>
      )}

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

        {/* Onboarding callout — shown only on fresh tracks with no sessions */}
        {(!sessions || sessions.length === 0) && (
          <Card className="border-dashed border-primary/40 bg-primary/5">
            <CardContent className="pt-4 pb-4 space-y-2">
              <p className="text-sm font-medium">Getting started with this track</p>
              <ol className="text-sm text-muted-foreground space-y-1 list-decimal list-inside">
                <li>
                  <Link to={`/tracks/${track.id}/context`} className="underline underline-offset-2 hover:text-foreground">
                    Edit Context
                  </Link>
                  {' '}— add or review <code className="text-xs bg-muted px-1 rounded">.md</code> files
                  that the question generator will study. This track already inherited context from its parent.
                </li>
                <li>Click <strong>+ Start Session</strong> below — questions are generated from the concept map + context files.</li>
              </ol>
              <p className="text-xs text-muted-foreground pt-1">
                The <code className="bg-muted px-1 rounded">prompts/</code> folder contains the system prompt templates
                used for each step — useful if you want to understand or adapt what the backend is doing.
              </p>
            </CardContent>
          </Card>
        )}

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
              onClick={() => startSession()}
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
