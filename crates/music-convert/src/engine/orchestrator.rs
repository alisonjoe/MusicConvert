use music_audio::{
    build_track_filename, convert_single_track, parse_cue_file, probe_metadata,
    resolve_album_output_dir, resolve_disc_output_dir, split_cue_track, sync_missing_tracks,
};
use music_core::{Config, Database, MusicFileInfo, ReleaseBatch, TrackMetadata};
use music_provider::{MetadataQuery, ProviderChain};
use std::collections::HashSet;
use std::fs;
use std::path::Path;
use std::sync::Arc;
use tracing::{error, info};

pub struct ConvertOrchestrator {
    pub config: Config,
    pub db: Arc<Database>,
    pub provider_chain: Arc<ProviderChain>,
}

impl ConvertOrchestrator {
    pub fn new(config: Config, db: Arc<Database>, provider_chain: Arc<ProviderChain>) -> Self {
        Self {
            config,
            db,
            provider_chain,
        }
    }

    pub async fn process_batch(&self, batch: &ReleaseBatch) {
        let mut unprocessed_files = Vec::new();
        for file in &batch.files {
            let p = file.file_path.as_ref().or(file.cue_path.as_ref());
            if let Some(path) = p {
                let path_str = path.to_string_lossy();
                match self.db.is_processed(&path_str) {
                    Ok(true) => {}
                    _ => unprocessed_files.push(file.clone()),
                }
            }
        }

        if unprocessed_files.is_empty() {
            return;
        }

        info!(
            "[Orchestrator] Processing ReleaseBatch at {:?} (unprocessed: {})",
            batch.release_context,
            unprocessed_files.len()
        );

        // 1. Extract context clues: directory names or tags
        let album_dir_name = batch
            .release_context
            .file_name()
            .and_then(|s| s.to_str())
            .unwrap_or("Unknown Album");
        let parent_dir_name = batch
            .release_context
            .parent()
            .and_then(|p| p.file_name())
            .and_then(|s| s.to_str())
            .unwrap_or("Unknown Artist");

        let mut query_artist = parent_dir_name.to_string();
        let mut query_album = album_dir_name.to_string();

        // Try to read tags from the first available file or cue
        for f in &unprocessed_files {
            if let Some(cue) = &f.cue_path {
                if let Ok(sheet) = parse_cue_file(cue) {
                    if !sheet.performer.is_empty() {
                        query_artist = sheet.performer;
                    }
                    if !sheet.title.is_empty() {
                        query_album = sheet.title;
                    }
                    break;
                }
            } else if let Some(audio) = &f.file_path {
                if let Ok(probe) = probe_metadata(audio).await {
                    if let Some(a) = probe.format.get_tag("artist") {
                        if !a.is_empty() {
                            query_artist = a.to_string();
                        }
                    }
                    if let Some(al) = probe.format.get_tag("album") {
                        if !al.is_empty() {
                            query_album = al.to_string();
                        }
                    }
                    break;
                }
            }
        }

        // 2. Fetch ReleaseMetadata via ProviderChain
        let query = MetadataQuery::new(&query_artist, &query_album);
        let online_meta = self.provider_chain.fetch(&query).await.unwrap_or(None);

        let final_artist = online_meta
            .as_ref()
            .map(|m| m.album_artist.clone())
            .unwrap_or(query_artist);
        let final_album = online_meta
            .as_ref()
            .map(|m| m.album.clone())
            .unwrap_or(query_album);
        let final_year = online_meta.as_ref().and_then(|m| m.year.clone());

        let out_root = Path::new(&self.config.output_root);
        let album_out_dir =
            resolve_album_output_dir(out_root, &final_artist, &final_album, final_year.as_deref());

        if let Err(e) = fs::create_dir_all(&album_out_dir) {
            error!(
                "[Orchestrator] Failed to create output directory {:?}: {}",
                album_out_dir, e
            );
            return;
        }

        let mut present_tracks = HashSet::new();
        let mut all_track_titles = Vec::new();

        if let Some(meta) = &online_meta {
            for t in &meta.tracks {
                all_track_titles.push(t.title.clone());
            }
        }

        // 3. Convert files
        for file in &unprocessed_files {
            self.convert_file_item(
                file,
                &album_out_dir,
                &final_artist,
                &final_album,
                final_year.as_deref(),
                &online_meta,
                &mut present_tracks,
            )
            .await;
        }

        // 4. Update missing_tracks.txt if online metadata provided a tracklist
        if !all_track_titles.is_empty() {
            let source_label = online_meta
                .as_ref()
                .map(|m| m.source.as_str())
                .unwrap_or("Online Provider");
            let _ = sync_missing_tracks(
                &album_out_dir,
                all_track_titles.len(),
                &all_track_titles,
                &present_tracks,
                source_label,
            );
        }
    }

