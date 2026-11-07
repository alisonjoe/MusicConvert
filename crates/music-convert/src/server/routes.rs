use crate::server::assets::static_handler;
use crate::server::handlers::{
    approve_task, browse_fs, cancel_task, events_sse, get_config, get_task, health_check,
    list_converted, list_tasks, reconvert_music, trigger_scan, update_src_path,
};
use crate::server::state::AppState;
use axum::{
    routing::{get, post},
    Router,
};
use tower_http::cors::{Any, CorsLayer};

pub fn create_router(state: AppState) -> Router {
    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any);

    Router::new()
        .route("/api/health", get(health_check))
        .route("/api/config", get(get_config))
        .route("/api/config/src-path", post(update_src_path))
        .route("/api/fs/browse", get(browse_fs))
        .route("/api/converted", get(list_converted))
        .route("/api/converted/reconvert", post(reconvert_music))
        .route("/api/scan", post(trigger_scan))
        .route("/api/tasks", get(list_tasks))
        .route("/api/tasks/{id}", get(get_task))
        .route("/api/tasks/{id}/approve", post(approve_task))
        .route("/api/tasks/{id}/cancel", post(cancel_task))
        .route("/api/events", get(events_sse))
        .fallback(static_handler)
        .layer(cors)
        .with_state(state)
}
