use std::sync::Arc;
use music_core::{Config, ReleaseMetadata};
use tracing::{debug, info, warn};
use crate::error::ProviderError;
use crate::itunes::ITunesProvider;
use crate::localization::localize_metadata;
use crate::musicbrainz::MusicBrainzProvider;
use crate::netease::NeteaseProvider;
use crate::query::MetadataQuery;
use crate::trait_provider::MetadataProvider;

pub struct ProviderChain {
    providers: Vec<Arc<dyn MetadataProvider>>,
}

impl ProviderChain {
    pub fn new() -> Self {
        Self {
            providers: Vec::new(),
        }
    }

    pub fn with_provider(mut self, provider: Arc<dyn MetadataProvider>) -> Self {
        self.providers.push(provider);
        self
    }

    pub fn from_config(config: &Config) -> Self {
        let mut chain = Self::new();

        // If query/context is predominantly Chinese, NetEase has priority.
        // In the chain, we register primary Western source (MusicBrainz), Chinese source (NetEase),
        // and iTunes as album-level supplement.
        chain = chain.with_provider(Arc::new(MusicBrainzProvider::new()));

        let netease_cookie = if !config.netease_cookie.is_empty() {
            Some(config.netease_cookie.as_str())
        } else {
            None
        };
        chain = chain.with_provider(Arc::new(NeteaseProvider::new(netease_cookie)));
        chain = chain.with_provider(Arc::new(ITunesProvider::new()));

        chain
    }

    pub async fn fetch(&self, query: &MetadataQuery) -> Result<Option<ReleaseMetadata>, ProviderError> {
        info!(
            "[ProviderChain] Querying metadata for artist={:?} album={:?} (chinese_ctx={})",
            query.artist, query.album, query.has_chinese_context
        );

        for provider in &self.providers {
            if !provider.is_enabled() {
                continue;
            }

            debug!("[ProviderChain] Trying provider: {}", provider.name());
            match provider.search_release(query).await {
                Ok(Some(mut meta)) => {
                    // Check if candidate is non-empty
                    if !meta.album.is_empty() {
                        // Apply Chinese localization & string normalization
                        localize_metadata(query, &mut meta);
                        info!(
                            "[ProviderChain] Matched release via {}: artist={:?} album={:?} tracks={}",
                            provider.name(),
                            meta.album_artist,
                            meta.album,
                            meta.tracks.len()
                        );
                        return Ok(Some(meta));
                    }
                }
                Ok(None) => {
                    debug!("[ProviderChain] Provider {} returned no match", provider.name());
                }
                Err(e) => {
                    warn!(
                        "[ProviderChain] Provider {} failed with error: {}, falling through",
                        provider.name(),
                        e
                    );
                }
            }
        }

        info!("[ProviderChain] No provider found acceptable metadata for {:?}", query.album);
        Ok(None)
    }
}

impl Default for ProviderChain {
    fn default() -> Self {
        Self::new()
    }
}
