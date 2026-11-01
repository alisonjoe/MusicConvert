use std::path::{Path, PathBuf};
use music_core::sanitize_filename;

pub fn resolve_album_output_dir<P: AsRef<Path>>(
    output_root: P,
    artist: &str,
    album: &str,
    year: Option<&str>,
) -> PathBuf {
    let clean_artist = sanitize_filename(artist);
    let mut clean_album = sanitize_filename(album);

    if let Some(y) = year {
        let clean_y = y.trim();
        if clean_y.len() >= 4 {
            let y_sub = &clean_y[..4];
            let year_tag = format!("[{}]", y_sub);
            if !clean_album.contains(&year_tag) {
                clean_album = format!("{} {}", clean_album, year_tag);
            }
        }
    }

    output_root.as_ref().join(clean_artist).join(clean_album)
}

pub fn resolve_disc_output_dir<P: AsRef<Path>>(
    album_dir: P,
    disc_num: u32,
    disc_total: u32,
) -> PathBuf {
    if disc_total > 1 {
        let disc_folder = format!("CD{}", if disc_num < 1 { 1 } else { disc_num });
        album_dir.as_ref().join(disc_folder)
    } else {
        album_dir.as_ref().to_path_buf()
    }
}

pub fn build_track_filename(track_num: u32, title: &str, ext: &str) -> String {
    let clean_title = sanitize_filename(title);
    let clean_ext = ext.trim_start_matches('.').to_lowercase();
    format!("{:02}. {}.{}", track_num, clean_title, clean_ext)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_path_resolution_multi_disc() {
        let root = Path::new("/music");
        let album_dir = resolve_album_output_dir(root, "Mono Inc. ", "Darkness", Some("2020"));
        assert_eq!(album_dir, PathBuf::from("/music/Mono Inc/Darkness [2020]"));

        let cd1 = resolve_disc_output_dir(&album_dir, 1, 2);
        assert_eq!(cd1, PathBuf::from("/music/Mono Inc/Darkness [2020]/CD1"));

        let cd2 = resolve_disc_output_dir(&album_dir, 2, 2);
        assert_eq!(cd2, PathBuf::from("/music/Mono Inc/Darkness [2020]/CD2"));

        let single = resolve_disc_output_dir(&album_dir, 1, 1);
        assert_eq!(single, album_dir);
    }

    #[test]
    fn test_build_track_filename() {
        assert_eq!(build_track_filename(1, "吻别", "flac"), "01. 吻别.flac");
        assert_eq!(build_track_filename(12, "Track/Title:?*", "FLAC"), "12. Track-Title---.flac");
    }
}
