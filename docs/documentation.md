# FlashPaper Documentation

Welcome to the FlashPaper documentation. This guide covers installation, configuration, API reference, and troubleshooting.

## Table of Contents

1. [Installation](#1-installation)
2. [Configuration](#2-configuration)
3. [API Reference](#3-api-reference)
4. [Client Integration](#4-client-integration)
5. [Troubleshooting](#5-troubleshooting)

---

## 1. Installation

### 1.1 Docker (Recommended)

The simplest way to deploy FlashPaper is using Docker Compose:

```bash
# Clone the repository
git clone https://github.com/liskl/flashpaper.git
cd flashpaper

# Start with PostgreSQL backend
docker-compose up -d

# View logs
docker-compose logs -f

# Access the application
open http://localhost:8080
```

### 1.2 Docker with SQLite

For simpler deployments without an external database:

```bash
docker run -d \
  --name flashpaper \
  -p 8080:8080 \
  -v flashpaper-data:/data \
  -e FLASHPAPER_MODEL_CLASS=Database \
  -e FLASHPAPER_MODEL_DRIVER=sqlite3 \
  -e FLASHPAPER_MODEL_DSN=/data/flashpaper.db \
  flashpaper:latest
```

### 1.3 Build from Source

Requires Go 1.21+ and CGO for SQLite support:

```bash
# Clone and build
git clone https://github.com/liskl/flashpaper.git
cd flashpaper
CGO_ENABLED=1 go build -o flashpaper ./cmd/flashpaper

# Run with default configuration
./flashpaper

# Run with custom config file
./flashpaper -config /path/to/config.ini
```

### 1.4 System Requirements

- **Memory:** 64MB minimum, 128MB recommended
- **Storage:** Depends on usage; ~1KB per paste average
- **CPU:** Any modern processor; single core sufficient for most deployments
- **Network:** HTTPS termination recommended (via reverse proxy)

---

## 2. Configuration

FlashPaper can be configured via INI file or environment variables. Environment variables override file settings and use the format: `FLASHPAPER_SECTION_KEY`

### 2.1 Core Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `FLASHPAPER_MAIN_NAME` | Application name displayed in the UI | "FlashPaper" |
| `FLASHPAPER_MAIN_HOST` | Bind address | "0.0.0.0" |
| `FLASHPAPER_MAIN_PORT` | HTTP port | 8080 |
| `FLASHPAPER_MAIN_BASEPATH` | URL base path for reverse proxy setups | "/" |
| `FLASHPAPER_MAIN_DISCUSSION` | Enable discussion/comments feature | true |
| `FLASHPAPER_MAIN_SIZELIMIT` | Maximum paste size in bytes | 10485760 (10MB) |

### 2.2 Storage Backend

| Variable | Description | Default |
|----------|-------------|---------|
| `FLASHPAPER_MODEL_CLASS` | Storage type: "Database" or "Filesystem" | "Database" |
| `FLASHPAPER_MODEL_DRIVER` | Database driver: "sqlite3", "postgres", or "mysql" | "sqlite3" |
| `FLASHPAPER_MODEL_DSN` | Database connection string | - |
| `FLASHPAPER_MODEL_DIR` | Directory for filesystem storage | - |

#### DSN Examples

```bash
# SQLite
FLASHPAPER_MODEL_DSN=/data/flashpaper.db

# PostgreSQL
FLASHPAPER_MODEL_DSN=postgres://user:password@localhost:5432/flashpaper?sslmode=disable

# MySQL
FLASHPAPER_MODEL_DSN=user:password@tcp(localhost:3306)/flashpaper?charset=utf8mb4
```

### 2.3 Expiration Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `FLASHPAPER_EXPIRE_DEFAULT` | Default expiration option | "1week" |

Available options: `5min`, `10min`, `1hour`, `1day`, `1week`, `1month`, `1year`, `never`

### 2.4 Rate Limiting

| Variable | Description | Default |
|----------|-------------|---------|
| `FLASHPAPER_TRAFFIC_LIMIT` | Minimum seconds between paste creations per IP (0 to disable) | 10 |

### 2.5 INI File Example

```ini
[main]
name = "FlashPaper"
host = "0.0.0.0"
port = 8080
basepath = "/"
discussion = true
sizelimit = 10485760

[model]
class = "Database"
driver = "postgres"
dsn = "postgres://flashpaper:password@db:5432/flashpaper?sslmode=disable"

[expire]
default = "1week"

[traffic]
limit = 10

[purge]
limit = 300
batchsize = 10
```

---

## 3. API Reference

FlashPaper implements a PrivateBin-compatible REST API. All responses use JSON format with a `status` field (0 = success, 1 = error).

### 3.1 Create Paste

**POST /**

Create a new encrypted paste.

#### Request Headers

| Header | Value | Required |
|--------|-------|----------|
| `Content-Type` | `application/json` | Yes |
| `X-Requested-With` | `JSONHttpRequest` | Recommended |

#### Request Body

| Field | Type | Description |
|-------|------|-------------|
| `v` | integer | API version (always 2) |
| `ct` | string | Base64-encoded ciphertext |
| `adata` | array | Authenticated data array (encryption params) |
| `meta.expire` | string | Expiration option (e.g., "1week") |

#### Example Request

```json
{
  "v": 2,
  "ct": "base64_encoded_ciphertext",
  "adata": [
    ["IV_base64", "salt_base64", 100000, 256, 128, "aes", "gcm", "none"],
    "plaintext",
    0,
    0
  ],
  "meta": {
    "expire": "1week"
  }
}
```

#### Example Response

```json
{
  "status": 0,
  "id": "f468483c313401e8",
  "url": "/?f468483c313401e8",
  "deletetoken": "a1b2c3d4e5f6..."
}
```

### 3.2 Retrieve Paste

**GET /?{pasteId}**

Retrieve an encrypted paste by ID.

#### Request Headers

| Header | Value | Required |
|--------|-------|----------|
| `X-Requested-With` | `JSONHttpRequest` | Yes (for JSON response) |

#### Example Response

```json
{
  "status": 0,
  "id": "f468483c313401e8",
  "url": "/?f468483c313401e8",
  "v": 2,
  "ct": "base64_encoded_ciphertext",
  "adata": [...],
  "meta": {
    "postdate": 1703001234,
    "opendiscussion": false
  },
  "comments": []
}
```

### 3.3 Delete Paste

**DELETE /**

Delete a paste using its delete token.

#### Request Body

| Field | Type | Description |
|-------|------|-------------|
| `pasteid` | string | 16-character paste ID |
| `deletetoken` | string | Delete token from paste creation |

#### Example Request

```json
{
  "pasteid": "f468483c313401e8",
  "deletetoken": "a1b2c3d4e5f6..."
}
```

#### Example Response

```json
{
  "status": 0,
  "id": "f468483c313401e8"
}
```

### 3.4 Health Check

**GET /health**

Returns service health status.

#### Example Response

```json
{"status": "ok"}
```

### 3.5 Error Responses

All error responses follow this format:

```json
{
  "status": 1,
  "message": "Error description"
}
```

| HTTP Status | Message | Description |
|-------------|---------|-------------|
| 400 | Invalid JSON | Malformed request body |
| 404 | Paste not found | Paste ID does not exist or has expired |
| 403 | Invalid delete token | Delete token does not match |
| 429 | Rate limit exceeded | Too many requests from this IP |

---

## 4. Client Integration

### 4.1 Encryption Requirements

Clients must implement client-side encryption before sending data to the API. See the [Implementation Details](implementation.md) for cryptographic specifications.

> **Key Point:** The server never receives plaintext data. All encryption and decryption must occur client-side using the key stored in the URL fragment.

### 4.2 URL Structure

```
https://example.com/?{pasteId}#{key}
        │           │         │
        │           │         └── Base58-encoded encryption key (never sent to server)
        │           └── 16-character hex paste ID
        └── Server origin
```

### 4.3 AData Structure

The `adata` array contains encryption parameters and paste settings:

```json
[
  [
    "IV_base64",      // Initialization vector
    "salt_base64",    // PBKDF2 salt
    100000,           // Iteration count
    256,              // Key size in bits
    128,              // Authentication tag size
    "aes",            // Algorithm
    "gcm",            // Mode
    "none"            // Compression ("none" or "zlib")
  ],
  "plaintext",        // Formatter: "plaintext", "syntaxhighlighting", "markdown"
  0,                  // Open discussion: 0 or 1
  0                   // Burn after reading: 0 or 1
]
```

---

## 5. Troubleshooting

### 5.1 Common Issues

#### Database Connection Errors

- Verify the DSN format matches your database driver
- Ensure `FLASHPAPER_MODEL_DRIVER` is set correctly
- Check network connectivity to the database server
- For PostgreSQL, ensure the database exists and user has permissions

#### Decryption Failures

- Ensure the complete URL including fragment (#key) is shared
- Verify the paste hasn't expired or been deleted
- Check if password protection was enabled (requires correct password)

#### Container Won't Start

- Check logs: `docker-compose logs flashpaper`
- Verify environment variables are correctly formatted
- Ensure the data volume has correct permissions

### 5.2 Debug Mode

Enable verbose logging by checking container logs:

```bash
docker-compose logs -f flashpaper
```

### 5.3 Getting Help

- **GitHub Issues:** [Report bugs or request features](https://github.com/liskl/flashpaper/issues)
- **Source Code:** [View the source](https://github.com/liskl/flashpaper)
