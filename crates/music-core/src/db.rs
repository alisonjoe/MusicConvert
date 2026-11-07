use chrono::Utc;
use rusqlite::{params, Connection, OptionalExtension};
use std::path::Path;
use std::sync::Mutex;
use thiserror::Error;

#[derive(Error, Debug)]
pub enum DbError {
    #[error("SQLite error: {0}")]
    Sqlite(#[from] rusqlite::Error),
    #[error("Empty prefix for delete")]
    EmptyPrefix,
}

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub struct MusicInfo {
    pub id: i64,
    pub music: String,
    pub state: i32,
    pub created: i64,
}

pub struct Database {
    conn: Mutex<Connection>,
}

impl Database {
    pub fn open<P: AsRef<Path>>(path: P) -> Result<Self, DbError> {
        let conn = Connection::open(path)?;
        let db = Self {
            conn: Mutex::new(conn),
        };
        db.init_table()?;
        Ok(db)
    }

    pub fn open_in_memory() -> Result<Self, DbError> {
        let conn = Connection::open_in_memory()?;
        let db = Self {
            conn: Mutex::new(conn),
        };
        db.init_table()?;
        Ok(db)
    }

    pub fn init_table(&self) -> Result<(), DbError> {
        let conn = self.conn.lock().unwrap();
        conn.execute(
            r#"
            CREATE TABLE IF NOT EXISTS music_info (
                id INTEGER PRIMARY KEY AUTOINCREMENT,
                music TEXT UNIQUE NOT NULL,
                state INT NULL,
                created BIGINT NULL
            );
            "#,
            [],
        )?;
        Ok(())
    }

    pub fn insert(&self, music: &str, state: i32) -> Result<i64, DbError> {
        let conn = self.conn.lock().unwrap();
        let now = Utc::now().timestamp();
        conn.execute(
            "INSERT INTO music_info(music, state, created) VALUES(?1, ?2, ?3)",
            params![music, state, now],
        )?;
        Ok(conn.last_insert_rowid())
    }

    pub fn is_processed(&self, music: &str) -> Result<bool, DbError> {
        let conn = self.conn.lock().unwrap();
        let mut stmt = conn.prepare("SELECT 1 FROM music_info WHERE music = ?1 LIMIT 1")?;
        let exists = stmt.query_row(params![music], |_| Ok(true)).optional()?;
        Ok(exists.unwrap_or(false))
    }

    pub fn query(&self, music: &str) -> Result<Vec<MusicInfo>, DbError> {
        let conn = self.conn.lock().unwrap();
        let mut stmt =
            conn.prepare("SELECT id, music, state, created FROM music_info WHERE music = ?1")?;
        let rows = stmt.query_map(params![music], |row| {
            Ok(MusicInfo {
                id: row.get(0)?,
                music: row.get(1)?,
                state: row.get(2).unwrap_or(0),
                created: row.get(3).unwrap_or(0),
            })
        })?;

        let mut list = Vec::new();
        for r in rows {
            list.push(r?);
        }
        Ok(list)
    }

    pub fn delete_by_prefix(&self, prefix: &str) -> Result<usize, DbError> {
        let clean = prefix.trim().trim_end_matches('/');
        if clean.is_empty() {
            return Err(DbError::EmptyPrefix);
        }
        let pattern = format!("{}/%", clean);
        let conn = self.conn.lock().unwrap();
        let affected = conn.execute(
            "DELETE FROM music_info WHERE music = ?1 OR music LIKE ?2",
            params![clean, pattern],
        )?;
        Ok(affected)
    }

    pub fn delete_all(&self) -> Result<usize, DbError> {
        let conn = self.conn.lock().unwrap();
        let affected = conn.execute("DELETE FROM music_info", [])?;
        Ok(affected)
    }

    pub fn delete_by_music(&self, music: &str) -> Result<usize, DbError> {
        let conn = self.conn.lock().unwrap();
        let affected = conn.execute("DELETE FROM music_info WHERE music = ?1", params![music])?;
        Ok(affected)
    }