    #[allow(clippy::too_many_arguments)]
    async fn convert_file_item(
        &self,
        item: &MusicFileInfo,
        album_out_dir: &Path,
        artist: &str,
        album: &str,
        year: Option<&str>,
        online_meta: &Option<music_core::ReleaseMetadata>,
        present_tracks: &mut HashSet<u32>,
    ) {
        // Disc resolution: check if parent folder is CD1/CD2
        let file_path = match item.file_path.as_ref().or(item.cue_path.as_ref()) {
            Some(p) => p,
            None => return,
        };
        let parent_name = file_path
            .parent()
            .and_then(|p| p.file_name())
            .and_then(|s| s.to_str())
            .unwrap_or("");

        let disc_num = if parent_name.to_uppercase().starts_with("CD") {
            parent_name[2..].parse::<u32>().unwrap_or(1)
        } else {
            1
        };

        let disc_total = 1;
        let disc_out_dir = resolve_disc_output_dir(album_out_dir, disc_num, disc_total);
        let _ = fs::create_dir_all(&disc_out_dir);

        // Case A: Split CUE sheet
        if self.config.is_split {
            if let (Some(cue_path), Some(audio_path)) = (&item.cue_path, &item.file_path) {
                if let Ok(sheet) = parse_cue_file(cue_path) {
                    for track in sheet.tracks {
                        let title = if let Some(meta) = online_meta {
                            meta.tracks
                                .iter()
                                .find(|t| t.track_number == track.track_num)
                                .map(|t| t.title.as_str())
                                .unwrap_or(&track.title)
                        } else {
                            &track.title
                        };

                        let out_filename =
                            build_track_filename(track.track_num, title, &self.config.desc_type);
                        let dst_file = disc_out_dir.join(out_filename);

                        let track_meta = TrackMetadata {
                            track_number: track.track_num,
                            title: title.to_string(),
                            artist: artist.to_string(),
                            album: album.to_string(),
                            album_artist: Some(artist.to_string()),
                            year: year.map(|y| y.to_string()),
                            disc_number: Some(disc_num),
                            disc_total: Some(disc_total),
                            ..Default::default()
                        };

                        match split_cue_track(
                            audio_path,
                            &dst_file,
                            track.index01_seconds,
                            track.duration_seconds,
                            Some(&track_meta),
                            &self.config.desc_type,
                        )
                        .await
                        {
                            Ok(_) => {
                                present_tracks.insert(track.track_num);
                            }
                            Err(e) => {
                                error!(
                                    "[Orchestrator] Failed to split track {}: {}",
                                    track.track_num, e
                                );
                            }
                        }
                    }

                    // Mark CUE and source file as processed
                    let _ = self.db.insert(&cue_path.to_string_lossy(), 1);
                    let _ = self.db.insert(&audio_path.to_string_lossy(), 1);

                    if self.config.is_del_src {
                        let _ = fs::remove_file(cue_path);
                        let _ = fs::remove_file(audio_path);
                    }
                    return;
                }
            }
        }

        // Case B: Single file conversion
        if let Some(src_audio) = &item.file_path {
            let track_no = 1;
            let title = src_audio
                .file_stem()
                .and_then(|s| s.to_str())
                .unwrap_or("Track 01");

            let out_filename = build_track_filename(track_no, title, &self.config.desc_type);
            let dst_file = disc_out_dir.join(out_filename);

            let track_meta = TrackMetadata {
                track_number: track_no,
                title: title.to_string(),
                artist: artist.to_string(),
                album: album.to_string(),
                album_artist: Some(artist.to_string()),
                year: year.map(|y| y.to_string()),
                disc_number: Some(disc_num),
                disc_total: Some(disc_total),
                ..Default::default()
            };

            match convert_single_track(
                src_audio,
                &dst_file,
                Some(&track_meta),
                &self.config.desc_type,
            )
            .await
            {
                Ok(_) => {
                    present_tracks.insert(track_no);
                    let _ = self.db.insert(&src_audio.to_string_lossy(), 1);
                    if self.config.is_del_src {
                        let _ = fs::remove_file(src_audio);
                    }
                }
                Err(e) => {
                    error!(
                        "[Orchestrator] Failed to convert single file {:?}: {}",
                        src_audio, e
                    );
                }
            }
        }
    }
}
