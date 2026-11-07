use music_convert::engine::{
    BatchTask, ConvertOrchestrator, EngineEvent, ReviewDecision, TaskRegistry, TaskStatus,
    TrackItem, TrackStatus,
};
use music_core::{Config, Database, MusicFileInfo, ReleaseBatch};
use music_provider::ProviderChain;
use std::sync::Arc;
use std::time::Duration;
use tempfile::tempdir;
use tokio::sync::RwLock;

#[test]
fn test_batch_and_track_task_modeling() {
    let mut task = BatchTask::new("t1", "/path/to/album", "Jay Chou", "Fantasy");
    assert_eq!(task.status, TaskStatus::Discovered);
    assert_eq!(task.tracks.len(), 0);

    task.add_track(TrackItem::new(1, "Love Before BC", "Jay Chou"));
    task.add_track(TrackItem::new(2, "Simple Love", "Jay Chou"));
    assert_eq!(task.tracks.len(), 2);

    task.set_status(TaskStatus::MatchingMetadata);
    assert_eq!(task.status, TaskStatus::MatchingMetadata);

    task.set_track_progress(1, 55.5, TrackStatus::Converting);
    assert_eq!(task.tracks[0].progress_percent, 55.5);
    assert_eq!(task.tracks[0].status, TrackStatus::Converting);

    // Test JSON Serialization / Deserialization
    let serialized = serde_json::to_string(&task).expect("must serialize");
    let deserialized: BatchTask = serde_json::from_str(&serialized).expect("must deserialize");
    assert_eq!(task, deserialized);
}

#[tokio::test]
async fn test_task_registry_lifecycle_and_ring_buffer() {
    let mut registry = TaskRegistry::new(3, 10);
    assert_eq!(registry.active_count(), 0);
    assert_eq!(registry.history_count(), 0);

    for i in 1..=4 {
        let tid = format!("task_{}", i);
        let task = BatchTask::new(&tid, format!("/music/{}", i), "Artist", "Album");
        registry.register_task(task);
        registry.complete_task(&tid);
    }

    // Active tasks must be 0
    assert_eq!(registry.active_count(), 0);
    // History buffer must be capped at 3
    assert_eq!(registry.history_count(), 3);

    // Oldest task (task_1) should have been evicted
    assert!(registry.get_task("task_1").is_none());
    assert!(registry.get_task("task_2").is_some());
    assert!(registry.get_task("task_3").is_some());
    assert!(registry.get_task("task_4").is_some());
}

#[tokio::test]
async fn test_event_bus_broadcast() {
    let mut registry = TaskRegistry::new(10, 20);
    let mut rx = registry.subscribe();

    let task = BatchTask::new("task_event_1", "/music/test", "Eason", "U87");
    registry.register_task(task);

    // 1. Should receive TaskDiscovered
    let ev1 = rx.recv().await.expect("must receive event 1");
    match ev1 {
        EngineEvent::TaskDiscovered(t) => {
            assert_eq!(t.task_id, "task_event_1");
            assert_eq!(t.artist, "Eason");
        }
        _ => panic!("Expected TaskDiscovered, got {:?}", ev1),
    }

    // 2. Status change
    registry.update_task_status("task_event_1", TaskStatus::Transcoding);
    let ev2 = rx.recv().await.expect("must receive event 2");
    match ev2 {
        EngineEvent::TaskStatusChanged { task_id, status } => {
            assert_eq!(task_id, "task_event_1");
            assert_eq!(status, TaskStatus::Transcoding);
        }
        _ => panic!("Expected TaskStatusChanged, got {:?}", ev2),
    }

    // 3. Complete task
    registry.complete_task("task_event_1");
    let ev3 = rx.recv().await.expect("must receive event 3");
    match ev3 {
        EngineEvent::TaskCompleted { task_id } => {
            assert_eq!(task_id, "task_event_1");
        }
        _ => panic!("Expected TaskCompleted, got {:?}", ev3),
    }
}

