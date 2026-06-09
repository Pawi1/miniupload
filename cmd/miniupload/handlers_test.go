package main

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTest(t *testing.T) func() {
	t.Helper()

	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0755); err != nil {
		t.Fatal(err)
	}

	cfg = config{
		BaseURL:          "http://example.com",
		UploadToken:      "testtoken",
		DataDir:          dir,
		DatabasePath:     filepath.Join(dir, "test.sqlite"),
		MaxFileSizeBytes: 10 * 1024 * 1024,
		DefaultTTL:       24 * time.Hour,
		CleanupInterval:  time.Hour,
		Port:             "3000",
	}

	if err := openDB(cfg.DatabasePath); err != nil {
		t.Fatal(err)
	}
	if err := initDB(); err != nil {
		t.Fatal(err)
	}

	return func() { db.Close() }
}

func buildUploadReq(t *testing.T, content []byte, filename, ttl, token string) *http.Request {
	t.Helper()

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	fw.Write(content)
	if ttl != "" {
		w.WriteField("ttl", ttl)
	}
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

func doUpload(t *testing.T, content []byte, filename, ttl, token string) *httptest.ResponseRecorder {
	t.Helper()
	rr := httptest.NewRecorder()
	handleUpload(rr, buildUploadReq(t, content, filename, ttl, token))
	return rr
}

func TestIndex(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handleIndex(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("GET / = %d, want 200", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
	if !strings.Contains(rr.Body.String(), "miniupload") {
		t.Error("response body does not contain 'miniupload'")
	}
}

func TestIndex_NotFound(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
	rr := httptest.NewRecorder()
	handleIndex(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("GET /robots.txt = %d, want 404", rr.Code)
	}
}

func TestUpload_NoToken(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if rr := doUpload(t, []byte("hi"), "f.txt", "24h", ""); rr.Code != http.StatusUnauthorized {
		t.Fatalf("no token = %d, want 401", rr.Code)
	}
}

func TestUpload_WrongToken(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if rr := doUpload(t, []byte("hi"), "f.txt", "24h", "wrong"); rr.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token = %d, want 401", rr.Code)
	}
}

func TestUpload_BadTTL(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if rr := doUpload(t, []byte("hi"), "f.txt", "99y", "testtoken"); rr.Code != http.StatusBadRequest {
		t.Fatalf("bad ttl = %d, want 400", rr.Code)
	}
}

func TestUpload_Success(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	content := []byte("hello world")
	rr := doUpload(t, content, "hello.txt", "7d", "testtoken")

	if rr.Code != http.StatusCreated {
		t.Fatalf("upload = %d, want 201; body: %s", rr.Code, rr.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	for _, key := range []string{"url", "id", "expires_at"} {
		if resp[key] == "" {
			t.Errorf("response missing %q", key)
		}
	}
	if !strings.HasPrefix(resp["url"], "http://example.com/") {
		t.Errorf("url %q has wrong prefix", resp["url"])
	}
}

func TestUpload_DefaultTTL(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	if rr := doUpload(t, []byte("data"), "f.bin", "", "testtoken"); rr.Code != http.StatusCreated {
		t.Fatalf("default ttl upload = %d, want 201", rr.Code)
	}
}

func TestDownload_Success(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	content := []byte("file content here")
	rr := doUpload(t, content, "myfile.txt", "24h", "testtoken")
	if rr.Code != http.StatusCreated {
		t.Fatalf("upload failed: %d", rr.Code)
	}

	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)

	mux := newMux()
	req := httptest.NewRequest(http.MethodGet, "/"+resp["id"], nil)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req)

	if rr2.Code != http.StatusOK {
		t.Fatalf("download = %d, want 200; body: %s", rr2.Code, rr2.Body.String())
	}
	if cd := rr2.Header().Get("Content-Disposition"); !strings.Contains(cd, "myfile.txt") {
		t.Errorf("Content-Disposition %q missing original filename", cd)
	}
	if got, _ := io.ReadAll(rr2.Body); !bytes.Equal(got, content) {
		t.Error("downloaded content differs from uploaded")
	}
}

func TestDownload_NotFound(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	mux := newMux()
	req := httptest.NewRequest(http.MethodGet, "/doesnotexist", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("unknown id = %d, want 404", rr.Code)
	}
}

func TestDownload_Expired(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	rr := doUpload(t, []byte("data"), "old.txt", "1h", "testtoken")
	if rr.Code != http.StatusCreated {
		t.Fatal("upload failed")
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)

	db.Exec(`UPDATE uploads SET expires_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Hour), resp["id"])

	mux := newMux()
	req := httptest.NewRequest(http.MethodGet, "/"+resp["id"], nil)
	rr2 := httptest.NewRecorder()
	mux.ServeHTTP(rr2, req)

	if rr2.Code != http.StatusGone {
		t.Fatalf("expired file = %d, want 410", rr2.Code)
	}
}

func TestCleanup(t *testing.T) {
	teardown := setupTest(t)
	defer teardown()

	rr := doUpload(t, []byte("cleanup-me"), "old.bin", "1h", "testtoken")
	if rr.Code != http.StatusCreated {
		t.Fatal("upload failed")
	}
	var resp map[string]string
	json.Unmarshal(rr.Body.Bytes(), &resp)
	id := resp["id"]

	db.Exec(`UPDATE uploads SET expires_at = ? WHERE id = ?`,
		time.Now().UTC().Add(-time.Hour), id)

	runCleanup()

	var count int
	db.QueryRow(`SELECT COUNT(*) FROM uploads WHERE id = ?`, id).Scan(&count)
	if count != 0 {
		t.Error("cleanup did not remove expired record from DB")
	}
}

func TestSanitizeFilename(t *testing.T) {
	cases := []struct{ in, want string }{
		{"hello.txt", "hello.txt"},
		{"../../../etc/passwd", "passwd"},
		{"фото.jpg", ".jpg"},
		{"my file (1).zip", "myfile1.zip"},
		{"", "file"},
		{"...", "..."},
	}
	for _, c := range cases {
		if got := sanitizeFilename(c.in); got != c.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestParseTTL(t *testing.T) {
	cfg.DefaultTTL = 24 * time.Hour

	for _, s := range []string{"1h", "6h", "24h", "3d", "7d", "30d"} {
		if _, ok := parseTTL(s); !ok {
			t.Errorf("parseTTL(%q) = false, want true", s)
		}
	}
	if _, ok := parseTTL("99y"); ok {
		t.Error("parseTTL('99y') = true, want false")
	}
	if d, ok := parseTTL(""); !ok || d != 24*time.Hour {
		t.Errorf("parseTTL('') = (%v, %v), want (24h, true)", d, ok)
	}
}
