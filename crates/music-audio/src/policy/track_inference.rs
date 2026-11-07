use music_core::ReleaseMetadata;
use regex::Regex;
use std::sync::LazyLock;

static RE_DIGITS_PREFIX: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"^(\d{1,3})[\s.\-_]+(.+)$").unwrap());
static RE_GENERIC_TRACK: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)^track\s*0*(\d{1,3})$").unwrap());

/// Parses a track number from common metadata tag formats (e.g. "9", "09", "09/14", "9/14").
pub fn parse_track_number_from_tag(tag: &str) -> Option<u32> {
    let clean = tag.trim();
    if clean.is_empty() {
        return None;
    }
    let num_str = match clean.split_once('/') {
        Some((first, _)) => first.trim(),
        None => clean,
    };
    num_str.parse::<u32>().ok().filter(|&n| n > 0 && n < 1000)
}

/// Parses track number and title candidate from a filename (without file extension).
pub fn parse_track_and_title_from_filename(filename: &str) -> (Option<u32>, Option<String>) {
    let base = filename.trim();
    if base.is_empty() {
        return (None, None);
    }

    // Pattern 1: Leading digits like "09. The Fame", "09 - The Fame", "09 The Fame"
    if let Some(caps) = RE_DIGITS_PREFIX.captures(base) {
        if let (Some(m_num), Some(m_title)) = (caps.get(1), caps.get(2)) {
            if let Ok(n) = m_num.as_str().parse::<u32>() {
                if n > 0 && n < 1000 {
                    let title = m_title.as_str().trim_matches(&[' ', '-', '.', '_'][..]);
                    return (Some(n), Some(title.to_string()));
                }
            }
        }
    }

    // Pattern 2: Multi-part separated by " - ", e.g. "Lady Gaga - The Fame - 09 - The Fame"
    let parts: Vec<&str> = base.split(" - ").map(|s| s.trim()).filter(|s| !s.is_empty()).collect();
    if parts.len() >= 3 {
        // Look for a pure digit segment
        for (idx, part) in parts.iter().enumerate() {
            if let Ok(n) = part.parse::<u32>() {
                if n > 0 && n < 1000 {
                    // Title is the segment(s) after the track number
                    let title = if idx + 1 < parts.len() {
                        parts[idx + 1..].join(" - ")
                    } else {
                        parts[0].to_string()
                    };
                    return (Some(n), Some(title));
                }
            }
        }
    } else if parts.len() == 2 {
        // e.g. "01 - Bad Romance" or "Lady Gaga - Bad Romance"
        if let Ok(n) = parts[0].parse::<u32>() {
            return (Some(n), Some(parts[1].to_string()));
        }
    }

    // Pattern 3: Generic "Track 01" / "Track09"
    if let Some(caps) = RE_GENERIC_TRACK.captures(base) {
        if let Some(m) = caps.get(1) {
            if let Ok(n) = m.as_str().parse::<u32>() {
                return (Some(n), None);
            }
        }
    }

    (None, Some(base.to_string()))
}

/// Helper function to normalize strings for fuzzy matching
pub fn clean_title_for_match(s: &str) -> String {
    s.to_lowercase()
        .chars()
        .filter(|c| c.is_alphanumeric() || *c == ' ' || *c > '\u{4E00}')
        .collect::<String>()
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ")
}

/// Check if two track titles match (case-insensitive, ignoring punctuation)
pub fn title_matches(a: &str, b: &str) -> bool {
    let ca = clean_title_for_match(a);
    let cb = clean_title_for_match(b);
    if ca.is_empty() || cb.is_empty() {
        return false;
    }
    if ca == cb || ca.contains(&cb) || cb.contains(&ca) {
        return true;
    }
    false
}

#[derive(Debug, Clone, PartialEq, Eq)]
pub struct InferredTrackInfo {
    pub track_num: u32,
    pub title: String,
    pub artist: String,
}

