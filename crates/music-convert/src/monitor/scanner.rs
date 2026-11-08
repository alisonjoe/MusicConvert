use music_audio::parse_disc_number_from_name;
use music_core::{is_supported_extension, resolve_existing_path, MusicFileInfo, ReleaseBatch};
use std::collections::HashMap;
use std::path::{Path, PathBuf};
use walkdir::WalkDir;

pub fn is_disc_directory_name(name: &str) -> bool {
    parse_disc_number_from_name(name).is_some() || name.eq_ignore_ascii_case("tracks")
}

pub fn scan_release_batches<P: AsRef<Path>>(root: P) -> Vec<ReleaseBatch> {
    let resolved_root = resolve_existing_path(root);
    if !resolved_root.exists() {
        return Vec::new();
    }

    // 1. Collect all files with supported extensions, strictly ignoring system dirs like @eaDir
    let mut files = Vec::new();
    for entry in WalkDir::new(&resolved_root)
        .into_iter()
        .filter_entry(|e| {
            if e.depth() == 0 {
                return true;
            }
            let name = e.file_name().to_string_lossy();
            !name.starts_with('.') && !name.starts_with('@') && name != "@eaDir"
        })
        .flatten()
    {
        if entry.file_type().is_file() {
            let path = entry.path().to_path_buf();
            if let Some(ext) = path.extension().and_then(|s| s.to_str()) {
                if is_supported_extension(ext) {
                    files.push(path);
                }
            }
        }
    }

    // 2. Pair audio files with .cue files sharing the same stem in the same directory
    // Key: (parent_dir, uppercase_file_stem)
    let mut file_map: HashMap<(PathBuf, String), MusicFileInfo> = HashMap::new();

    for file in files {
        let parent = file.parent().unwrap_or_else(|| Path::new("")).to_path_buf();
        let stem = file
            .file_stem()
            .and_then(|s| s.to_str())
            .unwrap_or("")
            .to_uppercase();
        let ext = file
            .extension()
            .and_then(|s| s.to_str())
            .unwrap_or("")
            .to_uppercase();

        let entry = file_map.entry((parent, stem)).or_default();
        if ext == "CUE" {
            entry.cue_path = Some(file);
        } else {
            entry.file_path = Some(file);
            entry.file_suffix = ext;
        }
    }

    // 3. Group paired files into ReleaseBatch based on ReleaseContext (merging CD1/CD2)
    let mut batch_map: HashMap<PathBuf, Vec<MusicFileInfo>> = HashMap::new();

    for ((parent, _stem), info) in file_map {
        let dir_name = parent.file_name().and_then(|s| s.to_str()).unwrap_or("");

        let release_context = if is_disc_directory_name(dir_name) {
            parent
                .parent()
                .map(|p| p.to_path_buf())
                .unwrap_or_else(|| parent.clone())
        } else {
            parent
        };

        batch_map.entry(release_context).or_default().push(info);
    }

    batch_map
        .into_iter()
        .map(|(ctx, batch_files)| ReleaseBatch {
            release_context: ctx,
            files: batch_files,
        })
        .collect()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs::{create_dir_all, File};
    use tempfile::tempdir;

    #[test]
    fn test_scan_release_batches_merges_discs() {
        let temp = tempdir().unwrap();
        let root = temp.path();

        let album_dir = root.join("Artist").join("Album");
        let cd1 = album_dir.join("CD1");
        let cd2 = album_dir.join("CD2");
        create_dir_all(&cd1).unwrap();
        create_dir_all(&cd2).unwrap();

        File::create(cd1.join("01.flac")).unwrap();
        File::create(cd1.join("01.cue")).unwrap();
        File::create(cd2.join("01.flac")).unwrap();

        let batches = scan_release_batches(root);
        assert_eq!(
            batches.len(),
            1,
            "CD1 and CD2 must be merged into 1 ReleaseBatch"
        );
        assert_eq!(batches[0].release_context, album_dir);
        assert_eq!(batches[0].files.len(), 2);

        // Verify pairing
        let cd1_item = batches[0]
            .files
            .iter()
            .find(|f| f.file_path.as_ref().unwrap().parent() == Some(&cd1))
            .unwrap();
        assert!(
            cd1_item.cue_path.is_some(),
            "CUE file must be paired with audio file"
        );
    }
}
