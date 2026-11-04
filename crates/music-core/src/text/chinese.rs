use ferrous_opencc::config::BuiltinConfig;
use ferrous_opencc::OpenCC;
use regex::Regex;
use std::sync::LazyLock;

static OPENCC_T2S: LazyLock<Option<OpenCC>> =
    LazyLock::new(|| OpenCC::from_config(BuiltinConfig::T2s).ok());

static RE_NO_PREFIX: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"(?i)\bNo\.\s*([0-9]+)").unwrap());

static RE_DISCOGS_DISAMBIGUATION: LazyLock<Regex> =
    LazyLock::new(|| Regex::new(r"\s*\(\d+\)$").unwrap());

pub fn to_simplified_chinese(s: &str) -> String {
    if s.is_empty() {
        return s.to_string();
    }
    if let Some(converter) = OPENCC_T2S.as_ref() {
        converter.convert(s)
    } else {
        s.to_string()
    }
}

pub fn normalize_chinese_for_compare(s: &str) -> String {
    if s.is_empty() {
        return s.to_string();
    }
    let simplified = to_simplified_chinese(s);
    simplified
        .chars()
        .filter(|c| !c.is_whitespace() && !c.is_control() && *c != '\u{200B}' && *c != '\u{FEFF}')
        .collect()
}

pub fn normalize_classical_no(s: &str) -> String {
    if s.is_empty() {
        return s.to_string();
    }
    let res = s
        .replace("№", "No.")
        .replace("N°", "No.")
        .replace("n°", "No.");
    RE_NO_PREFIX.replace_all(&res, "No. $1").to_string()
}

pub fn remove_discogs_suffix(s: &str) -> String {
    RE_DISCOGS_DISAMBIGUATION.replace(s.trim(), "").to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_chinese_conversion() {
        let t = "遙遠的她";
        let s = to_simplified_chinese(t);
        assert_eq!(s, "遥远的她");
    }

    #[test]
    fn test_classical_no_normalization() {
        assert_eq!(normalize_classical_no("Symphony N° 5"), "Symphony No. 5");
        assert_eq!(normalize_classical_no("Concerto № 1"), "Concerto No. 1");
    }

    #[test]
    fn test_discogs_suffix_removal() {
        assert_eq!(remove_discogs_suffix("Indecision (2)"), "Indecision");
        assert_eq!(remove_discogs_suffix("Blink-182"), "Blink-182");
    }
}
