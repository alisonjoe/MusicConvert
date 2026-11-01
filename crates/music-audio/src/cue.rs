use crate::error::AudioError;
use std::collections::HashMap;
use std::fs;
use std::path::Path;

#[derive(Debug, Clone, PartialEq, Default)]
pub struct CueTrack {
    pub track_num: u32,
    pub title: String,
    pub performer: String,
    pub rem: HashMap<String, String>,
    pub index01_time: String,
    pub index01_seconds: f64,
    pub duration_seconds: Option<f64>,
}

#[derive(Debug, Clone, PartialEq, Default)]
pub struct CueSheet {
    pub performer: String,
    pub title: String,
    pub file: String,
    pub rem: HashMap<String, String>,
    pub tracks: Vec<CueTrack>,
}

pub fn parse_time(time_str: &str) -> f64 {
    let parts: Vec<&str> = time_str.split(':').collect();
    if parts.len() != 3 {
        return 0.0;
    }
    let m = parts[0].parse::<f64>().unwrap_or(0.0);
    let s = parts[1].parse::<f64>().unwrap_or(0.0);
    let f = parts[2].parse::<f64>().unwrap_or(0.0);
    // 75 frames per second in CD audio standard
    m * 60.0 + s + (f / 75.0)
}

pub fn parse_cue_content(content: &str) -> CueSheet {
    let mut sheet = CueSheet::default();
    let mut current_track: Option<CueTrack> = None;

    for raw_line in content.lines() {
        let line = raw_line.trim();
        if line.is_empty() {
            continue;
        }

        if let Some(rest) = line.strip_prefix("REM ") {
            let parts: Vec<&str> = rest.split_whitespace().collect();
            if parts.len() >= 2 {
                let key = parts[0].to_uppercase();
                let val = parts[1..].join(" ").trim_matches('"').to_string();
                if let Some(track) = &mut current_track {
                    track.rem.insert(key, val);
                } else {
                    sheet.rem.insert(key, val);
                }
            }
        } else if let Some(rest) = line.strip_prefix("PERFORMER ") {
            let val = rest.trim().trim_matches('"').to_string();
            if let Some(track) = &mut current_track {
                track.performer = val;
            } else {
                sheet.performer = val;
            }
        } else if let Some(rest) = line.strip_prefix("TITLE ") {
            let val = rest.trim().trim_matches('"').to_string();
            if let Some(track) = &mut current_track {
                track.title = val;
            } else {
                sheet.title = val;
            }
        } else if let Some(rest) = line.strip_prefix("FILE ") {
            let parts: Vec<&str> = rest.split('"').collect();
            if parts.len() >= 2 {
                sheet.file = parts[1].to_string();
            } else {
                sheet.file = rest.split_whitespace().next().unwrap_or("").to_string();
            }
        } else if let Some(rest) = line.strip_prefix("TRACK ") {
            if let Some(track) = current_track.take() {
                sheet.tracks.push(track);
            }
            let parts: Vec<&str> = rest.split_whitespace().collect();
            let num = parts
                .first()
                .and_then(|s| s.parse::<u32>().ok())
                .unwrap_or(0);
            current_track = Some(CueTrack {
                track_num: num,
                ..Default::default()
            });
        } else if let Some(rest) = line.strip_prefix("INDEX 01 ") {
            let time_str = rest.trim();
            if let Some(track) = &mut current_track {
                track.index01_time = time_str.to_string();
                track.index01_seconds = parse_time(time_str);
            }
        }
    }

    if let Some(track) = current_track {
        sheet.tracks.push(track);
    }

    // Calculate duration for each track between consecutive INDEX 01
    for i in 0..sheet.tracks.len() {
        if i + 1 < sheet.tracks.len() {
            let dur = sheet.tracks[i + 1].index01_seconds - sheet.tracks[i].index01_seconds;
            if dur > 0.0 {
                sheet.tracks[i].duration_seconds = Some(dur);
            }
        }
    }

    sheet
}

pub fn parse_cue_file<P: AsRef<Path>>(path: P) -> Result<CueSheet, AudioError> {
    let content = fs::read_to_string(path)?;
    Ok(parse_cue_content(&content))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_cue_content() {
        let cue = r#"
REM GENRE "Pop"
REM DATE 1993
PERFORMER "张学友"
TITLE "吻别"
FILE "吻别.wav" WAVE
  TRACK 01 AUDIO
    TITLE "吻别"
    PERFORMER "张学友"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "情网"
    PERFORMER "张学友"
    INDEX 01 05:04:32
"#;
        let sheet = parse_cue_content(cue);
        assert_eq!(sheet.performer, "张学友");
        assert_eq!(sheet.title, "吻别");
        assert_eq!(sheet.file, "吻别.wav");
        assert_eq!(sheet.rem.get("DATE"), Some(&"1993".to_string()));
        assert_eq!(sheet.tracks.len(), 2);
        assert_eq!(sheet.tracks[0].title, "吻别");
        assert_eq!(sheet.tracks[0].index01_seconds, 0.0);
        assert_eq!(sheet.tracks[1].title, "情网");
        assert!(sheet.tracks[0].duration_seconds.unwrap() > 300.0);
    }
}
