use std::process::Output;
use std::time::Duration;
use tokio::process::Command;
use tokio::time::timeout;
use tracing::debug;
use crate::error::AudioError;

pub async fn run_command(cmd: &str, args: &[&str], timeout_secs: u64) -> Result<String, AudioError> {
    debug!("[AsyncCommand] Running: {} {}", cmd, args.join(" "));

    let mut command = Command::new(cmd);
    command.args(args);

    let child = command.output();
    let res: Result<std::io::Result<Output>, _> = timeout(Duration::from_secs(timeout_secs), child).await;

    match res {
        Ok(Ok(output)) => {
            if output.status.success() {
                let stdout = String::from_utf8_lossy(&output.stdout).to_string();
                Ok(stdout)
            } else {
                let stderr = String::from_utf8_lossy(&output.stderr).to_string();
                let stdout = String::from_utf8_lossy(&output.stdout).to_string();
                let combined = if stderr.is_empty() { stdout } else { stderr };
                Err(AudioError::CommandFailed(cmd.to_string(), combined))
            }
        }
        Ok(Err(io_err)) => Err(AudioError::Io(io_err)),
        Err(_) => Err(AudioError::Timeout(format!("{} timed out after {}s", cmd, timeout_secs))),
    }
}
