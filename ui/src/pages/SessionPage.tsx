import { useEffect, useRef, useState } from "react";
import { useParams, Link } from "react-router-dom";
import { api } from "@/api/client";
import type {
  ConceptMapUpdate,
  Evaluation,
  PromptPreviewResult,
  Question,
  Response,
  Synthesis,
  ThreadMessage,
} from "@/api/types";
import { buttonVariants } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";

// ─── constants ────────────────────────────────────────────────────────────────

const BLOOM_LABELS = [
  "",
  "Remember",
  "Understand",
  "Apply",
  "Analyze",
  "Evaluate",
  "Create",
];
const BLOOM_COLORS = [
  "",
  "bg-slate-500",
  "bg-blue-700",
  "bg-cyan-700",
  "bg-green-700",
  "bg-green-500",
  "bg-indigo-500",
];
const FORMAT_LABELS: Record<string, string> = {
  mcq: "MCQ",
  free_text: "Free Text",
  scenario_mcq: "Scenario",
  design: "Design",
};
const CONF_LABELS = [
  "",
  "Guessing",
  "Unsure",
  "Neutral",
  "Fairly sure",
  "Certain",
];

// ─── types ────────────────────────────────────────────────────────────────────

interface LocalAnswer {
  answer: string; // MCQ key or free-text content
  confidence: number; // 1–5
  explanation: string; // optional written explanation
}

type StreamPhase = "idle" | "streaming" | "done" | "error";

// ─── sub-components ───────────────────────────────────────────────────────────

function BloomBadge({ level, label }: { level: number; label?: string }) {
  return (
    <Badge
      variant='secondary'
      className={cn("text-white text-[10px]", BLOOM_COLORS[level])}
    >
      L{level} {label ?? BLOOM_LABELS[level]}
    </Badge>
  );
}

function ConfidencePicker({
  value,
  onChange,
}: {
  value: number;
  onChange: (v: number) => void;
}) {
  return (
    <div className='flex gap-1 items-center'>
      <span className='text-xs text-muted-foreground w-20 shrink-0'>
        Confidence:
      </span>
      {[1, 2, 3, 4, 5].map((n) => (
        <button
          key={n}
          onClick={() => onChange(n)}
          className={cn(
            "w-7 h-7 rounded text-xs font-medium border transition-colors",
            value === n
              ? "bg-primary text-primary-foreground border-primary"
              : "bg-muted/40 text-muted-foreground hover:bg-muted border-transparent",
          )}
          title={CONF_LABELS[n]}
        >
          {n}
        </button>
      ))}
      {value > 0 && (
        <span className='text-xs text-muted-foreground ml-1'>
          {CONF_LABELS[value]}
        </span>
      )}
    </div>
  );
}

// ─── answering card ───────────────────────────────────────────────────────────

function AnswerCard({
  q,
  index,
  answer,
  readonly,
  onChange,
}: {
  q: Question;
  index: number;
  answer: LocalAnswer;
  readonly: boolean;
  onChange: (a: LocalAnswer) => void;
}) {
  const isMCQ = q.format === "mcq" || q.format === "scenario_mcq";

  return (
    <Card className='overflow-hidden'>
      <CardHeader className='pb-2 flex-row items-start gap-3'>
        <span className='text-xs font-mono text-muted-foreground pt-0.5 w-6 shrink-0'>
          {index + 1}.
        </span>
        <div className='flex-1 min-w-0 space-y-1.5'>
          <p className='text-sm leading-relaxed'>{q.question}</p>
          <div className='flex flex-wrap gap-1.5'>
            <BloomBadge level={q.bloom_level} label={q.bloom_label} />
            <Badge variant='outline' className='text-[10px]'>
              {FORMAT_LABELS[q.format] ?? q.format}
            </Badge>
            {q.is_cross_branch && (
              <Badge variant='outline' className='text-[10px]'>
                Cross-branch
              </Badge>
            )}
          </div>
        </div>
      </CardHeader>

      <CardContent className='pt-0 pl-9 space-y-3'>
        {/* MCQ options */}
        {isMCQ && q.options && (
          <div className='space-y-1'>
            {(["A", "B", "C", "D"] as const).map((key) => {
              const text = q.options?.[key];
              if (!text) return null;
              const selected = answer.answer === key;
              return (
                <button
                  key={key}
                  disabled={readonly}
                  onClick={() => onChange({ ...answer, answer: key })}
                  className={cn(
                    "w-full flex items-start gap-2 p-2.5 rounded-lg text-sm text-left transition-colors border",
                    selected
                      ? "bg-primary/10 border-primary text-foreground"
                      : "bg-muted/30 border-transparent hover:bg-muted/60 text-muted-foreground",
                    readonly && "cursor-default",
                  )}
                >
                  <span className='font-mono font-bold shrink-0 mt-px'>
                    {key}.
                  </span>
                  <span>{text}</span>
                </button>
              );
            })}
          </div>
        )}

        {/* Free text */}
        {(q.format === "free_text" || q.format === "design") && (
          <textarea
            value={answer.answer}
            readOnly={readonly}
            onChange={(e) => onChange({ ...answer, answer: e.target.value })}
            placeholder='Write your answer…'
            className='w-full resize-none text-sm p-2.5 rounded-lg border bg-muted/30 focus:outline-none focus:ring-1 focus:ring-ring min-h-[80px]'
          />
        )}

        {/* Explanation (always optional) */}
        <textarea
          value={answer.explanation}
          readOnly={readonly}
          onChange={(e) => onChange({ ...answer, explanation: e.target.value })}
          placeholder='Explain your reasoning… (optional)'
          className='w-full resize-none text-xs font-mono p-2.5 rounded-lg border bg-muted/20 focus:outline-none focus:ring-1 focus:ring-ring min-h-[48px] text-muted-foreground'
        />

        {/* Confidence picker */}
        {!readonly ? (
          <ConfidencePicker
            value={answer.confidence}
            onChange={(v) => onChange({ ...answer, confidence: v })}
          />
        ) : (
          <p className='text-xs text-muted-foreground'>
            Confidence: <strong>{answer.confidence}/5</strong> —{" "}
            {CONF_LABELS[answer.confidence]}
          </p>
        )}
      </CardContent>
    </Card>
  );
}

