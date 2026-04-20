// Package server wires all dependencies and registers routes.
// This is the composition root — the only place that knows about all packages.
// Fiber is a pure JSON API server. The React UI (ui/) handles all rendering.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/valyala/fasthttp"

	"github.com/tyrohunt/axon/internal/commands"
	"github.com/tyrohunt/axon/internal/cqrs"
	"github.com/tyrohunt/axon/internal/domain"
	"github.com/tyrohunt/axon/internal/llm"
	"github.com/tyrohunt/axon/internal/llm/anthropic"
	"github.com/tyrohunt/axon/internal/llm/claudecli"
	"github.com/tyrohunt/axon/internal/metrics"
	"github.com/tyrohunt/axon/internal/queries"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// Config holds server configuration.
type Config struct {
	ExperimentsDir    string // absolute path to .experiments/ folder
	Port              string // e.g. "3456"
	DistDir           string // absolute path to web/dist/ (served as SPA in production)
	AnthropicAPIKey   string // ANTHROPIC_API_KEY — if empty, claudecli is used
	ContextTokenLimit int    // AXON_CONTEXT_LIMIT — max tokens for inherited context (default 80000)
	SoftTokenLimit    int    // AXON_SOFT_LIMIT — trigger split plan above this (default 250000)
	HardTokenLimit    int    // AXON_HARD_LIMIT — hard abort above this (default 300000)
}

