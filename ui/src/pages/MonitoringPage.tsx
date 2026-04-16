import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { MetricsSnapshot, GetContextTokensResult, ListTracksResult, ServerConfig } from '@/api/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'

// ── helpers ───────────────────────────────────────────────────────────────────

function fmtTokens(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`
  return String(n)
}

function fmtUptime(secs: number): string {
  if (secs < 60) return `${secs}s`
  if (secs < 3600) return `${Math.floor(secs / 60)}m ${secs % 60}s`
  const h = Math.floor(secs / 3600)
  const m = Math.floor((secs % 3600) / 60)
  return `${h}h ${m}m`
}

function pct(tokens: number, limit: number): number {
  return limit > 0 ? Math.min(100, (tokens / limit) * 100) : 0
}

function barColor(p: number): string {
  if (p >= 90) return 'bg-red-500'
  if (p >= 70) return 'bg-amber-500'
  return 'bg-emerald-500'
}

// ── API endpoint catalogue ───────────────────────────────────────────────────

const API_ENDPOINTS = [
  { method: 'GET', path: '/api/metrics', title: 'System Metrics', desc: 'Uptime, event counts, recent events log.' },
  { method: 'GET', path: '/api/tracks/:id/context-tokens', title: 'Context Token Breakdown', desc: 'Per-file token counts for a track\'s full inherited context, attributed to the source track in the ancestor chain.' },
  { method: 'GET', path: '/api/tracks/:id/sessions/:num/prompt-preview', title: 'Prompt Preview', desc: 'Exact system + user prompt that would be sent to the LLM — includes token estimates, context files list, and budget limits.' },
  { method: 'GET', path: '/api/tracks/:id/split-plan', title: 'Split Plan', desc: 'Reads the shard split plan (null if corpus is within the soft limit).' },
  { method: 'GET', path: '/api/tracks/:id/meta-synthesis/readiness', title: 'Meta-Synthesis Readiness', desc: 'Lists shards missing evaluated sessions (gates meta-synthesis generation).' },
]

// ── sub-components ────────────────────────────────────────────────────────────

function MetricsPanel({ snapshot }: { snapshot: MetricsSnapshot }) {
  const counts = snapshot.counts ?? {}
  const countKeys = Object.keys(counts).sort()

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base flex items-center gap-2">
          System Metrics
          <Badge variant="outline" className="text-xs font-mono">{fmtUptime(snapshot.uptime_seconds)} uptime</Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-4">
        {countKeys.length > 0 ? (
          <table className="w-full text-sm">
            <thead>
              <tr className="text-muted-foreground text-xs uppercase border-b">
                <th className="text-left py-1 pr-4">Event</th>
                <th className="text-right py-1">Count</th>
              </tr>
            </thead>
            <tbody>
              {countKeys.map(k => (
                <tr key={k} className="border-b border-border/40 last:border-0">
                  <td className="py-1.5 pr-4 font-mono text-xs">{k}</td>
                  <td className="py-1.5 text-right tabular-nums">{counts[k]}</td>
                </tr>
              ))}
            </tbody>
          </table>
        ) : (
          <p className="text-sm text-muted-foreground">No events recorded yet.</p>
        )}

        {snapshot.recent && snapshot.recent.length > 0 && (
          <div>
            <p className="text-xs text-muted-foreground uppercase tracking-wide mb-2">Recent events</p>
            <div className="space-y-1 max-h-64 overflow-y-auto">
              {snapshot.recent.map((ev, i) => (
                <div key={i} className="flex items-start gap-2 text-xs font-mono bg-muted/40 rounded px-2 py-1">
                  <span className="text-muted-foreground shrink-0">{new Date(ev.ts).toLocaleTimeString()}</span>
                  <span className="font-semibold shrink-0">{ev.event}</span>
                  {ev.track_id && <span className="text-muted-foreground">{ev.track_id}</span>}
                  {ev.tokens != null && ev.tokens > 0 && (
                    <span className="ml-auto shrink-0 text-primary">{fmtTokens(ev.tokens)} tok</span>
                  )}
                </div>
              ))}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ContextTokensPanel({
  tracks,
  config,
}: {
  tracks: ListTracksResult['tracks']
  config: ServerConfig | null
}) {
  const [selectedTrack, setSelectedTrack] = useState<string>(tracks[0]?.id ?? '')
  const [data, setData] = useState<GetContextTokensResult | null>(null)
  const [loading, setLoading] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    if (!selectedTrack) return
    setLoading(true)
    setErr(null)
    api.getContextTokens(selectedTrack)
      .then(setData)
      .catch(e => setErr(String(e)))
      .finally(() => setLoading(false))
  }, [selectedTrack])

  const softLimit = config?.soft_token_limit ?? 250_000
  const contextLimit = config?.context_token_limit ?? 50_000

  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base flex items-center gap-2">
          Context Token Breakdown
          {data && (
            <Badge variant="outline" className="text-xs font-mono">
              {fmtTokens(data.total_tokens)} total · {fmtTokens(contextLimit)} ctx limit · {fmtTokens(softLimit)} soft limit
            </Badge>
          )}
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center gap-2">
          <label className="text-xs text-muted-foreground shrink-0">Track:</label>
          <select
            className="text-sm border rounded px-2 py-1 bg-background"
            value={selectedTrack}
            onChange={e => setSelectedTrack(e.target.value)}
          >
            {tracks.map(t => (
              <option key={t.id} value={t.id}>{t.id}</option>
            ))}
          </select>
          {data && (
            <span className="text-xs text-muted-foreground ml-auto">
              {data.file_count} files · {fmtTokens(data.total_tokens)} total tokens
            </span>
          )}
        </div>

        {loading && <p className="text-sm text-muted-foreground">Loading…</p>}
        {err && <p className="text-sm text-red-500">{err}</p>}

        {data && (
          <>
            {/* Total bar vs context_limit and soft_limit */}
            <div className="space-y-1">
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>Context limit (per-session, inherited)</span>
                <span>{fmtTokens(data.total_tokens)} / {fmtTokens(contextLimit)}</span>
              </div>
              <div className="h-2 w-full rounded-full bg-muted overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all ${barColor(pct(data.total_tokens, contextLimit))}`}
                  style={{ width: `${pct(data.total_tokens, contextLimit)}%` }}
                />
              </div>
              <div className="flex justify-between text-xs text-muted-foreground">
                <span>Split plan soft limit</span>
                <span>{fmtTokens(data.total_tokens)} / {fmtTokens(softLimit)}</span>
              </div>
              <div className="h-1.5 w-full rounded-full bg-muted overflow-hidden">
                <div
                  className={`h-full rounded-full transition-all ${barColor(pct(data.total_tokens, softLimit))}`}
                  style={{ width: `${pct(data.total_tokens, softLimit)}%` }}
                />
              </div>
            </div>

            <table className="w-full text-sm mt-1">
              <thead>
                <tr className="text-muted-foreground text-xs uppercase border-b">
                  <th className="text-left py-1 pr-2">File</th>
                  <th className="text-left py-1 pr-2">Source track</th>
                  <th className="text-right py-1 pr-2">Tokens</th>
                  <th className="text-right py-1">% of total</th>
                </tr>
              </thead>
              <tbody>
                {data.files.map(f => {
                  const share = data.total_tokens > 0 ? (f.tokens / data.total_tokens) * 100 : 0
                  return (
                    <tr key={f.filename} className="border-b border-border/40 last:border-0">
                      <td className="py-1.5 pr-2 font-mono text-xs max-w-[180px] truncate">{f.filename}</td>
                      <td className="py-1.5 pr-2 text-xs text-muted-foreground">{f.source_track_id}</td>
                      <td className="py-1.5 pr-2 text-right tabular-nums">{fmtTokens(f.tokens)}</td>
                      <td className="py-1.5 text-right">
                        <div className="flex items-center justify-end gap-1.5">
                          <div className="w-16 h-1.5 rounded-full bg-muted overflow-hidden">
                            <div
                              className="h-full rounded-full bg-primary/60"
                              style={{ width: `${share}%` }}
                            />
                          </div>
                          <span className="text-xs tabular-nums w-9 text-right">{share.toFixed(1)}%</span>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </>
        )}
      </CardContent>
    </Card>
  )
}

