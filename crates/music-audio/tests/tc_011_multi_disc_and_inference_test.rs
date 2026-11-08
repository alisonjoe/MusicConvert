use music_audio::{
    build_track_filename, infer_single_track_metadata, parse_disc_number_from_name,
    parse_track_and_title_from_filename, resolve_disc_output_dir,
};
use std::path::Path;

#[test]
fn test_parse_disc_number_from_various_formats() {
    assert_eq!(parse_disc_number_from_name("CD 01"), Some(1));
    assert_eq!(parse_disc_number_from_name("CD 02"), Some(2));
    assert_eq!(parse_disc_number_from_name("CD1"), Some(1));
    assert_eq!(parse_disc_number_from_name("CD_02"), Some(2));
    assert_eq!(parse_disc_number_from_name("Disc 1"), Some(1));
    assert_eq!(parse_disc_number_from_name("DISC 02"), Some(2));
    assert_eq!(parse_disc_number_from_name("Disk 3"), Some(3));
    assert_eq!(parse_disc_number_from_name("Vol 1"), Some(1));
    assert_eq!(parse_disc_number_from_name("Volume 2"), Some(2));
    assert_eq!(parse_disc_number_from_name("Unknown Folder"), None);
}

#[test]
fn test_multi_disc_output_resolution() {
    let album_dir = Path::new("/output/Lady Gaga/The Fame [2008]");
    let cd1 = resolve_disc_output_dir(album_dir, 1, 2);
    let cd2 = resolve_disc_output_dir(album_dir, 2, 2);
    assert_eq!(cd1, album_dir.join("CD1"));
    assert_eq!(cd2, album_dir.join("CD2"));
}

#[test]
fn test_infer_track_with_artist_and_album_prefix() {
    let raw_name = "Lady Gaga - The Fame - 01 - Bad Romance";
    let (track_no, title) = parse_track_and_title_from_filename(raw_name);
    assert_eq!(track_no, Some(1));
    assert_eq!(title.as_deref(), Some("Bad Romance"));

    let inferred = infer_single_track_metadata(None, None, None, raw_name, 1, None);
    assert_eq!(inferred.track_num, 1);
    assert_eq!(inferred.title, "Bad Romance");

    let out_file = build_track_filename(inferred.track_num, &inferred.title, "flac");
    assert_eq!(out_file, "01. Bad Romance.flac");
}

#[test]
fn test_infer_chinese_track_with_artist_prefix_no_digits() {
    let raw_name = "薛之谦 - 认真的雪 (《妈妈像花儿一样》电视剧插曲)";
    let (track_no, title) = parse_track_and_title_from_filename(raw_name);
    assert_eq!(track_no, None);
    assert_eq!(
        title.as_deref(),
        Some("认真的雪 (《妈妈像花儿一样》电视剧插曲)")
    );

    let inferred = infer_single_track_metadata(None, None, None, raw_name, 5, None);
    assert_eq!(inferred.track_num, 5);
    assert_eq!(inferred.title, "认真的雪 (《妈妈像花儿一样》电视剧插曲)");
}

#[test]
fn test_parse_ffmpeg_stderr_metadata_tags() {
    let sample_stderr = r#"
Input #0, flac, from '/path/to/Bad Romance.flac':
  Metadata:
    ARTIST          : Lady Gaga
    TITLE           : Bad Romance
    ALBUM           : The Fame
    track           : 1
    disc            : 1
    TOTALDISCS      : 2
    DATE            : 2009-11-18
  Duration: 00:04:54.47, start: 0.000000, bitrate: 1062 kb/s
"#;
    let tags = music_audio::parse_ffmpeg_stderr_tags(sample_stderr);
    assert_eq!(tags.get("artist").map(|s| s.as_str()), Some("Lady Gaga"));
    assert_eq!(tags.get("title").map(|s| s.as_str()), Some("Bad Romance"));
    assert_eq!(tags.get("album").map(|s| s.as_str()), Some("The Fame"));
    assert_eq!(tags.get("track").map(|s| s.as_str()), Some("1"));
    assert_eq!(tags.get("disc").map(|s| s.as_str()), Some("1"));
    assert_eq!(tags.get("totaldiscs").map(|s| s.as_str()), Some("2"));
}

