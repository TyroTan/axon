---
generated_by: axon/analyze-job
track: track_3
date: 2026-04-19
---

## Role Signal

Maestro.io is an early-stage product (SMB BI platform) where the AI engineer is the sole owner of the intelligence layer — not a contributor to an existing ML team. The JD's emphasis on "production-grade, not demos" and "verifiable examples" signals a founder or small team that has been burned by contractors delivering POCs that never shipped. The stack (Supabase + pgvector, Xano, Claude API) is no-code/low-code adjacent, meaning the candidate must produce clean, handoff-ready APIs that non-engineers can consume — not sophisticated internal tooling. The unstated bar: the candidate must be able to make independent architectural decisions (chunking strategy, retrieval ranking, prompt structure) without a senior ML engineer to review their work, and must communicate those decisions clearly to a non-technical founder. Claude API fluency is a hard filter, not a nice-to-have.

---

## Must-Have Skills

- **Production RAG pipeline design** — the core deliverable; the client explicitly rejects demo-tier implementations and will vet against shipped examples.
- **Supabase + pgvector** — the preferred vector store; candidate must understand SQL-native vector search tradeoffs vs. dedicated vector DBs like Pinecone.
- **Chunking and embedding strategy** — directly called out in scope ("properly chunked, embedded, and ranked"); wrong chunking is the most common RAG failure mode in document retrieval.
- **Retrieval ranking / re-ranking** — JD explicitly requires ranking retrieved context before passing to model; candidate must know hybrid search, MMR, or cross-encoder re-ranking.
- **Anthropic Claude API (claude-sonnet / claude-opus)** — named model provider with preference for specific model tiers; candidate must know Claude's prompt format, context window limits, and system/user/assistant turn structure.
- **Prompt engineering for structured business output** — role requires returning "concise, actionable insights with 2–3 recommended next steps"; candidate must know output-shaping techniques (structured prompts, JSON mode, few-shot examples).
- **REST API design for no-code backend handoff** — Xano integration means the AI layer must expose clean, predictable REST endpoints consumable without custom SDK code; error contracts and payload schemas matter.
- **User context injection into LLM queries** — financial data, business stage, and industry must be incorporated into the query pipeline; candidate must understand context window management and dynamic prompt construction.

---

## Likely Interview Probes

| Skill Area | Likely Question / Scenario |
|---|---|
| RAG architecture | "Walk me through the end-to-end flow of your most recent RAG pipeline in production — from document ingestion to final LLM response." |
| Chunking strategy | "How did you decide on chunk size and overlap for your document corpus? What changed after you saw retrieval quality in production?" |
| pgvector / Supabase | "What are the tradeoffs between using pgvector inside Supabase versus a dedicated vector store like Pinecone for this use case?" |
| Retrieval ranking | "After embedding-based similarity search returns 20 chunks, how do you decide what actually goes into the context window?" |
| Claude API specifics | "How does Claude's system prompt behavior differ from GPT-style models, and how does that affect how you structure a RAG prompt?" |
| Prompt engineering for output shape | "How would you prompt Claude to reliably return exactly 2–3 actionable next steps rather than a free-form essay?" |
| Context injection | "A user has financial data (revenue, margin, stage) and a natural language question. How do you structure the context you pass to the model without exceeding token limits?" |
| REST API for Xano handoff | "What does your chat endpoint's request/response contract look like? How do you handle streaming vs. non-streaming for a no-code consumer?" |
| Production reliability | "What breaks in RAG pipelines between demo and production? Name a specific failure you encountered and how you fixed it." |
| Embedding model selection | "Which embedding model did you use and why? How did you evaluate retrieval quality?" |

---

## Interview Scenario Seeds

**Scenario 1 — Retrieval quality degradation:**
You are the AI engineer at Maestro.io. After launching the RAG pipeline, the founder reports that for queries like "how should a $2M ARR SaaS business reduce churn?", the system is returning generic small business advice rather than relevant case studies. The vector store contains 400 business case studies with metadata fields for industry, ARR range, and problem type. Describe your diagnostic process and the specific changes you would make to the chunking strategy, metadata filtering, and retrieval ranking to fix this. What query-time techniques would you apply before even touching the LLM prompt?