function EndpointDirectory() {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">API Monitoring Endpoints</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {API_ENDPOINTS.map(ep => (
            <div key={ep.path} className="flex flex-col gap-0.5 border-b border-border/40 last:border-0 pb-3 last:pb-0">
              <div className="flex items-center gap-2">
                <Badge variant="outline" className="text-xs font-mono shrink-0">{ep.method}</Badge>
                <code className="text-xs font-mono text-primary">{ep.path}</code>
              </div>
              <p className="text-xs font-medium">{ep.title}</p>
              <p className="text-xs text-muted-foreground">{ep.desc}</p>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}

function TokenBudgetInfo({ config }: { config: ServerConfig | null }) {
  const fmt = (n: number | undefined) => n != null ? fmtTokens(n) : '…'
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">Token Budget Reference</CardTitle>
      </CardHeader>
      <CardContent className="text-xs space-y-2 text-muted-foreground">
        <div className="grid grid-cols-2 gap-x-4 gap-y-1">
          <span className="font-medium text-foreground">Context limit (per session)</span>
          <span className="font-mono">{fmt(config?.context_token_limit)} tokens · AXON_CONTEXT_LIMIT</span>
          <span className="font-medium text-foreground">Soft limit (split plan trigger)</span>
          <span className="font-mono">{fmt(config?.soft_token_limit)} tokens · AXON_SOFT_LIMIT</span>
          <span className="font-medium text-foreground">Hard limit (abort)</span>
          <span className="font-mono">{fmt(config?.hard_token_limit)} tokens · AXON_HARD_LIMIT</span>
          <span className="font-medium text-foreground">Naive token estimate</span>
          <span className="font-mono">(len + 3) / 4  ≈ chars / 4</span>
        </div>
        <p className="pt-1">
          The three-zone model: <span className="text-foreground">0–250k</span> loads as-is,{' '}
          <span className="text-amber-500">250k–300k</span> triggers a split plan (HITL sharding),{' '}
          <span className="text-red-500">&gt;300k</span> hard-aborts. Compaction is optional noise reduction
          for individual verbose files — it never replaces splitting.
        </p>
        <p>
          <span className="text-foreground font-medium">Context compaction note:</span> manual compaction
          via the context editor writes <code>.compact.md</code> — it is not triggered automatically to
          avoid compaction loops. Each compaction call is one-shot and idempotent (hash-gated).
        </p>
        <p>
          <Link to="/" className="text-primary underline">
            View tracks →
          </Link>{' '}
          to open the context editor and inspect or compact individual files.
        </p>
      </CardContent>
    </Card>
  )
}

