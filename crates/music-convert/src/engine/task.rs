use chrono::Utc;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", content = "detail")]
pub enum TaskStatus {
    Discovered,
    MatchingMetadata,
    AwaitingReview,
    Transcoding,
    Completed,
    Failed { error: String },
    Cancelled,
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "type", content = "detail")]
pub enum TrackStatus {
    Pending,
    Converting,
    Completed,
    Failed { error: String },
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct TrackItem {
    pub track_number: u32,
    pub title: String,
    pub artist: String,
    pub progress_percent: f32,
    pub status: TrackStatus,
    pub error_message: Option<String>,
}

impl TrackItem {
    pub fn new(track_number: u32, title: impl Into<String>, artist: impl Into<String>) -> Self {
        Self {
            track_number,
            title: title.into(),
            artist: artist.into(),
            progress_percent: 0.0,
            status: TrackStatus::Pending,
            error_message: None,
        }
    }
}

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct BatchTask {
    pub task_id: String,
    pub release_path: String,
    pub artist: String,
    pub album: String,
    pub year: Option<String>,
    pub status: TaskStatus,
    pub confidence: f32,
    pub tracks: Vec<TrackItem>,
    pub created_at: String,
    pub updated_at: String,
    pub error_message: Option<String>,
}

impl BatchTask {
    pub fn new(
        task_id: impl Into<String>,
        release_path: impl Into<String>,
        artist: impl Into<String>,
        album: impl Into<String>,
    ) -> Self {
        let now = Utc::now().to_rfc3339();
        Self {
            task_id: task_id.into(),
            release_path: release_path.into(),
            artist: artist.into(),
            album: album.into(),
            year: None,
            status: TaskStatus::Discovered,
            confidence: 1.0,
            tracks: Vec::new(),
            created_at: now.clone(),
            updated_at: now,
            error_message: None,
        }
    }

    pub fn add_track(&mut self, track: TrackItem) {
        self.tracks.push(track);
        self.updated_at = Utc::now().to_rfc3339();
    }

    pub fn set_status(&mut self, status: TaskStatus) {
        if let TaskStatus::Failed { ref error } = status {
            self.error_message = Some(error.clone());
        }
        self.status = status;
        self.updated_at = Utc::now().to_rfc3339();
    }

    pub fn set_track_progress(&mut self, track_num: u32, progress: f32, status: TrackStatus) {
        if let Some(t) = self.tracks.iter_mut().find(|t| t.track_number == track_num) {
            t.progress_percent = progress.clamp(0.0, 100.0);
            if let TrackStatus::Failed { ref error } = status {
                t.error_message = Some(error.clone());
            }
            t.status = status;
            self.updated_at = Utc::now().to_rfc3339();
        }
    }
}
