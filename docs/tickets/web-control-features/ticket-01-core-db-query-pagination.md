# Ticket 01: Core SQLite Query & Pagination Foundation

- **Status**: Ready
- **Blocked by**: None
- **Module**: `crates/music-core/src/db.rs`

## Description
Extend `Database` in `music-core` to provide efficient paginated retrieval and searching of converted tracks from `music_info`, as well as individual item deletion for re-conversion.

## Acceptance Criteria
1. `list_records(limit: usize, offset: usize, search: Option<&str>) -> Result<Vec<MusicInfo>, DbError>`:
   - Queries `music_info` ordered by `created DESC, id DESC`.
   - If `search` is provided, filters with `WHERE music LIKE ?`.
   - Respects `limit` and `offset`.
2. `count_records(search: Option<&str>) -> Result<usize, DbError>`:
   - Returns the total count of records matching the optional search pattern.
3. `delete_by_music(music: &str) -> Result<usize, DbError>`:
   - Deletes exact match from `music_info`.
4. Unit tests in `crates/music-core/src/db.rs` verify all pagination, filtering, and deletion behaviors.
