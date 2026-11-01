use crate::error::AudioError;
use std::collections::HashSet;
use std::fs;
use std::path::Path;
use tracing::info;

pub fn sync_missing_tracks<P: AsRef<Path>>(
    album_dir: P,
    total_tracks: usize,
    track_titles: &[String],
    present_tracks: &HashSet<u32>,
    source: &str,
) -> Result<(), AudioError> {
    let report_path = album_dir.as_ref().join("missing_tracks.txt");

    let mut missing = 0;
    let mut missing_lines = Vec::new();

    for i in 1..=total_tracks {
        let t_num = i as u32;
        if !present_tracks.contains(&t_num) {
            missing += 1;
            let title = track_titles
                .get(i - 1)
                .map(|s| s.trim())
                .filter(|s| !s.is_empty())
                .unwrap_or("<unknown>");
            missing_lines.push(format!("{:02}. {}", i, title));
        }
    }

    if missing == 0 {
        if report_path.exists() {
            let _ = fs::remove_file(&report_path);
            info!(
                "[Missing Tracks] Removed missing_tracks.txt for complete album at {:?}",
                album_dir.as_ref()
            );
        }
        return Ok(());
    }

    let mut content = Vec::new();
    content.push(format!("Total tracks: {}", total_tracks));
    content.push(format!("Present tracks: {}", present_tracks.len()));
    content.push(format!("Track title source: {}", source));
    content.push("".to_string());
    content.push("Missing tracks:".to_string());
    content.extend(missing_lines);
    content.push("".to_string());

    fs::write(&report_path, content.join("\n"))?;
    info!(
        "[Missing Tracks] Updated {:?} ({} missing, source: {})",
        report_path, missing, source
    );
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use tempfile::tempdir;

    #[test]
    fn test_sync_missing_tracks_file() {
        let temp = tempdir().unwrap();
        let album_dir = temp.path();

        let total = 3;
        let titles = vec![
            "Track 1".to_string(),
            "Track 2".to_string(),
            "Track 3".to_string(),
        ];
        let mut present = HashSet::new();
        present.insert(1);
        present.insert(3); // Track 2 is missing

        sync_missing_tracks(album_dir, total, &titles, &present, "TestOnline").unwrap();

        let report_file = album_dir.join("missing_tracks.txt");
        assert!(report_file.exists());
        let content = fs::read_to_string(&report_file).unwrap();
        assert!(content.contains("Total tracks: 3"));
        assert!(content.contains("Present tracks: 2"));
        assert!(content.contains("02. Track 2"));

        // Now track 2 is added
        present.insert(2);
        sync_missing_tracks(album_dir, total, &titles, &present, "TestOnline").unwrap();
        assert!(
            !report_file.exists(),
            "Complete album must have missing_tracks.txt removed"
        );
    }
}
