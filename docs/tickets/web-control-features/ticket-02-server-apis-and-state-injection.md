# Ticket 02: Server API Endpoints & State Injection

- **Status**: Ready
- **Blocked by**: #1
- **Module**: `crates/music-convert/src/server/`

## Description
Inject `Database`, `scan_trigger` and dynamic source path into `AppState`. Implement the REST API endpoints for folder browsing, config management, converted tracks retrieval, re-conversion triggering, and manual scan.

## Acceptance Criteria
1. `AppState` holds `Arc<Database>`, `Arc<RwLock<PathBuf>>` (current source path), and a trigger channel for scan/re-conversion.
2. Endpoint `GET /api/config` returns current configuration info (`src_path`, `output_root`).
3. Endpoint `POST /api/config/src-path` accepts `{ "src_path": "..." }`, validates path existence, updates active source path.
4. Endpoint `GET /api/fs/browse?path=...` lists directory contents safely (returns subdirectories only, rejects traversal attacks).
5. Endpoint `GET /api/converted?limit=50&offset=0&search=...` returns `{ "total": N, "items": [...] }`.
6. Endpoint `POST /api/converted/reconvert` deletes the record from database and signals the orchestrator to re-process.
7. Endpoint `POST /api/scan` triggers immediate scan and batch processing.
8. Axum routing and integration tests verify all endpoints return expected status codes and payloads.
