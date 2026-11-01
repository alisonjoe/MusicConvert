use async_trait::async_trait;
use music_core::ReleaseMetadata;
use crate::error::ProviderError;
use crate::query::MetadataQuery;

#[async_trait]
pub trait MetadataProvider: Send + Sync {
    fn name(&self) -> &'static str;

    fn is_enabled(&self) -> bool {
        true
    }

    async fn search_release(&self, query: &MetadataQuery) -> Result<Option<ReleaseMetadata>, ProviderError>;
}
