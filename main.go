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

	// WebDir defaults to <experimentsDir>/web
	webDir := os.Getenv("AXON_WEB_DIR")
	if webDir == "" {
		webDir = filepath.Join(experimentsDir, "web")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "3456"
	}

	app := server.New(server.Config{
		ExperimentsDir: experimentsDir,
		WebDir:         webDir,
		Port:           port,
	})

	log.Printf("Axon running on http://localhost:%s", port)
	log.Fatal(app.Listen(":" + port))
}
