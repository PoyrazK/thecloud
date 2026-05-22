# ADR 020: Storage Streaming and Authenticated Encryption

## Status
Accepted

## Context
The initial implementation of the storage service and its distributed coordination layer relied on buffering entire file payloads into memory for encryption, replication, and persistence. This approach posed a critical risk of Out-Of-Memory (OOM) errors when handling large files (e.g., ISO images, database backups) and limited the platform's scalability. Additionally, the initial encryption used AES-CTR without authentication tags, which provided confidentiality but not integrity or authenticity for the data streams.

## Decision
We decided to refactor the entire storage data path to support end-to-end streaming with authenticated encryption.

### 1. Authenticated Streaming Encryption (AES-GCM Chunked)
-   Switch from simple AES-CTR to **AES-GCM with Chunked Framing**.
-   Data is divided into fixed-size chunks (default 64KB).
-   Each chunk is sealed using AES-GCM, producing a ciphertext plus a 16-byte authentication tag.
-   A unique nonce is derived for each chunk using a base nonce (stored at the start of the stream) combined with a 64-bit counter.
-   Framing format: `[Base Nonce (12B)] [[Chunk Length (4B)][Sealed Chunk (Data + Tag)]...]`.
-   This ensures that any tampering with the ciphertext is detected during the streaming decryption process.

### 2. Streaming gRPC Storage Protocol
-   Updated the `StorageNode` gRPC interface to use bidirectional and server-side streaming.
-   `Store(stream StoreRequest)`: Allows the Coordinator to pipe data chunks to Storage Nodes without buffering.
-   `Retrieve(RetrieveRequest) returns (stream RetrieveResponse)`: Allows Storage Nodes to stream data back to the Coordinator.

### 3. Constant-Memory Distributed Replication
-   The `Coordinator` now uses `io.TeeReader` and `io.Pipe` to broadcast data to multiple replicas simultaneously.
-   Memory usage is now proportional to the chunk size rather than the file size, enabling the platform to handle files of arbitrary size.

### 4. Robust Read Repair
-   Read operations now only fetch metadata initially to determine the "winning" replica (latest timestamp).
-   The winning stream is then piped to the caller and simultaneously used to "repair" stale or missing replicas in the background via streaming writes.

## Consequences
-   **Stability**: OOM risks for large file uploads/downloads are eliminated.
-   **Security**: Authenticated encryption ensures data integrity and protects against tampering.
-   **Performance**: Improved throughput due to overlapping I/O and encryption operations.
-   **Complexity**: Implementation of chunked AEAD framing and streaming coordination is more complex than simple buffering.
-   **Compatibility**: Existing stored objects (encrypted with the old AES-CTR scheme) will need to be migrated or handled via a legacy fallback path (currently not implemented, assuming fresh deployment or manual migration).

### Legacy AES-CTR Fallback Status (2026-05-19)

**Investigation findings:**
- The original AES-CTR implementation (`fc0f7b2d`) was replaced by AES-GCM in a later commit
- The legacy AES-CTR encryption/decryption code was not preserved when the migration occurred
- Without the original implementation details, a compatibility fallback cannot be reliably implemented
- Attempting to reverse-engineer the legacy format would be error-prone and could corrupt data

**Current recommendation:**
- For existing deployments with AES-CTR encrypted data: manual migration required
- For new deployments: only AES-GCM format is supported
- Any future storage encryption changes must preserve the previous implementation for backward compatibility
