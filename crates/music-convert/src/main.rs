use clap::Parser;
use music_convert::cli::Cli;
use music_convert::orchestrator::ConvertOrchestrator;
use music_convert::scanner::scan_release_batches;
use music_convert::watcher::watch_directory;
use music_core::{Config, Database};
use music_provider::ProviderChain;
use std::fs::create_dir_all;
use std::path::Path;
use std::sync::Arc;
use tracing::{info, warn};
use tracing_appender::rolling;
use tracing_subscriber::fmt;
use tracing_subscriber::layer::SubscriberExt;
use tracing_subscriber::util::SubscriberInitExt;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let cli = Cli::parse();

    // 1. Setup Logging (Console + logs/MusicConvert.log rolling by day)
    let _ = create_dir_all("logs");
    let file_appender = rolling::daily("logs", "MusicConvert.log");
    let (non_blocking, _guard) = tracing_appender::non_blocking(file_appender);

    tracing_subscriber::registry()
        .with(fmt::layer().with_writer(std::io::stdout))
        .with(fmt::layer().with_writer(non_blocking).with_ansi(false))
        .init();

    info!("══════════════════════════════════════════════════════════════");
    info!(" MusicConvert (Rust Edition) Starting");
    info!("══════════════════════════════════════════════════════════════");

    // 2. Load Configuration
    let config_path = &cli.config;
    let config = if config_path.exists() {
        info!("Loading configuration from {:?}", config_path);
        Config::load_from_file(config_path)?
    } else {
        warn!(
            "Config file not found at {:?}, using default settings",
            config_path
        );
        Config::default()
    };

    // 3. Initialize SQLite Database
    let db_path = Path::new("./musicConvert.db");
    let db = Arc::new(Database::open(db_path)?);

    // 4. Handle CLI rescan flags
    if cli.rescan_all {
        let deleted = db.delete_all()?;
        info!(
            "[CLI] --rescan-all executed: cleared {} records from DB",
            deleted
        );
    } else if let Some(prefix) = &cli.rescan {
        let deleted = db.delete_by_prefix(prefix)?;
        info!(
            "[CLI] --rescan executed: cleared {} records under prefix {:?}",
            deleted, prefix
        );
    }

    if cli.test_spotify {
        info!(
            "[CLI] --test-spotify: spotify_enabled = {}",
            config.is_spotify_enabled()
        );
        return Ok(());
    }

    // 5. Build Provider Chain & Orchestrator
    let provider_chain = Arc::new(ProviderChain::from_config(&config));
    let orchestrator = Arc::new(ConvertOrchestrator::new(
        config.clone(),
        db.clone(),
        provider_chain.clone(),
    ));

    // 6. Initial Directory Scan
    if !config.src_path.is_empty() {
        let src_path = Path::new(&config.src_path);
        info!("Running initial scan on {:?}", src_path);
        let batches = scan_release_batches(src_path);
        info!("Discovered {} ReleaseBatches", batches.len());
        for batch in batches {
            orchestrator.process_batch(&batch).await;
        }
    } else {
        warn!("config.src_path is empty, skipping initial scan.");
    }

    // 7. Watch for Filesystem Changes (Daemon Mode)
    if !config.src_path.is_empty() {
        let src_path = config.src_path.clone();
        let orch = orchestrator.clone();

        tokio::task::spawn_blocking(move || {
            let watch_path = src_path.clone();
            watch_directory(Path::new(&watch_path), move |paths| {
                info!("[Watcher] Detected {} changed paths", paths.len());
                let batches = scan_release_batches(Path::new(&src_path));
                let rt = tokio::runtime::Handle::current();
                let o = orch.clone();
                rt.spawn(async move {
                    for b in batches {
                        o.process_batch(&b).await;
                    }
                });
            });
        });
    }

    info!("MusicConvert daemon running. Press Ctrl+C to terminate.");
    tokio::signal::ctrl_c().await?;
    info!("Shutting down MusicConvert.");

    Ok(())
}
