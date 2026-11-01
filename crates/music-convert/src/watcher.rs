use std::path::{Path, PathBuf};
use std::sync::mpsc::channel;
use std::time::Duration;
use notify_debouncer_mini::{new_debouncer, DebouncedEvent};
use tracing::{error, info};

pub fn watch_directory<P: AsRef<Path>, F>(path: P, mut on_change: F)
where
    F: FnMut(Vec<PathBuf>) + Send + 'static,
{
    let target = path.as_ref().to_path_buf();
    info!("[Watcher] Starting debounced watcher on {:?}", target);

    let (tx, rx) = channel();

    let mut debouncer = match new_debouncer(Duration::from_secs(3), tx) {
        Ok(d) => d,
        Err(e) => {
            error!("[Watcher] Failed to create debouncer: {}", e);
            return;
        }
    };

    if let Err(e) = debouncer
        .watcher()
        .watch(&target, notify::RecursiveMode::Recursive)
    {
        error!("[Watcher] Failed to watch path {:?}: {}", target, e);
        return;
    }

    loop {
        match rx.recv() {
            Ok(Ok(events)) => {
                let paths: Vec<PathBuf> = events
                    .into_iter()
                    .map(|e: DebouncedEvent| e.path)
                    .collect();
                on_change(paths);
            }
            Ok(Err(err)) => {
                error!("[Watcher] Debounce error: {:?}", err);
            }
            Err(_) => {
                info!("[Watcher] Channel closed, stopping watcher.");
                break;
            }
        }
    }
}
