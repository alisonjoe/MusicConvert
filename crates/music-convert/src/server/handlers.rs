use crate::engine::{BatchTask, EngineEvent, ReviewDecision};
use crate::server::state::AppState;
use axum::{
    extract::{Path, Query, State},
    http::StatusCode,
    response::sse::{Event, Sse},
    Json,
};
use serde::{Deserialize, Serialize};
use std::convert::Infallible;
use tokio_stream::wrappers::BroadcastStream;
use tokio_stream::{Stream, StreamExt};

#[derive(Debug, Serialize, Deserialize)]
pub struct HealthResponse {
    pub status: String,
    pub version: String,
}

#[derive(Debug, Deserialize)]
pub struct TaskListQuery {
    pub status: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct TasksResponse {
    pub active: Vec<BatchTask>,
    pub history: Vec<BatchTask>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ActionResponse {
    pub success: bool,
    pub message: String,
}

pub async fn health_check() -> Json<HealthResponse> {
    Json(HealthResponse {
        status: "ok".to_string(),
        version: env!("CARGO_PKG_VERSION").to_string(),
    })
}

pub async fn list_tasks(
    State(state): State<AppState>,
    Query(params): Query<TaskListQuery>,
) -> Json<TasksResponse> {
    let reg = state.registry.read().await;
    let mode = params.status.as_deref().unwrap_or("all");

    let active = if mode == "history" {
        Vec::new()
    } else {
        reg.list_active()
    };

    let history = if mode == "active" {
        Vec::new()
    } else {
        reg.list_history()
    };

    Json(TasksResponse { active, history })
}

pub async fn get_task(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<BatchTask>, StatusCode> {
    let reg = state.registry.read().await;
    match reg.get_task(&id) {
        Some(task) => Ok(Json(task)),
        None => Err(StatusCode::NOT_FOUND),
    }
}

pub async fn approve_task(
    State(state): State<AppState>,
    Path(id): Path<String>,
    Json(decision): Json<ReviewDecision>,
) -> Result<Json<ActionResponse>, (StatusCode, Json<ActionResponse>)> {
    let mut reg = state.registry.write().await;
    if reg.approve_task(&id, decision) {
        Ok(Json(ActionResponse {
            success: true,
            message: format!("Task {} approved", id),
        }))
    } else {
        Err((
            StatusCode::BAD_REQUEST,
            Json(ActionResponse {
                success: false,
                message: format!("Task {} is not awaiting review or not found", id),
            }),
        ))
    }
}

pub async fn cancel_task(
    State(state): State<AppState>,
    Path(id): Path<String>,
) -> Result<Json<ActionResponse>, (StatusCode, Json<ActionResponse>)> {
    let mut reg = state.registry.write().await;
    if reg.cancel_task(&id) {
        Ok(Json(ActionResponse {
            success: true,
            message: format!("Task {} cancelled", id),
        }))
    } else {
        Err((
            StatusCode::NOT_FOUND,
            Json(ActionResponse {
                success: false,
                message: format!("Task {} not found", id),
            }),
        ))
    }
}

pub async fn events_sse(
    State(state): State<AppState>,
) -> Sse<impl Stream<Item = Result<Event, Infallible>>> {
    let rx = state.registry.read().await.subscribe();
    let stream = BroadcastStream::new(rx).filter_map(|msg| match msg {
        Ok(engine_event) => {
            let event_name = match &engine_event {
                EngineEvent::TaskDiscovered(_) => "TaskDiscovered",
                EngineEvent::TaskStatusChanged { .. } => "TaskStatusChanged",
                EngineEvent::TaskAwaitingReview { .. } => "TaskAwaitingReview",
                EngineEvent::TrackProgressUpdated { .. } => "TrackProgressUpdated",
                EngineEvent::TaskCompleted { .. } => "TaskCompleted",
                EngineEvent::TaskFailed { .. } => "TaskFailed",
                EngineEvent::LogEmitted { .. } => "LogEmitted",
            };
            let json_str = serde_json::to_string(&engine_event).unwrap_or_default();
            Some(Ok(Event::default().event(event_name).data(json_str)))
        }
        Err(_) => None,
    });

    Sse::new(stream).keep_alive(axum::response::sse::KeepAlive::default())
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ConfigResponse {
    pub src_path: String,
    pub output_root: String,
    pub desc_type: String,
    pub is_split: bool,
}

pub async fn get_config(State(state): State<AppState>) -> Json<ConfigResponse> {
    let src = state.current_src_path.read().await.to_string_lossy().to_string();
    Json(ConfigResponse {
        src_path: src,
        output_root: state.config.output_root.clone(),
        desc_type: state.config.desc_type.clone(),
        is_split: state.config.is_split,
    })
}

#[derive(Debug, Deserialize)]
pub struct UpdateSrcPathRequest {
    pub src_path: String,
}

pub async fn update_src_path(
    State(state): State<AppState>,
    Json(payload): Json<UpdateSrcPathRequest>,
) -> Result<Json<ActionResponse>, (StatusCode, Json<ActionResponse>)> {
    let target = std::path::Path::new(&payload.src_path);
    if !target.exists() || !target.is_dir() {
        return Err((
            StatusCode::BAD_REQUEST,
            Json(ActionResponse {
                success: false,
                message: format!("Directory does not exist: {}", payload.src_path),
            }),
        ));
    }

    let mut current = state.current_src_path.write().await;
    *current = target.to_path_buf();

    Ok(Json(ActionResponse {
        success: true,
        message: format!("Source path updated to {}", payload.src_path),
    }))
}

#[derive(Debug, Deserialize)]
pub struct BrowseQuery {
    pub path: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct BrowseResponse {
    pub current: String,
    pub parent: Option<String>,
    pub directories: Vec<String>,
}

pub async fn browse_fs(
    State(state): State<AppState>,
    Query(query): Query<BrowseQuery>,
) -> Result<Json<BrowseResponse>, (StatusCode, Json<ActionResponse>)> {
    let target_str = match &query.path {
        Some(p) if !p.is_empty() => p.clone(),
        _ => state.current_src_path.read().await.to_string_lossy().to_string(),
    };

    let target_path = std::path::Path::new(&target_str);
    if !target_path.exists() {
        return Err((
            StatusCode::NOT_FOUND,
            Json(ActionResponse {
                success: false,
                message: format!("Path not found: {}", target_str),
            }),
        ));
    }

    let mut directories = Vec::new();
    if let Ok(entries) = std::fs::read_dir(target_path) {
        for entry in entries.flatten() {
            if let Ok(file_type) = entry.file_type() {
                if file_type.is_dir() {
                    let name = entry.file_name().to_string_lossy().to_string();
                    if !name.starts_with('.') && name != "@eaDir" {
                        directories.push(name);
                    }
                }
            }
        }
    }
    directories.sort();

    let parent = target_path.parent().map(|p| p.to_string_lossy().to_string());

    Ok(Json(BrowseResponse {
        current: target_path.to_string_lossy().to_string(),
        parent,
        directories,
    }))
}

#[derive(Debug, Deserialize)]
pub struct ConvertedQuery {
    pub limit: Option<usize>,
    pub offset: Option<usize>,
    pub search: Option<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ConvertedResponse {
    pub total: usize,
    pub limit: usize,
    pub offset: usize,
    pub items: Vec<music_core::MusicInfo>,
}

pub async fn list_converted(
    State(state): State<AppState>,
    Query(query): Query<ConvertedQuery>,
) -> Result<Json<ConvertedResponse>, (StatusCode, Json<ActionResponse>)> {
    let limit = query.limit.unwrap_or(50);
    let offset = query.offset.unwrap_or(0);
    let search = query.search.as_deref().filter(|s| !s.trim().is_empty());

    let total = state.db.count_records(search).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ActionResponse {
                success: false,
                message: format!("Failed to count records: {}", e),
            }),
        )
    })?;

    let items = state.db.list_records(limit, offset, search).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ActionResponse {
                success: false,
                message: format!("Failed to list records: {}", e),
            }),
        )
    })?;

    Ok(Json(ConvertedResponse {
        total,
        limit,
        offset,
        items,
    }))
}

