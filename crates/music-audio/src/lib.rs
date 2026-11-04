pub mod cue;
pub mod drivers;
pub mod error;
pub mod policy;

// 历史模块兼容映射
pub use cue as cue_parser;
pub use drivers as command_drivers;
pub use drivers::cli as command;
pub use drivers::ffmpeg as converter;
pub use drivers::ffprobe;
pub use drivers::metaflac;
pub use policy::missing_tracks;
pub use policy::path_resolver;

// 顶层平滑导出
pub use cue::*;
pub use drivers::*;
pub use error::AudioError;
pub use policy::*;
