---
generated_by: axon/analyze-job
track: track_2
date: 2026-04-19
---

## Role Signal

Both postings converge on the same unstated bar: **production-hardened RAG engineering, not prototype RAG**. JD1 is terse by design — "deploy-and-configure" and "long-term relationship" signal they've been burned by consultants who demo well but can't own a production system. The RAGFlow mention is a tell: this is a small team running self-hosted, likely air-gapped or privacy-constrained infrastructure, and they want someone who has done this before, not someone who will learn on the job. JD2 (Yuktha) is more explicit: a baseline system exists and they need an auditor/optimizer, not a builder from scratch. The health domain framing ("high-trust health AI," clinical accuracy, PCOS) means hallucination is not just a quality issue — it's a liability issue. The latency SLA (<2–3s), WhatsApp integration, and multilingual hints point to a Southeast Asian/Indian market with real users. Both roles reward candidates who can articulate *tradeoffs* (precision vs. recall, cost vs. latency) over candidates who can enumerate techniques. The unstated bar: you must have debugged a live RAG system that was failing in a specific, explainable way.

---

## Must-Have Skills

- **RAGFlow or equivalent self-hosted RAG orchestration** — JD1 explicitly names RAGFlow; comfort with its ingestion pipeline, chunking config, and retrieval tuning is the baseline entry point.
- **Chunking strategy design** — Both JDs call this out; poor chunking is the most common source of retrieval failure and the first thing an auditor checks.
- **Embedding model selection and trade-off reasoning** — JD2 specifically requires embedding selection as a deliverable; candidates must know when to use dense vs. sparse, domain-specific vs. general, and the cost/quality curve.
- **Vector database operation** (Qdrant, Weaviate, Pinecone, pgvector, etc.) — JD2 references vector DBs directly; production use includes index configuration, recall tuning, and hybrid search setup.
- **Hybrid retrieval (semantic + keyword/BM25)** — JD2 lists this explicitly as a potential implementation; candidates must understand when pure semantic search fails and how to combine signals.
- **Query rewriting and expansion** — JD2 lists this under retrieval optimization; directly affects recall on short/ambiguous user queries.
- **Prompt engineering for structured and grounded outputs** — Both JDs require this; JD2 adds the clinical accuracy constraint, meaning prompts must constrain hallucination while producing structured plans/recommendations.
- **Hallucination diagnosis and mitigation** — Both JDs name this explicitly; candidates need a systematic methodology, not just "add a system prompt."
- **LLM API integration** (OpenAI, Anthropic, or open-source via Ollama/vLLM) — JD1 covers open-source and API-based; JD2 implies both; production wiring, retries, and cost tracking are implied.
- **RAG evaluation frameworks** — JD2 requires building one as a deliverable (RAGAS, custom metrics, automated + manual pipeline); JD1 implies it through "troubleshoot and improve retrieval quality."
- **Latency optimization** — JD2 has an explicit <2–3s SLA; candidates must know where latency lives (embedding inference, vector search, LLM TTFT) and how to reduce it.
- **Personalization / memory-aware context handling** — JD2 requires user-context injection (symptoms, history, test results); this is stateful RAG beyond single-turn retrieval.

---

## Likely Interview Probes

| Skill Area | Likely Question / Scenario |
|---|---|
| Chunking strategy | "Walk me through how you'd choose chunk size and overlap for a corpus of clinical PCOS guidelines vs. a corpus of user-uploaded PDFs. What breaks if you get this wrong?" |
| Retrieval failure diagnosis | "A user asks a reasonable question and gets a completely irrelevant answer. What's your step-by-step diagnostic process?" |
| Hybrid retrieval | "When would you add BM25/keyword retrieval on top of semantic search, and what are the risks of doing so naively?" |
| Embedding selection | "You have a health domain corpus in English and Hindi. How do you pick an embedding model, and what do you measure to validate the choice?" |
| Hallucination mitigation | "The system is returning medically plausible but unsupported claims. What are the three most likely root causes, and how do you confirm which it is?" |
| Evaluation framework design | "Design an evaluation pipeline for a PCOS recommendation RAG system. What metrics do you track, what does your test set look like, and how do you handle the ground-truth problem?" |
| Latency optimization | "Your RAG pipeline is averaging 4 seconds end-to-end. Walk me through how you'd profile and reduce that to under 2.5 seconds." |
| Prompt engineering | "Show me a prompt pattern you'd use to ensure the model cites only retrieved context and returns a structured plan (not free-form prose)." |
| RAGFlow / self-hosted deployment | "What are the operational concerns when deploying RAGFlow in a private environment — what breaks first, and how do you monitor it?" |
| Personalization layer | "How do you inject user history (symptoms logged over 6 weeks) into a RAG query without overflowing the context window or degrading retrieval?" |

---

## Interview Scenario Seeds

**Scenario 1 — Retrieval Audit**
You are brought in to audit a deployed RAG system for a women's health app. The product team reports that users with PCOS are receiving generic dietary advice that ignores their logged symptoms and test results. The system has a vector database of 40+ clinical guidelines and a user profile store. You have access to retrieval logs showing the top-5 chunks returned per query, the final prompt sent to the LLM, and the LLM output. Describe your audit process: what are the first three things you examine in the logs, what failure modes are you testing for, and what concrete changes would you propose if retrieval is correct but the output is still wrong?

