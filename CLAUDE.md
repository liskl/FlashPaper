# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

FlashPaper is a Go implementation of PrivateBin - a zero-knowledge encrypted pastebin where the server never sees unencrypted content. All encryption/decryption happens client-side in the browser.

**Key Features:**
- Full API compatibility with PrivateBin
- AES-256-GCM encryption (client-side)
- PBKDF2-SHA256 key derivation (100,000 iterations)
- Burn-after-reading option
- Configurable expiration (5min to never)
- Optional password protection
- Threaded discussions/comments
- Multiple storage backends (SQLite, PostgreSQL, MySQL, Filesystem)
- Single binary with embedded frontend
- Light/dark mode theme toggle with localStorage persistence

## Build and Run Commands

```bash
# Build the application (requires CGO for SQLite)
CGO_ENABLED=1 go build -o flashpaper ./cmd/flashpaper

# Run the application
./flashpaper

# Run with configuration file
./flashpaper -config config.ini

# Run tests
CGO_ENABLED=1 go test ./...

# Run tests with verbose output
CGO_ENABLED=1 go test -v ./...

# Run tests with coverage
CGO_ENABLED=1 go test -cover ./...

# Generate coverage report
CGO_ENABLED=1 go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# Using Makefile (recommended)
make build          # Build binary
make run            # Build and run
make test           # Run tests
make lint           # Run linter (golangci-lint)
make fmt            # Format code
make docker         # Build Docker image
make up             # Start with docker-compose
make dev            # Development with hot reload
make install-tools  # Install dev tools (air, golangci-lint)
make mod-tidy       # Clean up go.mod
make db-shell       # Open PostgreSQL shell

# E2E Tests (Playwright)
npm install                      # Install dependencies
npx playwright install chromium  # Install browser
npm run test:e2e                 # Run all e2e tests
npm run test:e2e:ui              # Run with interactive UI
```

## Architecture

```
flashpaper/
├── cmd/flashpaper/main.go       # Entry point, CLI flags, startup
├── internal/
│   ├── config/                  # INI configuration parsing
│   │   ├── config.go            # Config structs and loading
│   │   └── config_test.go       # Config tests
│   ├── handler/                 # HTTP request handlers (API endpoints)
│   │   ├── handler.go           # Main routing, template serving
│   │   ├── handler_test.go      # Handler tests
│   │   ├── paste.go             # Create, read, delete paste endpoints
│   │   └── comment.go           # Comment creation, rate limiting
│   ├── middleware/              # Security headers middleware
│   │   └── security.go          # CSP, X-Frame-Options, etc.
│   ├── model/                   # Data models (Paste, Comment)
│   │   ├── paste.go             # Paste struct and validation
│   │   ├── comment.go           # Comment struct and validation
│   │   ├── errors.go            # Domain error types
│   │   └── *_test.go            # Model tests
│   ├── server/                  # HTTP server setup
│   │   └── server.go            # Server configuration
│   ├── storage/                 # Storage interface and implementations
│   │   ├── storage.go           # Storage interface definition
│   │   ├── database.go          # SQLite/PostgreSQL/MySQL impl
│   │   ├── filesystem.go        # File-based storage impl
│   │   ├── mock.go              # Mock storage for testing
│   │   └── *_test.go            # Storage tests
│   └── util/                    # Crypto, ID generation utilities
│       ├── crypto.go            # HMAC, salt, vizhash generation
│       ├── id.go                # Paste/comment ID generation
│       └── *_test.go            # Util tests
├── web/
│   ├── static/
│   │   ├── js/flashpaper.js     # Client-side encryption, theme toggle
│   │   └── css/style.css        # Styles with light/dark theme support
│   └── templates/
│       ├── index.html           # Main HTML template
│       ├── docs.html            # Documentation page template
│       └── implementation.html  # How It Works page template
├── e2e/                         # Playwright end-to-end tests
│   ├── paste.spec.ts            # Paste CRUD and action tests
│   ├── theme.spec.ts            # Theme toggle tests
│   ├── burn.spec.ts             # Burn-after-reading tests
│   ├── password.spec.ts         # Password protection tests
│   └── navigation.spec.ts       # Navigation link tests
├── deploy/
│   └── kustomize/               # Kubernetes deployment manifests
│       ├── base/                # Base deployment and service
│       └── overlays/dev-example/ # Example development overlay with PostgreSQL
├── docs/                        # Markdown documentation source
│   ├── documentation.md         # User documentation
│   └── implementation.md        # Technical implementation details
├── embed.go                     # Go embed directives
├── package.json                 # Node.js dependencies (Playwright)
├── playwright.config.ts         # Playwright test configuration
├── Dockerfile                   # Multi-stage production build
├── docker-compose.yml           # Production stack (PostgreSQL)
└── docker-compose.dev.yml       # Development with hot reload
```

