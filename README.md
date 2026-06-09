# miniupload

Private file drop with short links, TTL, and token-based authorization.

```
upload via curl → short link → wget just works
```

---

## Local setup

```bash
# 1. Build
make build
# or: go build -o miniupload ./cmd/miniupload/

# 2. Configure
cp .env.example .env
# edit .env — set UPLOAD_TOKEN and BASE_URL

# 3. Run
source .env && ./miniupload
```

The server starts on `http://localhost:3000` (or whatever `PORT` is set to in .env).

---

## Configuration (.env)

| Variable                   | Description                               | Default                  |
|----------------------------|-------------------------------------------|--------------------------|
| `BASE_URL`                 | Public address used in generated links    | `http://localhost:PORT`  |
| `UPLOAD_TOKEN`             | Bearer token for upload authorization     | _(empty = no protection)_|
| `PORT`                     | HTTP port                                 | `3000`                   |
| `DATA_DIR`                 | Directory for files and database          | `./data`                 |
| `DATABASE_PATH`            | Path to SQLite database                   | `./data/uploads.sqlite`  |
| `MAX_FILE_SIZE_MB`         | Maximum file size in MB                   | `1024`                   |
| `DEFAULT_TTL`              | TTL when not specified by the client      | `24h`                    |
| `CLEANUP_INTERVAL_SECONDS` | How often to sweep expired files          | `300`                    |

---

## Usage

### Upload via curl

```bash
curl -X POST https://your-domain.com/upload \
  -H "Authorization: Bearer TOKEN" \
  -F "file=@archive.zip" \
  -F "ttl=7d"
```

Response:

```json
{
  "url": "https://your-domain.com/aB92kLm3xY",
  "id":  "aB92kLm3xY",
  "expires_at": "2026-06-16T12:00:00Z"
}
```

### Download via wget

```bash
wget https://your-domain.com/aB92kLm3xY
```

The file is saved under its original name via `Content-Disposition`.

### upload.sh helper

```bash
export UPLOAD_URL=https://your-domain.com
export UPLOAD_TOKEN=your-token

./upload.sh archive.zip 7d
# → https://your-domain.com/aB92kLm3xY
```

### Available TTL values

`1h` · `6h` · `24h` · `3d` · `7d` · `30d`

---

## Frontend

Open `https://your-domain.com/` in a browser.

- Token entered once is saved in localStorage
- Drag & drop or click to select a file
- Upload progress bar
- Generated link with a copy button and expiry date

---

## Make targets

```bash
make build        # compile with version from git describe
make test         # go test -race ./...
make vet          # go vet
make run          # build + run
make clean        # remove binary
make build-all    # cross-compile all platforms to dist/
```

---

## Deployment

### 1. Build and copy the binary

```bash
make build
scp miniupload user@server:/opt/miniupload/
```

### 2. Set up the server environment

```bash
sudo useradd -r -s /sbin/nologin -d /opt/miniupload miniupload
sudo mkdir -p /opt/miniupload/data/files
sudo chown -R miniupload:miniupload /opt/miniupload
sudo cp .env.example /opt/miniupload/.env
sudo nano /opt/miniupload/.env
sudo chmod 600 /opt/miniupload/.env
sudo chown miniupload:miniupload /opt/miniupload/.env
```

### 3a. systemd

```bash
sudo cp deploy/miniupload.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now miniupload
sudo systemctl status miniupload
```

### 3b. OpenRC (Alpine, Gentoo)

```bash
sudo cp deploy/miniupload.openrc /etc/init.d/miniupload
sudo chmod +x /etc/init.d/miniupload
sudo rc-update add miniupload default
sudo rc-service miniupload start
```

---

## Project structure

```
miniupload/
├── cmd/miniupload/
│   ├── main.go          # entry point, routing
│   ├── config.go        # config struct, env loading, TTL parsing
│   ├── db.go            # SQLite, schema, ID generation
│   ├── handlers.go      # HTTP handlers
│   ├── cleanup.go       # background cleanup goroutine
│   ├── fileutil.go      # filename sanitization, MIME detection
│   ├── handlers_test.go # integration tests
│   └── index.html       # embedded frontend
├── deploy/
│   ├── miniupload.service   # systemd
│   └── miniupload.openrc    # OpenRC
├── .github/workflows/
│   ├── ci.yml           # test on every push/PR
│   └── release.yml      # build + release on v* tag
├── Makefile
├── upload.sh            # CLI helper
├── .env.example
└── go.mod / go.sum
```

---

## Security

- Files on disk use random names — the original filename never touches the filesystem path
- Path traversal blocked at two levels: sanitization + `filepath.Abs` check
- Files are served as `attachment` — never executed by the server
- Token does not appear in logs
- Upload without token → 401
- Expired files → 410 Gone
- Body size capped via `MaxBytesReader` before multipart parsing
