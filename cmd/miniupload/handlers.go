package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := staticFiles.ReadFile("index.html")
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if cfg.UploadToken != "" {
		auth := r.Header.Get("Authorization")
		token, _ := strings.CutPrefix(auth, "Bearer ")
		if token != cfg.UploadToken {
			jsonError(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxFileSizeBytes+4096)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		if strings.Contains(err.Error(), "request body too large") {
			jsonError(w, fmt.Sprintf("file too large (max %d MB)", cfg.MaxFileSizeBytes/1024/1024), http.StatusRequestEntityTooLarge)
		} else {
			jsonError(w, "bad request: "+err.Error(), http.StatusBadRequest)
		}
		return
	}

	ttl, ok := parseTTL(r.FormValue("ttl"))
	if !ok {
		jsonError(w, "invalid ttl; allowed: 1h, 6h, 24h, 3d, 7d, 30d", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		jsonError(w, "missing file field", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if header.Size > cfg.MaxFileSizeBytes {
		jsonError(w, fmt.Sprintf("file too large (max %d MB)", cfg.MaxFileSizeBytes/1024/1024), http.StatusRequestEntityTooLarge)
		return
	}

	id, err := generateID()
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	storedName := id + filepath.Ext(sanitizeFilename(header.Filename))
	destPath := filepath.Join(cfg.DataDir, "files", storedName)

	if filepath.Dir(destPath) != filepath.Join(cfg.DataDir, "files") {
		jsonError(w, "invalid filename", http.StatusBadRequest)
		return
	}

	dest, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		log.Printf("cannot create file %s: %v", destPath, err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	written, err := io.Copy(dest, file)
	dest.Close()
	if err != nil {
		os.Remove(destPath)
		jsonError(w, "upload failed", http.StatusInternalServerError)
		return
	}

	ct := header.Header.Get("Content-Type")
	if ct == "" || ct == "application/octet-stream" {
		ct = detectContentType(destPath, header.Filename)
	}

	now := time.Now().UTC()
	expiresAt := now.Add(ttl)

	_, err = db.Exec(`INSERT INTO uploads
		(id, original_filename, stored_filename, content_type, size_bytes, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, header.Filename, storedName, ct, written, now, expiresAt)
	if err != nil {
		os.Remove(destPath)
		log.Printf("db insert error: %v", err)
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"url":        cfg.BaseURL + "/" + id,
		"id":         id,
		"expires_at": expiresAt.Format(time.RFC3339),
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.NotFound(w, r)
		return
	}

	var orig, stored, ct string
	var expiresAt time.Time

	err := db.QueryRowContext(context.Background(),
		`SELECT original_filename, stored_filename, content_type, expires_at
		 FROM uploads WHERE id = ?`, id).
		Scan(&orig, &stored, &ct, &expiresAt)
	if err == sql.ErrNoRows {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if time.Now().UTC().After(expiresAt) {
		http.Error(w, "gone — this file has expired", http.StatusGone)
		return
	}

	filePath := filepath.Join(cfg.DataDir, "files", stored)
	cleanPath, err := filepath.Abs(filePath)
	if err != nil || !strings.HasPrefix(cleanPath, filepath.Join(cfg.DataDir, "files")) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	f, err := os.Open(cleanPath)
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	db.Exec(`UPDATE uploads SET download_count = download_count + 1 WHERE id = ?`, id)

	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, orig))
	w.Header().Set("X-Expires-At", expiresAt.Format(time.RFC3339))
	w.Header().Set("Cache-Control", "private, no-store")

	http.ServeContent(w, r, stored, stat.ModTime(), f)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