// ── page ──────────────────────────────────────────────────────────────────────

export function MonitoringPage() {
  const [snapshot, setSnapshot] = useState<MetricsSnapshot | null>(null)
  const [tracks, setTracks] = useState<ListTracksResult['tracks']>([])
  const [config, setConfig] = useState<ServerConfig | null>(null)
  const [loadErr, setLoadErr] = useState<string | null>(null)

  useEffect(() => {
    Promise.all([api.getMetrics(), api.listTracks(), api.getConfig()])
      .then(([m, t, c]) => {
        setSnapshot(m)
        setTracks(t.tracks ?? [])
        setConfig(c)
      })
      .catch(e => setLoadErr(String(e)))
  }, [])

  if (loadErr) {
    return (
      <div className="p-6 text-red-500 text-sm">Failed to load monitoring data: {loadErr}</div>
    )
  }

  return (
    <div className="p-6 max-w-4xl mx-auto space-y-6">
      <div>
        <h1 className="text-2xl font-bold tracking-tight">Monitoring</h1>
        <p className="text-sm text-muted-foreground mt-1">
          System metrics, token budgets, and API observability.
        </p>
      </div>

      <TokenBudgetInfo config={config} />

      {snapshot ? <MetricsPanel snapshot={snapshot} /> : (
        <Card><CardContent className="py-6 text-sm text-muted-foreground">Loading metrics…</CardContent></Card>
      )}

      {tracks.length > 0 ? (
        <ContextTokensPanel tracks={tracks} config={config} />
      ) : (
        <Card><CardContent className="py-6 text-sm text-muted-foreground">Loading tracks…</CardContent></Card>
      )}

      <EndpointDirectory />
    </div>
  )
}
