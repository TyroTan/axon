import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import ReactMarkdown from 'react-markdown'
import { api } from '@/api/client'
import type {
  Conversation,
  ConversationMessage,
  ConversationIndex,
  ConversationPlyIndex,
  Track,
} from '@/api/types'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Badge } from '@/components/ui/badge'
import { Skeleton } from '@/components/ui/skeleton'
import { buttonVariants } from '@/components/ui/button'
import { cn } from '@/lib/utils'

// ─── helpers ──────────────────────────────────────────────────────────────────

function callModeBadge(mode: string) {
  const cls = mode === 'resumed'
    ? 'bg-green-900/60 text-green-300 border-green-700'
    : 'bg-amber-900/60 text-amber-300 border-amber-700'
  return (
    <span className={cn('text-[10px] font-mono border rounded px-1 py-px', cls)}>
      {mode}
    </span>
  )
}

function MsgMeta({ msg }: { msg: ConversationMessage }) {
  if (!msg.meta) return null
  return (
    <div className="flex flex-wrap gap-1 mt-1">
      {callModeBadge(msg.meta.call_mode)}
      {msg.meta.input_tokens_sent > 0 && (
        <span className="text-[10px] font-mono text-muted-foreground border border-border rounded px-1 py-px">
          {(msg.meta.input_tokens_sent / 1000).toFixed(1)}k tok
        </span>
      )}
      {msg.meta.context_chunks_used > 0 && (
        <span className="text-[10px] font-mono text-muted-foreground border border-border rounded px-1 py-px">
          {msg.meta.context_chunks_used} chunks
        </span>
      )}
      {msg.meta.claude_session_id && (
        <span className="text-[10px] font-mono text-muted-foreground border border-border rounded px-1 py-px">
          sid:{msg.meta.claude_session_id.slice(0, 8)}
        </span>
      )}
    </div>
  )
}

function MessageBubble({ msg, idx }: { msg: ConversationMessage; idx: number }) {
  const isUser = msg.role === 'user'
  return (
    <div className={cn('flex flex-col gap-0.5', isUser ? 'items-end' : 'items-start')}>
      <div className={cn(
        'max-w-[80%] rounded-lg px-3 py-2 text-sm',
        isUser
          ? 'bg-primary text-primary-foreground whitespace-pre-wrap'
          : 'bg-muted text-foreground',
      )}>
        <span className="text-[10px] font-mono opacity-50 select-none mr-2">#{idx}</span>
        {isUser ? msg.content : (
          <div className="space-y-2 [&_h1]:text-base [&_h1]:font-bold [&_h2]:text-sm [&_h2]:font-semibold [&_h3]:text-sm [&_h3]:font-medium [&_ul]:list-disc [&_ul]:pl-4 [&_ol]:list-decimal [&_ol]:pl-4 [&_li]:my-0.5 [&_code]:bg-black/30 [&_code]:rounded [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-xs [&_pre]:bg-black/30 [&_pre]:rounded [&_pre]:p-2 [&_pre]:overflow-x-auto [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_strong]:font-semibold [&_a]:underline [&_blockquote]:border-l-2 [&_blockquote]:pl-2 [&_blockquote]:opacity-70">
            <ReactMarkdown>{msg.content}</ReactMarkdown>
          </div>
        )}
      </div>
      {!isUser && <MsgMeta msg={msg} />}
    </div>
  )
}

// ─── Index Panel ──────────────────────────────────────────────────────────────

