use crate::engine::registry::TaskRegistry;
use crate::engine::task::{BatchTask, TaskStatus, TrackItem, TrackStatus};
use chrono::Utc;
use music_audio::{
    build_track_filename, convert_single_track, infer_single_track_metadata, parse_cue_file,
    parse_disc_number_from_name, parse_track_and_title_from_filename, probe_metadata,
    resolve_album_output_dir, resolve_disc_output_dir, split_cue_track, sync_missing_tracks,
    title_matches,
};
use music_core::{Config, Database, MusicFileInfo, ReleaseBatch, TrackMetadata};
use music_provider::{MetadataQuery, ProviderChain};
use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use tokio::sync::RwLock;
use tracing::{error, info, warn};

struct InFlightGuard {
    in_flight: Arc<Mutex<HashSet<PathBuf>>>,
    path: PathBuf,
}

impl Drop for InFlightGuard {
    fn drop(&mut self) {
        if let Ok(mut set) = self.in_flight.lock() {
            set.remove(&self.path);
        }
    }
}

pub struct ConvertOrchestrator {
    pub config: Arc<RwLock<Config>>,
    pub db: Arc<Database>,
    pub provider_chain: Arc<ProviderChain>,
    pub registry: Arc<RwLock<TaskRegistry>>,
    pub in_flight: Arc<Mutex<HashSet<PathBuf>>>,
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
            config: Arc::new(RwLock::new(config)),
            db,
            provider_chain,
            registry,
            in_flight: Arc::new(Mutex::new(HashSet::new())),
        }
    }

    pub async fn update_config(&self, new_config: Config) {
        let mut cfg = self.config.write().await;
        *cfg = new_config;
    }

    pub async fn get_config(&self) -> Config {
        self.config.read().await.clone()
    }

    async fn log_task(&self, task_id: &str, stage: &str, level: &str, message: impl Into<String>) {
        let msg = message.into();
        info!("[Task:{}] [{}] {}", task_id, stage, msg);
        let mut reg = self.registry.write().await;
        reg.append_task_log(task_id, stage, level, &msg);
    }

    pub async fn process_batch(&self, batch: &ReleaseBatch) {
        self.process_batch_ext(batch, false).await;
    }

    pub async fn process_batch_ext(&self, batch: &ReleaseBatch, force: bool) {
        let release_context = batch.release_context.clone();

        // Concurrency deduplication: ensure single in-flight task per release batch
        let _guard = {
            let mut guard = match self.in_flight.lock() {
                Ok(g) => g,
                Err(p) => p.into_inner(),
            };
            if !guard.insert(release_context.clone()) {
                info!(
                    "[Orchestrator] ReleaseBatch at {:?} is already in-flight! Skipping duplicate processing.",
                    release_context
                );
                return;
            }
            InFlightGuard {
                in_flight: self.in_flight.clone(),
                path: release_context.clone(),
            }
        };

        let cfg = self.config.read().await.clone();
        let should_force = force || cfg.force_reconvert;

        let mut unprocessed_files = Vec::new();
        for file in &batch.files {
            let p = file.file_path.as_ref().or(file.cue_path.as_ref());
            if let Some(path) = p {
                let path_str = path.to_string_lossy();
                if should_force {
                    unprocessed_files.push(file.clone());
                } else {
                    match self.db.is_processed(&path_str) {
                        Ok(true) => {}
                        _ => unprocessed_files.push(file.clone()),
                    }
                }
            }
        }

        if unprocessed_files.is_empty() {
            return;
        }

        // Natural sort files by disc number then filename/track number
        unprocessed_files.sort_by(|a, b| {
            let p_a = a.file_path.as_ref().or(a.cue_path.as_ref());
            let p_b = b.file_path.as_ref().or(b.cue_path.as_ref());

            let disc_a = p_a
                .and_then(|p| p.parent())
                .and_then(|p| p.file_name())
                .and_then(|s| s.to_str())
                .and_then(parse_disc_number_from_name)
                .unwrap_or(1);
            let disc_b = p_b
                .and_then(|p| p.parent())
                .and_then(|p| p.file_name())
                .and_then(|s| s.to_str())
                .and_then(parse_disc_number_from_name)
                .unwrap_or(1);

            if disc_a != disc_b {
                return disc_a.cmp(&disc_b);
            }

            let fn_a = p_a
                .and_then(|p| p.file_name())
                .and_then(|s| s.to_str())
                .unwrap_or("");
            let fn_b = p_b
                .and_then(|p| p.file_name())
                .and_then(|s| s.to_str())
                .unwrap_or("");

            let (track_a, _) = parse_track_and_title_from_filename(fn_a);
            let (track_b, _) = parse_track_and_title_from_filename(fn_b);

            match (track_a, track_b) {
                (Some(ta), Some(tb)) if ta != tb => ta.cmp(&tb),
                _ => fn_a.cmp(fn_b),
            }
        });

        let task_id = format!("task_{}", Utc::now().timestamp_millis());

        info!(
            "[Orchestrator] Processing ReleaseBatch at {:?} (task_id: {}, unprocessed: {}, force: {})",
            batch.release_context,
            task_id,
            unprocessed_files.len(),
            should_force
        );

        if should_force {
            self.log_task(
                &task_id,
                "LocalProbe",
                "WARN",
                "⚠️ 触发强制重新转换模式 (Force Reconvert)，跳过 SQLite 已转换缓存",
            )
            .await;
        }

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
                let (tag_track, tag_title, tag_artist) = if let Some(audio) = &f.file_path {
                    if let Ok(probe) = probe_metadata(audio).await {
                        (
                            probe
                                .format
                                .get_tag("track")
                                .or_else(|| probe.format.get_tag("tracknumber"))
                                .map(|s| s.to_string()),
                            probe.format.get_tag("title").map(|s| s.to_string()),
                            probe.format.get_tag("artist").map(|s| s.to_string()),
                        )
                    } else {
                        (None, None, None)
                    }
                } else {
                    (None, None, None)
                };

                let fn_stem = f
                    .file_path
                    .as_ref()
                    .and_then(|p| p.file_stem())
                    .and_then(|s| s.to_str())
                    .unwrap_or("Unknown Track");

                let inferred = infer_single_track_metadata(
                    tag_track.as_deref(),
                    tag_title.as_deref(),
                    tag_artist.as_deref(),
                    fn_stem,
                    (idx + 1) as u32,
                    None,
                );

                initial_tracks.push(TrackItem::new(
                    inferred.track_num,
                    inferred.title,
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

        self.log_task(
            &task_id,
            "LocalProbe",
            "INFO",
            format!(
                "本地批次发现: {:?}, 提取候选: 艺术家='{}', 专辑='{}', 包含 {} 首曲目",
                batch.release_context, query_artist, query_album, unprocessed_files.len()
            ),
        )
        .await;

        // 2. Fetch ReleaseMetadata via ProviderChain
        self.log_task(
            &task_id,
            "MetadataQuery",
            "INFO",
            format!("启动元数据责任链检索: 艺术家='{}', 专辑='{}'", query_artist, query_album),
        )
        .await;

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

        // Standardize registry task metadata if online meta available
        if let Some(meta) = &online_meta {
            self.log_task(
                &task_id,
                "MetadataQuery",
                "INFO",
                format!(
                    "元数据命中成功 [来源: {}]: 官方艺术家='{}', 专辑='{}', 发行年份={:?}, 包含 {} 首曲目",
                    meta.source, meta.album_artist, meta.album, meta.year, meta.tracks.len()
                ),
            )
            .await;

            {
                let mut reg = self.registry.write().await;
                if let Some(task) = reg.get_task_mut(&task_id) {
                    task.artist = final_artist.clone();
                    task.album = final_album.clone();
                    for track in &mut task.tracks {
                        if let Some(m_track) = meta.tracks.iter().find(|t| t.track_number == track.track_number) {
                            track.title = m_track.title.clone();
                        } else if let Some(m_track) = meta.tracks.iter().find(|t| title_matches(&track.title, &t.title)) {
                            track.track_number = m_track.track_number;
                            track.title = m_track.title.clone();
                        }
                    }
                }
            }
        } else {
            self.log_task(
                &task_id,
                "MetadataQuery",
                "WARN",
                "在线元数据源未匹配到高置信度结果，将继续使用本地识别标签",
            )
            .await;
        }

        // Confidence calculation
        let confidence = if online_meta.is_some() {
            0.95
        } else if query_artist == "Unknown Artist" || query_album == "Unknown Album" {
            0.40
        } else {
            0.75
        };

        self.log_task(
            &task_id,
            "Resolve",
            "INFO",
            format!("元数据置信度评分: {:.2} (人工审核阈值: 0.60)", confidence),
        )
        .await;

        // If confidence is low, suspend for human review (Mode A)
        if confidence < 0.60 {
            self.log_task(
                &task_id,
                "Resolve",
                "WARN",
                format!("置信度低于阈值 ({:.2} < 0.60)，任务进入 [待审阅] 状态，等待控制台人工核验", confidence),
            )
            .await;

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
                    self.log_task(
                        &task_id,
                        "Resolve",
                        "INFO",
                        format!("人工审阅放行: 最终艺术家='{}', 专辑='{}', 年份={:?}", final_artist, final_album, final_year),
                    )
                    .await;
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

        let out_root = Path::new(&cfg.output_root);
        let album_out_dir =
            resolve_album_output_dir(out_root, &final_artist, &final_album, final_year.as_deref());

        self.log_task(
            &task_id,
            "Transcode",
            "INFO",
            format!("进入转码流程，确保输出目录: {:?}", album_out_dir),
        )
        .await;

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

        // Calculate overall disc_total across all files in batch
        let disc_total = unprocessed_files
            .iter()
            .map(|f| {
                let p = f.file_path.as_ref().or(f.cue_path.as_ref());
                p.and_then(|p| p.parent())
                    .and_then(|p| p.file_name())
                    .and_then(|s| s.to_str())
                    .and_then(parse_disc_number_from_name)
                    .unwrap_or(1)
            })
            .max()
            .unwrap_or(1);

        // 3. Convert files
        for (idx, file) in unprocessed_files.iter().enumerate() {
            self.convert_file_item(
                &cfg,
                &task_id,
                (idx + 1) as u32,
                file,
                &album_out_dir,
                &final_artist,
                &final_album,
                final_year.as_deref(),
                &online_meta,
                &mut present_tracks,
                disc_total,
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
            self.log_task(
                &task_id,
                "Finalize",
                "INFO",
                format!(
                    "音轨完整性校验 [来源: {}]: 本地处理 {} 首 / 官方总曲目 {} 首",
                    source_label,
                    present_tracks.len(),
                    all_track_titles.len()
                ),
            )
            .await;
        }

        self.log_task(
            &task_id,
            "Finalize",
            "INFO",
            format!("批次转换任务 {} 全部完成，已记录至 SQLite 本地数据库", task_id),
        )
        .await;

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
        cfg: &Config,
        task_id: &str,
        fallback_index: u32,
        item: &MusicFileInfo,
        album_out_dir: &Path,
        artist: &str,
        album: &str,
        year: Option<&str>,
        online_meta: &Option<music_core::ReleaseMetadata>,
        present_tracks: &mut HashSet<u32>,
        disc_total: u32,
    ) {
        // Disc resolution: check if parent folder is CD1/CD2/Disc 1/etc.
        let file_path = match item.file_path.as_ref().or(item.cue_path.as_ref()) {
            Some(p) => p,
            None => return,
        };
        let parent_name = file_path
            .parent()
            .and_then(|p| p.file_name())
            .and_then(|s| s.to_str())
            .unwrap_or("");

        let disc_num = parse_disc_number_from_name(parent_name).unwrap_or(1);
        let disc_out_dir = resolve_disc_output_dir(album_out_dir, disc_num, disc_total);
        let _ = fs::create_dir_all(&disc_out_dir);

        // Case A: Split CUE sheet
        if cfg.is_split {
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
                            build_track_filename(track.track_num, title, &cfg.desc_type);
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
                            &cfg.desc_type,
                        )
                        .await
                        {
                            Ok(_) => {
                                present_tracks.insert(track.track_num);
                                {
                                    let mut reg = self.registry.write().await;
                                    reg.update_track_progress(
                                        task_id,
                                        track.track_num,
                                        100.0,
                                        TrackStatus::Completed,
                                    );
                                }
                                self.log_task(
                                    task_id,
                                    "Tagging",
                                    "INFO",
                                    format!("CUE 分轨完成 #{:02} '{}'，注入标签与时间戳", track.track_num, title),
                                )
                                .await;
                            }
                            Err(e) => {
                                error!(
                                    "[Orchestrator] Failed to split track {}: {}",
                                    track.track_num, e
                                );
                                {
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
                                self.log_task(
                                    task_id,
                                    "Transcode",
                                    "ERROR",
                                    format!("CUE 分轨失败 #{:02} '{}': {}", track.track_num, title, e),
                                )
                                .await;
                            }
                        }
                    }

                    // Mark CUE and source file as processed
                    let _ = self.db.insert(&cue_path.to_string_lossy(), 1);
                    let _ = self.db.insert(&audio_path.to_string_lossy(), 1);

                    if cfg.is_del_src {
                        let _ = fs::remove_file(cue_path);
                        let _ = fs::remove_file(audio_path);
                    }
                    return;
                }
            }
        }

        // Case B: Single file conversion
        if let Some(src_audio) = &item.file_path {
            let (tag_track, tag_title, tag_artist) =
                if let Ok(probe) = probe_metadata(src_audio).await {
                    (
                        probe
                            .format
                            .get_tag("track")
                            .or_else(|| probe.format.get_tag("tracknumber"))
                            .map(|s| s.to_string()),
                        probe.format.get_tag("title").map(|s| s.to_string()),
                        probe.format.get_tag("artist").map(|s| s.to_string()),
                    )
                } else {
                    (None, None, None)
                };

            let fn_stem = src_audio
                .file_stem()
                .and_then(|s| s.to_str())
                .unwrap_or("Track 01");

            let inferred = infer_single_track_metadata(
                tag_track.as_deref(),
                tag_title.as_deref(),
                tag_artist.as_deref(),
                fn_stem,
                fallback_index,
                online_meta.as_ref(),
            );

            let track_no = inferred.track_num;
            let title = inferred.title;

            self.log_task(
                task_id,
                "Transcode",
                "INFO",
                format!("单曲音轨推断: '{:?}' -> 确定音轨号 #{:02}, 曲名 '{}'", fn_stem, track_no, title),
            )
            .await;

            let out_filename = build_track_filename(track_no, &title, &cfg.desc_type);
            let dst_file = disc_out_dir.join(out_filename);

            let track_meta = TrackMetadata {
                track_number: track_no,
                title: title.clone(),
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
                &cfg.desc_type,
            )
            .await
            {
                Ok(_) => {
                    present_tracks.insert(track_no);
                    let _ = self.db.insert(&src_audio.to_string_lossy(), 1);
                    if cfg.is_del_src {
                        let _ = fs::remove_file(src_audio);
                    }
                    {
                        let mut reg = self.registry.write().await;
                        reg.update_track_progress(task_id, track_no, 100.0, TrackStatus::Completed);
                    }
                    self.log_task(
                        task_id,
                        "Tagging",
                        "INFO",
                        format!("单曲完成 #{:02} '{}'，注入标签与封面", track_no, title),
                    )
                    .await;
                }
                Err(e) => {
                    error!(
                        "[Orchestrator] Failed to convert single file {:?}: {}",
                        src_audio, e
                    );
                    {
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
                    self.log_task(
                        task_id,
                        "Transcode",
                        "ERROR",
                        format!("单曲 #{:02} '{}' 转码失败: {}", track_no, title, e),
                    )
                    .await;
                }
            }
        }
    }
}
