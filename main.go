package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/tyrohunt/axon/server"
)

func main() {
	// Default experiments dir = directory of the binary (i.e. .experiments/ itself).
	experimentsDir := os.Getenv("AXON_DIR")
	if experimentsDir == "" {
		exe, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		experimentsDir = filepath.Dir(exe)
	}

	// DistDir: built React app served as SPA in production.
	// Set to "" to disable (dev mode: Vite dev server handles the UI).
	distDir := os.Getenv("AXON_DIST_DIR")
	if distDir == "" {
		distDir = filepath.Join(experimentsDir, "web", "dist")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3456"
	}

	app := server.New(server.Config{
		ExperimentsDir:  experimentsDir,
		DistDir:         distDir,
		Port:            port,
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
	})

	log.Printf("Axon API on http://localhost:%s  |  UI on http://localhost:5173 (dev)", port)
	log.Fatal(app.Listen(":" + port))
}
