use crate::engine::{ConvertOrchestrator, TaskRegistry};
use music_core::{Config, Database};
use std::path::PathBuf;
use std::sync::Arc;
use tokio::sync::RwLock;

#[derive(Clone)]
pub struct AppState {
    pub registry: Arc<RwLock<TaskRegistry>>,
    pub config: Arc<RwLock<Config>>,
    pub db: Arc<Database>,
    pub orchestrator: Option<Arc<ConvertOrchestrator>>,
    pub current_src_path: Arc<RwLock<PathBuf>>,
    pub config_file_path: Option<PathBuf>,
}

impl AppState {
    pub fn new(
        registry: Arc<RwLock<TaskRegistry>>,
        config: Arc<RwLock<Config>>,
        db: Arc<Database>,
        orchestrator: Option<Arc<ConvertOrchestrator>>,
        config_file_path: Option<PathBuf>,
    ) -> Self {
        let current_path = {
            let cfg = config.try_read().map(|c| PathBuf::from(&c.src_path)).unwrap_or_default();
            cfg
        };
        let current_src_path = Arc::new(RwLock::new(current_path));
        Self {
            registry,
            config,
            db,
            orchestrator,
            current_src_path,
            config_file_path,
        }
    }

    pub fn with_defaults(registry: Arc<RwLock<TaskRegistry>>, config: Arc<Config>) -> Self {
        let db = Arc::new(Database::open_in_memory().unwrap());
        let rw_cfg = Arc::new(RwLock::new((*config).clone()));
        Self::new(registry, rw_cfg, db, None, None)
    }
}