#[tokio::test]
async fn test_awaiting_review_and_approve_flow() {
    let mut registry = TaskRegistry::new(10, 20);
    let mut rx = registry.subscribe();

    let task = BatchTask::new("task_review", "/music/pinyin", "Unknown", "Unknown");
    registry.register_task(task);
    let _ = rx.recv().await; // consume TaskDiscovered

    let (tx, review_rx) = tokio::sync::oneshot::channel();
    registry.mark_awaiting_review("task_review", tx);

    let ev = rx.recv().await.expect("must receive AwaitingReview");
    match ev {
        EngineEvent::TaskAwaitingReview { task_id, task } => {
            assert_eq!(task_id, "task_review");
            assert_eq!(task.status, TaskStatus::AwaitingReview);
        }
        _ => panic!("Expected TaskAwaitingReview, got {:?}", ev),
    }

    // User approves with overrides
    let decision = ReviewDecision {
        override_artist: Some("周杰伦".into()),
        override_album: Some("范特西".into()),
        override_year: Some("2001".into()),
        override_tracks: None,
    };
    let approved = registry.approve_task("task_review", decision);
    assert!(approved);

    // Receiver should get the decision
    let recvd_decision = review_rx.await.expect("must receive approved decision");
    assert_eq!(recvd_decision.override_artist.as_deref(), Some("周杰伦"));
    assert_eq!(recvd_decision.override_album.as_deref(), Some("范特西"));

    // Task inside registry should also reflect overridden metadata
    let task_snapshot = registry.get_task("task_review").expect("task must exist");
    assert_eq!(task_snapshot.artist, "周杰伦");
    assert_eq!(task_snapshot.album, "范特西");
}

#[tokio::test]
async fn test_orchestrator_integration_with_review() {
    let dir = tempdir().expect("tempdir");
    let src_dir = dir.path().join("src");
    let out_dir = dir.path().join("out");
    std::fs::create_dir_all(&src_dir).unwrap();
    std::fs::create_dir_all(&out_dir).unwrap();

    let db_path = dir.path().join("test.db");
    let db = Arc::new(Database::open(&db_path).expect("db"));

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

    // Create a dummy source file
    let album_dir = src_dir.join("Unknown Artist").join("Unknown Album");
    std::fs::create_dir_all(&album_dir).unwrap();
    let audio_file = album_dir.join("track01.wav");
    std::fs::write(&audio_file, b"fake wav header and dummy pcm").unwrap();

    let batch = ReleaseBatch {
        release_context: album_dir.clone(),
        files: vec![MusicFileInfo {
            file_path: Some(audio_file.clone()),
            cue_path: None,
            file_suffix: "wav".to_string(),
        }],
    };

    let mut rx = registry.read().await.subscribe();
    let orch_clone = orchestrator.clone();
    let reg_clone = registry.clone();

    // Spawn orchestrator processing
    let handle = tokio::spawn(async move {
        orch_clone.process_batch(&batch).await;
    });

    // Watch events until AwaitingReview
    let target_task_id = loop {
        match tokio::time::timeout(Duration::from_secs(3), rx.recv()).await {
            Ok(Ok(EngineEvent::TaskAwaitingReview { task_id, .. })) => {
                break task_id;
            }
            Ok(Ok(_)) => {}
            Err(_) => panic!("Timed out waiting for TaskAwaitingReview"),
            Ok(Err(e)) => panic!("Channel error: {:?}", e),
        }
    };

    assert!(!target_task_id.is_empty());

    // Approve the task with override
    let approved = reg_clone.write().await.approve_task(
        &target_task_id,
        ReviewDecision {
            override_artist: Some("Manual Artist".into()),
            override_album: Some("Manual Album".into()),
            override_year: Some("2025".into()),
            override_tracks: None,
        },
    );
    assert!(approved);

    // Wait for orchestrator to finish
    let _ = tokio::time::timeout(Duration::from_secs(5), handle).await;

    // Check registry: task must be completed and stored in history
    let reg = reg_clone.read().await;
    assert_eq!(reg.active_count(), 0);
    assert_eq!(reg.history_count(), 1);
    let final_task = reg.get_task(&target_task_id).expect("must find task");
    assert_eq!(final_task.artist, "Manual Artist");
    assert_eq!(final_task.album, "Manual Album");
}
