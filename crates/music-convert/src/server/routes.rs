use crate::server::handlers::{
    approve_task, cancel_task, events_sse, get_task, health_check, list_tasks,
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
        .route("/api/tasks", get(list_tasks))
        .route("/api/tasks/{id}", get(get_task))
        .route("/api/tasks/{id}/approve", post(approve_task))
        .route("/api/tasks/{id}/cancel", post(cancel_task))
        .route("/api/events", get(events_sse))
        .layer(cors)
        .with_state(state)
}