/// Infer complete track metadata from local tags, filename, fallback index, and optional online album tracklist
pub fn infer_single_track_metadata(
    tag_track: Option<&str>,
    tag_title: Option<&str>,
    tag_artist: Option<&str>,
    filename: &str,
    fallback_index: u32,
    online_meta: Option<&ReleaseMetadata>,
) -> InferredTrackInfo {
    // 1. Try track number from tag
    let mut track_num = tag_track.and_then(parse_track_number_from_tag);

    // 2. Parse filename
    let (fn_track, fn_title) = parse_track_and_title_from_filename(filename);
    if track_num.is_none() {
        track_num = fn_track;
    }

    // 3. Determine title candidate
    let mut title = tag_title
        .filter(|t| !t.trim().is_empty() && !RE_GENERIC_TRACK.is_match(t.trim()))
        .map(|t| t.trim().to_string())
        .or(fn_title)
        .unwrap_or_else(|| filename.to_string());

    // 4. Try matching with online metadata
    if let Some(meta) = online_meta {
        // If track_num is still unknown, attempt to infer it by matching track title with online tracks
        if track_num.is_none() {
            for t in &meta.tracks {
                if title_matches(&title, &t.title) || title_matches(filename, &t.title) {
                    track_num = Some(t.track_number);
                    title = t.title.clone();
                    break;
                }
            }
        } else if let Some(n) = track_num {
            // If track_num is known, adopt standardized online track title if available
            if let Some(matched) = meta.tracks.iter().find(|t| t.track_number == n) {
                if title_matches(&title, &matched.title) || RE_GENERIC_TRACK.is_match(&title) {
                    title = matched.title.clone();
                }
            }
        }
    }

    // 5. Fallback track number if still undetermined
    let final_track_num = track_num.unwrap_or(fallback_index);

    let artist = tag_artist
        .filter(|a| !a.trim().is_empty())
        .unwrap_or("Unknown Artist")
        .to_string();

    InferredTrackInfo {
        track_num: final_track_num,
        title,
        artist,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use music_core::TrackMetadata;

    #[test]
    fn test_parse_track_number_from_tag() {
        assert_eq!(parse_track_number_from_tag("9"), Some(9));
        assert_eq!(parse_track_number_from_tag("09"), Some(9));
        assert_eq!(parse_track_number_from_tag("09/14"), Some(9));
        assert_eq!(parse_track_number_from_tag("14/14"), Some(14));
        assert_eq!(parse_track_number_from_tag(""), None);
        assert_eq!(parse_track_number_from_tag("invalid"), None);
    }

    #[test]
    fn test_parse_track_and_title_from_filename() {
        // Pattern 1: Leading digits
        let (num, title) = parse_track_and_title_from_filename("09. The Fame");
        assert_eq!(num, Some(9));
        assert_eq!(title.as_deref(), Some("The Fame"));

        let (num, title) = parse_track_and_title_from_filename("01 - Bad Romance");
        assert_eq!(num, Some(1));
        assert_eq!(title.as_deref(), Some("Bad Romance"));

        // Pattern 2: Multi-part separated by " - "
        let (num, title) =
            parse_track_and_title_from_filename("Lady Gaga - The Fame - 09 - The Fame");
        assert_eq!(num, Some(9));
        assert_eq!(title.as_deref(), Some("The Fame"));

        let (num, title) =
            parse_track_and_title_from_filename("Lady Gaga - The Fame - 12 - Paper Gangsta");
        assert_eq!(num, Some(12));
        assert_eq!(title.as_deref(), Some("Paper Gangsta"));

        let (num, title) =
            parse_track_and_title_from_filename("Lady Gaga - The Fame - 06 - Eh, Eh (Nothing Else I Can Say)");
        assert_eq!(num, Some(6));
        assert_eq!(title.as_deref(), Some("Eh, Eh (Nothing Else I Can Say)"));

        // Pattern 3: Generic track
        let (num, title) = parse_track_and_title_from_filename("Track 04");
        assert_eq!(num, Some(4));
        assert_eq!(title, None);
    }

    #[test]
    fn test_infer_single_track_metadata_with_online_recovery() {
        let online = ReleaseMetadata {
            album_artist: "Lady Gaga".into(),
            album: "The Fame".into(),
            year: Some("2008".into()),
            source: "iTunes".into(),
            tracks: vec![
                TrackMetadata {
                    track_number: 1,
                    title: "Just Dance".into(),
                    artist: "Lady Gaga".into(),
                    ..Default::default()
                },
                TrackMetadata {
                    track_number: 9,
                    title: "The Fame".into(),
                    artist: "Lady Gaga".into(),
                    ..Default::default()
                },
            ],
            ..Default::default()
        };

        // File has title in name without leading track number
        let inferred = infer_single_track_metadata(
            None,
            None,
            Some("Lady Gaga"),
            "Lady Gaga - The Fame",
            1,
            Some(&online),
        );
        assert_eq!(inferred.track_num, 9);
        assert_eq!(inferred.title, "The Fame");

        // File has full Lady Gaga - The Fame - 09 - The Fame
        let inferred2 = infer_single_track_metadata(
            None,
            None,
            Some("Lady Gaga"),
            "Lady Gaga - The Fame - 09 - The Fame",
            1,
            Some(&online),
        );
        assert_eq!(inferred2.track_num, 9);
        assert_eq!(inferred2.title, "The Fame");
    }
}
