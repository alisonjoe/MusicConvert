pub mod app;
pub mod engine;
pub mod monitor;
pub mod server;

// 历史模块兼容映射
pub use app::cli;
pub use engine::orchestrator;
pub use monitor::{scanner, watcher};
pub use server::create_router;

// 顶层导出
pub use app::cli::Cli;
pub use engine::orchestrator::ConvertOrchestrator;
pub use monitor::scanner::scan_release_batches;
pub use monitor::watcher::watch_directory;
