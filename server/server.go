// Package server wires all dependencies and registers routes.
// This is the composition root — the only place that knows about all packages.
// Fiber is a pure JSON API server. The React UI (ui/) handles all rendering.
package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/tyrohunt/axon/internal/commands"
	"github.com/tyrohunt/axon/internal/cqrs"
	"github.com/tyrohunt/axon/internal/queries"
	"github.com/tyrohunt/axon/internal/store/filesystem"
)

// Config holds server configuration.
type Config struct {
	ExperimentsDir string // absolute path to .experiments/ folder
	Port           string // e.g. "3456"
	DistDir        string // absolute path to web/dist/ (served as SPA in production)
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

	// ── Command bus ──────────────────────────────────────────────────────────
	cmdBus := cqrs.NewCommandBus()
	cqrs.Register[commands.DuplicateTrackCommand](
		cmdBus, commands.NewDuplicateTrackHandler(trackStore),
	)
	cqrs.Register[commands.UpdateContextCommand](
		cmdBus, commands.NewUpdateContextHandler(trackStore),
	)

	// ── Fiber app ────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName: "Axon",
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// ── API routes ───────────────────────────────────────────────────────────
	api := app.Group("/api")

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
