use music_convert::engine::{
    BatchTask, ReviewDecision, TaskRegistry, TaskStatus, TrackItem, TrackStatus,
};
use music_convert::server::{create_router, AppState};
use music_core::Config;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;
use tokio::net::TcpListener;
use tokio::sync::RwLock;

async fn spawn_test_server(registry: Arc<RwLock<TaskRegistry>>) -> (SocketAddr, tokio::task::JoinHandle<()>) {
    let state = AppState::with_defaults(registry, Arc::new(Config::default()));
    let app = create_router(state);

    let listener = TcpListener::bind("127.0.0.1:0").await.expect("bind port");
    let addr = listener.local_addr().expect("local addr");

    let handle = tokio::spawn(async move {
        axum::serve(listener, app).await.unwrap();
    });

    (addr, handle)
}

#[tokio::test]
async fn test_health_endpoint() {
    let registry = Arc::new(RwLock::new(TaskRegistry::new(10, 20)));
    let (addr, _handle) = spawn_test_server(registry).await;

    let client = reqwest::Client::new();
    let res = client
        .get(format!("http://{}/api/health", addr))
        .send()
        .await
        .expect("send request");

    assert_eq!(res.status(), reqwest::StatusCode::OK);
    let json: serde_json::Value = res.json().await.expect("parse json");
    assert_eq!(json["status"], "ok");
    assert_eq!(json["version"], env!("CARGO_PKG_VERSION"));
}

#[tokio::test]
async fn test_tasks_list_and_get_detail() {
    let registry = Arc::new(RwLock::new(TaskRegistry::new(10, 20)));
    let (addr, _handle) = spawn_test_server(registry.clone()).await;

    // 1. Prepare tasks
    {
        let mut reg = registry.write().await;
        let mut t1 = BatchTask::new("task_active", "/path/1", "Jay Chou", "Fantasy");
        t1.add_track(TrackItem::new(1, "Track 1", "Jay Chou"));
        reg.register_task(t1);

        let t2 = BatchTask::new("task_history", "/path/2", "Eason", "U87");
        reg.register_task(t2);
        reg.complete_task("task_history");
    }

    let client = reqwest::Client::new();

    // 2. Query all tasks
    let res = client
        .get(format!("http://{}/api/tasks", addr))
        .send()
        .await
        .expect("send request");
    assert_eq!(res.status(), reqwest::StatusCode::OK);
    let json: serde_json::Value = res.json().await.unwrap();
    let active_list = json["active"].as_array().unwrap();
    let history_list = json["history"].as_array().unwrap();
    assert_eq!(active_list.len(), 1);
    assert_eq!(history_list.len(), 1);
    assert_eq!(active_list[0]["task_id"], "task_active");
    assert_eq!(history_list[0]["task_id"], "task_history");

    // 3. Query active only
    let res_active = client
        .get(format!("http://{}/api/tasks?status=active", addr))
        .send()
        .await
        .unwrap();
    let json_active: serde_json::Value = res_active.json().await.unwrap();
    assert_eq!(json_active["active"].as_array().unwrap().len(), 1);
    assert_eq!(json_active["history"].as_array().unwrap().len(), 0);

    // 4. Query single task detail
    let res_detail = client
        .get(format!("http://{}/api/tasks/task_active", addr))
        .send()
        .await
        .unwrap();
    assert_eq!(res_detail.status(), reqwest::StatusCode::OK);
    let json_detail: serde_json::Value = res_detail.json().await.unwrap();
    assert_eq!(json_detail["task_id"], "task_active");
    assert_eq!(json_detail["artist"], "Jay Chou");

    // 5. Query non-existent task
    let res_404 = client
        .get(format!("http://{}/api/tasks/not_found", addr))
        .send()
        .await
        .unwrap();
    assert_eq!(res_404.status(), reqwest::StatusCode::NOT_FOUND);
}

