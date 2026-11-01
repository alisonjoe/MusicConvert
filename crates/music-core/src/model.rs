use serde::{Deserialize, Serialize};
use std::path::PathBuf;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
pub enum MusicType {
    Wav,
    Flac,
    Ape,
    Dff,
    Dsf,
    Alac,
    Wv,
    Wma,
}

impl MusicType {
    pub fn from_extension(ext: &str) -> Option<Self> {
        match ext.to_uppercase().trim_start_matches('.') {
            "WAV" => Some(Self::Wav),
            "FLAC" => Some(Self::Flac),
            "APE" => Some(Self::Ape),
            "DFF" => Some(Self::Dff),
            "DSF" => Some(Self::Dsf),
            "ALAC" => Some(Self::Alac),
            "WV" => Some(Self::Wv),
            "WMA" => Some(Self::Wma),
            _ => None,
        }
    }
}

pub const SUPPORTED_AUDIO_EXTENSIONS: &[&str] = &[
    "FLAC", "APE", "WAV", "CUE", "MP3", "M4A", "DFF", "DSF", "WV", "WMA",
];

pub fn is_supported_extension(ext: &str) -> bool {
    let clean = ext.to_uppercase().trim_start_matches('.').to_string();
    SUPPORTED_AUDIO_EXTENSIONS.contains(&clean.as_str())
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct MusicFileInfo {
    pub file_path: Option<PathBuf>,
    pub file_suffix: String,
    pub cue_path: Option<PathBuf>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct ReleaseBatch {
    pub release_context: PathBuf,
    pub files: Vec<MusicFileInfo>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct TrackMetadata {
    pub track_number: u32,
    pub title: String,
    pub artist: String,
    pub album: String,
    pub album_artist: Option<String>,
    pub year: Option<String>,
    pub disc_number: Option<u32>,
    pub disc_total: Option<u32>,
    pub genre: Option<String>,
    pub composer: Option<String>,
    pub conductor: Option<String>,
    pub orchestra: Option<String>,
    pub comment: Option<String>,
    pub lyrics: Option<String>,
}

#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
pub struct ReleaseMetadata {
    pub album: String,
    pub album_artist: String,
    pub year: Option<String>,
    pub cover_url: Option<String>,
    pub tracks: Vec<TrackMetadata>,
    pub source: String,
}
