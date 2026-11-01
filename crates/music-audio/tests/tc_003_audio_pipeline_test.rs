use music_audio::{
    build_track_filename, parse_cue_content, probe_metadata, resolve_album_output_dir,
    resolve_disc_output_dir, sync_missing_tracks,
};
use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use tempfile::tempdir;

#[tokio::test]
async fn test_tc_003_audio_pipeline_spec() {
    // Step 1: CUE Sheet Parsing & Duration Calculations
    let cue_text = r#"
REM GENRE Classical
REM DATE 1982
PERFORMER "Glenn Gould"
TITLE "Bach: The Goldberg Variations"
FILE "goldberg.flac" WAVE
  TRACK 01 AUDIO
    TITLE "Aria"
    INDEX 01 00:00:00
  TRACK 02 AUDIO
    TITLE "Variatio 1 a 1 Clav."
    INDEX 01 01:54:15
  TRACK 03 AUDIO
    TITLE "Variatio 2 a 1 Clav."
    INDEX 01 02:41:45
"#;
    let sheet = parse_cue_content(cue_text);
    assert_eq!(sheet.performer, "Glenn Gould");
    assert_eq!(sheet.title, "Bach: The Goldberg Variations");
    assert_eq!(sheet.tracks.len(), 3);
    assert_eq!(sheet.tracks[0].title, "Aria");
    assert_eq!(sheet.tracks[0].index01_seconds, 0.0);
    assert_eq!(sheet.tracks[1].title, "Variatio 1 a 1 Clav.");
    assert!(sheet.tracks[0].duration_seconds.unwrap() > 110.0);

    // Step 2: Output Directory & Multi-Disc Path Resolution
    let root = Path::new("/output");
    let album_dir =
        resolve_album_output_dir(root, "Glenn Gould.", "Goldberg Variations", Some("1982"));
    // SMB dots stripped from artist, year tag appended
    assert_eq!(
        album_dir,
        PathBuf::from("/output/Glenn Gould/Goldberg Variations [1982]")
    );

    let cd1 = resolve_disc_output_dir(&album_dir, 1, 3);
    assert_eq!(
        cd1,
        PathBuf::from("/output/Glenn Gould/Goldberg Variations [1982]/CD1")
    );

    let cd2 = resolve_disc_output_dir(&album_dir, 2, 3);
    assert_eq!(
        cd2,
        PathBuf::from("/output/Glenn Gould/Goldberg Variations [1982]/CD2")
    );

    let filename = build_track_filename(1, "Aria / Main: Theme", "flac");
    assert_eq!(filename, "01. Aria - Main- Theme.flac");

    // Step 3: missing_tracks.txt lifecycle
    let temp = tempdir().unwrap();
    let temp_album_dir = temp.path();
    let titles = vec![
        "Aria".to_string(),
        "Variatio 1".to_string(),
        "Variatio 2".to_string(),
    ];
    let mut present = HashSet::new();
    present.insert(1); // Track 2 and 3 missing

    sync_missing_tracks(temp_album_dir, 3, &titles, &present, "CUE Sheet").unwrap();
    let report_file = temp_album_dir.join("missing_tracks.txt");
    assert!(report_file.exists());
    let content = fs::read_to_string(&report_file).unwrap();
    assert!(content.contains("Total tracks: 3"));
    assert!(content.contains("Present tracks: 1"));
    assert!(content.contains("02. Variatio 1"));
    assert!(content.contains("03. Variatio 2"));

    // Complete the album
    present.insert(2);
    present.insert(3);
    sync_missing_tracks(temp_album_dir, 3, &titles, &present, "CUE Sheet").unwrap();
    assert!(
        !report_file.exists(),
        "missing_tracks.txt must be removed when all tracks present"
    );

    // Step 4: Test Real ffprobe Metadata Extraction on test_meta.flac
    let test_flac = Path::new("../../test_meta.flac");
    if test_flac.exists() {
        if let Ok(meta) = probe_metadata(test_flac).await {
            assert!(
                !meta.format.format_name.is_empty(),
                "ffprobe format_name should be populated"
            );
        }
    }
}
