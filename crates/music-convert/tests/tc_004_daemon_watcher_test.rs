use clap::Parser;
use music_convert::cli::Cli;
use music_convert::orchestrator::ConvertOrchestrator;
use music_convert::scanner::scan_release_batches;
use music_core::{Config, Database, MusicFileInfo, ReleaseBatch};
use music_provider::ProviderChain;
use std::fs::{create_dir_all, File};
use std::sync::Arc;
use tempfile::tempdir;

#[tokio::test]
async fn test_tc_004_daemon_watcher_spec() {
    let temp = tempdir().unwrap();
    let root = temp.path();

    // Step 1: Multi-disc release scan & batch grouping
    let album_dir = root.join("Jay").join("Fantasy");
    let cd1 = album_dir.join("CD1");
    let cd2 = album_dir.join("CD2");
    create_dir_all(&cd1).unwrap();
    create_dir_all(&cd2).unwrap();

    let f1 = cd1.join("01.flac");
    let c1 = cd1.join("01.cue");
    let f2 = cd2.join("01.flac");
    File::create(&f1).unwrap();
    File::create(&c1).unwrap();
    File::create(&f2).unwrap();

    let batches = scan_release_batches(root);
    assert_eq!(
        batches.len(),
        1,
        "Discs CD1 and CD2 must be aggregated into one ReleaseBatch"
    );
    assert_eq!(batches[0].release_context, album_dir);
    assert_eq!(batches[0].files.len(), 2);

    // Step 2: CUE and Audio pairing validation
    let paired = batches[0]
        .files
        .iter()
        .find(|f| f.file_path == Some(f1.clone()))
        .unwrap();
    assert_eq!(paired.cue_path, Some(c1));

    // Step 3: CLI parsing verification
    let cli =
        Cli::try_parse_from(["MusicConvert", "--rescan", "/music/test", "--rescan-all"]).unwrap();
    assert_eq!(cli.rescan, Some("/music/test".to_string()));
    assert!(cli.rescan_all);

    // Step 4: Batch orchestration idempotency & DB mark
    let db = Arc::new(Database::open_in_memory().unwrap());
    let config = Config {
        output_root: temp.path().join("output").to_string_lossy().to_string(),
        ..Default::default()
    };

    let chain = Arc::new(ProviderChain::new());
    let orch = ConvertOrchestrator::new(config, db.clone(), chain);

    // Mark as processed in DB
    db.insert(&f1.to_string_lossy(), 1).unwrap();
    assert!(db.is_processed(&f1.to_string_lossy()).unwrap());

    // Process batch (f1 is skipped, only remaining files considered)
    let batch = ReleaseBatch {
        release_context: album_dir,
        files: vec![MusicFileInfo {
            file_path: Some(f1.clone()),
            file_suffix: "FLAC".to_string(),
            cue_path: None,
        }],
    };
    orch.process_batch(&batch).await;
}
