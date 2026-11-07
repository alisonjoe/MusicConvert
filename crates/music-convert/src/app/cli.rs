use clap::Parser;
use std::path::PathBuf;

#[derive(Parser, Debug, Clone)]
#[command(
    name = "MusicConvert",
    version = "1.0.0",
    about = "Music format converter and metadata normalizer daemon"
)]
pub struct Cli {
    #[arg(
        short,
        long,
        default_value = "./config.yaml",
        help = "Path to config.yaml"
    )]
    pub config: PathBuf,

    #[arg(long, help = "Rescan and re-convert a specific directory prefix")]
    pub rescan: Option<String>,

    #[arg(
        long,
        help = "Reset all converted database records and re-convert everything"
    )]
    pub rescan_all: bool,

    #[arg(long, help = "Test Spotify connection and credentials")]
    pub test_spotify: bool,

    #[arg(long, help = "Bind address for HTTP REST & SSE API server (e.g. 0.0.0.0:8080)")]
    pub server: Option<String>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_cli_parsing_defaults() {
        let args = vec!["MusicConvert"];
        let cli = Cli::try_parse_from(args).unwrap();
        assert_eq!(cli.config, PathBuf::from("./config.yaml"));
        assert!(!cli.rescan_all);
        assert!(cli.rescan.is_none());
        assert!(cli.server.is_none());
    }

    #[test]
    fn test_cli_parsing_rescan() {
        let args = vec![
            "MusicConvert",
            "--rescan",
            "/volume1/music/Album",
            "--rescan-all",
            "--server",
            "127.0.0.1:8080",
        ];
        let cli = Cli::try_parse_from(args).unwrap();
        assert_eq!(cli.rescan, Some("/volume1/music/Album".to_string()));
        assert!(cli.rescan_all);
        assert_eq!(cli.server, Some("127.0.0.1:8080".to_string()));
    }
}
