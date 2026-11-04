pub mod chain;
pub mod error;
pub mod query;
pub mod traits;

// 别名兼容
pub use traits as trait_provider;

pub use chain::ProviderChain;
pub use error::ProviderError;
pub use query::MetadataQuery;
pub use traits::MetadataProvider;
