use crate::query::MetadataQuery;
use music_core::{
    normalize_classical_no, remove_discogs_suffix, to_simplified_chinese, ReleaseMetadata,
};

pub fn localize_metadata(query: &MetadataQuery, meta: &mut ReleaseMetadata) {
    // 1. If local context already has a reliable Chinese artist, protect it from English overwrite
    if query.has_chinese_context && !query.artist.is_empty() {
        let meta_has_chinese = meta.album_artist.chars().any(is_cjk);
        if !meta_has_chinese {
            // Preserve the query's Chinese artist as album_artist
            meta.album_artist = query.artist.clone();
        }
    }

    // 2. Normalize artist and album strings
    meta.album_artist = clean_identity_string(&meta.album_artist);
    meta.album = clean_identity_string(&meta.album);

    // 3. Normalize track metadata
    for track in &mut meta.tracks {
        if query.has_chinese_context
            && !query.artist.is_empty()
            && !track.artist.chars().any(is_cjk)
        {
            track.artist = query.artist.clone();
        }
        track.artist = clean_identity_string(&track.artist);
        track.album = clean_identity_string(&track.album);
        track.title = clean_track_title(&track.title);

        if let Some(composer) = &mut track.composer {
            *composer = clean_identity_string(composer);
        }
        if let Some(conductor) = &mut track.conductor {
            *conductor = clean_identity_string(conductor);
        }
        if let Some(orchestra) = &mut track.orchestra {
            *orchestra = clean_identity_string(orchestra);
        }
    }
}

fn clean_identity_string(s: &str) -> String {
    let simplified = to_simplified_chinese(s);
    let no_discogs = remove_discogs_suffix(&simplified);
    let normalized = normalize_classical_no(&no_discogs);
    normalized.trim().to_string()
}

fn clean_track_title(s: &str) -> String {
    let simplified = to_simplified_chinese(s);
    let normalized = normalize_classical_no(&simplified);
    normalized.trim().to_string()
}

fn is_cjk(c: char) -> bool {
    matches!(c, '\u{4E00}'..='\u{9FFF}' | '\u{3400}'..='\u{4DBF}')
}

#[cfg(test)]
mod tests {
    use super::*;
    use music_core::TrackMetadata;

    #[test]
    fn test_protect_chinese_artist_from_english_overwrite() {
        let query = MetadataQuery::new("张学友", "真爱");
        assert!(query.has_chinese_context);

        let mut meta = ReleaseMetadata {
            album: "True Love".to_string(),
            album_artist: "Jacky Cheung (2)".to_string(),
            year: Some("1995".to_string()),
            cover_url: None,
            tracks: vec![TrackMetadata {
                track_number: 1,
                title: "一千個傷心的理由".to_string(),
                artist: "Jacky Cheung".to_string(),
                album: "True Love".to_string(),
                ..Default::default()
            }],
            source: "MusicBrainz".to_string(),
        };

        localize_metadata(&query, &mut meta);

        // Chinese artist preserved, simplified, discogs suffix removed
        assert_eq!(meta.album_artist, "张学友");
        assert_eq!(meta.tracks[0].artist, "张学友");
        assert_eq!(meta.tracks[0].title, "一千个伤心的理由");
    }

    #[test]
    fn test_classical_title_normalization() {
        let query = MetadataQuery::new("Beethoven", "Symphony No. 9");
        let mut meta = ReleaseMetadata {
            album: "Symphonies N° 5 & 9".to_string(),
            album_artist: "Ludwig van Beethoven".to_string(),
            tracks: vec![TrackMetadata {
                track_number: 1,
                title: "Symphony N° 9 in D minor, Op. 125".to_string(),
                artist: "Ludwig van Beethoven".to_string(),
                album: "Symphonies".to_string(),
                ..Default::default()
            }],
            ..Default::default()
        };

        localize_metadata(&query, &mut meta);
        assert_eq!(meta.album, "Symphonies No. 5 & 9");
        assert_eq!(meta.tracks[0].title, "Symphony No. 9 in D minor, Op. 125");
    }
}
