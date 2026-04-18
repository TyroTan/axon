// Package rag provides a naive, deterministic retrieval layer.
// No embeddings — uses markdown-heading-based chunking and word-overlap scoring.
// Designed for structured course-material markdown files.
package rag

import (
	"sort"
	"strings"
)

// Chunk is one retrievable section of a context file.
type Chunk struct {
	File    string  // source filename
	Heading string  // nearest markdown heading (# / ## / ###), or ""
	Content string  // raw text of this section
	Tokens  int     // naive token estimate: (len+3)/4
	Score   float64 // relevance score vs query (set by TopK)
}

// ChunkFiles splits context files into chunks by markdown headings.
// Each file is split at heading boundaries (lines starting with #).
// Chunks that exceed maxChunkTokens are further split by double-newline paragraphs
// so individual chunks stay manageable.
func ChunkFiles(files map[string]string, maxChunkTokens int) []Chunk {
	var chunks []Chunk
	for filename, content := range files {
		if strings.HasPrefix(filename, "_") {
			continue // skip _sources.md, _split_plan.md etc.
		}
		chunks = append(chunks, splitFile(filename, content, maxChunkTokens)...)
	}
	return chunks
}

// MinScore is the minimum relevance score a chunk must reach to be included
// in TopK results. Chunks scoring below this threshold are discarded as
// off-topic. Set to 0 to disable filtering.
const MinScore = 0.30

// TopK ranks chunks by relevance to query and returns the highest-scoring
// chunks that fit within tokenLimit total tokens.
// k is an upper bound on the number of chunks returned.
// Chunks with Score < MinScore are excluded.
func TopK(chunks []Chunk, query string, k, tokenLimit int) []Chunk {
	queryWords := tokenizeQuery(query)
	if len(queryWords) == 0 {
		// No query signal — return first-fit chunks up to limit.
		return budgetFill(chunks, k, tokenLimit)
	}

	scored := make([]Chunk, 0, len(chunks))
	for _, c := range chunks {
		c.Score = score(c.Content+" "+c.Heading, queryWords)
		if c.Score >= MinScore {
			scored = append(scored, c)
		}
	}
	sort.Slice(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	return budgetFill(scored, k, tokenLimit)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func splitFile(filename, content string, maxChunkTokens int) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var curHeading string
	var curLines []string

	flush := func() {
		if len(curLines) == 0 {
			return
		}
		text := strings.TrimSpace(strings.Join(curLines, "\n"))
		if text == "" {
			return
		}
		// If this section is too big, split by paragraphs.
		if (len(text)+3)/4 > maxChunkTokens {
			for _, para := range splitParagraphs(text, maxChunkTokens) {
				chunks = append(chunks, Chunk{
					File:    filename,
					Heading: curHeading,
					Content: para,
					Tokens:  (len(para) + 3) / 4,
				})
			}
		} else {
			chunks = append(chunks, Chunk{
				File:    filename,
				Heading: curHeading,
				Content: text,
				Tokens:  (len(text) + 3) / 4,
			})
		}
		curLines = nil
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			flush()
			curHeading = strings.TrimLeft(line, "#")
			curHeading = strings.TrimSpace(curHeading)
		} else {
			curLines = append(curLines, line)
		}
	}
	flush()
	return chunks
}

func splitParagraphs(text string, maxChunkTokens int) []string {
	paras := strings.Split(text, "\n\n")
	var result []string
	var buf []string
	bufTok := 0

	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		ptok := (len(p) + 3) / 4
		if bufTok+ptok > maxChunkTokens && len(buf) > 0 {
			result = append(result, strings.Join(buf, "\n\n"))
			buf = nil
			bufTok = 0
		}
		buf = append(buf, p)
		bufTok += ptok
	}
	if len(buf) > 0 {
		result = append(result, strings.Join(buf, "\n\n"))
	}
	return result
}

var stopwords = map[string]bool{
	"a": true, "an": true, "the": true, "is": true, "in": true, "on": true,
	"at": true, "to": true, "for": true, "of": true, "with": true, "and": true,
	"or": true, "but": true, "not": true, "this": true, "that": true, "it": true,
	"be": true, "as": true, "by": true, "from": true, "are": true, "was": true,
	"were": true, "do": true, "does": true, "did": true, "have": true, "has": true,
	"had": true, "will": true, "would": true, "can": true, "could": true,
	"what": true, "which": true, "how": true, "why": true, "when": true, "where": true,
}

func tokenizeQuery(q string) map[string]bool {
	words := make(map[string]bool)
	for _, w := range strings.Fields(strings.ToLower(q)) {
		w = strings.Trim(w, ".,?!;:\"'()")
		if w != "" && !stopwords[w] && len(w) > 2 {
			words[w] = true
		}
	}
	return words
}

func score(text string, queryWords map[string]bool) float64 {
	lower := strings.ToLower(text)
	hits := 0
	for w := range queryWords {
		if strings.Contains(lower, w) {
			hits++
		}
	}
	if len(queryWords) == 0 {
		return 0
	}
	return float64(hits) / float64(len(queryWords))
}

func budgetFill(chunks []Chunk, k, tokenLimit int) []Chunk {
	var result []Chunk
	total := 0
	for _, c := range chunks {
		if len(result) >= k {
			break
		}
		if total+c.Tokens > tokenLimit {
			continue // skip this chunk, try next (greedy best-fit)
		}
		result = append(result, c)
		total += c.Tokens
	}
	return result
}
