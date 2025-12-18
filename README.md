# FlashPaper

A zero-knowledge encrypted pastebin written in Go. A complete reimplementation of [PrivateBin](https://github.com/PrivateBin/PrivateBin) with full API compatibility.

## Screenshots

| Light Mode | Dark Mode |
|------------|-----------|
| ![Light Mode](docs/screenshot.png) | ![Dark Mode](docs/screenshot-dark.png) |

## Features

- **Zero-Knowledge Encryption**: All encryption happens in your browser. The server never sees your content.
- **AES-256-GCM**: Military-grade encryption with authenticated encryption.
- **Burn After Reading**: Automatically delete pastes after viewing.
- **Expiration Options**: From 5 minutes to never.
- **Password Protection**: Add an extra layer of security.
- **Discussions**: Threaded comments on pastes (optional).
- **Multiple Storage Backends**: SQLite, PostgreSQL, MySQL, or filesystem.
- **Single Binary**: Self-contained with embedded frontend assets.
- **Docker Ready**: Production and development Docker configurations included.
- **Dark/Light Mode**: Theme toggle with localStorage persistence.
- **Paste Actions**: Clone, Raw view, Copy URL, and Delete buttons.
- **CI/CD**: GitHub Actions for testing, building, and releases.
- **Kubernetes Ready**: Kustomize manifests for easy deployment.

## Quick Start

### Docker (Recommended)

```bash
# Start with PostgreSQL backend
docker-compose up -d

# View logs
docker-compose logs -f

# Access at http://localhost:8080
```

### Build from Source

```bash
# Requires Go 1.21+ and CGO (for SQLite)
CGO_ENABLED=1 go build -o flashpaper ./cmd/flashpaper

# Run with defaults (SQLite)
./flashpaper

# Run with config file
./flashpaper -config config.ini
```

## Configuration

FlashPaper can be configured via INI file or environment variables.

### INI Configuration

Copy `config.sample.ini` to `config.ini` and modify as needed:

```ini
[main]
name = "FlashPaper"
basepath = "/"
discussion = true
sizelimit = 10485760

[model]
class = "Database"
dsn = "/data/flashpaper.db"
```

### Environment Variables

All settings can be overridden with environment variables using the format:
`FLASHPAPER_SECTION_KEY`

Examples:
```bash
export FLASHPAPER_MAIN_NAME="My Pastebin"
export FLASHPAPER_MODEL_DSN="postgres://user:pass@localhost/flashpaper"
```

### Storage Backends

**SQLite** (default):
```ini
[model]
class = "Database"
dsn = "/data/flashpaper.db"
```

**PostgreSQL**:
```ini
[model]
class = "Database"
dsn = "postgres://user:password@localhost:5432/flashpaper?sslmode=disable"
```

**MySQL**:
```ini
[model]
class = "Database"
dsn = "user:password@tcp(localhost:3306)/flashpaper?charset=utf8mb4"
```

**Filesystem**:
```ini
[model]
class = "Filesystem"
dsn = "/data/pastes"
```

## API

FlashPaper implements the PrivateBin API for full client compatibility.

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/` | Web interface |
| GET | `/?{pasteID}` | View paste |
| POST | `/` | Create paste |
| DELETE | `/` | Delete paste |
| GET | `/health` | Health check |

## Security

- **Client-Side Encryption**: Content is encrypted in your browser before being sent to the server.
- **Key in URL Fragment**: The decryption key is in the URL fragment (`#...`), which is never sent to the server.
- **PBKDF2 Key Derivation**: 100,000 iterations for password-protected pastes.
- **No Logging**: The server cannot log content it never receives.
- **Secure Headers**: CSP, X-Frame-Options, and other security headers are set.

## Development

```bash
# Install tools
make install-tools

# Run with hot reload
make dev

# Build Docker image
make docker
```

## Testing

### Unit Tests

```bash
# Run all Go tests
make test

# With coverage report
CGO_ENABLED=1 go test -cover ./...
```

### E2E Tests (Playwright)

```bash
# Install Playwright
npm install
npx playwright install chromium

# Run tests
npm run test:e2e

# Interactive UI mode
npm run test:e2e:ui
```

36 end-to-end tests covering paste operations, theme toggle, burn-after-reading, password protection, and navigation.

## Kubernetes

Deploy using Kustomize:

```bash
# Example development environment (with PostgreSQL)
kubectl apply -k deploy/kustomize/overlays/dev-example

# Base deployment only
kubectl apply -k deploy/kustomize/base
```

## Documentation

- **[How It Works](/implementation)** - Technical implementation details
- **[Documentation](/docs)** - User guide and API reference

## License

MIT License - see LICENSE file for details.

## Credits

FlashPaper is a Go reimplementation of [PrivateBin](https://privatebin.info/), which was inspired by ZeroBin.
