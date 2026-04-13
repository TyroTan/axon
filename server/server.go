// Package server wires all dependencies and registers routes.
// This is the composition root — the only place that knows about all packages.
// Fiber is a pure JSON API server. The React UI (ui/) handles all rendering.
package server

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

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

	// ── Command bus ──────────────────────────────────────────────────────────
	cmdBus := cqrs.NewCommandBus()
	_ = cmdBus // commands registered in subsequent phases

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