#[tokio::test]
async fn test_task_approve_and_cancel_endpoints() {
    let registry = Arc::new(RwLock::new(TaskRegistry::new(10, 20)));
    let (addr, _handle) = spawn_test_server(registry.clone()).await;

    let (review_tx, review_rx) = tokio::sync::oneshot::channel();
    {
        let mut reg = registry.write().await;
        let t = BatchTask::new("task_wait", "/path/wait", "Unknown", "Unknown");
        reg.register_task(t);
        reg.mark_awaiting_review("task_wait", review_tx);

        let t2 = BatchTask::new("task_cancel", "/path/cancel", "Artist", "Album");
        reg.register_task(t2);
    }

    let client = reqwest::Client::new();

    // 1. Approve task_wait
    let decision = ReviewDecision {
        override_artist: Some("Corrected Artist".to_string()),
        override_album: Some("Corrected Album".to_string()),
        override_year: Some("2024".to_string()),
        override_tracks: None,
    };
    let res_approve = client
        .post(format!("http://{}/api/tasks/task_wait/approve", addr))
        .json(&decision)
        .send()
        .await
        .unwrap();
    assert_eq!(res_approve.status(), reqwest::StatusCode::OK);

    // Verify oneshot receiver received decision
    let recvd = review_rx.await.expect("oneshot receive");
    assert_eq!(recvd.override_artist.as_deref(), Some("Corrected Artist"));

    // 2. Cancel task_cancel
    let res_cancel = client
        .post(format!("http://{}/api/tasks/task_cancel/cancel", addr))
        .send()
        .await
        .unwrap();
    assert_eq!(res_cancel.status(), reqwest::StatusCode::OK);

    // Verify registry state: task_cancel should be moved to history with Cancelled status
    let reg = registry.read().await;
    assert_eq!(reg.active_count(), 1); // task_wait is still active
    let task_cancelled = reg.get_task("task_cancel").expect("must find cancelled task");
    assert_eq!(task_cancelled.status, TaskStatus::Cancelled);
}

#[tokio::test]
async fn test_sse_events_stream() {
    let registry = Arc::new(RwLock::new(TaskRegistry::new(10, 20)));
    let (addr, _handle) = spawn_test_server(registry.clone()).await;

    let client = reqwest::Client::new();
    let mut response = client
        .get(format!("http://{}/api/events", addr))
        .send()
        .await
        .expect("connect SSE");

    assert_eq!(response.status(), reqwest::StatusCode::OK);
    assert_eq!(
        response
            .headers()
            .get("content-type")
            .and_then(|v| v.to_str().ok()),
        Some("text/event-stream")
    );

    // Emit event in registry
    let reg_clone = registry.clone();
    tokio::spawn(async move {
        tokio::time::sleep(Duration::from_millis(50)).await;
        let mut reg = reg_clone.write().await;
        let mut t = BatchTask::new("sse_task_1", "/music/sse", "SSE Artist", "SSE Album");
        t.add_track(TrackItem::new(1, "Track 1", "SSE Artist"));
        reg.register_task(t);
        reg.update_track_progress("sse_task_1", 1, 100.0, TrackStatus::Completed);
        reg.complete_task("sse_task_1");
    });

    // Read SSE chunks
    let mut received_payloads = Vec::new();
    let start = std::time::Instant::now();
    while start.elapsed() < Duration::from_secs(3) {
        if let Ok(Some(chunk)) = response.chunk().await {
            let chunk_str = String::from_utf8_lossy(&chunk);
            received_payloads.push(chunk_str.to_string());
            if received_payloads.iter().any(|p| p.contains("TaskCompleted")) {
                break;
            }
        }
    }

    let combined = received_payloads.join("");
    assert!(
        combined.contains("TaskDiscovered"),
        "SSE stream should contain TaskDiscovered: {}",
        combined
    );
    assert!(
        combined.contains("TrackProgressUpdated"),
        "SSE stream should contain TrackProgressUpdated: {}",
        combined
    );
    assert!(
        combined.contains("TaskCompleted"),
        "SSE stream should contain TaskCompleted: {}",
        combined
    );
}
