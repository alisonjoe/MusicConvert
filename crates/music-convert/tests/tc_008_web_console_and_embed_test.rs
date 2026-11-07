use music_convert::engine::{ConvertOrchestrator, ReviewDecision, TaskRegistry};
use music_convert::server::{create_router, AppState};
use music_core::{Config, Database, MusicFileInfo, ReleaseBatch};
use music_provider::ProviderChain;
use std::net::SocketAddr;
use std::sync::Arc;
use std::time::Duration;
use tempfile::tempdir;
use tokio::net::TcpListener;
use tokio::sync::RwLock;

async fn spawn_server(registry: Arc<RwLock<TaskRegistry>>) -> (SocketAddr, tokio::task::JoinHandle<()>) {
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
async fn test_embedded_web_console_static_serving() {
    let registry = Arc::new(RwLock::new(TaskRegistry::new(10, 20)));
    let (addr, _handle) = spawn_server(registry).await;

    let client = reqwest::Client::new();

    // 1. Root index.html request
    let res = client
        .get(format!("http://{}/", addr))
        .send()
        .await
        .expect("get root");
    assert_eq!(res.status(), reqwest::StatusCode::OK);
    let content_type = res
        .headers()
        .get("content-type")
        .and_then(|v| v.to_str().ok())
        .unwrap_or("");
    assert!(content_type.contains("text/html"));
    let html = res.text().await.expect("read html text");
    assert!(html.contains("MusicConvert 控制台"));
    assert!(html.contains("EventSource('/api/events')"));

    // 2. SPA fallback request for arbitrary route
    let res_fallback = client
        .get(format!("http://{}/dashboard/tasks/review", addr))
        .send()
        .await
        .expect("get fallback route");
    assert_eq!(res_fallback.status(), reqwest::StatusCode::OK);
    let fallback_html = res_fallback.text().await.unwrap();
    assert!(fallback_html.contains("MusicConvert 控制台"));
}

#[tokio::test]
async fn test_full_lifecycle_review_and_approval_via_web_api() {
    let dir = tempdir().expect("tempdir");
    let src_dir = dir.path().join("src");
    let out_dir = dir.path().join("out");
    std::fs::create_dir_all(&src_dir).unwrap();
    std::fs::create_dir_all(&out_dir).unwrap();

    let db_path = dir.path().join("test.db");
    let db = Arc::new(Database::open(&db_path).expect("open db"));

    let config = Config {
        src_path: src_dir.to_string_lossy().to_string(),
        output_root: out_dir.to_string_lossy().to_string(),
        desc_type: "flac".to_string(),
        is_split: false,
        is_del_src: false,
        ..Default::default()
    };

    let registry = Arc::new(RwLock::new(TaskRegistry::new(10, 20)));
    let provider_chain = Arc::new(ProviderChain::new());
    let orchestrator = Arc::new(ConvertOrchestrator::with_registry(
        config,
        db,
        provider_chain,
        registry.clone(),
    ));

    let (addr, _server_handle) = spawn_server(registry.clone()).await;

    // Create a dummy file with unknown artist to trigger AwaitingReview (confidence = 0.40)
    let album_dir = src_dir.join("Unknown Artist").join("Unknown Album");
    std::fs::create_dir_all(&album_dir).unwrap();
    let audio_file = album_dir.join("track01.wav");
    std::fs::write(&audio_file, b"dummy audio bytes").unwrap();

    let batch = ReleaseBatch {
        release_context: album_dir.clone(),
        files: vec![MusicFileInfo {
            file_path: Some(audio_file.clone()),
            cue_path: None,
            file_suffix: "wav".to_string(),
        }],
    };

    let orch = orchestrator.clone();
    let _process_handle = tokio::spawn(async move {
        orch.process_batch(&batch).await;
    });

    let client = reqwest::Client::new();

    // 1. Poll /api/tasks until task enters AwaitingReview
    let mut awaiting_task_id = String::new();
    let start_wait = std::time::Instant::now();
    while start_wait.elapsed() < Duration::from_secs(4) {
        if let Ok(res) = client.get(format!("http://{}/api/tasks", addr)).send().await {
            if let Ok(json) = res.json::<serde_json::Value>().await {
                if let Some(active) = json["active"].as_array() {
                    for t in active {
                        let status_type = t["status"]["type"].as_str().or_else(|| t["status"].as_str());
                        if status_type == Some("AwaitingReview") {
                            awaiting_task_id = t["task_id"].as_str().unwrap().to_string();
                            break;
                        }
                    }
                }
            }
        }
        if !awaiting_task_id.is_empty() {
            break;
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }

    assert!(!awaiting_task_id.is_empty(), "Task should enter AwaitingReview");

    // 2. Submit approve via Web API
    let decision = ReviewDecision {
        override_artist: Some("Web User Approved Artist".to_string()),
        override_album: Some("Web User Approved Album".to_string()),
        override_year: Some("2026".to_string()),
        override_tracks: None,
    };

    let approve_res = client
        .post(format!("http://{}/api/tasks/{}/approve", addr, awaiting_task_id))
        .json(&decision)
        .send()
        .await
        .expect("send approve");
    assert_eq!(approve_res.status(), reqwest::StatusCode::OK);

    // 3. Verify task moves to Completed and is recorded in history
    let mut completed = false;
    let start_complete = std::time::Instant::now();
    while start_complete.elapsed() < Duration::from_secs(4) {
        if let Ok(res) = client.get(format!("http://{}/api/tasks", addr)).send().await {
            if let Ok(json) = res.json::<serde_json::Value>().await {
                if let Some(history) = json["history"].as_array() {
                    if history.iter().any(|t| t["task_id"] == awaiting_task_id) {
                        completed = true;
                        break;
                    }
                }
            }
        }
        tokio::time::sleep(Duration::from_millis(50)).await;
    }

    assert!(completed, "Task should complete and move into history");

    // 4. Verify task final metadata
    let reg = registry.read().await;
    let finished_task = reg.get_task(&awaiting_task_id).expect("must find finished task");
    assert_eq!(finished_task.artist, "Web User Approved Artist");
    assert_eq!(finished_task.album, "Web User Approved Album");
    assert_eq!(finished_task.year.as_deref(), Some("2026"));
}
