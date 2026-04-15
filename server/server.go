// Package server wires all dependencies and registers routes.
// This is the composition root — the only place that knows about all packages.
// Fiber is a pure JSON API server. The React UI (ui/) handles all rendering.
package server

import (
	"bufio"
	"encoding/json"
	"fmt"
	"strconv"

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
	cqrs.Register[commands.DuplicateTrackCommand](
		cmdBus, commands.NewDuplicateTrackHandler(trackStore),
	)
	cqrs.Register[commands.UpdateContextCommand](
		cmdBus, commands.NewUpdateContextHandler(trackStore),
	)

	createSessionHandler := commands.NewCreateSessionHandler(trackStore)
	generateQuestionsHandler := commands.NewGenerateQuestionsHandler(trackStore, llmClient, cfg.ContextTokenLimit, cfg.SoftTokenLimit, cfg.HardTokenLimit, rec)
	submitResponsesHandler := commands.NewSubmitResponsesHandler(trackStore)
	evaluateResponsesHandler := commands.NewEvaluateResponsesHandler(trackStore, llmClient, rec)
	generateSynthesisHandler := commands.NewGenerateSynthesisHandler(trackStore, llmClient, rec)
	applySynthesisHandler := commands.NewApplySynthesisHandler(trackStore)
	metaSynthesisHandler := commands.NewMetaSynthesisHandler(trackStore, llmClient, rec)
	applyMetaSynthesisHandler := commands.NewApplyMetaSynthesisHandler(trackStore)

	// ── Fiber app ────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName: "Axon",
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// ── API routes ───────────────────────────────────────────────────────────
	api := app.Group("/api")

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

	api.Post("/tracks/:id/duplicate", func(c *fiber.Ctx) error {
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
