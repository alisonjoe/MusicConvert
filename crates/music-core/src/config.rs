use serde::{Deserialize, Deserializer, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::Path;

fn deserialize_bool_or_int<'de, D>(deserializer: D) -> Result<bool, D::Error>
where
    D: Deserializer<'de>,
{
    #[derive(Deserialize)]
    #[serde(untagged)]
    enum BoolOrInt {
        Bool(bool),
        Int(i64),
        String(String),
    }

    match BoolOrInt::deserialize(deserializer)? {
        BoolOrInt::Bool(b) => Ok(b),
        BoolOrInt::Int(i) => Ok(i != 0),
        BoolOrInt::String(s) => {
            let lower = s.trim().to_lowercase();
            Ok(lower == "true" || lower == "1" || lower == "yes" || lower == "on")
        }
    }
}

fn default_desc_type() -> String {
    "FLAC".to_string()
}

fn default_netease_device() -> String {
    r#"{"deviceType":"andrwear","os":"otos","appVer":"0.1","channel":"hm","model":"kys","deviceId":"musicconvert001","brand":"hm","osVer":"8.1.0"}"#.to_string()
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Config {
    #[serde(default, deserialize_with = "deserialize_bool_or_int")]
    pub is_split: bool,

    #[serde(default, deserialize_with = "deserialize_bool_or_int")]
    pub is_del_src: bool,

    #[serde(default)]
    pub src_path: String,

    #[serde(default = "default_desc_type")]
    pub desc_type: String,

    #[serde(default)]
    pub desc_path: String,

    #[serde(default)]
    pub output_root: String,

    #[serde(default)]
    pub spotify_client_id: String,

    #[serde(default)]
    pub spotify_client_secret: String,

    #[serde(default)]
    pub spotify_enabled: bool,

    #[serde(default)]
    pub spotify_premium: bool,

    #[serde(default)]
    pub discogs_token: String,

    #[serde(default)]
    pub netease_cookie: String,

    #[serde(default)]
    pub netease_auth_mode: String,

    #[serde(default)]
    pub netease_app_id: String,

    #[serde(default)]
    pub netease_app_secret: String,

    #[serde(default)]
    pub netease_private_key: String,

    #[serde(default)]
    pub netease_access_token: String,

    #[serde(default)]
    pub netease_refresh_token: String,

    #[serde(default = "default_netease_device")]
    pub netease_device: String,

    #[serde(default)]
    pub track_overrides: HashMap<String, Vec<String>>,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            is_split: false,
            is_del_src: false,
            src_path: "".to_string(),
            desc_type: default_desc_type(),
            desc_path: "".to_string(),
            output_root: "".to_string(),
            spotify_client_id: "".to_string(),
            spotify_client_secret: "".to_string(),
            spotify_enabled: false,
            spotify_premium: false,
            discogs_token: "".to_string(),
            netease_cookie: "".to_string(),
            netease_auth_mode: "".to_string(),
            netease_app_id: "".to_string(),
            netease_app_secret: "".to_string(),
            netease_private_key: "".to_string(),
            netease_access_token: "".to_string(),
            netease_refresh_token: "".to_string(),
            netease_device: default_netease_device(),
            track_overrides: HashMap::new(),
        }
    }
}

impl Config {
    pub fn load_from_file<P: AsRef<Path>>(
        path: P,
    ) -> Result<Self, Box<dyn std::error::Error + Send + Sync>> {
        let content = fs::read_to_string(path)?;
        let config: Config = serde_yaml::from_str(&content)?;
        Ok(config)
    }

    pub fn is_spotify_enabled(&self) -> bool {
        self.spotify_enabled || self.spotify_premium
    }

    pub fn get_discogs_token(&self) -> &str {
        self.discogs_token.trim()
    }

    pub fn is_discogs_enabled(&self) -> bool {
        !self.get_discogs_token().is_empty()
    }

    pub fn netease_auth_mode_normalized(&self) -> String {
        self.netease_auth_mode.trim().to_lowercase()
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_deserialize_config_defaults() {
        let yaml = r#"
src_path: "/music/src"
output_root: "/music/out"
is_split: 1
is_del_src: 0
spotify_enabled: true
"#;
        let cfg: Config = serde_yaml::from_str(yaml).unwrap();
        assert_eq!(cfg.src_path, "/music/src");
        assert_eq!(cfg.output_root, "/music/out");
        assert!(cfg.is_split);
        assert!(!cfg.is_del_src);
        assert_eq!(cfg.desc_type, "FLAC");
        assert!(cfg.is_spotify_enabled());
        assert!(!cfg.is_discogs_enabled());
    }
}
