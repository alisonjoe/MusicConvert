use std::collections::HashMap;
use std::path::Path;
use serde::Deserialize;
use crate::command::run_command;
use crate::error::AudioError;

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

pub async fn probe_metadata<P: AsRef<Path>>(file: P) -> Result<FfprobeOutput, AudioError> {
    let p_str = file.as_ref().to_string_lossy();
    let stdout = run_command(
        "ffprobe",
        &["-v", "quiet", "-print_format", "json", "-show_format", &p_str],
        30,
    )
    .await?;

    let parsed: FfprobeOutput = serde_json::from_str(&stdout)
        .map_err(|e| AudioError::Metadata(format!("Failed to parse ffprobe json: {}", e)))?;
    Ok(parsed)
}
