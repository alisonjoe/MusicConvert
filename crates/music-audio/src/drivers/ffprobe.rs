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

    for line in output.lines() {
        let trimmed = line.trim();
        if trimmed == "Metadata:" {
            in_metadata = true;
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
                    tags.insert(key, val);
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
