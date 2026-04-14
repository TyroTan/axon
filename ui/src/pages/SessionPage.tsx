import { useEffect, useRef, useState } from 'react'
import { useParams, Link } from 'react-router-dom'
import { api } from '@/api/client'
import type { Evaluation, Question, Response } from '@/api/types'
import { buttonVariants } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

// ─── constants ────────────────────────────────────────────────────────────────

const BLOOM_LABELS = ['', 'Remember', 'Understand', 'Apply', 'Analyze', 'Evaluate', 'Create']
const BLOOM_COLORS = ['', 'bg-slate-500', 'bg-blue-700', 'bg-cyan-700', 'bg-green-700', 'bg-green-500', 'bg-indigo-500']
const FORMAT_LABELS: Record<string, string> = { mcq: 'MCQ', free_text: 'Free Text', scenario_mcq: 'Scenario', design: 'Design' }
const CONF_LABELS = ['', 'Guessing', 'Unsure', 'Neutral', 'Fairly sure', 'Certain']

// ─── types ────────────────────────────────────────────────────────────────────

interface LocalAnswer {
  answer: string        // MCQ key or free-text content
  confidence: number    // 1–5
  explanation: string   // optional written explanation
}

type StreamPhase = 'idle' | 'streaming' | 'done' | 'error'

// ─── sub-components ───────────────────────────────────────────────────────────

function BloomBadge({ level, label }: { level: number; label?: string }) {
  return (
    <Badge variant="secondary" className={cn('text-white text-[10px]', BLOOM_COLORS[level])}>
      L{level} {label ?? BLOOM_LABELS[level]}
    </Badge>
  )
}

function ConfidencePicker({ value, onChange }: { value: number; onChange: (v: number) => void }) {
  return (
    <div className="flex gap-1 items-center">
      <span className="text-xs text-muted-foreground w-20 shrink-0">Confidence:</span>
      {[1, 2, 3, 4, 5].map(n => (
        <button
          key={n}
          onClick={() => onChange(n)}
          className={cn(
            'w-7 h-7 rounded text-xs font-medium border transition-colors',
            value === n
              ? 'bg-primary text-primary-foreground border-primary'
              : 'bg-muted/40 text-muted-foreground hover:bg-muted border-transparent',
          )}
          title={CONF_LABELS[n]}
        >
          {n}
        </button>
      ))}
      {value > 0 && <span className="text-xs text-muted-foreground ml-1">{CONF_LABELS[value]}</span>}
    </div>
  )
}

// ─── answering card ───────────────────────────────────────────────────────────

function AnswerCard({
  q, index, answer, readonly, onChange,
}: {
  q: Question
  index: number
  answer: LocalAnswer
  readonly: boolean
  onChange: (a: LocalAnswer) => void
}) {
  const isMCQ = q.format === 'mcq' || q.format === 'scenario_mcq'

  return (
    <Card className="overflow-hidden">
      <CardHeader className="pb-2 flex-row items-start gap-3">
        <span className="text-xs font-mono text-muted-foreground pt-0.5 w-6 shrink-0">{index + 1}.</span>
        <div className="flex-1 min-w-0 space-y-1.5">
          <p className="text-sm leading-relaxed">{q.question}</p>
          <div className="flex flex-wrap gap-1.5">
            <BloomBadge level={q.bloom_level} label={q.bloom_label} />
            <Badge variant="outline" className="text-[10px]">{FORMAT_LABELS[q.format] ?? q.format}</Badge>
            {q.is_cross_branch && <Badge variant="outline" className="text-[10px]">Cross-branch</Badge>}
          </div>
        </div>
      </CardHeader>

      <CardContent className="pt-0 pl-9 space-y-3">
        {/* MCQ options */}
        {isMCQ && q.options && (
          <div className="space-y-1">
            {(['A', 'B', 'C', 'D'] as const).map(key => {
              const text = q.options?.[key]
              if (!text) return null
              const selected = answer.answer === key
              return (
                <button
                  key={key}
                  disabled={readonly}
                  onClick={() => onChange({ ...answer, answer: key })}
                  className={cn(
                    'w-full flex items-start gap-2 p-2.5 rounded-lg text-sm text-left transition-colors border',
                    selected
                      ? 'bg-primary/10 border-primary text-foreground'
                      : 'bg-muted/30 border-transparent hover:bg-muted/60 text-muted-foreground',
                    readonly && 'cursor-default',
                  )}
                >
                  <span className="font-mono font-bold shrink-0 mt-px">{key}.</span>
                  <span>{text}</span>
                </button>
              )
            })}
          </div>
        )}

        {/* Free text */}
        {(q.format === 'free_text' || q.format === 'design') && (
          <textarea
            value={answer.answer}
            readOnly={readonly}
            onChange={e => onChange({ ...answer, answer: e.target.value })}
            placeholder="Write your answer…"
            className="w-full resize-none text-sm p-2.5 rounded-lg border bg-muted/30 focus:outline-none focus:ring-1 focus:ring-ring min-h-[80px]"
          />
        )}

        {/* Explanation (always optional) */}
        <textarea
          value={answer.explanation}
          readOnly={readonly}
          onChange={e => onChange({ ...answer, explanation: e.target.value })}
          placeholder="Explain your reasoning… (optional)"
          className="w-full resize-none text-xs font-mono p-2.5 rounded-lg border bg-muted/20 focus:outline-none focus:ring-1 focus:ring-ring min-h-[48px] text-muted-foreground"
        />

        {/* Confidence picker */}
        {!readonly ? (
          <ConfidencePicker value={answer.confidence} onChange={v => onChange({ ...answer, confidence: v })} />
        ) : (
          <p className="text-xs text-muted-foreground">
            Confidence: <strong>{answer.confidence}/5</strong> — {CONF_LABELS[answer.confidence]}
          </p>
        )}
      </CardContent>
    </Card>
  )
}

