use regex::Regex;
use std::path::{Path, PathBuf};
use std::sync::LazyLock;

static RE_DOS_8_3: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)^[a-z0-9]{5,8}~[a-z0-9]$").unwrap());

pub fn sanitize_filename(filename: &str) -> String {
    let mut cleaned: String = filename
        .chars()
        .filter(|c| !c.is_control() && *c != '\u{200B}' && *c != '\u{FEFF}')
        .map(|c| match c {
            '/' | '\\' | ':' | '*' | '?' | '"' | '<' | '>' | '|' => '-',
            _ => c,
        })
        .collect();

    // Collapse multiple whitespace
    let words: Vec<&str> = cleaned.split_whitespace().collect();
    cleaned = words.join(" ");

    // Trim whitespace and dots (SMB / Windows compatibility)
    let trimmed = cleaned.trim_matches(|c| c == ' ' || c == '.');
    if trimmed.is_empty() {
        "Unknown".to_string()
    } else {
        trimmed.to_string()
    }
}

pub fn is_suspicious_8_3_name(name: &str) -> bool {
    let base = Path::new(name)
        .file_stem()
        .and_then(|s| s.to_str())
        .unwrap_or(name);
    RE_DOS_8_3.is_match(base.trim())
}

pub fn get_unique_path(path: &Path) -> PathBuf {
    if !path.exists() {
        return path.to_path_buf();
    }
    let parent = path.parent().unwrap_or_else(|| Path::new(""));
    let stem = path.file_stem().and_then(|s| s.to_str()).unwrap_or("file");
    let ext = path.extension().and_then(|s| s.to_str()).unwrap_or("");

    let mut counter = 1;
    loop {
        let new_file_name = if ext.is_empty() {
            format!("{} ({})", stem, counter)
        } else {
            format!("{} ({}).{}", stem, counter, ext)
        };
        let candidate = parent.join(new_file_name);
        if !candidate.exists() {
            return candidate;
        }
        counter += 1;
    }
}

pub fn fold_string_for_compare(s: &str) -> String {
    s.to_lowercase()
        .chars()
        .map(|c| match c {
            'á' | 'à' | 'â' | 'ä' | 'ã' | 'å' | 'ā' | 'ă' | 'ą' => 'a',
            'ç' | 'ć' | 'č' => 'c',
            'é' | 'è' | 'ê' | 'ë' | 'ē' | 'ė' | 'ę' => 'e',
            'í' | 'ì' | 'î' | 'ï' | 'ī' => 'i',
            'ñ' | 'ń' => 'n',
            'ó' | 'ò' | 'ô' | 'ö' | 'õ' | 'ø' | 'ō' => 'o',
            'ú' | 'ù' | 'û' | 'ü' | 'ū' => 'u',
            'ý' | 'ÿ' => 'y',
            'š' => 's',
            'ž' => 'z',
            'ř' => 'r',
            'ł' => 'l',
            other => other,
        })
        .collect()
}

pub fn resolve_existing_path<P: AsRef<Path>>(path: P) -> PathBuf {
    let p = path.as_ref();
    if p.exists() {
        return p.to_path_buf();
    }

    let mut current = PathBuf::new();
    for comp in p.components() {
        match comp {
            std::path::Component::RootDir => {
                current.push(std::path::MAIN_SEPARATOR.to_string());
            }
            std::path::Component::Prefix(prefix) => {
                current.push(prefix.as_os_str());
            }
            std::path::Component::CurDir => {}
            std::path::Component::ParentDir => {
                current.pop();
            }
            std::path::Component::Normal(os_name) => {
                let name = os_name.to_string_lossy();
                let next = current.join(&*name);
                if next.exists() {
                    current = next;
                    continue;
                }

                if let Ok(entries) = std::fs::read_dir(&current) {
                    let want = fold_string_for_compare(&name);
                    let mut matched = None;
                    for entry in entries.flatten() {
                        let entry_name = entry.file_name().to_string_lossy().to_string();
                        if fold_string_for_compare(&entry_name) == want {
                            matched = Some(entry_name);
                            break;
                        }
                    }
                    if let Some(m) = matched {
                        current = current.join(m);
                    } else {
                        current = current.join(&*name);
                    }
                } else {
                    current = current.join(&*name);
                }
            }
        }
    }
    current
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_sanitize_filename() {
        assert_eq!(sanitize_filename("Mono Inc.  "), "Mono Inc");
        assert_eq!(
            sanitize_filename("Artist / Track : Name?"),
            "Artist - Track - Name-"
        );
        assert_eq!(sanitize_filename("   "), "Unknown");
        assert_eq!(sanitize_filename("Valid.Title..."), "Valid.Title");
    }

    #[test]
    fn test_is_suspicious_8_3_name() {
        assert!(is_suspicious_8_3_name("MBF14M~I"));
        assert!(is_suspicious_8_3_name("TRACK0~1.FLAC"));
        assert!(!is_suspicious_8_3_name("Normal Title"));
    }
}
