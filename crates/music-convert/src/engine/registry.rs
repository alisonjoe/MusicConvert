use crate::engine::event::EngineEvent;
use crate::engine::task::{BatchTask, TaskStatus, TrackStatus};
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, VecDeque};
use tokio::sync::{broadcast, oneshot};
use tracing::warn;

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ReviewDecision {
    pub override_artist: Option<String>,
    pub override_album: Option<String>,
    pub override_year: Option<String>,
    pub override_tracks: Option<Vec<String>>,
}

pub struct TaskRegistry {
    active_tasks: HashMap<String, BatchTask>,
    history_buffer: VecDeque<BatchTask>,
    history_capacity: usize,
    event_tx: broadcast::Sender<EngineEvent>,
    review_senders: HashMap<String, oneshot::Sender<ReviewDecision>>,
}

impl TaskRegistry {
    pub fn new(history_capacity: usize, event_bus_capacity: usize) -> Self {
        let (event_tx, _) = broadcast::channel(event_bus_capacity.max(64));
        Self {
            active_tasks: HashMap::new(),
            history_buffer: VecDeque::with_capacity(history_capacity),
            history_capacity: history_capacity.max(1),
            event_tx,
            review_senders: HashMap::new(),
        }
    }

    pub fn subscribe(&self) -> broadcast::Receiver<EngineEvent> {
        self.event_tx.subscribe()
    }

    pub fn emit(&self, event: EngineEvent) {
        let _ = self.event_tx.send(event);
    }

    pub fn register_task(&mut self, task: BatchTask) {
        let event = EngineEvent::TaskDiscovered(task.clone());
        self.active_tasks.insert(task.task_id.clone(), task);
        self.emit(event);
    }

    pub fn update_task_status(&mut self, task_id: &str, status: TaskStatus) {
        if let Some(task) = self.active_tasks.get_mut(task_id) {
            task.set_status(status.clone());
            self.emit(EngineEvent::TaskStatusChanged {
                task_id: task_id.to_string(),
                status,
            });
        }
    }

    pub fn update_track_progress(
        &mut self,
        task_id: &str,
        track_number: u32,
        progress: f32,
        status: TrackStatus,
    ) {
        if let Some(task) = self.active_tasks.get_mut(task_id) {
            task.set_track_progress(track_number, progress, status.clone());
            self.emit(EngineEvent::TrackProgressUpdated {
                task_id: task_id.to_string(),
                track_number,
                progress_percent: progress,
                status,
            });
        }
    }

    pub fn mark_awaiting_review(
        &mut self,
        task_id: &str,
        waiter: oneshot::Sender<ReviewDecision>,
    ) {
        if let Some(task) = self.active_tasks.get_mut(task_id) {
            task.set_status(TaskStatus::AwaitingReview);
            let snapshot = task.clone();
            self.review_senders.insert(task_id.to_string(), waiter);
            self.emit(EngineEvent::TaskAwaitingReview {
                task_id: task_id.to_string(),
                task: snapshot,
            });
        } else {
            warn!("[TaskRegistry] mark_awaiting_review: task {} not found", task_id);
        }
    }

    pub fn approve_task(&mut self, task_id: &str, decision: ReviewDecision) -> bool {
        if let Some(task) = self.active_tasks.get_mut(task_id) {
            if let Some(artist) = &decision.override_artist {
                task.artist = artist.clone();
            }
            if let Some(album) = &decision.override_album {
                task.album = album.clone();
            }
            if let Some(year) = &decision.override_year {
                task.year = Some(year.clone());
            }
        }

        if let Some(sender) = self.review_senders.remove(task_id) {
            sender.send(decision).is_ok()
        } else {
            false
        }
    }

    pub fn cancel_task(&mut self, task_id: &str) -> bool {
        let _ = self.review_senders.remove(task_id);
        if let Some(mut task) = self.active_tasks.remove(task_id) {
            task.set_status(TaskStatus::Cancelled);
            self.emit(EngineEvent::TaskStatusChanged {
                task_id: task_id.to_string(),
                status: TaskStatus::Cancelled,
            });
            self.push_history(task);
            true
        } else {
            false
        }
    }

    pub fn complete_task(&mut self, task_id: &str) {
        let _ = self.review_senders.remove(task_id);
        if let Some(mut task) = self.active_tasks.remove(task_id) {
            task.set_status(TaskStatus::Completed);
            self.emit(EngineEvent::TaskCompleted {
                task_id: task_id.to_string(),
            });
            self.push_history(task);
        }
    }

    pub fn fail_task(&mut self, task_id: &str, error: String) {
        let _ = self.review_senders.remove(task_id);
        if let Some(mut task) = self.active_tasks.remove(task_id) {
            task.set_status(TaskStatus::Failed {
                error: error.clone(),
            });
            self.emit(EngineEvent::TaskFailed {
                task_id: task_id.to_string(),
                error,
            });
            self.push_history(task);
        }
    }

    fn push_history(&mut self, task: BatchTask) {
        if self.history_buffer.len() >= self.history_capacity {
            self.history_buffer.pop_front();
        }
        self.history_buffer.push_back(task);
    }

    pub fn get_task(&self, task_id: &str) -> Option<BatchTask> {
        self.active_tasks
            .get(task_id)
            .cloned()
            .or_else(|| self.history_buffer.iter().find(|t| t.task_id == task_id).cloned())
    }

    pub fn get_task_mut(&mut self, task_id: &str) -> Option<&mut BatchTask> {
        self.active_tasks.get_mut(task_id)
    }

    pub fn list_active(&self) -> Vec<BatchTask> {
        let mut list: Vec<BatchTask> = self.active_tasks.values().cloned().collect();
        list.sort_by(|a, b| b.created_at.cmp(&a.created_at));
        list
    }

    pub fn list_history(&self) -> Vec<BatchTask> {
        self.history_buffer.iter().cloned().collect()
    }

    pub fn active_count(&self) -> usize {
        self.active_tasks.len()
    }

    pub fn history_count(&self) -> usize {
        self.history_buffer.len()
    }
}
