use super::cli::run_command;
use crate::error::AudioError;
use serde::Deserialize;
use std::collections::HashMap;
use std::path::Path;

#[derive(Debug, Clone, Deserialize, Default)]
pub struct FfprobeOutput {
    #[serde(default)]
    pub format: FfprobeFormat,
}

#[derive(Debug, Clone, Deserialize, Default)]
pub struct FfprobeFormat {
    #[serde(default)]
    pub filename: String,
    #[serde(default)]
    pub format_name: String,
    #[serde(default)]
    pub duration: Option<String>,
    #[serde(default)]
    pub tags: HashMap<String, String>,
}

impl FfprobeFormat {
    pub fn get_tag(&self, key: &str) -> Option<&str> {
        let key_lower = key.to_lowercase();
        for (k, v) in &self.tags {
            if k.to_lowercase() == key_lower {
                return Some(v.as_str());
            }
        }
        None
    }
}

pub fn parse_ffmpeg_stderr_tags(output: &str) -> HashMap<String, String> {
    let mut tags = HashMap::new();
    let mut in_metadata = false;
    let mut in_video_stream = false;

    for line in output.lines() {
        let trimmed = line.trim();

        if trimmed.starts_with("Stream #") {
            in_video_stream = trimmed.contains("Video:");
            in_metadata = false;
            continue;
        }

        if trimmed.starts_with("Duration:") {
            in_metadata = false;
            continue;
        }

        if trimmed == "Metadata:" {
            // Ignore video (cover art) stream metadata
            if in_video_stream {
                in_metadata = false;
            } else {
                in_metadata = true;
            }
            continue;
        }

        if in_metadata {
            if !line.starts_with("    ") && !line.starts_with('\t') {
                if trimmed.starts_with("Duration:") || trimmed.starts_with("Stream #") {
                    in_metadata = false;
                    continue;
                }
            }
            if let Some((k, v)) = trimmed.split_once(':') {
                let key = k.trim().to_lowercase();
                let val = v.trim().to_string();
                if !key.is_empty() && !val.is_empty() {
                    // Prevent image filenames like 'cover.jpg' from polluting the track title
                    if key == "title" {
                        let lower = val.to_lowercase();
                        if lower.ends_with(".jpg")
                            || lower.ends_with(".jpeg")
                            || lower.ends_with(".png")
                            || lower.ends_with(".webp")
                        {
                            continue;
                        }
                    }
                    // Container tags take precedence over subsequent stream tags
                    tags.entry(key).or_insert(val);
                }
            }
        }
    }
    tags
}

pub async fn probe_metadata<P: AsRef<Path>>(file: P) -> Result<FfprobeOutput, AudioError> {
    let p_str = file.as_ref().to_string_lossy();
    let ffprobe_res = run_command(
        "ffprobe",
        &[
            "-v",
            "quiet",
            "-print_format",
            "json",
            "-show_format",
            &p_str,
        ],
        30,
    )
    .await;

    match ffprobe_res {
        Ok(stdout) => {
            let parsed: FfprobeOutput = serde_json::from_str(&stdout)
                .map_err(|e| AudioError::Metadata(format!("Failed to parse ffprobe json: {}", e)))?;
            Ok(parsed)
        }
        Err(_) => {
            // Fallback: run ffmpeg -i <file> to extract metadata from stderr
            let ffmpeg_out = run_command("ffmpeg", &["-i", &p_str], 30).await;
            let output_str = match ffmpeg_out {
                Ok(s) => s,
                Err(AudioError::CommandFailed(_, stderr)) => stderr,
                Err(e) => return Err(e),
            };

            let tags = parse_ffmpeg_stderr_tags(&output_str);
            if tags.is_empty() {
                return Err(AudioError::Metadata(format!(
                    "Neither ffprobe nor ffmpeg could extract tags from {}",
                    p_str
                )));
            }

            Ok(FfprobeOutput {
                format: FfprobeFormat {
                    filename: p_str.to_string(),
                    format_name: "audio".to_string(),
                    duration: None,
                    tags,
                },
            })
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_ffmpeg_stderr_tags_with_embedded_cover() {
        let sample = r#"
Input #0, flac, from '/path/to/01.flac':
  Metadata:
    ARTIST          : Lady Gaga
    TITLE           : Bad Romance
    ALBUM           : The Fame
    track           : 1
    disc            : 1
  Duration: 00:04:54.47, start: 0.000000, bitrate: 1062 kb/s
    Stream #0:0: Audio: flac, 44100 Hz, stereo, s16
    Stream #0:1: Video: mjpeg, yuvj420p
    Metadata:
      comment         : Cover (front)
      title           : cover.jpg
At least one output file must be specified
"#;
        let tags = parse_ffmpeg_stderr_tags(sample);
        assert_eq!(tags.get("artist").map(|s| s.as_str()), Some("Lady Gaga"));
        assert_eq!(tags.get("title").map(|s| s.as_str()), Some("Bad Romance"));
        assert_eq!(tags.get("album").map(|s| s.as_str()), Some("The Fame"));
        assert_eq!(tags.get("track").map(|s| s.as_str()), Some("1"));
    }
}
