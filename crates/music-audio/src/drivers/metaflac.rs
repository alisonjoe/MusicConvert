use super::cli::run_command;
use crate::error::AudioError;
use std::path::Path;

pub async fn apply_flac_tags<P: AsRef<Path>>(
    flac_file: P,
    track_num: u32,
    total_tracks: u32,
    disc_num: u32,
    disc_total: u32,
    year: Option<&str>,
) -> Result<(), AudioError> {
    let p_str = flac_file.as_ref().to_string_lossy();
    let d_num = if disc_num < 1 { 1 } else { disc_num };
    let d_total = if disc_total < 1 { 1 } else { disc_total };

    let set_track = format!("--set-tag=TRACKNUMBER={}", track_num);
    let set_track_total = format!("--set-tag=TRACKTOTAL={}", total_tracks);
    let set_disc = format!("--set-tag=DISCNUMBER={}", d_num);
    let set_disc_total = format!("--set-tag=DISCTOTAL={}", d_total);
    let set_track_lower = format!("--set-tag=track={}", track_num);
    let set_disc_lower = format!("--set-tag=disc={}", d_num);
    let set_total_tracks = format!("--set-tag=TOTALTRACKS={}", total_tracks);
    let set_total_discs = format!("--set-tag=TOTALDISCS={}", d_total);

    let mut args: Vec<&str> = vec![
        "--remove-tag=TRACKNUMBER",
        "--remove-tag=TRACKTOTAL",
        "--remove-tag=DISCNUMBER",
        "--remove-tag=DISCTOTAL",
        "--remove-tag=track",
        "--remove-tag=disc",
        "--remove-tag=TOTALTRACKS",
        "--remove-tag=TOTALDISCS",
        &set_track,
        &set_track_total,
        &set_disc,
        &set_disc_total,
        &set_track_lower,
        &set_total_tracks,
        &set_disc_lower,
        &set_total_discs,
    ];

    let year_tag = year.map(|y| format!("--set-tag=ORIGINALYEAR={}", y));
    if let Some(yt) = &year_tag {
        args.push(yt.as_str());
    }

    args.push(&p_str);

    let _ = run_command("metaflac", &args, 30).await?;
    Ok(())
}