    pub fn count_records(&self, search: Option<&str>) -> Result<usize, DbError> {
        let conn = self.conn.lock().unwrap();
        let count: i64 = if let Some(s) = search {
            let pattern = format!("%{}%", s);
            conn.query_row(
                "SELECT COUNT(*) FROM music_info WHERE music LIKE ?1",
                params![pattern],
                |row| row.get(0),
            )?
        } else {
            conn.query_row("SELECT COUNT(*) FROM music_info", [], |row| row.get(0))?
        };
        Ok(count as usize)
    }

    pub fn list_records(
        &self,
        limit: usize,
        offset: usize,
        search: Option<&str>,
    ) -> Result<Vec<MusicInfo>, DbError> {
        let conn = self.conn.lock().unwrap();
        let mut list = Vec::new();
        if let Some(s) = search {
            let pattern = format!("%{}%", s);
            let mut stmt = conn.prepare(
                "SELECT id, music, state, created FROM music_info WHERE music LIKE ?1 ORDER BY created DESC, id DESC LIMIT ?2 OFFSET ?3",
            )?;
            let rows = stmt.query_map(params![pattern, limit as i64, offset as i64], |row| {
                Ok(MusicInfo {
                    id: row.get(0)?,
                    music: row.get(1)?,
                    state: row.get(2).unwrap_or(0),
                    created: row.get(3).unwrap_or(0),
                })
            })?;
            for r in rows {
                list.push(r?);
            }
        } else {
            let mut stmt = conn.prepare(
                "SELECT id, music, state, created FROM music_info ORDER BY created DESC, id DESC LIMIT ?1 OFFSET ?2",
            )?;
            let rows = stmt.query_map(params![limit as i64, offset as i64], |row| {
                Ok(MusicInfo {
                    id: row.get(0)?,
                    music: row.get(1)?,
                    state: row.get(2).unwrap_or(0),
                    created: row.get(3).unwrap_or(0),
                })
            })?;
            for r in rows {
                list.push(r?);
            }
        }
        Ok(list)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_db_insert_query_dedup() {
        let db = Database::open_in_memory().unwrap();

        assert!(!db.is_processed("/music/test.flac").unwrap());

        db.insert("/music/test.flac", 1).unwrap();
        assert!(db.is_processed("/music/test.flac").unwrap());

        let results = db.query("/music/test.flac").unwrap();
        assert_eq!(results.len(), 1);
        assert_eq!(results[0].music, "/music/test.flac");
        assert_eq!(results[0].state, 1);

        // Duplicate insert fails with UNIQUE constraint
        assert!(db.insert("/music/test.flac", 1).is_err());

        // Delete by prefix
        db.insert("/music/subdir/track.flac", 1).unwrap();
        let deleted = db.delete_by_prefix("/music").unwrap();
        assert_eq!(deleted, 2);
        assert!(!db.is_processed("/music/test.flac").unwrap());
    }

    #[test]
    fn test_db_list_count_and_delete_by_music() {
        let db = Database::open_in_memory().unwrap();

        db.insert("/music/artist1/song1.flac", 1).unwrap();
        db.insert("/music/artist1/song2.flac", 1).unwrap();
        db.insert("/music/artist2/other.flac", 1).unwrap();

        assert_eq!(db.count_records(None).unwrap(), 3);
        assert_eq!(db.count_records(Some("song")).unwrap(), 2);
        assert_eq!(db.count_records(Some("other")).unwrap(), 1);
        assert_eq!(db.count_records(Some("nonexistent")).unwrap(), 0);

        let page1 = db.list_records(2, 0, None).unwrap();
        assert_eq!(page1.len(), 2);

        let page2 = db.list_records(2, 2, None).unwrap();
        assert_eq!(page2.len(), 1);

        let filtered = db.list_records(10, 0, Some("artist2")).unwrap();
        assert_eq!(filtered.len(), 1);
        assert_eq!(filtered[0].music, "/music/artist2/other.flac");

        let deleted = db.delete_by_music("/music/artist2/other.flac").unwrap();
        assert_eq!(deleted, 1);
        assert_eq!(db.count_records(None).unwrap(), 2);
        assert!(!db.is_processed("/music/artist2/other.flac").unwrap());
    }
}