**Scenario 2 — Token budget and context window management:**
A client reports that Maestro's chat endpoint sometimes returns truncated or incoherent responses when a user has a long conversation history and their financial profile is large. You are using claude-sonnet with a 200k context window. The system prompt is 800 tokens, the user's financial context object serializes to approximately 1,200 tokens, and you retrieve the top-8 chunks at ~300 tokens each. Describe how you would audit the token budget, which components you would compress or summarize, and how you would implement a sliding window or summarization strategy for conversation history without losing critical user context.

**Scenario 3 — Structured output reliability:**
You are designing the prompt template for Maestro's core insight endpoint. The product requirement is: every response must contain exactly one summary paragraph and a numbered list of 2–3 next steps, returned as a JSON object so Xano can render them in separate UI components. During testing, Claude occasionally returns prose without the JSON wrapper, or returns 4 next steps instead of 3. Describe the prompt engineering and post-processing strategy you would implement to enforce this output contract reliably. How would you test and monitor output compliance in production?

---

## Self-Assessment Anchors

**Strong if...**
- Candidate can describe a specific production RAG system they shipped: corpus size, chunking approach, embedding model, retrieval method, and what they changed after seeing real query traffic.
- Candidate knows the difference between `pgvector` cosine similarity search and hybrid search (BM25 + vector), and when to use each.
- Candidate has worked directly with the Claude Messages API and can articulate how system prompts, user turns, and assistant prefill interact.
- Candidate has designed REST APIs consumed by non-engineers and has opinions on response schema design for predictability.

**Gap if...**
- Candidate's only RAG experience is LangChain or LlamaIndex tutorials with default settings — no evidence of tuning chunk size, embedding model, or retrieval pipeline.
- Candidate defaults to OpenAI and has never used the Claude API; may not understand Anthropic's system prompt conventions or how `claude-sonnet` vs `claude-opus` differ in instruction-following.
- Candidate has never had to make their AI layer consumable by a no-code backend; thinks of the API as internal tooling.
- Candidate cannot explain why naive top-k similarity retrieval fails for multi-aspect business queries.

**Close by...**
- Build a minimal RAG system on Supabase + pgvector using the Claude API; retrieve from a small document corpus and iterate on chunk size and retrieval ranking until results are qualitatively strong.
- Read Anthropic's prompt engineering docs specifically on structured output, system prompts, and claude-sonnet vs claude-opus tradeoffs.
- Practice designing a REST endpoint schema (request body, response envelope, error codes) as if handing off to a Xano workflow builder with no Python access.

---

## Question Format Guidance

**Question type weighting:**
- `interview_scenario`: 50% — this role is evaluated on production judgment, not textbook recall; scenario questions directly match how this client will interview.
- `free_text`: 30% — prompt engineering, API design decisions, and retrieval strategy require open-ended explanation; MCQ cannot probe nuance here.
- `mcq`: 20% — use only for discrete factual distinctions: pgvector vs. Pinecone tradeoffs, Claude API parameter behavior, chunking terminology.

**Misconceptions to probe as distractors (MCQ):**
- Larger chunk size always improves retrieval quality (false — precision degrades).
- The system prompt in Claude functions identically to the OpenAI system role (partially false — behavioral differences in instruction-following).
- Top-k cosine similarity is sufficient for production retrieval without re-ranking.
- Streaming and non-streaming responses require the same endpoint contract for Xano consumption (false).
- claude-opus is always the right choice over claude-sonnet for this use case (false — latency/cost tradeoffs matter for real-time chat).

**Minimum Bloom level:** Apply (Level 3) — questions must require the candidate to use knowledge in a novel pipeline design context, not just recall definitions. Evaluate (Level 5) preferred for scenario seeds.

**Domain-specific framing:**
- All scenarios should be grounded in the SMB BI context: financial data, business stage, churn, growth benchmarks — not generic QA or document search.
- Latency SLA framing: Maestro is a chat interface; questions should acknowledge that end-to-end response time matters and probe how candidates balance retrieval depth against latency.
- Client handoff constraint: at least one question per session should require the candidate to describe their output from the perspective of a Xano developer consuming the API — not the AI engineer who built it.
- Verifiability constraint: free-text questions should ask candidates to describe a specific system they built, mirroring the JD's "applicants without verifiable examples will not be considered" filter.