// ─── evaluation card ──────────────────────────────────────────────────────────

function EvalCard({
  q, evalResult, response, trackId, sessionNum,
}: {
  q: Question;
  evalResult: Evaluation;
  response?: Response;
  trackId: string;
  sessionNum: number;
}) {
  const pct = Math.round(evalResult.correctness * 100);
  const correct = evalResult.correctness >= 0.5;
  const isMCQ = q.format === 'mcq' || q.format === 'scenario_mcq'
  const selectedKey = response?.selected_answer
  const explanationScore = Math.round((evalResult.explanation_score ?? 0) * 100)

  // ── thread state ──────────────────────────────────────────────────────────
  const [threadOpen, setThreadOpen] = useState(false)
  const [messages, setMessages] = useState<ThreadMessage[]>([])
  const [threadLoaded, setThreadLoaded] = useState(false)
  const [seeding, setSeeding] = useState(false)
  const [streaming, setStreaming] = useState(false)
  const [inputText, setInputText] = useState('')
  const [threadErr, setThreadErr] = useState<string | null>(null)
  const [lastMeta, setLastMeta] = useState<{ sessionId: string; inputTokens: number } | null>(null)
  const streamRef = useRef('')
  const messagesEndRef = useRef<HTMLDivElement>(null)

  // Load existing thread when panel opens for the first time
  useEffect(() => {
    if (!threadOpen || threadLoaded) return
    api.getThread(trackId, sessionNum, q.id)
      .then(r => {
        setMessages(r.thread?.messages ?? [])
        setThreadLoaded(true)
      })
      .catch(e => setThreadErr(String(e)))
  }, [threadOpen, threadLoaded, trackId, sessionNum, q.id])

  // Auto-scroll to bottom on new messages
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [messages, seeding, streaming])

  async function seedConversation() {
    setSeeding(true)
    setThreadErr(null)
    streamRef.current = ''
    // Optimistically add a streaming placeholder
    const placeholder: ThreadMessage = { role: 'assistant', content: '', ts: new Date().toISOString() }
    setMessages(prev => [...prev, placeholder])
    try {
      let accumulated = ''
      const meta = await api.threadTurn(trackId, sessionNum, q.id, '', (text) => {
        accumulated += text
        setMessages(prev => {
          const next = [...prev]
          next[next.length - 1] = { ...placeholder, content: accumulated }
          return next
        })
      })
      setLastMeta(meta)
      // Reload thread to get persisted messages with meta
      const r = await api.getThread(trackId, sessionNum, q.id)
      setMessages(r.thread?.messages ?? [])
    } catch (e) {
      setThreadErr(String(e))
      setMessages(prev => prev.slice(0, -1)) // remove placeholder
    } finally {
      setSeeding(false)
    }
  }

  async function sendMessage() {
    if (!inputText.trim() || streaming) return
    const userMsg: ThreadMessage = { role: 'user', content: inputText.trim(), ts: new Date().toISOString() }
    const placeholder: ThreadMessage = { role: 'assistant', content: '', ts: new Date().toISOString() }
    setMessages(prev => [...prev, userMsg, placeholder])
    setInputText('')
    setStreaming(true)
    setThreadErr(null)
    try {
      let accumulated = ''
      const meta = await api.threadTurn(trackId, sessionNum, q.id, userMsg.content, (text) => {
        accumulated += text
        setMessages(prev => {
          const next = [...prev]
          next[next.length - 1] = { ...placeholder, content: accumulated }
          return next
        })
      })
      setLastMeta(meta)
      const r = await api.getThread(trackId, sessionNum, q.id)
      setMessages(r.thread?.messages ?? [])
    } catch (e) {
      setThreadErr(String(e))
    } finally {
      setStreaming(false)
    }
  }

  const hasThread = threadLoaded && messages.length > 0

  return (
    <Card
      className={cn(
        "overflow-hidden border-l-4",
        correct ? "border-l-green-500" : "border-l-red-500",
      )}
    >
      <CardHeader className='pb-2 flex-row items-start gap-3'>
        <span className='text-xs font-mono text-muted-foreground pt-0.5 w-6 shrink-0' />
        <div className='flex-1 min-w-0 space-y-1.5'>
          <p className='text-sm leading-relaxed'>{q.question}</p>

          {/* Score bar */}
          <div className='flex items-center gap-2'>
            <div className='flex-1 h-1.5 rounded-full bg-muted overflow-hidden'>
              <div
                className={cn(
                  "h-full rounded-full transition-all",
                  correct ? "bg-green-500" : "bg-red-500",
                )}
                style={{ width: `${pct}%` }}
              />
            </div>
            <span className='text-xs font-mono w-8 text-right'>{pct}%</span>
          </div>

          {/* Badges */}
          <div className='flex flex-wrap gap-1.5'>
            <BloomBadge level={evalResult.bloom_level_demonstrated} />
            {evalResult.explanation_score != null && (
              <Badge variant='outline' className='text-[10px]'>
                explanation {explanationScore}%
              </Badge>
            )}
            {evalResult.calibration_flag && (
              <Badge variant='outline' className='text-[10px] text-yellow-600 border-yellow-400'>
                {evalResult.calibration_flag}
              </Badge>
            )}
            {evalResult.error_taxonomy && (
              <Badge variant='outline' className='text-[10px] text-red-500 border-red-300'>
                {evalResult.error_taxonomy.replace("_", " ")}
              </Badge>
            )}
          </div>
        </div>
      </CardHeader>

      <CardContent className='pt-0 pl-9 space-y-3'>
        {/* Your answer */}
        {isMCQ && selectedKey && q.options && (
          <div className='space-y-1'>
            {(['A', 'B', 'C', 'D'] as const).map(key => {
              const text = q.options?.[key]
              if (!text) return null
              const isSelected = selectedKey === key
              const isCorrect = q.correct === key
              return (
                <div
                  key={key}
                  className={cn(
                    'w-full flex items-start gap-2 p-2 rounded-lg text-sm border',
                    isSelected && isCorrect && 'bg-green-500/10 border-green-500 text-foreground',
                    isSelected && !isCorrect && 'bg-red-500/10 border-red-400 text-foreground',
                    !isSelected && isCorrect && 'bg-green-500/5 border-green-500/40 text-muted-foreground',
                    !isSelected && !isCorrect && 'border-transparent text-muted-foreground/50',
                  )}
                >
                  <span className='font-mono font-bold shrink-0'>{key}.</span>
                  <span>{text}</span>
                  {isSelected && <span className='ml-auto text-xs shrink-0'>{isCorrect ? '✓ your answer' : '✗ your answer'}</span>}
                  {!isSelected && isCorrect && <span className='ml-auto text-xs shrink-0 text-green-500'>correct</span>}
                </div>
              )
            })}
          </div>
        )}

        {/* Your explanation */}
        {response?.explanation && (
          <div className='space-y-1'>
            <p className='text-xs font-semibold text-muted-foreground uppercase tracking-wider'>Your explanation</p>
            <p className='text-xs font-mono text-muted-foreground bg-muted/30 rounded p-2 leading-relaxed'>
              {response.explanation}
            </p>
          </div>
        )}

        {/* AI feedback */}
        {evalResult.feedback_for_learner && (
          <div className='space-y-1'>
            <p className='text-xs font-semibold text-muted-foreground uppercase tracking-wider'>Feedback</p>
            <p className='text-sm text-muted-foreground leading-relaxed border-l-2 border-muted pl-3'>
              {evalResult.feedback_for_learner}
            </p>
          </div>
        )}
        {evalResult.misconception_identified && (
          <p className='text-xs text-orange-500'>
            Misconception: {evalResult.misconception_identified}
          </p>
        )}

        {/* Correct answer + explanation */}
        {q.correct && (
          <div className='space-y-1 pt-1 border-t border-border/50'>
            <p className='text-xs text-muted-foreground'>
              Correct: <span className='font-mono font-bold text-green-500'>{q.correct}</span>
              {q.options?.[q.correct] && ` — ${q.options[q.correct]}`}
            </p>
            {q.correct_explanation && (
              <p className='text-xs text-muted-foreground leading-relaxed'>{q.correct_explanation}</p>
            )}
          </div>
        )}

        {/* ── Follow-up thread ───────────────────────────────────────────────── */}
        <div className='pt-1 border-t border-border/40'>
          {!threadOpen ? (
            <button
              onClick={() => setThreadOpen(true)}
              className='text-xs text-primary hover:underline'
            >
              {hasThread ? 'Continue conversation' : 'Start conversation'}
            </button>
          ) : (
            <div className='space-y-2'>
              <div className='flex items-center justify-between'>
                <span className='text-xs font-semibold text-muted-foreground uppercase tracking-wider'>Follow-up</span>
                {lastMeta && (
                  <span className={cn(
                    'text-[10px] font-mono px-1.5 py-0.5 rounded',
                    lastMeta.sessionId ? 'bg-emerald-100 text-emerald-700' : 'bg-amber-100 text-amber-700'
                  )}>
                    {lastMeta.sessionId ? `resumed · ${(lastMeta.inputTokens / 1000).toFixed(1)}k tok sent` : `fresh · ${(lastMeta.inputTokens / 1000).toFixed(1)}k tok sent`}
                  </span>
                )}
              </div>

              {/* Messages */}
              {messages.length === 0 && !seeding && (
                <div className='text-center py-3'>
                  <button
                    onClick={seedConversation}
                    className={cn(buttonVariants({ size: 'sm' }), 'text-xs')}
                    disabled={seeding}
                  >
                    Open tutoring conversation
                  </button>
                  <p className='text-[10px] text-muted-foreground mt-1'>
                    Sends question + your answer + evaluation + relevant context chunks to Claude
                  </p>
                </div>
              )}

              {messages.length > 0 && (
                <div className='space-y-2 max-h-80 overflow-y-auto pr-1'>
                  {messages.map((m, i) => (
                    <div key={i} className={cn('text-xs rounded-lg p-2.5', m.role === 'user' ? 'bg-primary/10 ml-6' : 'bg-muted/50 mr-6')}>
                      {m.role === 'assistant' && m.meta && (
                        <div className='flex gap-2 mb-1 flex-wrap'>
                          <span className={cn(
                            'text-[9px] px-1 rounded font-mono',
                            m.meta.call_mode === 'resumed' ? 'bg-emerald-100 text-emerald-700' :
                            m.meta.call_mode === 'seeded' ? 'bg-blue-100 text-blue-700' :
                            'bg-amber-100 text-amber-700'
                          )}>
                            {m.meta.call_mode}
                          </span>
                          {m.meta.input_tokens_sent > 0 && (
                            <span className='text-[9px] text-muted-foreground font-mono'>
                              {(m.meta.input_tokens_sent / 1000).toFixed(1)}k tok sent
                            </span>
                          )}
                          {m.meta.context_chunks_used > 0 && (
                            <span className='text-[9px] text-muted-foreground font-mono'>
                              {m.meta.context_chunks_used} RAG chunks
                            </span>
                          )}
                          {m.meta.claude_session_id && (
                            <span className='text-[9px] text-muted-foreground font-mono'>
                              session {m.meta.claude_session_id.slice(0, 8)}…
                            </span>
                          )}
                        </div>
                      )}
                      <p className='leading-relaxed whitespace-pre-wrap'>{m.content}</p>
                    </div>
                  ))}
                  <div ref={messagesEndRef} />
                </div>
              )}

              {threadErr && <p className='text-xs text-red-500'>{threadErr}</p>}

              {/* Input */}
              {(messages.length > 0 || seeding) && (
                <div className='flex gap-1.5'>
                  <input
                    type='text'
                    value={inputText}
                    onChange={e => setInputText(e.target.value)}
                    onKeyDown={e => e.key === 'Enter' && !e.shiftKey && sendMessage()}
                    placeholder='Ask a follow-up…'
                    disabled={streaming || seeding}
                    className='flex-1 text-xs border rounded px-2 py-1.5 bg-background focus:outline-none focus:ring-1 focus:ring-primary'
                  />
                  <button
                    onClick={sendMessage}
                    disabled={streaming || seeding || !inputText.trim()}
                    className={cn(buttonVariants({ size: 'sm' }), 'text-xs px-3')}
                  >
                    {streaming ? '…' : 'Send'}
                  </button>
                </div>
              )}
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}

// ─── streaming overlay ────────────────────────────────────────────────────────

function StreamOverlay({ label, text }: { label: string; text: string }) {
  return (
    <Card>
      <CardHeader className='pb-2 flex-row items-center gap-2'>
        <span className='w-2 h-2 rounded-full bg-primary animate-pulse shrink-0' />
        <span className='text-sm text-muted-foreground'>{label}</span>
      </CardHeader>
      <CardContent>
        <pre className='text-xs font-mono text-muted-foreground whitespace-pre-wrap max-h-48 overflow-y-auto leading-relaxed'>
          {text || " "}
        </pre>
      </CardContent>
    </Card>
  );
}

// ─── synthesis panel ──────────────────────────────────────────────────────────

const BLOOM_LABELS_SHORT = ["", "R", "U", "Ap", "An", "E", "C"];

function SynthesisPanel({
  synthesis,
  onSynthesize,
  onApply,
  synthPhase,
  synthError,
  streamText,
  evaluationsExist,
}: {
  synthesis: Synthesis | null;
  onSynthesize: () => void;
  onApply: () => void;
  synthPhase: StreamPhase;
  synthError: string | null;
  streamText: string;
  evaluationsExist: boolean;
}) {
  if (synthPhase === "streaming") {
    return <StreamOverlay label='Synthesising session…' text={streamText} />;
  }

  if (!synthesis && evaluationsExist) {
    return (
      <Card>
        <CardContent className='py-6 flex flex-col items-center gap-3 text-center'>
          <p className='text-sm text-muted-foreground'>
            All questions evaluated. Generate a synthesis to update your concept
            map.
          </p>
          {synthError && (
            <p className='text-xs text-destructive'>{synthError}</p>
          )}
          <button
            onClick={onSynthesize}
            className={cn(buttonVariants({ size: "sm" }))}
          >
            Synthesise Session
          </button>
        </CardContent>
      </Card>
    );
  }

  if (!synthesis) return null;

  return (
    <Card className='border-primary/30'>
      <CardHeader className='pb-3 flex-row items-start justify-between gap-2'>
        <div>
          <p className='text-sm font-semibold'>Session Synthesis</p>
          <p className='text-xs text-muted-foreground mt-0.5'>
            {synthesis.session_date}
          </p>
        </div>
        <div className='flex items-center gap-2 shrink-0'>
          {synthesis.applied ? (
            <Badge
              variant='secondary'
              className='bg-green-500/20 text-green-600 text-[10px]'
            >
              Applied
            </Badge>
          ) : (
            <>
              {synthError && (
                <p className='text-xs text-destructive'>{synthError}</p>
              )}
              <button
                onClick={onApply}
                className={cn(buttonVariants({ size: "sm" }), "text-xs h-7")}
              >
                Apply to Concept Map
              </button>
            </>
          )}
          <button
            onClick={onSynthesize}
            className={cn(
              buttonVariants({ variant: "ghost", size: "sm" }),
              "text-xs h-7",
            )}
          >
            Re-synthesise
          </button>
        </div>
      </CardHeader>

      <CardContent className='space-y-4'>
        {/* Learner summary */}
        <p className='text-sm text-muted-foreground leading-relaxed border-l-2 border-primary/40 pl-3'>
          {synthesis.learner_summary}
        </p>

        {/* Concept map updates */}
        {synthesis.concept_map_updates.length > 0 && (
          <div>
            <p className='text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2'>
              Concept Updates
            </p>
            <div className='space-y-1.5'>
              {synthesis.concept_map_updates.map((u: ConceptMapUpdate) => {
                const delta = u.bloom_current_after - u.bloom_current_before;
                return (
                  <div
                    key={u.concept_index}
                    className='flex items-center gap-3 text-xs'
                  >
                    <span className='font-mono text-muted-foreground w-6 text-right'>
                      [{u.concept_index}]
                    </span>
                    <div className='flex items-center gap-1.5'>
                      <span
                        className={cn(
                          "inline-flex items-center justify-center w-6 h-6 rounded text-white text-[10px] font-bold",
                          BLOOM_COLORS[u.bloom_current_before],
                        )}
                      >
                        {BLOOM_LABELS_SHORT[u.bloom_current_before]}
                      </span>
                      <span
                        className={cn(
                          "text-[10px] font-bold",
                          delta > 0
                            ? "text-green-500"
                            : delta < 0
                              ? "text-red-500"
                              : "text-muted-foreground",
                        )}
                      >
                        {delta > 0 ? `+${delta}` : delta < 0 ? `${delta}` : "→"}
                      </span>
                      <span
                        className={cn(
                          "inline-flex items-center justify-center w-6 h-6 rounded text-white text-[10px] font-bold",
                          BLOOM_COLORS[u.bloom_current_after],
                        )}
                      >
                        {BLOOM_LABELS_SHORT[u.bloom_current_after]}
                      </span>
                    </div>
                    {u.spaced_repetition.next_review && (
                      <span className='text-muted-foreground ml-auto'>
                        review {u.spaced_repetition.next_review}
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  );
}

// ─── main page ────────────────────────────────────────────────────────────────

export function SessionPage() {
  const { trackId, sessionNum } = useParams<{
    trackId: string;
    sessionNum: string;
  }>();
  const num = Number(sessionNum);

  // Remote data
  const [questions, setQuestions] = useState<Question[] | null>(null);
  const [savedResponses, setSavedResponses] = useState<Response[] | null>(null);
  const [evaluations, setEvaluations] = useState<Evaluation[] | null>(null);
  const [synthesis, setSynthesis] = useState<Synthesis | null>(null);
  const [loading, setLoading] = useState(true);

  // Local answer state (pre-populated from savedResponses when present)
  const [answers, setAnswers] = useState<Record<string, LocalAnswer>>({});

  // Streaming states
  const [genPhase, setGenPhase] = useState<StreamPhase>("idle");
  const [evalPhase, setEvalPhase] = useState<StreamPhase>("idle");
  const [synthPhase, setSynthPhase] = useState<StreamPhase>("idle");
  const [streamText, setStreamText] = useState("");
  const [streamError, setStreamError] = useState<string | null>(null);
  const [synthError, setSynthError] = useState<string | null>(null);
  const [applying, setApplying] = useState(false);
  const streamRef = useRef("");

  // Submit state
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  // Token budget panel
  const [preview, setPreview] = useState<PromptPreviewResult | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewLoading, setPreviewLoading] = useState(false);

  // Load all session data on mount
  useEffect(() => {
    if (!trackId || !num) return;
    Promise.allSettled([
      api.getSessionQuestions(trackId, num),
      api.getSessionResponses(trackId, num),
      api.getSessionEvaluations(trackId, num),
      api.getSynthesis(trackId, num),
    ])
      .then(([qRes, rRes, eRes, sRes]) => {
        const qs = qRes.status === "fulfilled" ? qRes.value.questions : null;
        const rs = rRes.status === "fulfilled" ? rRes.value.responses : null;
        const es = eRes.status === "fulfilled" ? eRes.value.evaluations : null;
        const sy = sRes.status === "fulfilled" ? sRes.value.synthesis : null;

        setQuestions(qs);
        setSavedResponses(rs);
        setEvaluations(es);
        setSynthesis(sy);

        // Pre-populate answers from saved responses or blank defaults
        if (qs) {
          const init: Record<string, LocalAnswer> = {};
          for (const q of qs) {
            const saved = rs?.find((r) => r.question_id === q.id);
            init[q.id] = saved
              ? {
                  answer: saved.selected_answer,
                  confidence: saved.confidence,
                  explanation: saved.explanation,
                }
              : { answer: "", confidence: 3, explanation: "" };
          }
          setAnswers(init);
        }
      })
      .finally(() => setLoading(false));
  }, [trackId, num]);

  // ── actions ─────────────────────────────────────────────────────────────────

  async function generateQuestions() {
    if (!trackId || !num) return;
    setGenPhase("streaming");
    setStreamError(null);
    streamRef.current = "";
    setStreamText("");
    try {
      await api.generateQuestions(trackId, num, (text) => {
        streamRef.current += text;
        setStreamText(streamRef.current);
      });
      const result = await api.getSessionQuestions(trackId, num);
      setQuestions(result.questions);
      const init: Record<string, LocalAnswer> = {};
      for (const q of result.questions)
        init[q.id] = { answer: "", confidence: 3, explanation: "" };
      setAnswers(init);
      setSavedResponses(null);
      setEvaluations(null);
      setGenPhase("done");
    } catch (e) {
      setGenPhase("error");
      setStreamError(String(e));
    }
  }

  async function submitAll() {
    if (!trackId || !num || !questions) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      const responses: Response[] = questions.map((q) => ({
        question_id: q.id,
        generation_id: q.generation_id,
        selected_answer: answers[q.id]?.answer ?? "",
        confidence: answers[q.id]?.confidence ?? 3,
        explanation: answers[q.id]?.explanation ?? "",
        time_seconds: 0,
      }));
      await api.submitResponses(trackId, num, responses);
      setSavedResponses(responses);
    } catch (e) {
      setSubmitError(String(e));
    } finally {
      setSubmitting(false);
    }
  }

  async function evaluate() {
    if (!trackId || !num) return;
    setEvalPhase("streaming");
    setStreamError(null);
    streamRef.current = "";
    setStreamText("");
    try {
      await api.evaluateResponses(trackId, num, (text) => {
        streamRef.current += text;
        setStreamText(streamRef.current);
      });
      const result = await api.getSessionEvaluations(trackId, num);
      setEvaluations(result.evaluations);
      setEvalPhase("done");
    } catch (e) {
      setEvalPhase("error");
      setStreamError(String(e));
    }
  }

  async function synthesize() {
    if (!trackId || !num) return;
    setSynthPhase("streaming");
    setSynthError(null);
    streamRef.current = "";
    setStreamText("");
    try {
      await api.generateSynthesis(trackId, num, (text) => {
        streamRef.current += text;
        setStreamText(streamRef.current);
      });
      const result = await api.getSynthesis(trackId, num);
      setSynthesis(result.synthesis);
      setSynthPhase("done");
    } catch (e) {
      setSynthPhase("error");
      setSynthError(String(e));
    }
  }

  async function applySynthesis() {
    if (!trackId || !num || applying) return;
    setApplying(true);
    setSynthError(null);
    try {
      await api.applySynthesis(trackId, num);
      // Reload synthesis to get applied=true
      const result = await api.getSynthesis(trackId, num);
      setSynthesis(result.synthesis);
    } catch (e) {
      setSynthError(String(e));
    } finally {
      setApplying(false);
    }
  }

  // ── derived state ────────────────────────────────────────────────────────────

  const answeredCount = questions
    ? questions.filter((q) => (answers[q.id]?.answer ?? "") !== "").length
    : 0;
  const allAnswered = questions ? answeredCount === questions.length : false;
  const evalByID = evaluations
    ? Object.fromEntries(evaluations.map((e) => [e.question_id, e]))
    : null;
  const responseByID = savedResponses
    ? Object.fromEntries(savedResponses.map((r) => [r.question_id, r]))
    : {};

  // ── render ───────────────────────────────────────────────────────────────────

  if (loading)
    return (
      <div className='max-w-3xl space-y-3'>
        <Skeleton className='h-7 w-48' />
        <Skeleton className='h-36 rounded-xl' />
        <Skeleton className='h-36 rounded-xl' />
      </div>
    );

  return (
    <div className='max-w-3xl space-y-5'>
      {/* Header */}
      <div className='flex items-center justify-between'>
        <h1 className='text-lg font-bold font-mono'>
          {trackId} / Session {num}
        </h1>
        <div className='flex items-center gap-2'>
          <button
            onClick={() => {
              const next = !previewOpen;
              setPreviewOpen(next);
              if (next && !preview && trackId) {
                setPreviewLoading(true);
                api.getPromptPreview(trackId, num)
                  .then(setPreview)
                  .finally(() => setPreviewLoading(false));
              }
            }}
            className={cn(buttonVariants({ variant: "outline", size: "sm" }))}
            title="Show token budget for this session's LLM call"
          >
            {previewOpen ? "Hide budget" : "Token budget"}
          </button>
          <Link
            to={`/tracks/${trackId}`}
            className={cn(buttonVariants({ variant: "outline", size: "sm" }))}
          >
            ← Track
          </Link>
        </div>
      </div>

      {/* ── token budget panel ───────────────────────────────────────────────── */}
      {previewOpen && (
        <Card className='text-sm'>
          <CardContent className='pt-4 space-y-3'>
            {previewLoading && (
              <p className='text-muted-foreground text-xs'>Loading token budget…</p>
            )}
            {preview && (
              <>
                <div className='flex items-center gap-2 flex-wrap'>
                  <span className={cn(
                    'text-xs font-semibold px-2 py-0.5 rounded-full',
                    preview.call_mode === 'resumed'
                      ? 'bg-emerald-100 text-emerald-700'
                      : 'bg-amber-100 text-amber-700'
                  )}>
                    {preview.call_mode === 'resumed'
                      ? `Riding Claude context · session ${preview.claude_session_id.slice(0, 8)}…`
                      : 'Fresh call — full context will be sent'}
                  </span>
                  {preview.call_mode === 'resumed' && preview.accumulated_input_tokens > 0 && (
                    <span className='text-xs text-muted-foreground'>
                      {(preview.accumulated_input_tokens / 1000).toFixed(1)}k tokens accumulated so far
                    </span>
                  )}
                </div>

                <div className='grid grid-cols-2 gap-x-4 gap-y-1 text-xs font-mono'>
                  <span className='text-muted-foreground'>System prompt</span>
                  <span>{preview.system_prompt_tokens.toLocaleString()} tok</span>
                  <span className='text-muted-foreground'>Context files</span>
                  <span>{preview.context_tokens.toLocaleString()} tok ({preview.context_files_included.length} files)</span>
                  <span className='text-muted-foreground'>User prompt</span>
                  <span>{preview.user_prompt_tokens.toLocaleString()} tok</span>
                  <span className='text-muted-foreground font-semibold text-foreground'>Would send now</span>
                  <span className='font-semibold text-foreground'>{preview.tokens_to_send.toLocaleString()} tok</span>
                  {preview.call_mode === 'resumed' && (
                    <>
                      <span className='text-muted-foreground'>Saved by resume</span>
                      <span className='text-emerald-600'>−{preview.tokens_saved_by_resume.toLocaleString()} tok</span>
                    </>
                  )}
                  <span className='text-muted-foreground'>Context limit</span>
                  <span className={preview.total_tokens > preview.context_limit ? 'text-red-500' : ''}>
                    {(preview.context_limit / 1000).toFixed(0)}k
                    {preview.total_tokens > preview.context_limit ? ' ⚠ over' : ''}
                  </span>
                </div>

                {preview.context_files_included.length > 0 && (
                  <details className='text-xs'>
                    <summary className='cursor-pointer text-muted-foreground hover:text-foreground'>
                      Context files ({preview.context_files_included.length})
                    </summary>
                    <ul className='mt-1 ml-3 space-y-0.5 font-mono text-muted-foreground'>
                      {preview.context_files_included.sort().map(f => (
                        <li key={f}>{f}</li>
                      ))}
                    </ul>
                  </details>
                )}
              </>
            )}
          </CardContent>
        </Card>
      )}

      {/* ── no questions yet ─────────────────────────────────────────────────── */}
      {(!questions || questions.length === 0) && genPhase === "idle" && (
        <Card>
          <CardContent className='py-10 flex flex-col items-center gap-3'>
            <p className='text-sm text-muted-foreground'>
              No questions generated yet.
            </p>
            <button
              onClick={generateQuestions}
              className={cn(buttonVariants({ size: "sm" }))}
            >
              Generate Questions
            </button>
          </CardContent>
        </Card>
      )}

      {/* ── generation streaming ─────────────────────────────────────────────── */}
      {genPhase === "streaming" && (
        <StreamOverlay label='Generating questions…' text={streamText} />
      )}

      {/* ── questions exist ──────────────────────────────────────────────────── */}
      {questions !== null && questions.length > 0 && (
        <>
          {/* Phase header */}
          <div className='flex items-center justify-between'>
            <p className='text-sm text-muted-foreground'>
              {evaluations && evaluations.length > 0
                ? `${evaluations.length} evaluations complete`
                : savedResponses && savedResponses.length > 0
                  ? "Responses submitted — ready to evaluate"
                  : `${answeredCount} / ${questions.length} answered`}
            </p>
            <div className='flex gap-2'>
              {genPhase !== "streaming" && evalPhase !== "streaming" && (
                <button
                  onClick={generateQuestions}
                  className={cn(
                    buttonVariants({ variant: "ghost", size: "sm" }),
                    "text-xs",
                  )}
                >
                  Regenerate
                </button>
              )}
              {savedResponses &&
                savedResponses.length > 0 &&
                !(evaluations && evaluations.length > 0) &&
                evalPhase === "idle" && (
                  <button
                    onClick={evaluate}
                    className={cn(buttonVariants({ size: "sm" }))}
                  >
                    Evaluate with AI
                  </button>
                )}
            </div>
          </div>

          {/* Error banners */}
          {submitError && (
            <p className='text-xs text-destructive border border-destructive/30 rounded p-2'>
              {submitError}
            </p>
          )}
          {streamError && (
            <div className='text-xs text-destructive border border-destructive/30 rounded p-2 flex items-center justify-between'>
              {streamError}
              <button
                onClick={evalPhase === "error" ? evaluate : generateQuestions}
                className={cn(
                  buttonVariants({ variant: "ghost", size: "sm" }),
                  "text-xs",
                )}
              >
                Retry
              </button>
            </div>
          )}

          {/* Evaluation streaming overlay */}
          {evalPhase === "streaming" && (
            <StreamOverlay label='Evaluating responses…' text={streamText} />
          )}

          {/* Questions */}
          <div className='space-y-4'>
            {questions.map((q, i) => {
              const evalResult = evalByID?.[q.id];
              if (evalResult) {
                return <EvalCard key={q.id} q={q} evalResult={evalResult} response={responseByID[q.id]} trackId={trackId!} sessionNum={num} />;
              }
              return (
                <AnswerCard
                  key={q.id}
                  q={q}
                  index={i}
                  answer={
                    answers[q.id] ?? {
                      answer: "",
                      confidence: 4,
                      explanation: "",
                    }
                  }
                  readonly={!!(savedResponses && savedResponses.length > 0)}
                  onChange={(a) =>
                    setAnswers((prev) => ({ ...prev, [q.id]: a }))
                  }
                />
              );
            })}
          </div>

          {/* Synthesis panel — shown when evaluations exist */}
          {evaluations &&
            evaluations.length > 0 &&
            synthPhase !== "streaming" && (
              <SynthesisPanel
                synthesis={synthesis}
                onSynthesize={synthesize}
                onApply={applySynthesis}
                synthPhase={synthPhase}
                synthError={synthError}
                streamText={streamText}
                evaluationsExist={evaluations.length > 0}
              />
            )}
          {synthPhase === "streaming" && (
            <StreamOverlay label='Synthesising session…' text={streamText} />
          )}

          {/* Submit bar — only shown in answering phase */}
          {!(savedResponses && savedResponses.length > 0) &&
            evalPhase === "idle" &&
            genPhase !== "streaming" && (
              <div className='sticky bottom-4 flex justify-end'>
                <button
                  onClick={submitAll}
                  disabled={submitting || !allAnswered}
                  className={cn(
                    buttonVariants({ size: "sm" }),
                    "shadow-lg",
                    (!allAnswered || submitting) &&
                      "opacity-60 cursor-not-allowed",
                  )}
                >
                  {submitting
                    ? "Saving…"
                    : `Submit Responses (${answeredCount}/${questions.length})`}
                </button>
              </div>
            )}
        </>
      )}
    </div>
  );
}
