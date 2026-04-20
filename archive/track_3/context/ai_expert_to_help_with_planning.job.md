---
generated_by: axon/analyze-job
track: track_3
date: 2026-04-19
---

## Role Signal

This is a freelance advisory engagement, not a product engineering role. The client is technical and self-implementing — they need an expert who can transfer *mental models and decision frameworks*, not someone who writes code for them. The JD signals a client at early RAG architecture phase (data layout and chunking are unsolved), likely prototyping or preparing a production ingestion pipeline. The unstated bar is pedagogical fluency: the candidate must be able to justify trade-offs on the fly, adapt explanations to a technical peer, and structure ambiguous design spaces into actionable guidance. Stack maturity is mid-level (familiar with the tooling ecosystem but needs expert scaffolding). A successful candidate will have opinions, not just knowledge — they should be able to say "here's what I'd actually do and why" rather than listing options neutrally.

---

## Must-Have Skills

- **RAG pipeline architecture** — the entire engagement is structured around guiding RAG design decisions; shallow familiarity is immediately disqualifying.
- **Chunking strategies (fixed-size, semantic, hierarchical, etc.)** — explicitly listed as a key focus area; candidate must know when each applies and what retrieval quality trade-offs follow.
- **Metadata and tagging schema design for retrieval** — directly named; candidate must understand how metadata filters interact with vector search and hybrid retrieval.
- **Embedding model selection and preprocessing** — listed as a focus area; candidate must know tokenization limits, model trade-offs (OpenAI vs open-source), and preprocessing decisions that affect embedding quality.
- **Vector database fundamentals (indexing, similarity metrics, filtering)** — required in the JD; underpins all retrieval architecture discussions.
- **Communication and concept explanation under technical questioning** — explicitly called "critical"; the engagement is discussion-based, so this is load-bearing, not soft.
- **Trade-off reasoning across RAG architectures** — JD explicitly asks for guidance on trade-offs between architectures (naive RAG, advanced RAG, agentic RAG, etc.).

---

## Likely Interview Probes

| Skill Area | Likely Question / Scenario |
|---|---|
| Chunking strategy | "Walk me through how you'd decide between fixed-size vs semantic chunking for a corpus of mixed-length technical documents." |
| Metadata design | "How would you design a tagging schema for a document store with 10K+ files across multiple domains to support filtered retrieval?" |
| Embedding preprocessing | "What preprocessing steps do you apply before embedding, and how do those decisions affect retrieval recall?" |
| Vector DB selection | "Compare two vector databases you've used — what drove the choice in a real project?" |
| RAG architecture trade-offs | "When would you recommend a naive RAG setup vs a more complex re-ranking or agentic retrieval approach?" |
| Retrieval quality | "A client reports that their RAG system returns semantically similar but contextually irrelevant chunks. What's your diagnostic process?" |
| Data layout for ingestion | "How do you structure a document ingestion pipeline to handle updates and deletions without a full re-index?" |
| Hybrid search | "Explain when you'd layer BM25 keyword search on top of vector search, and what the implementation cost is." |
| Mentoring/communication | "How would you explain the difference between cosine similarity and dot product to a technical client who is not an ML practitioner?" |
| LangChain/LlamaIndex (nice-to-have) | "Have you used LlamaIndex's node parser or LangChain's text splitters in a production context? What limitations did you hit?" |

---

## Interview Scenario Seeds

**Scenario 1 — Chunking and Schema Design**
You are advising a client who has a corpus of 8,000 internal PDF documents: engineering specs (5–80 pages), meeting notes (1–3 pages), and policy documents (10–25 pages). They want a single RAG system that answers questions across all three types. Walk the client through how you would approach chunking strategy and metadata schema design. What decisions do you make up front, and what trade-offs do you flag for their specific document distribution?

**Scenario 2 — Retrieval Quality Diagnosis**
A client reports that their RAG pipeline returns chunks that are semantically close to the query but consistently miss the actual answer, which appears a few paragraphs away from the retrieved chunk. They are using 512-token fixed-size chunks with 0 overlap. Diagnose the likely root cause, propose a remediation strategy, and explain the trade-offs of each option you consider. Your answer should be structured as a mentoring conversation, not a code walkthrough.

**Scenario 3 — Architecture Selection**
A client is deciding between three RAG setups: (a) naive single-stage retrieval with a general-purpose embedding model, (b) a two-stage pipeline with a cross-encoder re-ranker, and (c) an agentic retrieval loop with query decomposition. They have a budget of 2 weeks for the first working version and a technical team that can implement any of the three. Guide them through the decision. What questions do you ask before recommending an approach, and what is your actual recommendation given the constraints?

---

## Self-Assessment Anchors

**Chunking & Document Structure**
- Strong if: can explain parent-child chunking, sliding window overlap, and semantic sentence boundary detection without prompting, and knows which retrieval failure modes each addresses.
- Gap if: only knows fixed-size chunking or defaults to LangChain's `RecursiveCharacterTextSplitter` without understanding what it's optimizing.
- Close by: read the LlamaIndex docs on node parsers; study the "lost in the middle" paper and its implications for chunk sizing.

**Metadata and Hybrid Retrieval**
- Strong if: has designed metadata filter schemas that interact with vector search (e.g., pre-filter by doc type before ANN search) and understands the precision/recall trade-off of filtering vs. embedding-only retrieval.
- Gap if: treats metadata as an afterthought or has only used keyword search independently, not in hybrid configurations.
- Close by: build a small demo with Weaviate or Qdrant using both vector and payload filters; read the Pinecone metadata filtering docs for real-world schema patterns.

**Communication Under Pressure**
- Strong if: has run client-facing design sessions, documented architectural decision records (ADRs), or taught technical concepts in workshops.
- Gap if: experience is primarily heads-down implementation with no client-facing or mentoring component.
- Close by: practice explaining RAG concepts out loud to a non-expert; record yourself answering "what is chunking and why does it matter" in under 2 minutes.

---

## Question Format Guidance

**Question type weighting:**
- `interview_scenario`: 50% — this role is communication and judgment-heavy; scenario questions are the primary signal.
- `free_text`: 30% — probe explanation quality, not just factual recall; assess whether the candidate can structure a trade-off argument.
- `mcq`: 20% — use only for factual anchors (e.g., cosine vs. dot product behavior, chunking terminology); avoid for architecture decisions.

**Misconceptions to probe as distractors (MCQ):**
- "Larger chunks always improve retrieval quality" (false — context window stuffing degrades answer quality)
- "Embedding model choice doesn't affect chunking strategy" (false — tokenizer limits and model context windows directly constrain chunk size)
- "Metadata filters are a replacement for better embeddings" (false — they are complementary, not substitutes)
- "Re-ranking always improves end-to-end accuracy" (false — adds latency and may hurt if the base retrieval is already precise)

**Minimum Bloom level:** Apply (Level 3) — factual recall questions are insufficient for this role. All scenario and free_text questions must require the candidate to apply principles to a novel context or evaluate trade-offs. Target Evaluate (Level 5) for scenario seeds.

**Domain-specific framing:**
- Frame all scenarios as advisory interactions ("you are explaining to a technical client...") to match the mentoring nature of the engagement.
- Explicitly include a constraint in each scenario (budget, timeline, corpus characteristics) to force prioritization decisions.
- Avoid questions that can be answered by naming a tool — probe the *why* behind tool selection, not the tool name itself.
- Flag communication quality as a scoreable dimension in `free_text` rubrics: clarity, structure, and appropriateness for a technical-but-not-ML-expert audience.