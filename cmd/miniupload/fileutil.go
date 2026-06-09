package main

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') ||
			c == '.' || c == '-' || c == '_' {
			b.WriteRune(c)
		}
	}
	s := b.String()
	if s == "" || s == "." {
		return "file"
	}
	return s
}

func detectContentType(path, filename string) string {
	f, err := os.Open(path)
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	ct := http.DetectContentType(buf[:n])

	if ct == "application/octet-stream" {
		if ext := filepath.Ext(filename); ext != "" {
			if m := mimeByExt(ext); m != "" {
				return m
			}
		}
	}
	return ct
}

var extMIME = map[string]string{
	".zip":  "application/zip",
	".tar":  "application/x-tar",
	".gz":   "application/gzip",
	".bz2":  "application/x-bzip2",
	".xz":   "application/x-xz",
	".7z":   "application/x-7z-compressed",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".mp3":  "audio/mpeg",
	".ogg":  "audio/ogg",
	".txt":  "text/plain",
	".log":  "text/plain",
	".json": "application/json",
	".xml":  "application/xml",
	".sh":   "text/x-shellscript",
	".py":   "text/x-python",
	".go":   "text/x-go",
}

func mimeByExt(ext string) string {
	return extMIME[strings.ToLower(ext)]
}
