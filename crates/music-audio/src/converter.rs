use crate::command::run_command;
use crate::error::AudioError;
use crate::metaflac::apply_flac_tags;
use music_core::TrackMetadata;
use std::path::Path;

pub async fn convert_single_track<P: AsRef<Path>, Q: AsRef<Path>>(
    src_file: P,
    dst_file: Q,
    metadata: Option<&TrackMetadata>,
    desc_type: &str,
) -> Result<(), AudioError> {
    let src_str = src_file.as_ref().to_string_lossy();
    let dst_str = dst_file.as_ref().to_string_lossy();

    let mut args: Vec<String> = vec!["-y".to_string(), "-i".to_string(), src_str.to_string()];

    if desc_type.eq_ignore_ascii_case("FLAC") {
        args.push("-c:a".to_string());
        args.push("flac".to_string());
    } else if desc_type.eq_ignore_ascii_case("MP3") {
        args.push("-c:a".to_string());
        args.push("libmp3lame".to_string());
        args.push("-q:a".to_string());
        args.push("0".to_string());
    }

    if let Some(meta) = metadata {
        args.push("-metadata".to_string());
        args.push(format!("title={}", meta.title));
        args.push("-metadata".to_string());
        args.push(format!("artist={}", meta.artist));
        args.push("-metadata".to_string());
        args.push(format!("album={}", meta.album));
        if let Some(aa) = &meta.album_artist {
            args.push("-metadata".to_string());
            args.push(format!("album_artist={}", aa));
        }
        if let Some(y) = &meta.year {
            args.push("-metadata".to_string());
            args.push(format!("date={}", y));
        }
        args.push("-metadata".to_string());
        args.push(format!("track={}", meta.track_number));
        if let Some(d) = meta.disc_number {
            args.push("-metadata".to_string());
            args.push(format!("disc={}", d));
        }
    }

    args.push(dst_str.to_string());

    let str_args: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
    run_command("ffmpeg", &str_args, 300).await?;

    // If target is FLAC, apply Swinsian tags using metaflac for complete player compatibility
    if desc_type.eq_ignore_ascii_case("FLAC") {
        if let Some(meta) = metadata {
            let _ = apply_flac_tags(
                dst_file.as_ref(),
                meta.track_number,
                1,
                meta.disc_number.unwrap_or(1),
                meta.disc_total.unwrap_or(1),
                meta.year.as_deref(),
            )
            .await;
        }
    }

    Ok(())
}

pub async fn split_cue_track<P: AsRef<Path>, Q: AsRef<Path>>(
    src_audio: P,
    dst_file: Q,
    start_seconds: f64,
    duration_seconds: Option<f64>,
    metadata: Option<&TrackMetadata>,
    desc_type: &str,
) -> Result<(), AudioError> {
    let src_str = src_audio.as_ref().to_string_lossy();
    let dst_str = dst_file.as_ref().to_string_lossy();

    let start_str = format!("{:.3}", start_seconds);

    let mut args: Vec<String> = vec!["-y".to_string(), "-ss".to_string(), start_str];

    if let Some(dur) = duration_seconds {
        args.push("-t".to_string());
        args.push(format!("{:.3}", dur));
    }

    args.push("-i".to_string());
    args.push(src_str.to_string());

    if desc_type.eq_ignore_ascii_case("FLAC") {
        args.push("-c:a".to_string());
        args.push("flac".to_string());
    } else if desc_type.eq_ignore_ascii_case("MP3") {
        args.push("-c:a".to_string());
        args.push("libmp3lame".to_string());
        args.push("-q:a".to_string());
        args.push("0".to_string());
    }

    if let Some(meta) = metadata {
        args.push("-metadata".to_string());
        args.push(format!("title={}", meta.title));
        args.push("-metadata".to_string());
        args.push(format!("artist={}", meta.artist));
        args.push("-metadata".to_string());
        args.push(format!("album={}", meta.album));
        if let Some(aa) = &meta.album_artist {
            args.push("-metadata".to_string());
            args.push(format!("album_artist={}", aa));
        }
        if let Some(y) = &meta.year {
            args.push("-metadata".to_string());
            args.push(format!("date={}", y));
        }
        args.push("-metadata".to_string());
        args.push(format!("track={}", meta.track_number));
        if let Some(d) = meta.disc_number {
            args.push("-metadata".to_string());
            args.push(format!("disc={}", d));
        }
    }

    args.push(dst_str.to_string());

    let str_args: Vec<&str> = args.iter().map(|s| s.as_str()).collect();
    run_command("ffmpeg", &str_args, 300).await?;

    if desc_type.eq_ignore_ascii_case("FLAC") {
        if let Some(meta) = metadata {
            let _ = apply_flac_tags(
                dst_file.as_ref(),
                meta.track_number,
                1,
                meta.disc_number.unwrap_or(1),
                meta.disc_total.unwrap_or(1),
                meta.year.as_deref(),
            )
            .await;
        }
    }

    Ok(())
}
