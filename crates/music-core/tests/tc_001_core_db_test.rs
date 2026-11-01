use music_core::{
    is_suspicious_8_3_name, normalize_chinese_for_compare, normalize_classical_no,
    remove_discogs_suffix, sanitize_filename, to_simplified_chinese, Config, Database,
};
use tempfile::tempdir;

#[test]
fn test_tc_001_core_db_spec() {
    // Step 1: Config Parsing & Default Fallbacks
    let yaml = r#"
src_path: "/volume1/music_raw"
output_root: "/volume1/music"
desc_type: "FLAC"
is_split: 1
is_del_src: 0
spotify_enabled: false
spotify_premium: true
netease_cookie: "MUSIC_U=fake_cookie;"
track_overrides:
  "Various Artists|Classic 100":
    - "Track 01"
    - "Track 02"
"#;
    let cfg: Config = serde_yaml::from_str(yaml).expect("Failed to parse YAML");
    assert_eq!(cfg.src_path, "/volume1/music_raw");
    assert_eq!(cfg.output_root, "/volume1/music");
    assert!(cfg.is_split);
    assert!(!cfg.is_del_src);
    assert!(cfg.is_spotify_enabled(), "spotify_premium should activate spotify");
    assert_eq!(cfg.track_overrides.len(), 1);

    // Step 2: SQLite Persistence & Idempotency
    let temp = tempdir().expect("Failed to create tempdir");
    let db_path = temp.path().join("musicConvert.db");
    let db = Database::open(&db_path).expect("Failed to open DB");

    let test_file = "/volume1/music_raw/Artist - Album/01. Track.flac";
    assert!(!db.is_processed(test_file).expect("is_processed check failed"));

    db.insert(test_file, 1).expect("Failed to insert record");
    assert!(db.is_processed(test_file).expect("is_processed check failed"));

    // Duplicate insertion must return Err
    assert!(db.insert(test_file, 1).is_err(), "Duplicate insertion must fail");

    let queried = db.query(test_file).expect("Query failed");
    assert_eq!(queried.len(), 1);
    assert_eq!(queried[0].music, test_file);
    assert_eq!(queried[0].state, 1);

    // Step 3: Chinese Simplification, Classical Punctuation & SMB Path Sanitization
    assert_eq!(to_simplified_chinese("遙遠的她"), "遥远的她");
    assert_eq!(normalize_chinese_for_compare(" 遙遠 的 她 \u{200B} "), "遥远的她");
    assert_eq!(normalize_classical_no("Beethoven: Symphony N° 9"), "Beethoven: Symphony No. 9");
    assert_eq!(normalize_classical_no("Mozart: Concerto № 21"), "Mozart: Concerto No. 21");
    assert_eq!(remove_discogs_suffix("The Artist (2)"), "The Artist");
    assert_eq!(sanitize_filename("Mono Inc.  "), "Mono Inc");
    assert_eq!(sanitize_filename("Valid/Slash:Colon*Star"), "Valid-Slash-Colon-Star");
    assert!(is_suspicious_8_3_name("MBF14M~I"));
    assert!(!is_suspicious_8_3_name("Mono Inc"));
}
