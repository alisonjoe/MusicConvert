pub mod assets;
pub mod handlers;
pub mod routes;
pub mod state;

pub use assets::static_handler;
pub use routes::create_router;
pub use state::AppState;

use std::net::SocketAddr;
use tokio::net::TcpListener;
use tracing::info;

pub async fn start_server(state: AppState, addr: SocketAddr) -> Result<(), std::io::Error> {
    let app = create_router(state);
    let listener = TcpListener::bind(addr).await?;
    info!("[Server] Axum HTTP & SSE server listening on http://{}", addr);
    axum::serve(listener, app).await
}
