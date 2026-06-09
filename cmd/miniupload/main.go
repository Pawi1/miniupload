package main

import (
	"embed"
	"log"
	"net/http"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

//go:embed index.html
var staticFiles embed.FS

// version is set via -ldflags="-X main.version=v1.2.3" at build time.
var version = "dev"

func main() {
	cfg = loadConfig()

	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "files"), 0755); err != nil {
		log.Fatalf("cannot create data dir: %v", err)
	}

	if err := openDB(cfg.DatabasePath + "?_journal=WAL&_timeout=5000"); err != nil {
		log.Fatalf("cannot open database: %v", err)
	}
	defer db.Close()

	if err := initDB(); err != nil {
		log.Fatalf("cannot init database: %v", err)
	}

	go cleanupLoop()

	mux := newMux()

	log.Printf("miniupload %s listening on :%s", version, cfg.Port)
	log.Printf("base URL: %s", cfg.BaseURL)
	log.Printf("data dir: %s", cfg.DataDir)
	log.Printf("max file size: %d MB", cfg.MaxFileSizeBytes/1024/1024)

	if err := http.ListenAndServe(":"+cfg.Port, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func newMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /upload", handleUpload)
	mux.HandleFunc("GET /", handleIndex)
	mux.HandleFunc("GET /{id}", handleDownload)
	return mux
}
