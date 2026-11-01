use thiserror::Error;

#[derive(Error, Debug)]
pub enum ProviderError {
    #[error("Network error: {0}")]
    Network(#[from] reqwest::Error),

    #[error("Serialization error: {0}")]
    Serialization(#[from] serde_json::Error),

    #[error("Provider {0} error: {1}")]
    Custom(String, String),

    #[error("Rate limited")]
    RateLimited,

    #[error("Provider disabled or unconfigured: {0}")]
    Unconfigured(String),
}
