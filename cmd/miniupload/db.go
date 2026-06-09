package main

import (
	"crypto/rand"
	"database/sql"
	"math/big"
)

const idAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const idLength = 10

var db *sql.DB

func openDB(dsn string) error {
	var err error
	db, err = sql.Open("sqlite", dsn)
	return err
}

func initDB() error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS uploads (
		id                TEXT PRIMARY KEY,
		original_filename TEXT NOT NULL,
		stored_filename   TEXT NOT NULL,
		content_type      TEXT NOT NULL,
		size_bytes        INTEGER NOT NULL,
		created_at        DATETIME NOT NULL,
		expires_at        DATETIME NOT NULL,
		download_count    INTEGER NOT NULL DEFAULT 0
	)`)
	return err
}

func generateID() (string, error) {
	b := make([]byte, idLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(idAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = idAlphabet[n.Int64()]
	}
	return string(b), nil
}
