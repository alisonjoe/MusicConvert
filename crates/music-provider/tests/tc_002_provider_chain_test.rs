use async_trait::async_trait;
use music_core::{ReleaseMetadata, TrackMetadata};
use music_provider::{MetadataProvider, MetadataQuery, ProviderChain, ProviderError};
use std::sync::Arc;

struct MockFailingProvider;

#[async_trait]
impl MetadataProvider for MockFailingProvider {
    fn name(&self) -> &'static str {
        "MockFailing"
    }

    async fn search_release(
        &self,
        _query: &MetadataQuery,
    ) -> Result<Option<ReleaseMetadata>, ProviderError> {
        Err(ProviderError::Custom(
            "MockFailing".to_string(),
            "Network timeout".to_string(),
        ))
    }
}

struct MockSuccessProvider {
    meta: ReleaseMetadata,
}

#[async_trait]
impl MetadataProvider for MockSuccessProvider {
    fn name(&self) -> &'static str {
        "MockSuccess"
    }

    async fn search_release(
        &self,
        _query: &MetadataQuery,
    ) -> Result<Option<ReleaseMetadata>, ProviderError> {
        Ok(Some(self.meta.clone()))
    }
}

#[tokio::test]
async fn test_tc_002_provider_chain_fallback_and_localization() {
    // Step 1: Chain fails over from MockFailingProvider to MockSuccessProvider
    let success_meta = ReleaseMetadata {
        album: "Greatest Hits (2)".to_string(),
        album_artist: "Jacky Cheung (2)".to_string(),
        year: Some("2000".to_string()),
        cover_url: Some("https://example.com/cover.jpg".to_string()),
        tracks: vec![
            TrackMetadata {
                track_number: 1,
                title: "吻別".to_string(),
                artist: "Jacky Cheung".to_string(),
                album: "Greatest Hits".to_string(),
                ..Default::default()
            },
            TrackMetadata {
                track_number: 2,
                title: "Symphony N° 5 in C minor".to_string(),
                artist: "Jacky Cheung".to_string(),
                album: "Greatest Hits".to_string(),
                ..Default::default()
            },
        ],
        source: "MockSuccess".to_string(),
    };

    let chain = ProviderChain::new()
        .with_provider(Arc::new(MockFailingProvider))
        .with_provider(Arc::new(MockSuccessProvider { meta: success_meta }));

    let query = MetadataQuery::new("张学友", "吻别").with_track_titles(vec!["吻别".to_string()]);

    let result = chain.fetch(&query).await.expect("Fetch failed");
    assert!(
        result.is_some(),
        "Expected chain to return metadata from fallback provider"
    );

    let meta = result.unwrap();

    // Step 2: Assert Chinese Localization & Discogs Suffix Protection
    // Chinese query artist "张学友" was preserved over "Jacky Cheung"
    assert_eq!(meta.album_artist, "张学友");
    assert_eq!(meta.tracks[0].artist, "张学友");
    // Discogs suffix ` (2)` was stripped from album
    assert_eq!(meta.album, "Greatest Hits");
    // Traditional Chinese "吻別" was simplified to "吻别"
    assert_eq!(meta.tracks[0].title, "吻别");

    // Step 3: Classical N° normalization
    assert_eq!(meta.tracks[1].title, "Symphony No. 5 in C minor");
}
