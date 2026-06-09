package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var allowedTTLs = map[string]time.Duration{
	"1h":  time.Hour,
	"6h":  6 * time.Hour,
	"24h": 24 * time.Hour,
	"3d":  72 * time.Hour,
	"7d":  168 * time.Hour,
	"30d": 720 * time.Hour,
}

type config struct {
	BaseURL          string
	UploadToken      string
	DataDir          string
	DatabasePath     string
	MaxFileSizeBytes int64
	DefaultTTL       time.Duration
	CleanupInterval  time.Duration
	Port             string
}

var cfg config

func loadConfig() config {
	maxMB := int64(1024)
	if v := os.Getenv("MAX_FILE_SIZE_MB"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			maxMB = n
		}
	}

	defaultTTL := 24 * time.Hour
	if v := os.Getenv("DEFAULT_TTL"); v != "" {
		if d, ok := allowedTTLs[v]; ok {
			defaultTTL = d
		}
	}

	cleanupSecs := 300
	if v := os.Getenv("CLEANUP_INTERVAL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cleanupSecs = n
		}
	}

	dataDir := "./data"
	if v := os.Getenv("DATA_DIR"); v != "" {
		dataDir = v
	}

	dbPath := filepath.Join(dataDir, "uploads.sqlite")
	if v := os.Getenv("DATABASE_PATH"); v != "" {
		dbPath = v
	}

	port := "3000"
	if v := os.Getenv("PORT"); v != "" {
		port = v
	}

	baseURL := "http://localhost:" + port
	if v := os.Getenv("BASE_URL"); v != "" {
		baseURL = strings.TrimRight(v, "/")
	}

	token := os.Getenv("UPLOAD_TOKEN")
	if token == "" {
		log.Println("WARNING: UPLOAD_TOKEN is not set — upload endpoint is unprotected!")
	}

	return config{
		BaseURL:          baseURL,
		UploadToken:      token,
		DataDir:          dataDir,
		DatabasePath:     dbPath,
		MaxFileSizeBytes: maxMB * 1024 * 1024,
		DefaultTTL:       defaultTTL,
		CleanupInterval:  time.Duration(cleanupSecs) * time.Second,
		Port:             port,
	}
}

func parseTTL(s string) (time.Duration, bool) {
	if s == "" {
		return cfg.DefaultTTL, true
	}
	d, ok := allowedTTLs[s]
	return d, ok
}
