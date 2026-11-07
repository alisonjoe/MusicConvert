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
