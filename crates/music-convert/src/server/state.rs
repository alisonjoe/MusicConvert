use crate::engine::TaskRegistry;
use music_core::Config;
use std::sync::Arc;
use tokio::sync::RwLock;

#[derive(Clone)]
pub struct AppState {
    pub registry: Arc<RwLock<TaskRegistry>>,
    pub config: Arc<Config>,
}

impl AppState {
    pub fn new(registry: Arc<RwLock<TaskRegistry>>, config: Arc<Config>) -> Self {
        Self { registry, config }
    }
}