function IndexPanel({ idx }: { idx: ConversationIndex }) {
  return (
    <div className="space-y-3 text-sm">
      <p className="text-muted-foreground">{idx.summary}</p>

      {idx.topics.length > 0 && (
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1">Topics</p>
          <div className="flex flex-wrap gap-1">
            {idx.topics.map(t => <Badge key={t} variant="secondary">{t}</Badge>)}
          </div>
        </div>
      )}

      {idx.key_decisions.length > 0 && (
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1">Key decisions</p>
          <ul className="list-disc list-inside space-y-0.5 text-muted-foreground">
            {idx.key_decisions.map((d, i) => <li key={i}>{d}</li>)}
          </ul>
        </div>
      )}

      {idx.open_questions.length > 0 && (
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1">Open questions</p>
          <ul className="list-disc list-inside space-y-0.5 text-muted-foreground">
            {idx.open_questions.map((q, i) => <li key={i}>{q}</li>)}
          </ul>
        </div>
      )}

      {idx.concept_indexes_referenced.length > 0 && (
        <div>
          <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-1">Concept map references</p>
          <div className="flex flex-wrap gap-1">
            {idx.concept_indexes_referenced.map(n => (
              <span key={n} className="text-[10px] font-mono border border-border rounded px-1 py-px text-muted-foreground">[{n}]</span>
            ))}
          </div>
        </div>
      )}

      {idx.ply_index.length > 0 && (
        <details className="group">
          <summary className="cursor-pointer text-xs font-semibold uppercase tracking-wider text-muted-foreground select-none">
            Per-turn index ({idx.ply_index.length} plies)
          </summary>
          <div className="mt-2 space-y-2 border-l border-border pl-3">
            {idx.ply_index.map((p: ConversationPlyIndex) => (
              <div key={p.turn} className="text-xs">
                <span className="font-mono text-muted-foreground">#{p.turn} {p.role}</span>
                {p.tokens > 0 && <span className="text-muted-foreground ml-1">· {p.tokens}t</span>}
                <p className="text-foreground mt-0.5">{p.summary}</p>
                {p.key_points && p.key_points.length > 0 && (
                  <ul className="list-disc list-inside text-muted-foreground mt-0.5">
                    {p.key_points.map((kp, i) => <li key={i}>{kp}</li>)}
                  </ul>
                )}
              </div>
            ))}
          </div>
        </details>
      )}

      <p className="text-[10px] text-muted-foreground font-mono">
        indexed_at: {new Date(idx.indexed_at).toLocaleString()} · gen: {idx.generation_id.slice(0, 8)}
      </p>
    </div>
  )
}

// ─── New Conversation Form ─────────────────────────────────────────────────────

export function NewConversationPage() {
  const navigate = useNavigate()
  const [allTracks, setAllTracks] = useState<Track[]>([])
  const [selected, setSelected] = useState<string[]>([])
  const [title, setTitle] = useState('')
  const [creating, setCreating] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  useEffect(() => {
    api.listTracks().then(r => {
      const flat: Track[] = []
      const walk = (ts: Track[]) => ts.forEach(t => { flat.push(t); if (t.children) walk(t.children) })
      walk(r.tracks ?? [])
      setAllTracks(flat)
    })
  }, [])

  async function create() {
    if (selected.length === 0 || creating) return
    setCreating(true)
    try {
      const conv = await api.createConversation(selected, title)
      navigate(`/conversations/${conv.id}`)
    } catch (e) {
      setErr(String(e))
      setCreating(false)
    }
  }

  return (
    <div className="max-w-lg space-y-5">
      <div>
        <h1 className="text-xl font-bold">New Conversation</h1>
        <p className="text-sm text-muted-foreground mt-1">
          Select the context tracks to load, then start chatting. You can add more tracks mid-conversation.
        </p>
      </div>

      {err && <p className="text-sm text-red-400">{err}</p>}

      <div className="space-y-2">
        <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Context tracks</p>
        <div className="space-y-1 max-h-64 overflow-y-auto border border-border rounded p-2">
          {allTracks.map(t => (
            <label key={t.id} className="flex items-center gap-2 text-sm cursor-pointer">
              <input
                type="checkbox"
                className="accent-primary"
                checked={selected.includes(t.id)}
                onChange={e => setSelected(prev =>
                  e.target.checked ? [...prev, t.id] : prev.filter(x => x !== t.id)
                )}
              />
              <span className="font-mono">{t.id}</span>
              {t.is_composite && <Badge variant="outline" className="text-[10px] py-0 border-amber-500/50 text-amber-400">composite</Badge>}
              {t.major_branches && (
                <span className="text-xs text-muted-foreground truncate">{t.major_branches.slice(0, 2).join(' · ')}</span>
              )}
            </label>
          ))}
        </div>
      </div>

      <div className="space-y-1">
        <p className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">Title (optional)</p>
        <input
          type="text"
          value={title}
          onChange={e => setTitle(e.target.value)}
          placeholder="e.g. RAG internals deep dive"
          className="w-full bg-background border border-border rounded px-3 py-1.5 text-sm"
          onKeyDown={e => e.key === 'Enter' && create()}
        />
      </div>

      <button
        disabled={selected.length === 0 || creating}
        onClick={create}
        className={cn(buttonVariants(), (selected.length === 0 || creating) && 'opacity-50 cursor-not-allowed')}
      >
        {creating ? 'Creating…' : `Start conversation (${selected.length} track${selected.length !== 1 ? 's' : ''})`}
      </button>
    </div>
  )
}