// ─── evaluation card ──────────────────────────────────────────────────────────

function EvalCard({ q, evalResult }: { q: Question; evalResult: Evaluation }) {
  const pct = Math.round(evalResult.correctness * 100)
  const correct = evalResult.correctness >= 0.5
  return (
    <Card className={cn('overflow-hidden border-l-4', correct ? 'border-l-green-500' : 'border-l-red-500')}>
      <CardHeader className="pb-2 flex-row items-start gap-3">
        <span className="text-xs font-mono text-muted-foreground pt-0.5 w-6 shrink-0" />
        <div className="flex-1 min-w-0 space-y-1.5">
          <p className="text-sm leading-relaxed">{q.question}</p>

          {/* Score bar */}
          <div className="flex items-center gap-2">
            <div className="flex-1 h-1.5 rounded-full bg-muted overflow-hidden">
              <div
                className={cn('h-full rounded-full transition-all', correct ? 'bg-green-500' : 'bg-red-500')}
                style={{ width: `${pct}%` }}
              />
            </div>
            <span className="text-xs font-mono w-8 text-right">{pct}%</span>
          </div>

          {/* Badges */}
          <div className="flex flex-wrap gap-1.5">
            <BloomBadge level={evalResult.bloom_level_demonstrated} />
            {evalResult.calibration_flag && (
              <Badge variant="outline" className="text-[10px] text-yellow-600 border-yellow-400">
                {evalResult.calibration_flag}
              </Badge>
            )}
            {evalResult.error_taxonomy && (
              <Badge variant="outline" className="text-[10px] text-red-500 border-red-300">
                {evalResult.error_taxonomy.replace('_', ' ')}
              </Badge>
            )}
          </div>
        </div>
      </CardHeader>

      <CardContent className="pt-0 pl-9 space-y-2">
        {evalResult.feedback_for_learner && (
          <p className="text-sm text-muted-foreground leading-relaxed border-l-2 border-muted pl-3">
            {evalResult.feedback_for_learner}
          </p>
        )}
        {evalResult.misconception_identified && (
          <p className="text-xs text-orange-500">
            Misconception: {evalResult.misconception_identified}
          </p>
        )}
        {/* Correct answer reveal */}
        {q.correct && (
          <p className="text-xs text-muted-foreground">
            Correct answer: <span className="font-mono font-bold text-green-500">{q.correct}</span>
            {q.options?.[q.correct] && ` — ${q.options[q.correct]}`}
          </p>
        )}
        {q.correct_explanation && (
          <p className="text-xs text-muted-foreground leading-relaxed">{q.correct_explanation}</p>
        )}
      </CardContent>
    </Card>
  )
}

// ─── streaming overlay ────────────────────────────────────────────────────────

function StreamOverlay({ label, text }: { label: string; text: string }) {
  return (
    <Card>
      <CardHeader className="pb-2 flex-row items-center gap-2">
        <span className="w-2 h-2 rounded-full bg-primary animate-pulse shrink-0" />
        <span className="text-sm text-muted-foreground">{label}</span>
      </CardHeader>
      <CardContent>
        <pre className="text-xs font-mono text-muted-foreground whitespace-pre-wrap max-h-48 overflow-y-auto leading-relaxed">
          {text || ' '}
        </pre>
      </CardContent>
    </Card>
  )
}