**Scenario 2 — Latency vs. Quality Tradeoff**
A client reports that the RAG pipeline is accurate but too slow — average response time is 4.8 seconds on WhatsApp, and users are abandoning the conversation. The pipeline currently re-ranks all retrieved chunks using a cross-encoder before sending to the LLM. The embedding model is a 768-dim multilingual model running on CPU. You are asked to bring latency under 2.5 seconds without a significant drop in answer quality. Walk through the tradeoffs of each optimization lever available to you, and explain which you would apply first and why, given that this is a health domain where accuracy degradation is a serious risk.

**Scenario 3 — Evaluation Framework Design**
You are building an evaluation framework for the Yuktha RAG system before it goes to 10,000 active users. The system delivers personalized supplement, diet, and lifestyle recommendations. There is no labeled ground-truth dataset. Describe how you would construct a test set, what metrics you would use (and their limitations), how you would handle the safety dimension (the system must never recommend contraindicated supplements), and how you would set up automated regression testing so future prompt or retrieval changes don't silently degrade quality.

---

## Self-Assessment Anchors

**Embeddings & Vector Search**
- Strong if: you have tuned HNSW index parameters, chosen between dense/sparse/hybrid retrieval for a real corpus, and can explain why cosine similarity fails for certain query types.
- Gap if: you've only used a managed vector DB with default settings and have never diagnosed a recall failure.
- Close by: implement a small hybrid retrieval experiment using Qdrant or Weaviate; measure precision@k and recall@k against a manual evaluation set; tune ef_construction and m parameters.

**Hallucination Diagnosis**
- Strong if: you can distinguish between retrieval hallucination (wrong chunks retrieved), grounding hallucination (model ignores chunks), and knowledge hallucination (model uses parametric knowledge), and you have a workflow for each.
- Gap if: your fix for hallucinations is to add "only use the provided context" to the system prompt and hope.
- Close by: study RAGAS metrics (faithfulness, answer relevance, context recall); run a structured evaluation on a small RAG system you control; practice attributing specific output errors to specific pipeline stages.

**Production RAG Operations**
- Strong if: you have handled chunking pipeline failures on heterogeneous document types (PDFs, tables, scanned images), managed embedding model versioning, and set up retrieval monitoring in a live system.
- Gap if: your RAG experience is limited to OpenAI cookbook examples or LangChain quickstarts against clean text corpora.
- Close by: deploy RAGFlow locally with a real document corpus (clinical PDFs preferred); configure chunking and test with adversarial queries; read the RAGFlow source to understand its ingestion pipeline.

**Domain Sensitivity (Healthcare)**
- Strong if: you understand why health AI requires explicit safety constraints in prompts, why "I don't know" is a valid and preferred response in clinical contexts, and why evaluation must include adversarial safety tests.
- Gap if: you've only built general-purpose RAG and haven't considered the asymmetry between false positives and false negatives in health recommendations.
- Close by: review NHS/WHO AI guidance on clinical decision support; study how systems like K Health or Ada Health handle the uncertainty problem in LLM outputs.

---

## Question Format Guidance

**Question type weighting:**
- `interview_scenario`: 50% — the role is fundamentally diagnostic and system-thinking; scenario questions best surface whether candidates have done this in production.
- `free_text`: 30% — use for "walk me through your process" probes (audit methodology, evaluation design, latency diagnosis); these can't be distracted.
- `mcq`: 20% — use only for specific technical knowledge checks (e.g., chunking parameter effects, HNSW configuration, BM25 vs. cosine similarity behavior).

**Misconceptions to probe as MCQ distractors:**
- "Larger chunks always improve context quality" (false; hurts precision and dilutes relevance signal)
- "Adding 'do not hallucinate' to the system prompt is a reliable mitigation" (false; grounding requires retrieval quality, not prompt instruction)
- "Semantic similarity = answer relevance" (false; high cosine similarity does not guarantee the chunk answers the question)
- "Re-ranking always improves quality worth the latency cost" (false; depends on corpus and query distribution)
- "A higher-dimensional embedding model is always better" (false; domain fit matters more than dimension count)

**Minimum Bloom level:** Apply (Level 3) for all questions. Most questions should target Analyze (Level 4) or Evaluate (Level 5). No recall-only questions — the JDs explicitly screen for trade-off reasoning and debugging skill, not enumeration.

**Domain-specific framing constraints:**
- All scenario seeds must acknowledge the **health safety constraint**: responses that are confidently wrong are worse than "I don't know." Quiz questions should reward candidates who identify safety-first response design.
- **Latency SLA** of <2–3 seconds should be present in at least one scenario as a hard constraint that forces explicit trade-off decisions.
- **Private/self-hosted deployment** framing should appear in at least one question — this is not a managed SaaS context; operational and infrastructure awareness matters.
- Where possible, anchor questions to **existing system audit** (JD2 framing) rather than greenfield build — this role requires diagnosis of inherited systems.