### Key Components

**Storage Layer** (`internal/storage/`):
- `Storage` interface defines all persistence operations
- `DatabaseStorage` supports SQLite, PostgreSQL, MySQL
- `FilesystemStorage` stores pastes as files with nested directories
- `Mock` storage for testing handlers without database
- Tables: `paste`, `comment`, `config`

**Handlers** (`internal/handler/`):
- `handler.go`: Main routing, template serving, JSON helpers
- `paste.go`: Create, read, delete paste endpoints
- `comment.go`: Comment creation, rate limiting, IP hashing

**Client-Side JavaScript** (`web/static/js/flashpaper.js`):
- AES-256-GCM via Web Crypto API
- PBKDF2-SHA256 with 100,000 iterations
- 256-bit random key in URL fragment (Base58)
- Zlib compression for content
- Theme toggle (light/dark mode with localStorage)
- Delete token stored in sessionStorage for paste deletion

**CSS Theming** (`web/static/css/style.css`):
- Light mode (default) with CSS custom properties
- Dark mode activated via `[data-theme="dark"]` attribute
- Greyscale dark theme palette

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/` | Serve UI |
| GET | `/?{pasteID}` | View paste (HTML) or get paste data (JSON if X-Requested-With header) |
| POST | `/` | Create paste or comment |
| DELETE | `/` | Delete paste (with deletetoken) |
| GET | `/health` | Health check |
| GET | `/implementation` | How It Works page (technical details) |
| GET | `/docs` | Documentation page (user guide) |
| GET | `/js/*`, `/css/*` | Static assets (embedded) |

### Request/Response Format

**Create Paste Request:**
```json
{
  "v": 2,
  "ct": "base64_ciphertext",
  "adata": [["iv","salt",100000,256,128,"aes","gcm","zlib"],"plaintext",0,0],
  "meta": {"expire": "1day"}
}
```

**Create Paste Response:**
```json
{
  "status": 0,
  "id": "f468483c313401e8",
  "url": "/?f468483c313401e8",
  "deletetoken": "hex_hmac_sha256"
}
```

**Delete Paste Request:**
```json
{
  "pasteid": "f468483c313401e8",
  "deletetoken": "hex_hmac_sha256"
}
```

## Configuration

Configuration via INI file or environment variables:

```ini
[main]
name = "FlashPaper"              # Instance name shown in UI
basepath = "/"                   # URL base path
discussion = true                # Enable threaded comments
sizelimit = 10485760             # Max paste size in bytes (10MB)
burnafterreadingselected = false # Default burn checkbox state
opendiscussion = true            # Allow discussions without password
formatter = "plaintext"          # Default: plaintext, syntaxhighlighting, markdown
password = true                  # Enable password protection feature
fileupload = false               # Enable file attachments (not implemented)
icon = "identicon"               # Comment icons: identicon, vizhash, none

[expire]
default = "1week"                # Default expiration

[traffic]
limit = 10                       # Seconds between paste creations (rate limit)
header = "X-Forwarded-For"       # Header for real IP (X-Forwarded-For, X-Real-IP, CF-Connecting-IP)
exempted = ""                    # Comma-separated IPs exempt from rate limiting
creators = ""                    # Whitelist of IPs allowed to create pastes (empty = all)

[purge]
limit = 300                      # Minimum seconds between purge runs
batchsize = 10                   # Number of expired pastes to delete per run

[model]
class = "Database"               # Storage backend: Database, Filesystem
dsn = "/data/flashpaper.db"      # Connection string (see Storage Backends)
```

Environment variable format: `FLASHPAPER_SECTION_KEY`
Example: `FLASHPAPER_MODEL_DSN=postgres://...`

### Storage Backend DSN Examples

**SQLite (default):**
```
/data/flashpaper.db
```

**PostgreSQL:**
```
postgres://user:password@localhost:5432/flashpaper?sslmode=disable
```

**MySQL:**
```
user:password@tcp(localhost:3306)/flashpaper?charset=utf8mb4
```

## Testing

Tests require CGO for SQLite support:

```bash
# Install build tools (Ubuntu/Debian)
sudo apt-get install -y build-essential

# Run all tests
CGO_ENABLED=1 go test ./...

# Run specific package tests
CGO_ENABLED=1 go test -v ./internal/handler/...

# Run with race detector
CGO_ENABLED=1 go test -race ./...
```

### Test Coverage (as of latest)

| Package | Coverage |
|---------|----------|
| internal/middleware | 100.0% |
| internal/model | 98.6% |
| internal/util | 90.7% |
| internal/handler | 86.0% |
| internal/config | 82.0% |
| internal/storage | 60.1% |

### Handler Test Examples

The handler tests use mock storage (`storage.Mock`) with these patterns:

```go
// Create test handler with mock storage
h, mockStore := newTestHandler(t)

// Inject test data
paste := model.NewPaste()
paste.Data = "encrypted-content"
mockStore.CreatePaste("abcdef1234567890", paste)

// Make HTTP request
req := httptest.NewRequest(http.MethodGet, "/?abcdef1234567890", nil)
req.Header.Set("Accept", "application/json")
rr := httptest.NewRecorder()

h.handleGet(rr, req)

// Assert response
if rr.Code != http.StatusOK {
    t.Errorf("expected 200, got %d", rr.Code)
}
```

## E2E Testing

Playwright tests for browser-based end-to-end testing in `e2e/` directory:

```bash
# Install dependencies
npm install

# Install Chromium browser
npx playwright install chromium

# Run all e2e tests
npm run test:e2e

# Run with interactive UI mode
npm run test:e2e:ui
```

### E2E Test Coverage (36 tests)

| Test File | Coverage |
|-----------|----------|
| `paste.spec.ts` | Paste creation, viewing, clone, raw, copy URL, delete |
| `theme.spec.ts` | Theme toggle, localStorage persistence |
| `burn.spec.ts` | Burn-after-reading flow, 404 on second access |
| `password.spec.ts` | Password protection, decryption prompts |
| `navigation.spec.ts` | Navigation between /implementation and /docs pages |

## CI/CD

GitHub Actions workflows in `.github/workflows/`:

**docker.yml** - Build and Push Pipeline:
```
test (Go unit tests) → e2e-test (Playwright) → build (Docker multi-arch) → merge (manifest)
```
- Runs on push to main, tags, PRs, and manual dispatch
- Coverage threshold: 60% minimum enforced
- Multi-arch builds: linux/amd64, linux/arm64
- Images pushed to ghcr.io

**release.yml** - Semantic Release:
- Automatic versioning via conventional commits
- Changelog generation
- GitHub release creation
- Triggers Docker build on new release

## Kubernetes Deployment

Kustomize manifests in `deploy/kustomize/`:

```bash
# Deploy to Kubernetes (dev-example overlay with PostgreSQL)
kubectl apply -k deploy/kustomize/overlays/dev-example

# Deploy base only
kubectl apply -k deploy/kustomize/base
```

Structure:
- `base/` - Deployment, Service, ConfigMap
- `overlays/dev-example/` - Example development configuration with PostgreSQL

## Docker

```bash
# Build and run with PostgreSQL
docker-compose up -d

# Development with hot reload
docker-compose -f docker-compose.dev.yml up

# View logs
docker-compose logs -f

# Access the application
open http://localhost:8080
```

## Frontend Features

### Theme Toggle
- Button in top-right corner of the header
- Light mode (default): Clean white/gray palette
- Dark mode: Greyscale palette (#121212, #1e1e1e, #2d2d2d)
- Preference saved to localStorage

### Paste Operations
- Create: Enter text, select expiration, optionally set password
- View: Decrypts client-side using key from URL fragment
- Delete: Available after viewing if you created the paste (uses sessionStorage for token)
- Clone: Copy paste content to create a new paste
- Raw: Open plain text content in a new browser tab
- Copy URL: Copy the full paste URL (with decryption key) to clipboard

## Code Style

- Package-level comments explain purpose
- Function comments explain what, why, and side effects
- Use meaningful variable names
- Table-driven tests for comprehensive coverage
- Custom error types for domain errors (`internal/model/errors.go`)
- Paste IDs: 16 lowercase hexadecimal characters (a-f, 0-9)
- Always use commitlint-styled commit messages (conventional commits)

## Commit Message Format

Use conventional commits format for semantic-release compatibility:

```
<type>(<scope>): <subject>

[optional body]

[optional footer]
```

**Types:**
- `feat`: New feature (minor version bump)
- `fix`: Bug fix (patch version bump)
- `docs`: Documentation only
- `style`: Code style (formatting, semicolons, etc.)
- `refactor`: Code change that neither fixes a bug nor adds a feature
- `perf`: Performance improvement
- `test`: Adding or updating tests
- `build`: Build system or external dependencies
- `ci`: CI configuration files and scripts
- `chore`: Other changes that don't modify src or test files

**Breaking changes:** Add `!` after type or include `BREAKING CHANGE:` in footer for major version bump.

**Examples:**
```bash
feat: add password strength indicator
fix(handler): prevent nil pointer on empty request
docs: update API reference
feat!: redesign encryption format
```

## Security Notes

- All encryption happens client-side; server never sees plaintext
- Decryption key is in URL fragment (never sent to server)
- Delete tokens are HMAC-SHA256 of paste ID with server salt
- Server salt is base64-encoded and stored in database
- Rate limiting by IP hash (configurable)
- Security headers set via middleware (CSP, X-Frame-Options, etc.)
- Always run tests or validate improvements before committing

### Security Headers (Middleware)

The `internal/middleware/security.go` sets these headers on all responses:

| Header | Value | Purpose |
|--------|-------|---------|
| `X-Frame-Options` | `DENY` | Prevent clickjacking |
| `X-Content-Type-Options` | `nosniff` | Prevent MIME sniffing |
| `Referrer-Policy` | `no-referrer` | Don't leak URLs |
| `Content-Security-Policy` | (strict policy) | XSS protection |

### Burn-After-Reading

Burn-after-reading pastes have special behavior:

- **URL Format**: Fragment uses dash prefix: `#-{key}` instead of `#{key}`
- **Warning Modal**: Shows confirmation before revealing content
- **Mutual Exclusion**: Discussions are disabled when burn is enabled
- **Delete on View**: Paste is deleted from server after first decryption
- **E2E Tested**: Second access returns 404

## Error Handling

Custom error types in `internal/model/errors.go`:

| Error | Description |
|-------|-------------|
| `ErrPasteNotFound` | Paste ID doesn't exist |
| `ErrInvalidPasteID` | Malformed paste ID (must be 16 hex chars) |
| `ErrPasteExpired` | Paste has passed expiration time |
| `ErrInvalidRequest` | Malformed JSON or missing fields |

API error response format:
```json
{
  "status": 1,
  "message": "Paste not found"
}
```

Status codes: `0` = success, `1` = error