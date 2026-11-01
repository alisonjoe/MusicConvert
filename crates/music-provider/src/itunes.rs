use crate::error::ProviderError;
use crate::query::MetadataQuery;
use crate::trait_provider::MetadataProvider;
use async_trait::async_trait;
use music_core::{ReleaseMetadata, TrackMetadata};
use reqwest::Client;
use serde::Deserialize;
use std::time::Duration;

pub struct ITunesProvider {
    client: Client,
    base_url: String,
}

impl ITunesProvider {
    pub fn new() -> Self {
        Self::with_base_url("https://itunes.apple.com")
    }

    pub fn with_base_url(base_url: impl Into<String>) -> Self {
        let client = Client::builder()
            .timeout(Duration::from_secs(15))
            .build()
            .unwrap_or_default();
        Self {
            client,
            base_url: base_url.into(),
        }
    }
}

impl Default for ITunesProvider {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Deserialize)]
struct ITunesSearchResp {
    #[serde(default)]
    results: Vec<ITunesSearchResult>,
}

#[derive(Deserialize)]
struct ITunesSearchResult {
    #[serde(rename = "collectionId", default)]
    collection_id: Option<i64>,
    #[serde(rename = "collectionName", default)]
    collection_name: Option<String>,
    #[serde(rename = "artistName", default)]
    artist_name: Option<String>,
    #[serde(rename = "primaryGenreName", default)]
    primary_genre_name: Option<String>,
    #[serde(rename = "releaseDate", default)]
    release_date: Option<String>,
    #[serde(rename = "discCount", default)]
    disc_count: Option<u32>,
    #[serde(rename = "artworkUrl100", default)]
    artwork_url_100: Option<String>,
    // For lookup item (song)
    #[serde(rename = "wrapperType", default)]
    wrapper_type: Option<String>,
    #[serde(rename = "trackName", default)]
    track_name: Option<String>,
    #[serde(rename = "trackNumber", default)]
    track_number: Option<u32>,
    #[serde(rename = "discNumber", default)]
    disc_number: Option<u32>,
}

#[async_trait]
impl MetadataProvider for ITunesProvider {
    fn name(&self) -> &'static str {
        "iTunes"
    }

    async fn search_release(
        &self,
        query: &MetadataQuery,
    ) -> Result<Option<ReleaseMetadata>, ProviderError> {
        let term = format!("{} {}", query.album, query.artist);
        let url = format!("{}/search", self.base_url);

        let resp = self
            .client
            .get(&url)
            .query(&[("term", term.as_str()), ("entity", "album"), ("limit", "5")])
            .send()
            .await?;

        if !resp.status().is_success() {
            return Ok(None);
        }

        let data: ITunesSearchResp = match resp.json().await {
            Ok(d) => d,
            Err(_) => return Ok(None),
        };

        if data.results.is_empty() {
            return Ok(None);
        }

        let album_match = &data.results[0];
        let collection_id = match album_match.collection_id {
            Some(id) => id,
            None => return Ok(None),
        };

        let collection_name = album_match
            .collection_name
            .clone()
            .unwrap_or_else(|| query.album.clone());
        let album_artist = album_match
            .artist_name
            .clone()
            .unwrap_or_else(|| query.artist.clone());
        let year = album_match
            .release_date
            .as_ref()
            .and_then(|d| d.get(0..4))
            .map(|s| s.to_string());

        // Upgrade artwork to 1200x1200bb
        let high_res_cover = album_match.artwork_url_100.as_ref().map(|u| {
            u.replace("100x100bb", "1200x1200bb")
                .replace("100x100", "1200x1200")
        });

        // Query lookup for tracks
        let lookup_url = format!("{}/lookup", self.base_url);
        let mut tracks = Vec::new();
        if let Ok(lookup_resp) = self
            .client
            .get(&lookup_url)
            .query(&[
                ("id", &collection_id.to_string()),
                ("entity", &"song".to_string()),
            ])
            .send()
            .await
        {
            if let Ok(lookup_data) = lookup_resp.json::<ITunesSearchResp>().await {
                for item in lookup_data.results {
                    if item.wrapper_type.as_deref() == Some("track") {
                        if let Some(track_name) = item.track_name {
                            tracks.push(TrackMetadata {
                                track_number: item.track_number.unwrap_or(1),
                                title: track_name,
                                artist: album_artist.clone(),
                                album: collection_name.clone(),
                                disc_number: item.disc_number,
                                disc_total: album_match.disc_count,
                                genre: album_match.primary_genre_name.clone(),
                                year: year.clone(),
                                ..Default::default()
                            });
                        }
                    }
                }
            }
        }

        Ok(Some(ReleaseMetadata {
            album: collection_name,
            album_artist,
            year,
            cover_url: high_res_cover,
            tracks,
            source: self.name().to_string(),
        }))
    }
}
