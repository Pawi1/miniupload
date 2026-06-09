package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func cleanupLoop() {
	ticker := time.NewTicker(cfg.CleanupInterval)
	defer ticker.Stop()
	runCleanup()
	for range ticker.C {
		runCleanup()
	}
}

func runCleanup() {
	rows, err := db.Query(
		`SELECT id, stored_filename FROM uploads WHERE expires_at <= ?`,
		time.Now().UTC(),
	)
	if err != nil {
		log.Printf("cleanup query error: %v", err)
		return
	}
	defer rows.Close()

	var ids []string
	var paths []string
	for rows.Next() {
		var id, name string
		if err := rows.Scan(&id, &name); err != nil {
			continue
		}
		ids = append(ids, id)
		paths = append(paths, filepath.Join(cfg.DataDir, "files", name))
	}
	rows.Close()

	if len(ids) == 0 {
		return
	}

	for i, p := range paths {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			log.Printf("cleanup: cannot remove %s: %v", p, err)
		} else {
			log.Printf("cleanup: removed %s (id=%s)", p, ids[i])
		}
	}

	placeholders := strings.Repeat("?,", len(ids))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	if _, err := db.Exec("DELETE FROM uploads WHERE id IN ("+placeholders+")", args...); err != nil {
		log.Printf("cleanup delete error: %v", err)
	}
}
