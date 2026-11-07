use music_convert::engine::TaskRegistry;
use music_convert::server::{create_router, AppState};
use music_core::{Config, Database};
use serde_json::Value;
use std::fs;
use std::net::SocketAddr;
use std::sync::Arc;
use tempfile::tempdir;
use tokio::net::TcpListener;
use tokio::sync::RwLock;

async fn spawn_test_server(state: AppState) -> (SocketAddr, tokio::task::JoinHandle<()>) {
    let app = create_router(state);
    let listener = TcpListener::bind("127.0.0.1:0").await.expect("bind port");
    let addr = listener.local_addr().expect("local addr");
    let handle = tokio::spawn(async move {
        axum::serve(listener, app).await.unwrap();
    });
    (addr, handle)
}

#[tokio::test]
async fn test_tc_009_web_control_api_suite() {
    let temp_root = tempdir().unwrap();
    let src_dir = temp_root.path().join("music_src");
    fs::create_dir_all(&src_dir).unwrap();
    fs::create_dir_all(src_dir.join("AlbumA")).unwrap();
    fs::create_dir_all(src_dir.join("AlbumB")).unwrap();
    fs::write(src_dir.join("ignore_me.txt"), "hello").unwrap();

    let mut config = Config::default();
    config.src_path = src_dir.to_str().unwrap().to_string();
    config.output_root = temp_root.path().join("output").to_str().unwrap().to_string();

    let db = Arc::new(Database::open_in_memory().unwrap());
    db.insert("/music/artist/song1.flac", 1).unwrap();
    db.insert("/music/artist/song2.flac", 1).unwrap();
    db.insert("/music/other/track.flac", 1).unwrap();

    let registry = Arc::new(RwLock::new(TaskRegistry::new(10, 10)));
    let state = AppState::new(registry, Arc::new(config), db.clone(), None);
    let (addr, _handle) = spawn_test_server(state).await;

    let client = reqwest::Client::new();
    let base_url = format!("http://{}", addr);

    // 1. GET /api/config
    let res = client
        .get(format!("{}/api/config", base_url))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), reqwest::StatusCode::OK);
    let json: Value = res.json().await.unwrap();
    assert_eq!(json["src_path"], src_dir.to_str().unwrap());

    // 2. GET /api/fs/browse
    let res = client
        .get(format!("{}/api/fs/browse", base_url))
        .query(&[("path", src_dir.to_str().unwrap())])
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), reqwest::StatusCode::OK);
    let json: Value = res.json().await.unwrap();
    let dirs = json["directories"].as_array().unwrap();
    let dir_names: Vec<&str> = dirs.iter().map(|d| d.as_str().unwrap()).collect();
    assert!(dir_names.contains(&"AlbumA"));
    assert!(dir_names.contains(&"AlbumB"));
    assert!(!dir_names.contains(&"ignore_me.txt"));

    // 3. POST /api/config/src-path (invalid & valid)
    let res = client
        .post(format!("{}/api/config/src-path", base_url))
        .json(&serde_json::json!({ "src_path": "/nonexistent/random/directory" }))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), reqwest::StatusCode::BAD_REQUEST);

    let new_src = temp_root.path().join("new_src");
    fs::create_dir_all(&new_src).unwrap();
    let res = client
        .post(format!("{}/api/config/src-path", base_url))
        .json(&serde_json::json!({ "src_path": new_src.to_str().unwrap() }))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), reqwest::StatusCode::OK);

    // Verify /api/config updated
    let res = client
        .get(format!("{}/api/config", base_url))
        .send()
        .await
        .unwrap();
    let json: Value = res.json().await.unwrap();
    assert_eq!(json["src_path"], new_src.to_str().unwrap());

    // 4. GET /api/converted (pagination & search)
    let res = client
        .get(format!("{}/api/converted?limit=2&offset=0", base_url))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), reqwest::StatusCode::OK);
    let json: Value = res.json().await.unwrap();
    assert_eq!(json["total"], 3);
    assert_eq!(json["items"].as_array().unwrap().len(), 2);

    let res = client
        .get(format!("{}/api/converted?search=other", base_url))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), reqwest::StatusCode::OK);
    let json: Value = res.json().await.unwrap();
    assert_eq!(json["total"], 1);
    assert_eq!(json["items"][0]["music"], "/music/other/track.flac");

    // 5. POST /api/converted/reconvert
    assert!(db.is_processed("/music/artist/song1.flac").unwrap());
    let res = client
        .post(format!("{}/api/converted/reconvert", base_url))
        .json(&serde_json::json!({ "music": "/music/artist/song1.flac" }))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), reqwest::StatusCode::OK);
    assert!(!db.is_processed("/music/artist/song1.flac").unwrap());

    // 6. POST /api/scan
    let res = client
        .post(format!("{}/api/scan", base_url))
        .json(&serde_json::json!({}))
        .send()
        .await
        .unwrap();
    assert_eq!(res.status(), reqwest::StatusCode::OK);
    let json: Value = res.json().await.unwrap();
    assert_eq!(json["success"], true);
}