// New wires dependencies and returns a configured Fiber app.
func New(cfg Config) *fiber.App {
	// ── Stores ───────────────────────────────────────────────────────────────
	trackStore := filesystem.NewTrackStore(cfg.ExperimentsDir)

	// ── Query bus ────────────────────────────────────────────────────────────
	qryBus := cqrs.NewQueryBus()
	cqrs.RegisterQuery[queries.ListTracksQuery, queries.ListTracksResult](
		qryBus, queries.NewListTracksHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.GetTrackQuery, queries.GetTrackResult](
		qryBus, queries.NewGetTrackHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.GetTrackContextQuery, queries.GetTrackContextResult](
		qryBus, queries.NewGetTrackContextHandler(trackStore),
	)

	cqrs.RegisterQuery[queries.GetSessionQuestionsQuery, queries.GetSessionQuestionsResult](
		qryBus, queries.NewGetSessionQuestionsHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.GetSessionResponsesQuery, queries.GetSessionResponsesResult](
		qryBus, queries.NewGetSessionResponsesHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.GetSessionEvaluationsQuery, queries.GetSessionEvaluationsResult](
		qryBus, queries.NewGetSessionEvaluationsHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.GetSynthesisQuery, queries.GetSynthesisResult](
		qryBus, queries.NewGetSynthesisHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.GetMetaSynthesisQuery, queries.GetMetaSynthesisResult](
		qryBus, queries.NewGetMetaSynthesisHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.GetContextTokensQuery, queries.GetContextTokensResult](
		qryBus, queries.NewGetContextTokensHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.GetThreadQuery, queries.GetThreadResult](
		qryBus, queries.NewGetThreadHandler(trackStore),
	)

	// ── LLM client ───────────────────────────────────────────────────────────
	// Default: claude CLI (uses existing auth, no API key needed).
	// Override: set ANTHROPIC_API_KEY to use the HTTP API directly instead.
	var llmClient llm.Client
	if cfg.AnthropicAPIKey != "" {
		llmClient = anthropic.New(cfg.AnthropicAPIKey)
	} else {
		llmClient = claudecli.New("")
	}

	// ── Metrics recorder ─────────────────────────────────────────────────────
	rec, err := metrics.New(cfg.ExperimentsDir)
	if err != nil {
		panic("axon: metrics init: " + err.Error())
	}

	// ── Command bus ──────────────────────────────────────────────────────────
	cmdBus := cqrs.NewCommandBus()
	createTrackHandler := commands.NewCreateTrackHandler(trackStore)

	cqrs.Register[commands.UpdateContextCommand](
		cmdBus, commands.NewUpdateContextHandler(trackStore),
	)

	convTurnHandler := commands.NewConversationTurnHandler(trackStore, llmClient, cfg.ContextTokenLimit)
	indexConvHandler := commands.NewIndexConversationHandler(trackStore, llmClient, rec)

	cqrs.RegisterQuery[queries.GetConversationQuery, queries.GetConversationResult](
		qryBus, queries.NewGetConversationHandler(trackStore),
	)
	cqrs.RegisterQuery[queries.ListConversationsQuery, queries.ListConversationsResult](
		qryBus, queries.NewListConversationsHandler(trackStore),
	)

	createSessionHandler := commands.NewCreateSessionHandler(trackStore)
	generateQuestionsHandler := commands.NewGenerateQuestionsHandler(trackStore, llmClient, cfg.ContextTokenLimit, cfg.SoftTokenLimit, cfg.HardTokenLimit, rec)
	submitResponsesHandler := commands.NewSubmitResponsesHandler(trackStore)
	evaluateResponsesHandler := commands.NewEvaluateResponsesHandler(trackStore, llmClient, rec)
	generateSynthesisHandler := commands.NewGenerateSynthesisHandler(trackStore, llmClient, rec)
	applySynthesisHandler := commands.NewApplySynthesisHandler(trackStore)
	metaSynthesisHandler := commands.NewMetaSynthesisHandler(trackStore, llmClient, rec)
	applyMetaSynthesisHandler := commands.NewApplyMetaSynthesisHandler(trackStore)
	compactFileHandler := commands.NewCompactFileHandler(trackStore, llmClient, rec)
	distillThreadsHandler := commands.NewDistillThreadsHandler(trackStore, llmClient)
	cqrs.Register[commands.DuplicateTrackCommand](
		cmdBus, commands.NewDuplicateTrackHandler(trackStore, distillThreadsHandler),
	)
	mergeTracksHandler := commands.NewMergeTracksHandler(trackStore, distillThreadsHandler)
	analyzeJobHandler := commands.NewAnalyzeJobHandler(trackStore, llmClient)
	threadTurnHandler := commands.NewThreadTurnHandler(trackStore, llmClient, cfg.ContextTokenLimit)
	detectStateHandler := commands.NewDetectLearnerStateHandler(trackStore, llmClient)

	// ── Fiber app ────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName: "Axon",
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// ── API routes ───────────────────────────────────────────────────────────
	api := app.Group("/api")

	// GET /api/config — server limits (read from process config, not env at request time).
	api.Get("/config", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"context_token_limit": cfg.ContextTokenLimit,
			"soft_token_limit":    cfg.SoftTokenLimit,
			"hard_token_limit":    cfg.HardTokenLimit,
		})
	})

	// GET /api/metrics — aggregated event counts + last 50 events with timestamps and track context.
	api.Get("/metrics", func(c *fiber.Ctx) error {
		return c.JSON(rec.Snapshot(50))
	})

	api.Get("/tracks", func(c *fiber.Ctx) error {
		result, err := cqrs.Ask[queries.ListTracksQuery, queries.ListTracksResult](
			c.Context(), qryBus, queries.ListTracksQuery{},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	api.Get("/tracks/:id", func(c *fiber.Ctx) error {
		result, err := cqrs.Ask[queries.GetTrackQuery, queries.GetTrackResult](
			c.Context(), qryBus, queries.GetTrackQuery{TrackID: c.Params("id")},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		return c.JSON(result)
	})

	// POST /api/tracks — create a new root track.
	// Blank:  { "branches": ["RAG Architecture", "LLM Systems"] }
	// Clone:  { "source_id": "track_1" }  — copies concept map + context from source
	// Clone with rename: { "source_id": "track_1", "branches": ["New Branch"] }
	api.Post("/tracks", func(c *fiber.Ctx) error {
		var body struct {
			Branches []string `json:"branches"`
			SourceID string   `json:"source_id"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
		}
		result, err := createTrackHandler.Handle(c.Context(), commands.CreateTrackCommand{
			Branches:      body.Branches,
			SourceTrackID: body.SourceID,
		})
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	})

	api.Post("/tracks/:id/fork", func(c *fiber.Ctx) error {
		// Predict the new track ID before dispatch (NextTrackID is deterministic;
		// no concurrent writers, so the handler will claim the same slot).
		newID, err := trackStore.NextTrackID(c.Params("id"))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if err := cmdBus.Dispatch(c.Context(), commands.DuplicateTrackCommand{
			SourceTrackID: c.Params("id"),
		}); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(fiber.Map{"new_track_id": newID})
	})

	// POST /api/tracks/merge — create a composite track from multiple source tracks.
	// Body: { "source_ids": ["track_1", "track_2"], "parent_id": "" }
	api.Post("/tracks/merge", func(c *fiber.Ctx) error {
		var body struct {
			SourceIDs []string `json:"source_ids"`
			ParentID  string   `json:"parent_id"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
		}
		if len(body.SourceIDs) < 2 {
			return fiber.NewError(fiber.StatusBadRequest, "source_ids must contain at least 2 track IDs")
		}
		result, err := mergeTracksHandler.Handle(c.Context(), commands.MergeTracksCommand{
			SourceIDs: body.SourceIDs,
			ParentID:  body.ParentID,
		})
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	})

	api.Get("/tracks/:id/context", func(c *fiber.Ctx) error {
		result, err := cqrs.Ask[queries.GetTrackContextQuery, queries.GetTrackContextResult](
			c.Context(), qryBus, queries.GetTrackContextQuery{TrackID: c.Params("id")},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	api.Put("/tracks/:id/context/:filename", func(c *fiber.Ctx) error {
		if err := cmdBus.Dispatch(c.Context(), commands.UpdateContextCommand{
			TrackID:  c.Params("id"),
			Filename: c.Params("filename"),
			Content:  string(c.Body()),
		}); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// POST /api/tracks/:id/context/:filename/compact — SSE stream.
	// Body (optional JSON): { "target_tokens": 5000 }
	api.Post("/tracks/:id/context/:filename/compact", func(c *fiber.Ctx) error {
		var body struct {
			TargetTokens int `json:"target_tokens"`
		}
		_ = json.Unmarshal(c.Body(), &body)
		chunks := compactFileHandler.Stream(c.Context(), commands.CompactFileCommand{
			TrackID:      c.Params("id"),
			Filename:     c.Params("filename"),
			TargetTokens: body.TargetTokens,
		})
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			for chunk := range chunks {
				var payload []byte
				if chunk.Error != nil {
					payload, _ = json.Marshal(map[string]string{"type": "error", "message": chunk.Error.Error()})
				} else if chunk.Done {
					payload, _ = json.Marshal(map[string]string{"type": "done"})
				} else {
					payload, _ = json.Marshal(map[string]string{"type": "chunk", "text": chunk.Text})
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.Flush()
				if chunk.Done || chunk.Error != nil {
					return
				}
			}
		}))
		return nil
	})

	// POST /api/tracks/:id/distill-threads — SSE stream.
	// Reads all tutoring threads across all sessions, extracts learning signal,
	// writes context/session_insights.snapshot.md.
	api.Post("/tracks/:id/distill-threads", func(c *fiber.Ctx) error {
		chunks := distillThreadsHandler.Stream(c.Context(), commands.DistillThreadsCommand{
			TrackID: c.Params("id"),
		})
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			for chunk := range chunks {
				var payload []byte
				if chunk.Error != nil {
					payload, _ = json.Marshal(map[string]string{"type": "error", "message": chunk.Error.Error()})
				} else if chunk.Done {
					payload, _ = json.Marshal(map[string]string{"type": "done"})
				} else {
					payload, _ = json.Marshal(map[string]string{"type": "chunk", "text": chunk.Text})
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.Flush()
				if chunk.Done || chunk.Error != nil {
					return
				}
			}
		}))
		return nil
	})

	// POST /api/tracks/:id/context/import-job — SSE stream.
	// Body: { "role_label": "ragflow_expert", "job_text": "..." }
	// Analyzes a raw job description and writes {role}.job.md to context/.
	// Done event carries the written filename in the session_id field.
	api.Post("/tracks/:id/context/import-job", func(c *fiber.Ctx) error {
		var body struct {
			RoleLabel string `json:"role_label"`
			JobText   string `json:"job_text"`
		}
		if err := json.Unmarshal(c.Body(), &body); err != nil || strings.TrimSpace(body.JobText) == "" {
			return c.Status(400).JSON(fiber.Map{"error": "job_text is required"})
		}
		chunks := analyzeJobHandler.Stream(c.Context(), commands.AnalyzeJobCommand{
			TrackID:   c.Params("id"),
			RoleLabel: body.RoleLabel,
			JobText:   body.JobText,
		})
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			for chunk := range chunks {
				var payload []byte
				if chunk.Error != nil {
					payload, _ = json.Marshal(map[string]string{"type": "error", "message": chunk.Error.Error()})
				} else if chunk.Done {
					payload, _ = json.Marshal(map[string]string{"type": "done", "filename": chunk.SessionID})
				} else {
					payload, _ = json.Marshal(map[string]string{"type": "chunk", "text": chunk.Text})
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.Flush()
				if chunk.Done || chunk.Error != nil {
					return
				}
			}
		}))
		return nil
	})

	// GET /api/tracks/:id/split-plan — returns the parsed split plan, or null if none.
	api.Get("/tracks/:id/split-plan", func(c *fiber.Ctx) error {
		plan, err := trackStore.ReadSplitPlan(c.Context(), c.Params("id"))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(fiber.Map{"split_plan": plan})
	})

	// POST /api/tracks/:id/sessions — allocate a new session directory.
	// Body (optional JSON): { "shard_id": "shard_1" }
	api.Post("/tracks/:id/sessions", func(c *fiber.Ctx) error {
		var body struct {
			ShardID string `json:"shard_id"`
		}
		// Ignore parse errors — empty body means no shard (full context).
		_ = json.Unmarshal(c.Body(), &body)
		result, err := createSessionHandler.Handle(c.Context(), commands.CreateSessionCommand{
			TrackID: c.Params("id"),
			ShardID: body.ShardID,
		})
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(result)
	})

	// GET /api/tracks/:id/sessions/:num/questions — returns empty array if not yet generated.
	api.Get("/tracks/:id/sessions/:num/questions", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		result, err := cqrs.Ask[queries.GetSessionQuestionsQuery, queries.GetSessionQuestionsResult](
			c.Context(), qryBus, queries.GetSessionQuestionsQuery{
				TrackID: c.Params("id"), SessionNumber: num,
			},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	// POST /api/tracks/:id/sessions/:num/questions/generate — SSE stream.
	api.Post("/tracks/:id/sessions/:num/questions/generate", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		chunks := generateQuestionsHandler.Stream(c.Context(), commands.GenerateQuestionsCommand{
			TrackID: c.Params("id"), SessionNumber: num,
		})

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			for chunk := range chunks {
				var payload []byte
				if chunk.Error != nil {
					payload, _ = json.Marshal(map[string]string{"type": "error", "message": chunk.Error.Error()})
				} else if chunk.Done {
					payload, _ = json.Marshal(map[string]string{"type": "done"})
				} else {
					payload, _ = json.Marshal(map[string]string{"type": "chunk", "text": chunk.Text})
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.Flush()
				if chunk.Done || chunk.Error != nil {
					return
				}
			}
		}))
		return nil
	})

	// POST /api/tracks/:id/sessions/:num/responses — batch save responses.
	api.Post("/tracks/:id/sessions/:num/responses", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		var responses []domain.Response
		if err := json.Unmarshal(c.Body(), &responses); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, err.Error())
		}
		if err := submitResponsesHandler.Handle(c.Context(), commands.SubmitResponsesCommand{
			TrackID:       c.Params("id"),
			SessionNumber: num,
			Responses:     responses,
		}); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// GET /api/tracks/:id/sessions/:num/responses
	api.Get("/tracks/:id/sessions/:num/responses", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		result, err := cqrs.Ask[queries.GetSessionResponsesQuery, queries.GetSessionResponsesResult](
			c.Context(), qryBus, queries.GetSessionResponsesQuery{
				TrackID: c.Params("id"), SessionNumber: num,
			},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		return c.JSON(result)
	})

	// GET /api/tracks/:id/sessions/:num/evaluations — returns empty array if not yet evaluated.
	api.Get("/tracks/:id/sessions/:num/evaluations", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		result, err := cqrs.Ask[queries.GetSessionEvaluationsQuery, queries.GetSessionEvaluationsResult](
			c.Context(), qryBus, queries.GetSessionEvaluationsQuery{
				TrackID: c.Params("id"), SessionNumber: num,
			},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	// POST /api/tracks/:id/sessions/:num/evaluate — SSE evaluation stream.
	api.Post("/tracks/:id/sessions/:num/evaluate", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		chunks := evaluateResponsesHandler.Stream(c.Context(), commands.EvaluateResponsesCommand{
			TrackID: c.Params("id"), SessionNumber: num,
		})

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			for chunk := range chunks {
				var payload []byte
				if chunk.Error != nil {
					payload, _ = json.Marshal(map[string]string{"type": "error", "message": chunk.Error.Error()})
				} else if chunk.Done {
					payload, _ = json.Marshal(map[string]string{"type": "done"})
				} else {
					payload, _ = json.Marshal(map[string]string{"type": "chunk", "text": chunk.Text})
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.Flush()
				if chunk.Done || chunk.Error != nil {
					return
				}
			}
		}))
		return nil
	})

	// GET /api/tracks/:id/sessions/:num/synthesis
	api.Get("/tracks/:id/sessions/:num/synthesis", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		result, err := cqrs.Ask[queries.GetSynthesisQuery, queries.GetSynthesisResult](
			c.Context(), qryBus, queries.GetSynthesisQuery{
				TrackID: c.Params("id"), SessionNumber: num,
			},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	// POST /api/tracks/:id/sessions/:num/synthesize — SSE synthesis stream.
	api.Post("/tracks/:id/sessions/:num/synthesize", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		chunks := generateSynthesisHandler.Stream(c.Context(), commands.GenerateSynthesisCommand{
			TrackID: c.Params("id"), SessionNumber: num,
		})

		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")

		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			for chunk := range chunks {
				var payload []byte
				if chunk.Error != nil {
					payload, _ = json.Marshal(map[string]string{"type": "error", "message": chunk.Error.Error()})
				} else if chunk.Done {
					payload, _ = json.Marshal(map[string]string{"type": "done"})
				} else {
					payload, _ = json.Marshal(map[string]string{"type": "chunk", "text": chunk.Text})
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.Flush()
				if chunk.Done || chunk.Error != nil {
					return
				}
			}
		}))
		return nil
	})

	// POST /api/tracks/:id/sessions/:num/apply-synthesis — idempotent.
	api.Post("/tracks/:id/sessions/:num/apply-synthesis", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		if err := applySynthesisHandler.Handle(c.Context(), commands.ApplySynthesisCommand{
			TrackID: c.Params("id"), SessionNumber: num,
		}); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// POST /api/tracks/:id/sessions/:num/detect-state — infer composite learner state,
	// patch synthesis with composite_state + nudge_suggestion. Returns updated synthesis.
	api.Post("/tracks/:id/sessions/:num/detect-state", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		synthesis, err := detectStateHandler.Handle(c.Context(), commands.DetectLearnerStateCommand{
			TrackID: c.Params("id"), SessionNumber: num,
		})
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(synthesis)
	})

	// GET /api/tracks/:id/meta-synthesis — returns meta_synthesis.json or null.
	api.Get("/tracks/:id/meta-synthesis", func(c *fiber.Ctx) error {
		result, err := cqrs.Ask[queries.GetMetaSynthesisQuery, queries.GetMetaSynthesisResult](
			c.Context(), qryBus, queries.GetMetaSynthesisQuery{TrackID: c.Params("id")},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	// GET /api/tracks/:id/meta-synthesis/readiness — returns missing shards.
	api.Get("/tracks/:id/meta-synthesis/readiness", func(c *fiber.Ctx) error {
		missing, err := metaSynthesisHandler.ReadinessCheck(c.Context(), c.Params("id"))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(fiber.Map{"missing_shards": missing, "ready": len(missing) == 0})
	})

	// POST /api/tracks/:id/meta-synthesis/generate — SSE stream.
	api.Post("/tracks/:id/meta-synthesis/generate", func(c *fiber.Ctx) error {
		chunks := metaSynthesisHandler.Stream(c.Context(), commands.MetaSynthesisCommand{
			TrackID: c.Params("id"),
		})
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			for chunk := range chunks {
				var payload []byte
				if chunk.Error != nil {
					payload, _ = json.Marshal(map[string]string{"type": "error", "message": chunk.Error.Error()})
				} else if chunk.Done {
					payload, _ = json.Marshal(map[string]string{"type": "done"})
				} else {
					payload, _ = json.Marshal(map[string]string{"type": "chunk", "text": chunk.Text})
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.Flush()
				if chunk.Done || chunk.Error != nil {
					return
				}
			}
		}))
		return nil
	})

	// POST /api/tracks/:id/meta-synthesis/apply — idempotent.
	api.Post("/tracks/:id/meta-synthesis/apply", func(c *fiber.Ctx) error {
		if err := applyMetaSynthesisHandler.Handle(c.Context(), commands.ApplyMetaSynthesisCommand{
			TrackID: c.Params("id"),
		}); err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.SendStatus(fiber.StatusNoContent)
	})

	// GET /api/tracks/:id/sessions/:num/threads/:qid — load thread history for one question.
	api.Get("/tracks/:id/sessions/:num/threads/:qid", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		result, err := cqrs.Ask[queries.GetThreadQuery, queries.GetThreadResult](
			c.Context(), qryBus, queries.GetThreadQuery{
				TrackID:       c.Params("id"),
				SessionNumber: num,
				QuestionID:    c.Params("qid"),
			},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	// GET /api/tracks/:id/sessions/:num/threads/:qid/preview — dry-run seed context.
	api.Get("/tracks/:id/sessions/:num/threads/:qid/preview", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		result, err := threadTurnHandler.Preview(c.Context(), c.Params("id"), num, c.Params("qid"))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	// POST /api/tracks/:id/sessions/:num/threads/:qid — send a message (SSE stream).
	// Body (optional JSON): { "message": "why is option B wrong?" }
	// Empty/absent message seeds the conversation from the evaluation.
	api.Post("/tracks/:id/sessions/:num/threads/:qid", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		var body struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(c.Body(), &body)

		chunks := threadTurnHandler.Stream(c.Context(), commands.ThreadTurnCommand{
			TrackID:       c.Params("id"),
			SessionNumber: num,
			QuestionID:    c.Params("qid"),
			UserMessage:   body.Message,
		})
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Set("Transfer-Encoding", "chunked")
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			for chunk := range chunks {
				var payload []byte
				if chunk.Error != nil {
					payload, _ = json.Marshal(map[string]string{"type": "error", "message": chunk.Error.Error()})
				} else if chunk.Done {
					payload, _ = json.Marshal(map[string]any{
						"type":         "done",
						"session_id":   chunk.SessionID,
						"input_tokens": chunk.InputTokens,
					})
				} else {
					payload, _ = json.Marshal(map[string]string{"type": "chunk", "text": chunk.Text})
				}
				fmt.Fprintf(w, "data: %s\n\n", payload)
				w.Flush()
				if chunk.Done || chunk.Error != nil {
					return
				}
			}
		}))
		return nil
	})

	// GET /api/tracks/:id/context-tokens — per-file token breakdown for the full inherited context.
	api.Get("/tracks/:id/context-tokens", func(c *fiber.Ctx) error {
		result, err := cqrs.Ask[queries.GetContextTokensQuery, queries.GetContextTokensResult](
			c.Context(), qryBus, queries.GetContextTokensQuery{TrackID: c.Params("id")},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	// GET /api/tracks/:id/sessions/:num/prompt-preview — builds the exact prompt
	// that would be sent to the LLM without actually calling it.
	api.Get("/tracks/:id/sessions/:num/prompt-preview", func(c *fiber.Ctx) error {
		num, err := strconv.Atoi(c.Params("num"))
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid session number")
		}
		trackID := c.Params("id")

		cm, err := trackStore.GetConceptMap(c.Context(), trackID)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}

		// Shard-aware context loading — mirrors GenerateQuestionsHandler.run
		var contextFiles map[string]string
		var contextTokens int
		var shardID string
		meta, _ := trackStore.ReadSessionMetadata(c.Context(), trackID, num)
		if meta.ShardID != "" {
			shardID = meta.ShardID
			shardFiles, shardTok, serr := trackStore.LoadContextForShard(c.Context(), trackID, meta.ShardID)
			if serr != nil {
				return fiber.NewError(fiber.StatusInternalServerError, serr.Error())
			}
			contextFiles = shardFiles
			contextTokens = shardTok
		} else {
			budget, berr := trackStore.LoadInheritedContext(c.Context(), trackID, cfg.ContextTokenLimit)
			if berr != nil {
				return fiber.NewError(fiber.StatusInternalServerError, berr.Error())
			}
			contextFiles = budget.Files
			contextTokens = budget.TokensUsed
		}

		systemPrompt := commands.BuildSystemPrompt()
		userPrompt := commands.BuildUserPrompt(trackID, "preview", cm, contextFiles)
		systemTokens := (len(systemPrompt) + 3) / 4
		userTokens := (len(userPrompt) + 3) / 4
		totalTokens := systemTokens + userTokens

		fileList := make([]string, 0, len(contextFiles))
		for name := range contextFiles {
			fileList = append(fileList, name)
		}

		// ── Context-management / resume awareness ────────────────────────────
		// call_mode: "fresh"    — no claude session ID stored; full context will be sent.
		// call_mode: "resumed"  — a claude --resume session ID exists; only the new
		//                         user prompt (delta) would be sent, saving context tokens.
		//
		// tokens_to_send: what the next LLM call would actually transmit.
		// tokens_saved:   tokens avoided by resume (0 when fresh).
		// accumulated_input_tokens: running total of tokens sent in prior LLM calls this session.
		callMode := "fresh"
		tokensToSend := totalTokens
		tokensSaved := 0
		if meta.ClaudeSessionID != "" {
			callMode = "resumed"
			tokensToSend = userTokens // only the new user message goes over the wire
			tokensSaved = systemTokens + contextTokens
		}

		return c.JSON(fiber.Map{
			"track_id":               trackID,
			"session_number":         num,
			"shard_id":               shardID,
			"call_mode":              callMode,
			"claude_session_id":      meta.ClaudeSessionID,
			"accumulated_input_tokens": meta.AccumulatedInputTokens,
			"tokens_to_send":         tokensToSend,
			"tokens_saved_by_resume": tokensSaved,
			"system_prompt":          systemPrompt,
			"user_prompt":            userPrompt,
			"system_prompt_tokens":   systemTokens,
			"user_prompt_tokens":     userTokens,
			"context_tokens":         contextTokens,
			"total_tokens":           totalTokens,
			"context_files_included": fileList,
			"soft_limit":             cfg.SoftTokenLimit,
			"hard_limit":             cfg.HardTokenLimit,
			"context_limit":          cfg.ContextTokenLimit,
		})
	})

	// ── Conversations ─────────────────────────────────────────────────────────

	// GET /api/conversations — list all conversations, newest first
	api.Get("/conversations", func(c *fiber.Ctx) error {
		result, err := cqrs.Ask[queries.ListConversationsQuery, queries.ListConversationsResult](
			c.Context(), qryBus, queries.ListConversationsQuery{},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(result)
	})

	// POST /api/conversations — create a new conversation
	// Body: { "track_ids": ["track_1", ...], "title": "optional" }
	api.Post("/conversations", func(c *fiber.Ctx) error {
		var body struct {
			TrackIDs []string `json:"track_ids"`
			Title    string   `json:"title"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
		}
		if len(body.TrackIDs) == 0 {
			return fiber.NewError(fiber.StatusBadRequest, "track_ids required")
		}
		conv, err := convTurnHandler.Create(c.Context(), body.TrackIDs, body.Title)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.Status(fiber.StatusCreated).JSON(conv)
	})

	// GET /api/conversations/:id — get conversation + its index (if any)
	api.Get("/conversations/:id", func(c *fiber.Ctx) error {
		result, err := cqrs.Ask[queries.GetConversationQuery, queries.GetConversationResult](
			c.Context(), qryBus, queries.GetConversationQuery{ConversationID: c.Params("id")},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusNotFound, err.Error())
		}
		return c.JSON(result)
	})

	// POST /api/conversations/:id/turn — SSE: one user turn, streams assistant reply
	// Body: { "message": "...", "adhoc_text": "..." }
	api.Post("/conversations/:id/turn", func(c *fiber.Ctx) error {
		var body struct {
			Message   string `json:"message"`
			AdHocText string `json:"adhoc_text"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
		}
		if strings.TrimSpace(body.Message) == "" {
			return fiber.NewError(fiber.StatusBadRequest, "message required")
		}
		cmd := commands.ConversationTurnCommand{
			ConversationID: c.Params("id"),
			UserMessage:    body.Message,
			AdHocText:      body.AdHocText,
		}
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			var conv domain.Conversation
			var turnErr error
			conv, turnErr = convTurnHandler.Stream(c.Context(), cmd, func(text string) {
				b, _ := json.Marshal(map[string]string{"type": "chunk", "text": text})
				fmt.Fprintf(w, "data: %s\n\n", b)
				w.Flush()
			})
			if turnErr != nil {
				b, _ := json.Marshal(map[string]string{"type": "error", "message": turnErr.Error()})
				fmt.Fprintf(w, "data: %s\n\n", b)
				w.Flush()
				return
			}
			// Send the last assistant message meta on Done.
			var meta *domain.ConversationMessageMeta
			for i := len(conv.Messages) - 1; i >= 0; i-- {
				if conv.Messages[i].Role == "assistant" {
					meta = conv.Messages[i].Meta
					break
				}
			}
			done := map[string]any{"type": "done"}
			if meta != nil {
				done["call_mode"] = meta.CallMode
				done["input_tokens"] = meta.InputTokensSent
				done["context_chunks"] = meta.ContextChunks
				done["claude_session_id"] = meta.ClaudeSessionID
			}
			b, _ := json.Marshal(done)
			fmt.Fprintf(w, "data: %s\n\n", b)
			w.Flush()
		}))
		return nil
	})

	// POST /api/conversations/:id/context — add track IDs to an existing conversation
	// Body: { "track_ids": ["track_2"] }
	api.Post("/conversations/:id/context", func(c *fiber.Ctx) error {
		var body struct {
			TrackIDs []string `json:"track_ids"`
		}
		if err := c.BodyParser(&body); err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "invalid JSON body")
		}
		conv, err := convTurnHandler.AddContext(c.Context(), c.Params("id"), body.TrackIDs)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.JSON(conv)
	})

	// GET /api/conversations/:id/index — get the structured index (or 404 if not indexed yet)
	api.Get("/conversations/:id/index", func(c *fiber.Ctx) error {
		idx, err := trackStore.ReadConversationIndex(c.Context(), c.Params("id"))
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		if idx.ConversationID == "" {
			return fiber.NewError(fiber.StatusNotFound, "not indexed yet — POST /index to generate")
		}
		return c.JSON(idx)
	})

	// POST /api/conversations/:id/index — SSE: generate structured index
	api.Post("/conversations/:id/index", func(c *fiber.Ctx) error {
		cmd := commands.IndexConversationCommand{ConversationID: c.Params("id")}
		c.Set("Content-Type", "text/event-stream")
		c.Set("Cache-Control", "no-cache")
		c.Set("Connection", "keep-alive")
		c.Context().SetBodyStreamWriter(fasthttp.StreamWriter(func(w *bufio.Writer) {
			idx, err := indexConvHandler.Stream(c.Context(), cmd, func(text string) {
				b, _ := json.Marshal(map[string]string{"type": "chunk", "text": text})
				fmt.Fprintf(w, "data: %s\n\n", b)
				w.Flush()
			})
			if err != nil {
				b, _ := json.Marshal(map[string]string{"type": "error", "message": err.Error()})
				fmt.Fprintf(w, "data: %s\n\n", b)
				w.Flush()
				return
			}
			done := map[string]any{
				"type":         "done",
				"turn_count":   idx.TurnCount,
				"topics":       idx.Topics,
				"generation_id": idx.GenerationID,
			}
			b, _ := json.Marshal(done)
			fmt.Fprintf(w, "data: %s\n\n", b)
			w.Flush()
		}))
		return nil
	})

	// ── SPA static serving (production) ──────────────────────────────────────
	// In dev: Vite dev server on :5173 proxies /api to here.
	// In prod: serve the built React app from web/dist/. All non-API routes
	// fall through to index.html so React Router handles client-side navigation.
	if cfg.DistDir != "" {
		app.Static("/", cfg.DistDir)
		app.Get("/*", func(c *fiber.Ctx) error {
			return c.SendFile(cfg.DistDir + "/index.html")
		})
	}

	return app
}
