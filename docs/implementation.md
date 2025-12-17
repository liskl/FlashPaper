# FlashPaper Implementation Details

## Abstract

FlashPaper is a zero-knowledge encrypted pastebin implementation that ensures the server maintains no access to plaintext content at any point during the data lifecycle. This document provides a comprehensive technical analysis of the cryptographic protocols, system architecture, and security guarantees implemented within the system. The implementation achieves full API compatibility with PrivateBin while being written entirely in Go for improved performance and simplified deployment.

## Table of Contents

1. [Cryptographic Implementation](#1-cryptographic-implementation)
2. [Key Management and Derivation](#2-key-management-and-derivation)
3. [Data Flow Architecture](#3-data-flow-architecture)
4. [Storage Layer](#4-storage-layer)
5. [API Protocol](#5-api-protocol)
6. [Security Analysis](#6-security-analysis)
7. [Threat Model](#7-threat-model)

---

## 1. Cryptographic Implementation

### 1.1 Symmetric Encryption

FlashPaper employs AES-256-GCM (Advanced Encryption Standard with 256-bit keys in Galois/Counter Mode) for all content encryption. GCM mode provides both confidentiality and authenticity through authenticated encryption with associated data (AEAD).

| Parameter | Value | Specification |
|-----------|-------|---------------|
| Algorithm | AES-256-GCM | NIST SP 800-38D |
| Key Size | 256 bits | Provides 128-bit security level |
| IV/Nonce Size | 128 bits (16 bytes) | Randomly generated per encryption |
| Authentication Tag | 128 bits | Appended to ciphertext |

### 1.2 Key Derivation Function

Encryption keys are derived using PBKDF2 (Password-Based Key Derivation Function 2) with SHA-256 as the underlying hash function. This approach enables optional password protection while maintaining security through computational hardening.

```
DerivedKey = PBKDF2(
    PRF: HMAC-SHA256,
    Password: MasterKey || UserPassword,
    Salt: RandomSalt[8 bytes],
    Iterations: 100,000,
    KeyLength: 256 bits
)
```

> **Implementation Note:** The iteration count of 100,000 is calibrated to provide approximately 100ms of computation time on modern hardware, balancing security against user experience. This value aligns with OWASP recommendations for PBKDF2-HMAC-SHA256.

### 1.3 Authenticated Data Structure

The Additional Authenticated Data (AAD) in GCM mode binds the ciphertext to its metadata, preventing tampering with encryption parameters:

```
AData = [
    [IV_base64, Salt_base64, Iterations, KeySize, TagSize, "aes", "gcm", Compression],
    Formatter,      // "plaintext" | "syntaxhighlighting" | "markdown"
    OpenDiscussion, // 0 | 1
    BurnAfterReading // 0 | 1
]
```

---

## 2. Key Management and Derivation

### 2.1 Master Key Generation

For each paste, the client generates a cryptographically secure random 256-bit master key using the Web Crypto API's `crypto.getRandomValues()`. This function provides access to the operating system's cryptographic random number generator (CSPRNG).

```javascript
// Client-side key generation
const masterKey = new Uint8Array(32);  // 256 bits
crypto.getRandomValues(masterKey);
```

### 2.2 Key Distribution via URL Fragment

The master key is encoded using Base58 (Bitcoin alphabet) and placed in the URL fragment identifier. Per RFC 3986 §3.5, fragment identifiers are processed exclusively by the user agent and are *never* transmitted to the server.

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              URL Structure                                  │
├─────────────────────────────────────────────────────────────────────────────┤
│  https://example.com/?f468483c313401e8#7JqYpr5K2zXmNvQWRtBgHsC              │
│  └────────┬────────┘ └───────┬────────┘└─────────────┬──────────┘           │
│        Origin            Paste ID              Fragment (Key)               │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐   │
│  │ Transmitted to Server: https://example.com/?f468483c313401e8         │   │
│  │ Client-Only:           #7JqYpr5K2zXmNvQWRtBgHsC (never sent)         │   │
│  └──────────────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 2.3 Base58 Encoding

Base58 encoding (Bitcoin variant) is employed for key representation, omitting visually ambiguous characters (0, O, I, l) to reduce transcription errors:

```
Alphabet: 123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz
```

---

## 3. Data Flow Architecture

### 3.1 Paste Creation Sequence

```
┌──────────┐                                              ┌──────────┐
│  Client  │                                              │  Server  │
└────┬─────┘                                              └────┬─────┘
     │                                                         │
     │  1. Generate MasterKey (256-bit random)                 │
     │  2. Generate IV (128-bit random)                        │
     │  3. Generate Salt (64-bit random)                       │
     │  4. DerivedKey = PBKDF2(MasterKey||Password, Salt)      │
     │  5. Ciphertext = AES-GCM-Encrypt(Plaintext, DerivedKey) │
     │                                                         │
     │  POST {ct: Ciphertext, adata: [IV, Salt, ...]}          │
     │ ─────────────────────────────────────────────────────►  │
     │                                                         │
     │                          {id: "f468...", deletetoken}   │
     │ ◄─────────────────────────────────────────────────────  │
     │                                                         │
     │  6. Construct URL: /?{id}#{Base58(MasterKey)}           │
     │                                                         │
     ▼                                                         ▼
```

### 3.2 Paste Retrieval Sequence

```
┌──────────┐                                              ┌──────────┐
│  Client  │                                              │  Server  │
└────┬─────┘                                              └────┬─────┘
     │                                                         │
     │  1. Parse URL: extract PasteID and MasterKey            │
     │                                                         │
     │  GET /?{PasteID}                                        │
     │ ─────────────────────────────────────────────────────►  │
     │                                                         │
     │              {ct: Ciphertext, adata: [IV, Salt, ...]}   │
     │ ◄─────────────────────────────────────────────────────  │
     │                                                         │
     │  2. Extract IV, Salt from adata                         │
     │  3. DerivedKey = PBKDF2(MasterKey||Password, Salt)      │
     │  4. Plaintext = AES-GCM-Decrypt(Ciphertext, DerivedKey) │
     │  5. Verify authentication tag (implicit in GCM)         │
     │  6. Display Plaintext                                   │
     │                                                         │
     ▼                                                         ▼
```

---

## 4. Storage Layer

### 4.1 Supported Backends

FlashPaper implements a storage abstraction layer supporting multiple backends:

| Backend | Use Case | Characteristics |
|---------|----------|-----------------|
| SQLite | Single-instance deployment | Embedded, zero-configuration, file-based |
| PostgreSQL | Production deployment | ACID compliant, concurrent access, replication support |
| MySQL | Existing infrastructure | Wide compatibility, mature tooling |
| Filesystem | Simple deployments | No database required, directory-based sharding |

### 4.2 Data Schema

The storage layer persists only encrypted ciphertext and associated metadata. At no point does the server process, log, or store plaintext content.

```
Paste {
    id:              string    // 16 hex characters
    data:            blob      // Base64-encoded ciphertext
    adata:           json      // Authenticated data (IV, salt, params)
    attachment:      blob      // Optional encrypted attachment
    attachment_name: string    // Optional encrypted filename
    expire_date:     timestamp // Unix timestamp (0 = never)
    burn_after_read: boolean   // Delete on first retrieval
    open_discussion: boolean   // Allow encrypted comments
    created_at:      timestamp // Creation time
}
```

### 4.3 Delete Token Generation

Delete tokens are generated server-side using HMAC-SHA256 with a persistent server salt, enabling paste deletion by the original creator without authentication:

```
DeleteToken = HMAC-SHA256(ServerSalt, PasteID)
// Returned only during paste creation
// Must be presented for deletion authorization
```

---

## 5. API Protocol

### 5.1 Endpoints

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/` | Serve web interface |
| `GET` | `/?{pasteID}` | Retrieve paste (JSON if X-Requested-With header) |
| `POST` | `/` | Create paste or comment |
| `DELETE` | `/` | Delete paste (requires deletetoken) |
| `GET` | `/health` | Health check endpoint |

### 5.2 Request/Response Format

**Create Paste Request:**

```json
{
    "v": 2,
    "ct": "base64_encoded_ciphertext",
    "adata": [
        ["IV_b64", "salt_b64", 100000, 256, 128, "aes", "gcm", "none"],
        "plaintext",
        0,
        0
    ],
    "meta": {
        "expire": "1week"
    }
}
```

**Create Paste Response:**

```json
{
    "status": 0,
    "id": "f468483c313401e8",
    "url": "/?f468483c313401e8",
    "deletetoken": "a1b2c3d4..."
}
```

---

## 6. Security Analysis

### 6.1 Zero-Knowledge Property

The system achieves zero-knowledge through architectural separation: the decryption key exists only in the URL fragment, which is processed exclusively client-side per HTTP specifications. The server receives and stores only ciphertext, making it computationally infeasible to recover plaintext without the key.

> **Security Consideration:** The zero-knowledge property depends on correct client implementation. A malicious or compromised client could transmit the key to the server. Users should verify they are using official client code and secure HTTPS connections.

### 6.2 Forward Secrecy

Each paste uses a unique, randomly generated master key. Compromise of one paste's key does not affect the confidentiality of other pastes. The PBKDF2 salt ensures identical plaintexts encrypted with the same password produce different ciphertexts.

### 6.3 Authentication Guarantees

AES-GCM provides authenticated encryption, ensuring:

- **Integrity:** Any modification to ciphertext or AAD causes decryption failure
- **Authenticity:** Only parties with the correct key can produce valid ciphertext
- **Confidentiality:** Ciphertext reveals no information about plaintext

### 6.4 Burn-After-Reading Implementation

When enabled, the paste is atomically deleted from storage upon first successful retrieval. The deletion occurs before the response is sent, ensuring the data cannot be retrieved again even if the response transmission fails.

---

## 7. Threat Model

### 7.1 Protected Against

- **Server compromise:** Attacker gaining database access cannot decrypt stored pastes
- **Network eavesdropping:** HTTPS protects transmission; key never leaves client
- **Server-side logging:** Only encrypted data and metadata are processed
- **Database breach:** Stored ciphertext is computationally indistinguishable from random

### 7.2 Not Protected Against

- **Client compromise:** Malware on user's device can capture plaintext
- **Key exposure:** Sharing the full URL exposes the decryption key
- **Browser vulnerabilities:** XSS or other client-side attacks could leak data
- **Traffic analysis:** Timing and size of requests may leak metadata
- **Malicious server code:** Server could serve modified JavaScript to capture keys

### 7.3 Mitigations

- Content Security Policy (CSP) headers prevent inline script injection
- Subresource Integrity (SRI) can verify JavaScript integrity
- HTTPS enforcement prevents network-level tampering
- Rate limiting mitigates brute-force attempts
- Automatic expiration limits exposure window

---

## References

- NIST SP 800-38D: Recommendation for Block Cipher Modes of Operation: Galois/Counter Mode (GCM)
- RFC 2898: PKCS #5: Password-Based Cryptography Specification Version 2.0
- RFC 3986: Uniform Resource Identifier (URI): Generic Syntax
- OWASP Password Storage Cheat Sheet
- Web Crypto API Specification (W3C)
