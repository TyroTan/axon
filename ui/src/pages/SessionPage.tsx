import { useEffect, useRef, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { Question } from '@/api/types'
import { buttonVariants } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

const BLOOM_LABELS = ['', 'Remember', 'Understand', 'Apply', 'Analyze', 'Evaluate', 'Create']
const BLOOM_COLORS = [
  '', 'bg-slate-500', 'bg-blue-700', 'bg-cyan-700',
  'bg-green-700', 'bg-green-500', 'bg-indigo-500',
]
const FORMAT_LABELS: Record<string, string> = {
  mcq: 'MCQ', free_text: 'Free Text', scenario_mcq: 'Scenario', design: 'Design',
}

function QuestionCard({ q, index }: { q: Question; index: number }) {
  const [revealed, setRevealed] = useState(false)
  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2 flex-row items-start gap-3">
        <span className="text-xs font-mono text-muted-foreground pt-0.5 w-6 shrink-0">
          {index + 1}.
        </span>
        <div className="flex-1 min-w-0">
          <p className="text-sm leading-relaxed">{q.question}</p>
          <div className="flex flex-wrap gap-1.5 mt-2">
            <Badge
              variant="secondary"
              className={cn('text-white text-[10px]', BLOOM_COLORS[q.bloom_level])}
            >
              L{q.bloom_level} {BLOOM_LABELS[q.bloom_level]}
            </Badge>
            <Badge variant="outline" className="text-[10px]">{FORMAT_LABELS[q.format] ?? q.format}</Badge>
            {q.is_cross_branch && <Badge variant="outline" className="text-[10px]">Cross-branch</Badge>}
          </div>
        </div>
      </CardHeader>

      {q.options && (
        <CardContent className="pt-0 pl-9 space-y-1">
          {Object.entries(q.options).map(([key, text]) => (
            <div
              key={key}
              className={cn(
                'flex items-start gap-2 p-2 rounded text-sm transition-colors',
                revealed && key === q.correct
                  ? 'bg-green-500/15 text-green-600 dark:text-green-400'
                  : revealed && key !== q.correct
                    ? 'text-muted-foreground'
                    : 'hover:bg-muted/50',
              )}
            >
              <span className="font-mono font-bold shrink-0">{key}.</span>
              <span>{text}</span>
            </div>
          ))}
          {!revealed ? (
            <button
              onClick={() => setRevealed(true)}
              className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'mt-1 text-xs')}
            >
              Show answer
            </button>
          ) : (
            <div className="mt-2 text-xs text-muted-foreground leading-relaxed border-l-2 border-green-500/50 pl-2">
              {q.correct_explanation}
            </div>
          )}
        </CardContent>
      )}

      {q.format === 'free_text' && (
        <CardContent className="pt-0 pl-9">
          <textarea
            placeholder="Type your answer…"
            className="w-full resize-none text-sm p-2 rounded border bg-muted/30 focus:outline-none focus:ring-1 focus:ring-ring min-h-[80px]"
          />
          {!revealed ? (
            <button
              onClick={() => setRevealed(true)}
              className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'mt-1 text-xs')}
            >
              Show model answer
            </button>
          ) : (
            <div className="mt-2 text-xs text-muted-foreground leading-relaxed border-l-2 border-green-500/50 pl-2">
              {q.correct_explanation}
            </div>
          )}
        </CardContent>
      )}
    </Card>
  )
}

type GenState = 'idle' | 'generating' | 'done' | 'error'

export function SessionPage() {
  const { trackId, sessionNum } = useParams<{ trackId: string; sessionNum: string }>()
  const num = Number(sessionNum)

  const [questions, setQuestions] = useState<Question[] | null>(null)
  const [loading, setLoading] = useState(true)

  const [genState, setGenState] = useState<GenState>('idle')
  const [genError, setGenError] = useState<string | null>(null)
  const [streamText, setStreamText] = useState('')
  const streamRef = useRef('')

  useEffect(() => {
    if (!trackId || !num) return
    api.getSessionQuestions(trackId, num)
      .then(r => setQuestions(r.questions))
      .catch(() => setQuestions(null)) // 404 = no questions yet, not an error
      .finally(() => setLoading(false))
  }, [trackId, num])

  async function startGeneration() {
    if (!trackId || !num) return
    setGenState('generating')
    setGenError(null)
    setStreamText('')
    streamRef.current = ''
    try {
      await api.generateQuestions(trackId, num, (text) => {
        streamRef.current += text
        setStreamText(streamRef.current)
      })
      // Reload questions from disk.
      const result = await api.getSessionQuestions(trackId, num)
      setQuestions(result.questions)
      setGenState('done')
    } catch (e) {
      setGenState('error')
      setGenError(String(e))
    }
  }

  if (loading) return (
    <div className="space-y-3 max-w-3xl">
      <Skeleton className="h-7 w-48" />
      <Skeleton className="h-32 rounded-xl" />
      <Skeleton className="h-32 rounded-xl" />
    </div>
  )

  const hasQuestions = questions && questions.length > 0

  return (
    <div className="max-w-3xl space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-bold font-mono">{trackId} / Session {num}</h1>
        </div>
        <Link
          to={`/tracks/${trackId}`}
          className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}
        >
          ← Track
        </Link>
      </div>

      {/* Generation panel — shown when no questions yet, or always as re-gen */}
      {!hasQuestions && genState === 'idle' && (
        <Card>
          <CardContent className="py-8 flex flex-col items-center gap-3 text-center">
            <p className="text-sm text-muted-foreground">No questions generated yet.</p>
            <button
              onClick={startGeneration}
              className={cn(buttonVariants({ size: 'sm' }))}
            >
              Generate Questions
            </button>
          </CardContent>
        </Card>
      )}

      {genState === 'generating' && (
        <Card>
          <CardHeader className="pb-2">
            <CardTitle className="text-sm text-muted-foreground flex items-center gap-2">
              <span className="inline-block w-2 h-2 rounded-full bg-primary animate-pulse" />
              Generating questions…
            </CardTitle>
          </CardHeader>
          <CardContent>
            <pre className="text-xs text-muted-foreground font-mono whitespace-pre-wrap max-h-64 overflow-y-auto leading-relaxed">
              {streamText || ' '}
            </pre>
          </CardContent>
        </Card>
      )}

      {genState === 'error' && (
        <div className="text-sm text-destructive p-3 rounded-lg border border-destructive/30 bg-destructive/5">
          {genError}
          <button
            onClick={startGeneration}
            className={cn(buttonVariants({ variant: 'outline', size: 'sm' }), 'ml-3')}
          >
            Retry
          </button>
        </div>
      )}

      {/* Questions list */}
      {hasQuestions && (
        <>
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">{questions.length} questions</p>
            {genState !== 'generating' && (
              <button
                onClick={startGeneration}
                className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'text-xs')}
              >
                Regenerate
              </button>
            )}
          </div>
          <div className="space-y-3">
            {questions.map((q, i) => <QuestionCard key={q.id} q={q} index={i} />)}
          </div>
        </>
      )}
    </div>
  )
}