// ─── Conversation Page ────────────────────────────────────────────────────────

export function ConversationPage() {
  const { convId } = useParams<{ convId: string }>()
  const [conv, setConv] = useState<Conversation | null>(null)
  const [idx, setIdx] = useState<ConversationIndex | null>(null)
  const [loading, setLoading] = useState(true)
  const [err, setErr] = useState<string | null>(null)

  // Input state
  const [input, setInput] = useState('')
  const [adhoc, setAdhoc] = useState('')
  const [adhocOpen, setAdhocOpen] = useState(false)
  const [streaming, setStreaming] = useState(false)
  const [streamBuf, setStreamBuf] = useState('')

  // Context add
  const [ctxOpen, setCtxOpen] = useState(false)
  const [allTracks, setAllTracks] = useState<Track[]>([])
  const [ctxSelected, setCtxSelected] = useState<string[]>([])
  const [addingCtx, setAddingCtx] = useState(false)

  // Index
  const [indexOpen, setIndexOpen] = useState(false)
  const [indexing, setIndexing] = useState(false)
  const [indexStream, setIndexStream] = useState('')

  const messagesEndRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!convId) return
    api.getConversation(convId)
      .then(r => { setConv(r.conversation); if (r.index) setIdx(r.index) })
      .catch(e => setErr(String(e)))
      .finally(() => setLoading(false))
  }, [convId])

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [conv?.messages.length, streaming])

  async function send() {
    if (!convId || !conv || streaming || !input.trim()) return
    setStreaming(true)
    setStreamBuf('')
    const userMsg = input.trim()
    const adhocMsg = adhoc.trim()
    setInput('')
    // Optimistically append user message
    setConv(prev => prev ? {
      ...prev,
      messages: [...prev.messages, { role: 'user', content: userMsg, ts: new Date().toISOString() }]
    } : prev)
    try {
      await api.conversationTurn(convId, userMsg, adhocMsg, chunk => {
        setStreamBuf(prev => prev + chunk)
      })
      // Reload full conversation to get persisted state with meta
      const r = await api.getConversation(convId)
      setConv(r.conversation)
      if (r.index) setIdx(r.index)
    } catch (e) {
      setErr(String(e))
    } finally {
      setStreaming(false)
      setStreamBuf('')
      setAdhoc('')
    }
  }

  async function addContext() {
    if (!convId || ctxSelected.length === 0 || addingCtx) return
    setAddingCtx(true)
    try {
      const updated = await api.addConversationContext(convId, ctxSelected)
      setConv(updated)
      setCtxSelected([])
      setCtxOpen(false)
    } catch (e) {
      setErr(String(e))
    } finally {
      setAddingCtx(false)
    }
  }

  async function openCtxPanel() {
    if (ctxOpen) { setCtxOpen(false); return }
    const r = await api.listTracks()
    const flat: Track[] = []
    const walk = (ts: Track[]) => ts.forEach(t => { flat.push(t); if (t.children) walk(t.children) })
    walk(r.tracks ?? [])
    const existing = new Set(conv?.track_ids ?? [])
    setAllTracks(flat.filter(t => !existing.has(t.id)))
    setCtxOpen(true)
  }

  async function runIndex() {
    if (!convId || indexing) return
    setIndexing(true)
    setIndexStream('')
    try {
      const result = await api.indexConversation(convId, chunk => {
        setIndexStream(prev => prev + chunk)
      })
      setIdx(result)
      setIndexStream('')
    } catch (e) {
      setErr(String(e))
    } finally {
      setIndexing(false)
    }
  }

  if (loading) return (
    <div className="space-y-3 max-w-2xl">
      {[1,2,3].map(i => <Skeleton key={i} className="h-12 w-full rounded" />)}
    </div>
  )
  if (err) return <p className="text-red-400 text-sm">{err}</p>
  if (!conv) return <p className="text-muted-foreground text-sm">Conversation not found.</p>

  return (
    <div className="flex flex-col h-[calc(100vh-4rem)] max-w-3xl gap-3">

      {/* Header */}
      <div className="flex items-start justify-between shrink-0">
        <div>
          <h1 className="text-lg font-bold">{conv.title || 'Conversation'}</h1>
          <div className="flex flex-wrap gap-1 mt-1">
            {conv.track_ids.map(tid => (
              <Link key={tid} to={`/tracks/${tid}`} className="text-[11px] font-mono text-muted-foreground hover:text-foreground underline underline-offset-2">{tid}</Link>
            ))}
            {conv.accumulated_input_tokens > 0 && (
              <span className="text-[10px] font-mono text-muted-foreground border border-border rounded px-1 py-px">
                {(conv.accumulated_input_tokens / 1000).toFixed(1)}k tok total
              </span>
            )}
          </div>
        </div>
        <div className="flex gap-2 shrink-0">
          <button
            onClick={openCtxPanel}
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
          >
            + Context
          </button>
          <button
            onClick={() => setIndexOpen(o => !o)}
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), idx && 'border-green-600')}
          >
            {idx ? 'Index ✓' : 'Index'}
          </button>
        </div>
      </div>

      {/* Add context panel */}
      {ctxOpen && (
        <Card className="shrink-0 border-sky-500/40 bg-sky-950/10">
          <CardHeader className="pb-1 pt-2">
            <CardTitle className="text-xs font-semibold text-sky-400 flex justify-between">
              <span>Add context tracks</span>
              <button onClick={() => setCtxOpen(false)} className="text-muted-foreground hover:text-foreground">✕</button>
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-2 pb-3">
            <div className="space-y-1 max-h-40 overflow-y-auto">
              {allTracks.length === 0
                ? <p className="text-xs text-muted-foreground">All tracks already loaded.</p>
                : allTracks.map(t => (
                  <label key={t.id} className="flex items-center gap-2 text-sm cursor-pointer">
                    <input
                      type="checkbox"
                      className="accent-sky-400"
                      checked={ctxSelected.includes(t.id)}
                      onChange={e => setCtxSelected(prev =>
                        e.target.checked ? [...prev, t.id] : prev.filter(x => x !== t.id)
                      )}
                    />
                    <span className="font-mono">{t.id}</span>
                    {t.major_branches && <span className="text-xs text-muted-foreground">{t.major_branches.slice(0,2).join(' · ')}</span>}
                  </label>
                ))
              }
            </div>
            <button
              disabled={ctxSelected.length === 0 || addingCtx}
              onClick={addContext}
              className={cn(buttonVariants({ size: 'sm' }), (ctxSelected.length === 0 || addingCtx) && 'opacity-50 cursor-not-allowed')}
            >
              {addingCtx ? 'Adding…' : `Add ${ctxSelected.length} track${ctxSelected.length !== 1 ? 's' : ''}`}
            </button>
          </CardContent>
        </Card>
      )}

      {/* Index panel */}
      {indexOpen && (
        <Card className="shrink-0 border-violet-500/40 bg-violet-950/10">
          <CardHeader className="pb-1 pt-2">
            <CardTitle className="text-xs font-semibold text-violet-400 flex justify-between">
              <span>Conversation Index</span>
              <button onClick={() => setIndexOpen(false)} className="text-muted-foreground hover:text-foreground">✕</button>
            </CardTitle>
          </CardHeader>
          <CardContent className="pb-3 space-y-2">
            {!idx && !indexing && (
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">
                  Generate a structured index: summary, topics, key decisions, open questions, per-turn breakdown, concept map references.
                </p>
                <button
                  onClick={runIndex}
                  className={cn(buttonVariants({ size: 'sm' }))}
                >
                  Generate index
                </button>
              </div>
            )}
            {indexing && (
              <div className="space-y-1">
                <p className="text-xs text-muted-foreground">Indexing…</p>
                {indexStream && (
                  <pre className="text-xs text-muted-foreground bg-muted rounded p-2 max-h-32 overflow-y-auto whitespace-pre-wrap">{indexStream}</pre>
                )}
              </div>
            )}
            {idx && !indexing && (
              <div className="space-y-2">
                <IndexPanel idx={idx} />
                <button
                  onClick={runIndex}
                  className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
                >
                  Re-index
                </button>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-y-auto space-y-3 pr-1">
        {conv.messages.length === 0 && (
          <p className="text-sm text-muted-foreground text-center mt-8">
            Start the conversation below. Context from {conv.track_ids.length} track{conv.track_ids.length !== 1 ? 's' : ''} will be retrieved per turn.
          </p>
        )}
        {conv.messages.map((m, i) => (
          <MessageBubble key={i} msg={m} idx={i} />
        ))}
        {streaming && streamBuf && (
          <div className="flex flex-col items-start gap-0.5">
            <div className="max-w-[80%] rounded-lg px-3 py-2 text-sm bg-muted text-foreground">
              <div className="space-y-2 [&_h1]:text-base [&_h1]:font-bold [&_h2]:text-sm [&_h2]:font-semibold [&_h3]:text-sm [&_h3]:font-medium [&_ul]:list-disc [&_ul]:pl-4 [&_ol]:list-decimal [&_ol]:pl-4 [&_li]:my-0.5 [&_code]:bg-black/30 [&_code]:rounded [&_code]:px-1 [&_code]:py-0.5 [&_code]:font-mono [&_code]:text-xs [&_pre]:bg-black/30 [&_pre]:rounded [&_pre]:p-2 [&_pre]:overflow-x-auto [&_pre_code]:bg-transparent [&_pre_code]:p-0 [&_strong]:font-semibold [&_a]:underline [&_blockquote]:border-l-2 [&_blockquote]:pl-2 [&_blockquote]:opacity-70">
                <ReactMarkdown>{streamBuf}</ReactMarkdown>
              </div>
              <span className="animate-pulse ml-1">▍</span>
            </div>
          </div>
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Ad-hoc context inject */}
      {adhocOpen && (
        <div className="shrink-0 space-y-1">
          <p className="text-xs text-muted-foreground font-semibold uppercase tracking-wider">
            Ad-hoc context (injected into next turn only)
          </p>
          <textarea
            value={adhoc}
            onChange={e => setAdhoc(e.target.value)}
            placeholder="Paste a document excerpt, code snippet, or any text you want the LLM to have for this turn…"
            rows={4}
            className="w-full bg-background border border-border rounded px-3 py-2 text-sm font-mono resize-none"
          />
        </div>
      )}

      {/* Input bar */}
      <div className="shrink-0 flex gap-2 items-end">
        <button
          onClick={() => setAdhocOpen(o => !o)}
          title="Inject ad-hoc context for next turn"
          className={cn(
            buttonVariants({ variant: 'outline', size: 'sm' }),
            adhocOpen && 'border-amber-500 text-amber-400',
            adhoc && 'border-amber-500',
          )}
        >
          {adhoc ? '📄✓' : '📄'}
        </button>
        <textarea
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={e => {
            if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); send() }
          }}
          placeholder="Ask anything… (Enter to send, Shift+Enter for newline)"
          rows={2}
          disabled={streaming}
          className="flex-1 bg-background border border-border rounded px-3 py-2 text-sm resize-none disabled:opacity-50"
        />
        <button
          onClick={send}
          disabled={streaming || !input.trim()}
          className={cn(buttonVariants({ size: 'sm' }), (streaming || !input.trim()) && 'opacity-50 cursor-not-allowed')}
        >
          {streaming ? '…' : 'Send'}
        </button>
      </div>
    </div>
  )
}
