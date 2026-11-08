use crate::engine::task::{BatchTask, TaskLogItem, TaskStatus, TrackStatus};
use chrono::Utc;
use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(tag = "event", content = "data")]
pub enum EngineEvent {
    TaskDiscovered(BatchTask),
    TaskStatusChanged {
        task_id: String,
        status: TaskStatus,
    },
    TaskAwaitingReview {
        task_id: String,
        task: BatchTask,
    },
    TrackProgressUpdated {
        task_id: String,
        track_number: u32,
        progress_percent: f32,
        status: TrackStatus,
    },
    TaskLogAppended {
        task_id: String,
        log: TaskLogItem,
    },
    TaskCompleted {
        task_id: String,
    },
    TaskFailed {
        task_id: String,
        error: String,
    },
    LogEmitted {
        level: String,
        message: String,
        timestamp: String,
    },
}

impl EngineEvent {
    pub fn log(level: impl Into<String>, message: impl Into<String>) -> Self {
        Self::LogEmitted {
            level: level.into(),
            message: message.into(),
            timestamp: Utc::now().to_rfc3339(),
        }
    }
}
