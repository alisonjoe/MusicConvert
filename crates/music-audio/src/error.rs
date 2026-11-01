use thiserror::Error;

#[derive(Error, Debug)]
pub enum AudioError {
    #[error("I/O error: {0}")]
    Io(#[from] std::io::Error),

    #[error("Command execution failed for {0}: {1}")]
    CommandFailed(String, String),

    #[error("CUE parse error: {0}")]
    CueParse(String),

    #[error("Metadata parse error: {0}")]
    Metadata(String),

    #[error("Timeout executing command: {0}")]
    Timeout(String),
}
