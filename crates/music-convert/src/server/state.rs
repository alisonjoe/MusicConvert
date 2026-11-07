use crate::engine::{ConvertOrchestrator, TaskRegistry};
use music_core::{Config, Database};
use std::path::PathBuf;
use std::sync::Arc;
use tokio::sync::RwLock;

#[derive(Clone)]
pub struct AppState {
    pub registry: Arc<RwLock<TaskRegistry>>,
    pub config: Arc<Config>,
    pub db: Arc<Database>,
    pub orchestrator: Option<Arc<ConvertOrchestrator>>,
    pub current_src_path: Arc<RwLock<PathBuf>>,
}

impl AppState {
    pub fn new(
        registry: Arc<RwLock<TaskRegistry>>,
        config: Arc<Config>,
        db: Arc<Database>,
        orchestrator: Option<Arc<ConvertOrchestrator>>,
    ) -> Self {
        let current_src_path = Arc::new(RwLock::new(PathBuf::from(&config.src_path)));
        Self {
            registry,
            config,
            db,
            orchestrator,
            current_src_path,
        }
    }

    pub fn with_defaults(registry: Arc<RwLock<TaskRegistry>>, config: Arc<Config>) -> Self {
        let db = Arc::new(Database::open_in_memory().unwrap());
        Self::new(registry, config, db, None)
    }
}
