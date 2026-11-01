pub mod command;
pub mod converter;
pub mod cue;
pub mod error;
pub mod ffprobe;
pub mod metaflac;
pub mod missing_tracks;
pub mod path_resolver;

pub use command::run_command;
pub use converter::{convert_single_track, split_cue_track};
pub use cue::{parse_cue_content, parse_cue_file, CueSheet, CueTrack};
pub use error::AudioError;
pub use ffprobe::{probe_metadata, FfprobeFormat, FfprobeOutput};
pub use metaflac::apply_flac_tags;
pub use missing_tracks::sync_missing_tracks;
pub use path_resolver::{build_track_filename, resolve_album_output_dir, resolve_disc_output_dir};
