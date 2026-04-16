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

interface EndpointDef {
  method: 'GET' | 'POST'
  path: string   // e.g. /api/tracks/:id/sessions/:num/prompt-preview
  title: string
  desc: string
}

const API_ENDPOINTS: EndpointDef[] = [
  { method: 'GET', path: '/api/config', title: 'Server Config', desc: 'Live token limits: context_token_limit, soft_token_limit, hard_token_limit.' },
  { method: 'GET', path: '/api/metrics', title: 'System Metrics', desc: 'Uptime, per-event counts, recent event log (last 50).' },
  { method: 'GET', path: '/api/tracks', title: 'List Tracks', desc: 'All tracks with parent/child tree, branch list, created_at.' },
  { method: 'GET', path: '/api/tracks/:id', title: 'Get Track', desc: 'Track detail: concept map (all concepts + bloom levels) + session list.' },
  { method: 'GET', path: '/api/tracks/:id/context', title: 'Context Files', desc: 'All context files for a track (filename → raw content map).' },
  { method: 'GET', path: '/api/tracks/:id/context-tokens', title: 'Context Token Breakdown', desc: 'Per-file token counts for the full inherited context, attributed to the source track in the ancestor chain. Sorted by size descending.' },
  { method: 'GET', path: '/api/tracks/:id/split-plan', title: 'Split Plan', desc: 'Current shard split plan — null if corpus is within soft limit. Shows shard status, file assignments, token counts.' },
  { method: 'GET', path: '/api/tracks/:id/meta-synthesis', title: 'Meta-Synthesis', desc: 'Track-level synthesis aggregated across all shard sessions. Null if not yet generated.' },
  { method: 'GET', path: '/api/tracks/:id/meta-synthesis/readiness', title: 'Meta-Synthesis Readiness', desc: 'Which approved shards are still missing evaluated sessions. ready=true when all shards are covered.' },
  { method: 'GET', path: '/api/tracks/:id/sessions/:num/questions', title: 'Session Questions', desc: 'Generated questions for a session. Empty array if not yet generated.' },
  { method: 'GET', path: '/api/tracks/:id/sessions/:num/responses', title: 'Session Responses', desc: 'Submitted responses for a session.' },
  { method: 'GET', path: '/api/tracks/:id/sessions/:num/evaluations', title: 'Session Evaluations', desc: 'LLM evaluations per question — correctness, bloom level demonstrated, Brier score, calibration flag, feedback.' },
  { method: 'GET', path: '/api/tracks/:id/sessions/:num/synthesis', title: 'Session Synthesis', desc: 'Concept map update plan for a session — bloom advancement per concept, spaced repetition schedule.' },
  { method: 'GET', path: '/api/tracks/:id/sessions/:num/prompt-preview', title: 'Prompt Preview', desc: 'Exact system + user prompt that would be sent to the LLM for question generation — token estimates, call_mode (fresh/resumed), tokens_to_send, context files list.' },
  { method: 'GET', path: '/api/tracks/:id/sessions/:num/threads/:qid', title: 'Question Thread', desc: 'Full follow-up conversation history for one question. Each assistant message includes meta: call_mode, input_tokens_sent, context_chunks_used, claude_session_id.' },
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

// ── JSON syntax highlighter ──────────────────────────────────────────────────

function JSONView({ data }: { data: unknown }) {
  const raw = JSON.stringify(data, null, 2)
  // Colorize: keys, strings, numbers, booleans, null
  const html = raw
    .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;') // escape HTML first
    .replace(/("(?:[^"\\]|\\.)*")(\s*:)/g, '<span style="color:#7dd3fc">$1</span>$2') // keys → sky
    .replace(/:\s*("(?:[^"\\]|\\.)*")/g, (m, s) => m.replace(s, `<span style="color:#86efac">${s}</span>`)) // string values → green
    .replace(/:\s*(-?\d+\.?\d*(?:[eE][+-]?\d+)?)/g, (m, n) => m.replace(n, `<span style="color:#fcd34d">${n}</span>`)) // numbers → amber
    .replace(/:\s*(true|false)/g, (m, b) => m.replace(b, `<span style="color:#c4b5fd">${b}</span>`)) // booleans → violet
    .replace(/:\s*(null)/g, (m, n) => m.replace(n, `<span style="color:#f87171">${n}</span>`)) // null → red
  return (
    <pre
      className="text-xs font-mono bg-zinc-950 text-zinc-200 rounded p-3 overflow-auto max-h-[32rem] leading-relaxed"
      dangerouslySetInnerHTML={{ __html: html }}
    />
  )
}

// ── single endpoint row ───────────────────────────────────────────────────────

function EndpointRow({ ep }: { ep: EndpointDef }) {
  // Extract path params: ":id", ":num", ":qid" → ["id", "num", "qid"]
  const paramNames = (ep.path.match(/:[a-z]+/g) ?? []).map(p => p.slice(1))

  const [open, setOpen] = useState(false)
  const [params, setParams] = useState<Record<string, string>>(
    Object.fromEntries(paramNames.map(p => [p, '']))
  )
  const [result, setResult] = useState<unknown>(null)
  const [status, setStatus] = useState<number | null>(null)
  const [elapsed, setElapsed] = useState<number | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function execute() {
    setLoading(true)
    setError(null)
    setResult(null)
    setStatus(null)
    setElapsed(null)

    // Build resolved URL
    let url = ep.path
    for (const [k, v] of Object.entries(params)) {
      url = url.replace(`:${k}`, encodeURIComponent(v || `<${k}>`))
    }

    const t0 = performance.now()
    try {
      const res = await fetch(url)
      const ms = Math.round(performance.now() - t0)
      setStatus(res.status)
      setElapsed(ms)
      const body = await res.json().catch(() => null)
      setResult(body)
    } catch (e) {
      setElapsed(Math.round(performance.now() - t0))
      setError(String(e))
    } finally {
      setLoading(false)
    }
  }

  const methodColor = ep.method === 'GET'
    ? 'border-emerald-500 text-emerald-600'
    : 'border-amber-500 text-amber-600'

  const resolvedPath = paramNames.reduce(
    (p, k) => p.replace(`:${k}`, params[k] || `:${k}`),
    ep.path,
  )

  return (
    <div className="border border-border/50 rounded-lg overflow-hidden">
      {/* Header row — always visible, click to expand */}
      <button
        className="w-full flex items-center gap-3 px-4 py-3 text-left hover:bg-muted/30 transition-colors"
        onClick={() => setOpen(o => !o)}
      >
        <Badge variant="outline" className={`text-[10px] font-mono shrink-0 ${methodColor}`}>
          {ep.method}
        </Badge>
        <code className="text-xs font-mono text-primary flex-1 truncate">{ep.path}</code>
        <span className="text-xs text-muted-foreground shrink-0">{ep.title}</span>
        <span className="text-muted-foreground text-xs shrink-0">{open ? '▲' : '▼'}</span>
      </button>

      {/* Expanded panel */}
      {open && (
        <div className="border-t border-border/40 px-4 py-3 space-y-3 bg-muted/10">
          <p className="text-xs text-muted-foreground">{ep.desc}</p>

          {/* Param inputs */}
          {paramNames.length > 0 && (
            <div className="flex flex-wrap gap-2 items-center">
              {paramNames.map(p => (
                <label key={p} className="flex items-center gap-1.5 text-xs">
                  <span className="text-muted-foreground font-mono">:{p}</span>
                  <input
                    type="text"
                    value={params[p]}
                    onChange={e => setParams(prev => ({ ...prev, [p]: e.target.value }))}
                    onKeyDown={e => e.key === 'Enter' && execute()}
                    placeholder={p}
                    className="border rounded px-2 py-1 text-xs font-mono bg-background w-32 focus:outline-none focus:ring-1 focus:ring-primary"
                  />
                </label>
              ))}
            </div>
          )}

          {/* Resolved URL + execute */}
          <div className="flex items-center gap-2 flex-wrap">
            <code className="text-[10px] font-mono text-muted-foreground bg-muted/40 px-2 py-1 rounded flex-1 truncate">
              {resolvedPath}
            </code>
            <button
              onClick={execute}
              disabled={loading}
              className="text-xs px-3 py-1.5 rounded bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-50 font-medium shrink-0"
            >
              {loading ? 'Loading…' : 'Execute'}
            </button>
            {status != null && (
              <span className={`text-xs font-mono shrink-0 ${status < 300 ? 'text-emerald-500' : 'text-red-500'}`}>
                {status} · {elapsed}ms
              </span>
            )}
          </div>

          {/* Result */}
          {error && <p className="text-xs text-red-500">{error}</p>}
          {result != null && <JSONView data={result} />}
        </div>
      )}
    </div>
  )
}

// ── explorer ──────────────────────────────────────────────────────────────────

function APIExplorer() {
  return (
    <Card>
      <CardHeader>
        <CardTitle className="text-base">API Explorer</CardTitle>
      </CardHeader>
      <CardContent className="space-y-2">
        {API_ENDPOINTS.map(ep => (
          <EndpointRow key={ep.method + ep.path} ep={ep} />
        ))}
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

      <APIExplorer />
    </div>
  )
}