// ─── main page ────────────────────────────────────────────────────────────────

export function SessionPage() {
  const { trackId, sessionNum } = useParams<{ trackId: string; sessionNum: string }>()
  const num = Number(sessionNum)

  // Remote data
  const [questions, setQuestions] = useState<Question[] | null>(null)
  const [savedResponses, setSavedResponses] = useState<Response[] | null>(null)
  const [evaluations, setEvaluations] = useState<Evaluation[] | null>(null)
  const [loading, setLoading] = useState(true)

  // Local answer state (pre-populated from savedResponses when present)
  const [answers, setAnswers] = useState<Record<string, LocalAnswer>>({})

  // Streaming states
  const [genPhase, setGenPhase] = useState<StreamPhase>('idle')
  const [evalPhase, setEvalPhase] = useState<StreamPhase>('idle')
  const [streamText, setStreamText] = useState('')
  const [streamError, setStreamError] = useState<string | null>(null)
  const streamRef = useRef('')

  // Submit state
  const [submitting, setSubmitting] = useState(false)
  const [submitError, setSubmitError] = useState<string | null>(null)

  // Load all session data on mount
  useEffect(() => {
    if (!trackId || !num) return
    Promise.allSettled([
      api.getSessionQuestions(trackId, num),
      api.getSessionResponses(trackId, num),
      api.getSessionEvaluations(trackId, num),
    ]).then(([qRes, rRes, eRes]) => {
      const qs = qRes.status === 'fulfilled' ? qRes.value.questions : null
      const rs = rRes.status === 'fulfilled' ? rRes.value.responses : null
      const es = eRes.status === 'fulfilled' ? eRes.value.evaluations : null

      setQuestions(qs)
      setSavedResponses(rs)
      setEvaluations(es)

      // Pre-populate answers from saved responses or blank defaults
      if (qs) {
        const init: Record<string, LocalAnswer> = {}
        for (const q of qs) {
          const saved = rs?.find(r => r.question_id === q.id)
          init[q.id] = saved
            ? { answer: saved.selected_answer, confidence: saved.confidence, explanation: saved.explanation }
            : { answer: '', confidence: 3, explanation: '' }
        }
        setAnswers(init)
      }
    }).finally(() => setLoading(false))
  }, [trackId, num])

  // ── actions ─────────────────────────────────────────────────────────────────

  async function generateQuestions() {
    if (!trackId || !num) return
    setGenPhase('streaming')
    setStreamError(null)
    streamRef.current = ''
    setStreamText('')
    try {
      await api.generateQuestions(trackId, num, text => {
        streamRef.current += text
        setStreamText(streamRef.current)
      })
      const result = await api.getSessionQuestions(trackId, num)
      setQuestions(result.questions)
      const init: Record<string, LocalAnswer> = {}
      for (const q of result.questions) init[q.id] = { answer: '', confidence: 3, explanation: '' }
      setAnswers(init)
      setSavedResponses(null)
      setEvaluations(null)
      setGenPhase('done')
    } catch (e) {
      setGenPhase('error')
      setStreamError(String(e))
    }
  }

  async function submitAll() {
    if (!trackId || !num || !questions) return
    setSubmitting(true)
    setSubmitError(null)
    try {
      const responses: Response[] = questions.map(q => ({
        question_id: q.id,
        generation_id: q.generation_id,
        selected_answer: answers[q.id]?.answer ?? '',
        confidence: answers[q.id]?.confidence ?? 3,
        explanation: answers[q.id]?.explanation ?? '',
        time_seconds: 0,
      }))
      await api.submitResponses(trackId, num, responses)
      setSavedResponses(responses)
    } catch (e) {
      setSubmitError(String(e))
    } finally {
      setSubmitting(false)
    }
  }

  async function evaluate() {
    if (!trackId || !num) return
    setEvalPhase('streaming')
    setStreamError(null)
    streamRef.current = ''
    setStreamText('')
    try {
      await api.evaluateResponses(trackId, num, text => {
        streamRef.current += text
        setStreamText(streamRef.current)
      })
      const result = await api.getSessionEvaluations(trackId, num)
      setEvaluations(result.evaluations)
      setEvalPhase('done')
    } catch (e) {
      setEvalPhase('error')
      setStreamError(String(e))
    }
  }

  // ── derived state ────────────────────────────────────────────────────────────

  const answeredCount = questions
    ? questions.filter(q => (answers[q.id]?.answer ?? '') !== '').length
    : 0
  const allAnswered = questions ? answeredCount === questions.length : false
  const evalByID = evaluations
    ? Object.fromEntries(evaluations.map(e => [e.question_id, e]))
    : null

  // ── render ───────────────────────────────────────────────────────────────────

  if (loading) return (
    <div className="max-w-3xl space-y-3">
      <Skeleton className="h-7 w-48" />
      <Skeleton className="h-36 rounded-xl" />
      <Skeleton className="h-36 rounded-xl" />
    </div>
  )

  return (
    <div className="max-w-3xl space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold font-mono">{trackId} / Session {num}</h1>
        <Link to={`/tracks/${trackId}`} className={cn(buttonVariants({ variant: 'outline', size: 'sm' }))}>
          ← Track
        </Link>
      </div>

      {/* ── no questions yet ─────────────────────────────────────────────────── */}
      {!questions && genPhase === 'idle' && (
        <Card>
          <CardContent className="py-10 flex flex-col items-center gap-3">
            <p className="text-sm text-muted-foreground">No questions generated yet.</p>
            <button onClick={generateQuestions} className={cn(buttonVariants({ size: 'sm' }))}>
              Generate Questions
            </button>
          </CardContent>
        </Card>
      )}

      {/* ── generation streaming ─────────────────────────────────────────────── */}
      {genPhase === 'streaming' && (
        <StreamOverlay label="Generating questions…" text={streamText} />
      )}

      {/* ── questions exist ──────────────────────────────────────────────────── */}
      {questions && questions.length > 0 && (
        <>
          {/* Phase header */}
          <div className="flex items-center justify-between">
            <p className="text-sm text-muted-foreground">
              {evaluations
                ? `${evaluations.length} evaluations complete`
                : savedResponses
                  ? 'Responses submitted — ready to evaluate'
                  : `${answeredCount} / ${questions.length} answered`}
            </p>
            <div className="flex gap-2">
              {genPhase !== 'streaming' && evalPhase !== 'streaming' && (
                <button
                  onClick={generateQuestions}
                  className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'text-xs')}
                >
                  Regenerate
                </button>
              )}
              {savedResponses && !evaluations && evalPhase === 'idle' && (
                <button onClick={evaluate} className={cn(buttonVariants({ size: 'sm' }))}>
                  Evaluate with AI
                </button>
              )}
            </div>
          </div>

          {/* Error banners */}
          {submitError && (
            <p className="text-xs text-destructive border border-destructive/30 rounded p-2">{submitError}</p>
          )}
          {streamError && (
            <div className="text-xs text-destructive border border-destructive/30 rounded p-2 flex items-center justify-between">
              {streamError}
              <button
                onClick={evalPhase === 'error' ? evaluate : generateQuestions}
                className={cn(buttonVariants({ variant: 'ghost', size: 'sm' }), 'text-xs')}
              >
                Retry
              </button>
            </div>
          )}

          {/* Evaluation streaming overlay */}
          {evalPhase === 'streaming' && (
            <StreamOverlay label="Evaluating responses…" text={streamText} />
          )}

          {/* Questions */}
          <div className="space-y-4">
            {questions.map((q, i) => {
              const evalResult = evalByID?.[q.id]
              if (evalResult) {
                return <EvalCard key={q.id} q={q} evalResult={evalResult} />
              }
              return (
                <AnswerCard
                  key={q.id}
                  q={q}
                  index={i}
                  answer={answers[q.id] ?? { answer: '', confidence: 3, explanation: '' }}
                  readonly={!!savedResponses}
                  onChange={a => setAnswers(prev => ({ ...prev, [q.id]: a }))}
                />
              )
            })}
          </div>

          {/* Submit bar — only shown in answering phase */}
          {!savedResponses && evalPhase === 'idle' && genPhase !== 'streaming' && (
            <div className="sticky bottom-4 flex justify-end">
              <button
                onClick={submitAll}
                disabled={submitting || !allAnswered}
                className={cn(
                  buttonVariants({ size: 'sm' }),
                  'shadow-lg',
                  (!allAnswered || submitting) && 'opacity-60 cursor-not-allowed',
                )}
              >
                {submitting ? 'Saving…' : `Submit Responses (${answeredCount}/${questions.length})`}
              </button>
            </div>
          )}
        </>
      )}
    </div>
  )
}
