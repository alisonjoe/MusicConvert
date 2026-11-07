use crate::engine::registry::TaskRegistry;
use crate::engine::task::{BatchTask, TaskStatus, TrackItem, TrackStatus};
use chrono::Utc;
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
use tokio::sync::RwLock;
use tracing::{error, info, warn};

pub struct ConvertOrchestrator {
    pub config: Config,
    pub db: Arc<Database>,
    pub provider_chain: Arc<ProviderChain>,
    pub registry: Arc<RwLock<TaskRegistry>>,
}

impl ConvertOrchestrator {
    pub fn new(
        config: Config,
        db: Arc<Database>,
        provider_chain: Arc<ProviderChain>,
    ) -> Self {
        let registry = Arc::new(RwLock::new(TaskRegistry::new(50, 128)));
        Self::with_registry(config, db, provider_chain, registry)
    }

    pub fn with_registry(
        config: Config,
        db: Arc<Database>,
        provider_chain: Arc<ProviderChain>,
        registry: Arc<RwLock<TaskRegistry>>,
    ) -> Self {
        Self {
            config,
            db,
            provider_chain,
            registry,
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

        let task_id = format!("task_{}", Utc::now().timestamp_millis());

        info!(
            "[Orchestrator] Processing ReleaseBatch at {:?} (task_id: {}, unprocessed: {})",
            batch.release_context,
            task_id,
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

        let mut initial_tracks = Vec::new();

        // Try to read tags from the first available file or cue
        for f in &unprocessed_files {
            if let Some(cue) = &f.cue_path {
                if let Ok(sheet) = parse_cue_file(cue) {
                    if !sheet.performer.is_empty() {
                        query_artist = sheet.performer.clone();
                    }
                    if !sheet.title.is_empty() {
                        query_album = sheet.title.clone();
                    }
                    for t in sheet.tracks {
                        let artist = if t.performer.is_empty() {
                            query_artist.clone()
                        } else {
                            t.performer
                        };
                        initial_tracks.push(TrackItem::new(t.track_num, t.title, artist));
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

        if initial_tracks.is_empty() {
            for (idx, f) in unprocessed_files.iter().enumerate() {
                let title = f
                    .file_path
                    .as_ref()
                    .and_then(|p| p.file_stem())
                    .and_then(|s| s.to_str())
                    .unwrap_or("Unknown Track");
                initial_tracks.push(TrackItem::new(
                    (idx + 1) as u32,
                    title,
                    query_artist.clone(),
                ));
            }
        }

        // Register BatchTask into TaskRegistry
        let mut batch_task = BatchTask::new(
            &task_id,
            batch.release_context.to_string_lossy(),
            &query_artist,
            &query_album,
        );
        for t in initial_tracks {
            batch_task.add_track(t);
        }

        {
            let mut reg = self.registry.write().await;
            reg.register_task(batch_task);
            reg.update_task_status(&task_id, TaskStatus::MatchingMetadata);
        }

        // 2. Fetch ReleaseMetadata via ProviderChain
        let query = MetadataQuery::new(&query_artist, &query_album);
        let online_meta = self.provider_chain.fetch(&query).await.unwrap_or(None);

        let mut final_artist = online_meta
            .as_ref()
            .map(|m| m.album_artist.clone())
            .unwrap_or_else(|| query_artist.clone());
        let mut final_album = online_meta
            .as_ref()
            .map(|m| m.album.clone())
            .unwrap_or_else(|| query_album.clone());
        let mut final_year = online_meta.as_ref().and_then(|m| m.year.clone());

        // Confidence calculation
        let confidence = if online_meta.is_some() {
            0.95
        } else if query_artist == "Unknown Artist" || query_album == "Unknown Album" {
            0.40
        } else {
            0.75
        };

        // If confidence is low, suspend for human review (Mode A)
        if confidence < 0.60 {
            info!(
                "[Orchestrator] Task {} confidence is low ({:.2}), entering AwaitingReview",
                task_id, confidence
            );
            let (review_tx, review_rx) = tokio::sync::oneshot::channel();
            {
                let mut reg = self.registry.write().await;
                reg.mark_awaiting_review(&task_id, review_tx);
            }

            match review_rx.await {
                Ok(decision) => {
                    info!(
                        "[Orchestrator] Task {} approved with overrides: {:?}",
                        task_id, decision
                    );
                    if let Some(a) = decision.override_artist {
                        final_artist = a;
                    }
                    if let Some(al) = decision.override_album {
                        final_album = al;
                    }
                    if let Some(y) = decision.override_year {
                        final_year = Some(y);
                    }
                }
                Err(_) => {
                    warn!(
                        "[Orchestrator] Review channel dropped/cancelled for task {}",
                        task_id
                    );
                    let mut reg = self.registry.write().await;
                    reg.cancel_task(&task_id);
                    return;
                }
            }
        }

        // Transition to Transcoding
        {
            let mut reg = self.registry.write().await;
            reg.update_task_status(&task_id, TaskStatus::Transcoding);
        }

        let out_root = Path::new(&self.config.output_root);
        let album_out_dir =
            resolve_album_output_dir(out_root, &final_artist, &final_album, final_year.as_deref());

        if let Err(e) = fs::create_dir_all(&album_out_dir) {
            error!(
                "[Orchestrator] Failed to create output directory {:?}: {}",
                album_out_dir, e
            );
            let mut reg = self.registry.write().await;
            reg.fail_task(&task_id, format!("Failed to create directory: {}", e));
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
                &task_id,
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

        // Complete the task
        {
            let mut reg = self.registry.write().await;
            reg.complete_task(&task_id);
        }

        info!("[Orchestrator] Finished task {}", task_id);
    }

    #[allow(clippy::too_many_arguments)]
    async fn convert_file_item(
        &self,
        task_id: &str,
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

                        {
                            let mut reg = self.registry.write().await;
                            reg.update_track_progress(
                                task_id,
                                track.track_num,
                                0.0,
                                TrackStatus::Converting,
                            );
                        }

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
                                let mut reg = self.registry.write().await;
                                reg.update_track_progress(
                                    task_id,
                                    track.track_num,
                                    100.0,
                                    TrackStatus::Completed,
                                );
                            }
                            Err(e) => {
                                error!(
                                    "[Orchestrator] Failed to split track {}: {}",
                                    track.track_num, e
                                );
                                let mut reg = self.registry.write().await;
                                reg.update_track_progress(
                                    task_id,
                                    track.track_num,
                                    0.0,
                                    TrackStatus::Failed {
                                        error: e.to_string(),
                                    },
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

            {
                let mut reg = self.registry.write().await;
                reg.update_track_progress(task_id, track_no, 0.0, TrackStatus::Converting);
            }

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
                    let mut reg = self.registry.write().await;
                    reg.update_track_progress(task_id, track_no, 100.0, TrackStatus::Completed);
                }
                Err(e) => {
                    error!(
                        "[Orchestrator] Failed to convert single file {:?}: {}",
                        src_audio, e
                    );
                    let mut reg = self.registry.write().await;
                    reg.update_track_progress(
                        task_id,
                        track_no,
                        0.0,
                        TrackStatus::Failed {
                            error: e.to_string(),
                        },
                    );
                }
            }
        }
    }
}