#[derive(Debug, Deserialize)]
pub struct ReconvertRequest {
    pub music: String,
}

pub async fn reconvert_music(
    State(state): State<AppState>,
    Json(payload): Json<ReconvertRequest>,
) -> Result<Json<ActionResponse>, (StatusCode, Json<ActionResponse>)> {
    let affected = state.db.delete_by_music(&payload.music).map_err(|e| {
        (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(ActionResponse {
                success: false,
                message: format!("Failed to delete record: {}", e),
            }),
        )
    })?;

    if let Some(orch) = &state.orchestrator {
        let p = std::path::PathBuf::from(&payload.music);
        let dir = if p.is_dir() {
            p
        } else {
            p.parent().map(|d| d.to_path_buf()).unwrap_or(p)
        };
        let orch = orch.clone();
        tokio::spawn(async move {
            if dir.exists() {
                let batches = crate::monitor::scan_release_batches(&dir);
                for b in batches {
                    orch.process_batch(&b).await;
                }
            }
        });
    }

    Ok(Json(ActionResponse {
        success: true,
        message: format!(
            "Removed {} record(s) and triggered reconversion for {}",
            affected, payload.music
        ),
    }))
}

#[derive(Debug, Deserialize, Default)]
pub struct ScanRequest {
    pub path: Option<String>,
}

pub async fn trigger_scan(
    State(state): State<AppState>,
    Json(payload): Json<ScanRequest>,
) -> Json<ActionResponse> {
    let scan_path = match payload.path {
        Some(p) if !p.is_empty() => std::path::PathBuf::from(p),
        _ => state.current_src_path.read().await.clone(),
    };

    if let Some(orch) = &state.orchestrator {
        let orch = orch.clone();
        let target = scan_path.clone();
        tokio::spawn(async move {
            if target.exists() {
                let batches = crate::monitor::scan_release_batches(&target);
                for b in batches {
                    orch.process_batch(&b).await;
                }
            }
        });
    }

    Json(ActionResponse {
        success: true,
        message: format!("Scan triggered for {:?}", scan_path),
    })
}
