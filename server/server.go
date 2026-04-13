// Package server wires all dependencies and registers routes.
// This is the composition root — the only place that knows about all packages.
package server

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/template/html/v2"

	"github.com/tyrohunt/axon/internal/cqrs"
	"github.com/tyrohunt/axon/internal/queries"
	"github.com/tyrohunt/axon/internal/store/filesystem"
	"github.com/tyrohunt/axon/internal/view"
)

// Config holds server configuration.
type Config struct {
	ExperimentsDir string // absolute path to .experiments/ folder
	Port           string // e.g. "3456"
	WebDir         string // absolute path to web/ folder (templates + static)
}

// New wires dependencies and returns a configured Fiber app.
func New(cfg Config) *fiber.App {
	// ── Template engine ──────────────────────────────────────────────────────
	engine := html.New(cfg.WebDir+"/templates", ".html")
	engine.AddFunc("branchInitials", func(branches []string) string {
		out := make([]string, 0, len(branches))
		for _, b := range branches {
			words := strings.Fields(b)
			initials := ""
			for _, w := range words {
				if len(w) > 0 {
					initials += strings.ToUpper(string(w[0]))
				}
			}
			out = append(out, initials)
		}
		return strings.Join(out, "·")
	})

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
	_ = cmdBus // commands added in subsequent phases

	// ── Fiber app ────────────────────────────────────────────────────────────
	app := fiber.New(fiber.Config{
		AppName: "Axon",
		Views:   engine,
	})
	app.Use(recover.New())
	app.Use(logger.New())

	// ── Static files ─────────────────────────────────────────────────────────
	app.Static("/static", cfg.WebDir+"/static")

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

	// Sidebar partial — HTMX fetches this on load.
	api.Get("/sidebar", func(c *fiber.Ctx) error {
		result, err := cqrs.Ask[queries.ListTracksQuery, queries.ListTracksResult](
			c.Context(), qryBus, queries.ListTracksQuery{},
		)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, err.Error())
		}
		return c.Render("partials/sidebar", fiber.Map{
			"Tracks": view.ToSidebarTracks(result.Tracks, ""),
		})
	})

	// ── HTML routes ──────────────────────────────────────────────────────────
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{
			"Title":       "Home",
			"Breadcrumbs": []view.Breadcrumb{{Label: "Home"}},
		}, "layout")
	})

	return app
}
