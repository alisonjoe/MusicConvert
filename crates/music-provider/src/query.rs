use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct MetadataQuery {
    pub artist: String,
    pub album: String,
    pub year: Option<String>,
    pub track_titles: Vec<String>,
    pub track_count: Option<usize>,
    pub has_chinese_context: bool,
    pub catalog_id: Option<String>,
    pub is_classical: bool,
}

impl MetadataQuery {
    pub fn new(artist: impl Into<String>, album: impl Into<String>) -> Self {
        let artist_s = artist.into();
        let album_s = album.into();
        let has_chinese = artist_s.chars().any(is_cjk) || album_s.chars().any(is_cjk);
        Self {
            artist: artist_s,
            album: album_s,
            has_chinese_context: has_chinese,
            ..Default::default()
        }
    }

    pub fn with_track_titles(mut self, titles: Vec<String>) -> Self {
        self.track_count = Some(titles.len());
        self.track_titles = titles;
        self
    }

    pub fn with_catalog(mut self, catalog: impl Into<String>) -> Self {
        self.catalog_id = Some(catalog.into());
        self
    }

    pub fn with_classical(mut self, is_classical: bool) -> Self {
        self.is_classical = is_classical;
        self
    }
}

fn is_cjk(c: char) -> bool {
    matches!(c, '\u{4E00}'..='\u{9FFF}' | '\u{3400}'..='\u{4DBF}')
}
