use std::sync::Arc;
use std::time::Duration;
use async_trait::async_trait;
use music_core::{ReleaseMetadata, TrackMetadata};
use reqwest::header::{HeaderMap, HeaderValue, ACCEPT, USER_AGENT};
use reqwest::Client;
use serde::Deserialize;
use tokio::sync::Mutex;
use tokio::time::sleep;
use crate::error::ProviderError;
use crate::query::MetadataQuery;
use crate::trait_provider::MetadataProvider;

pub struct MusicBrainzProvider {
    client: Client,
    base_url: String,
    last_request: Arc<Mutex<Option<std::time::Instant>>>,
}

impl MusicBrainzProvider {
    pub fn new() -> Self {
        Self::with_base_url("https://musicbrainz.org/ws/2")
    }

    pub fn with_base_url(base_url: impl Into<String>) -> Self {
        let mut headers = HeaderMap::new();
        headers.insert(
            USER_AGENT,
            HeaderValue::from_static("MusicConvertTool/1.2.0 ( https://github.com/alisonjoe/MusicConvert )"),
        );
        headers.insert(ACCEPT, HeaderValue::from_static("application/json"));

        let client = Client::builder()
            .default_headers(headers)
            .timeout(Duration::from_secs(30))
            .build()
            .unwrap_or_default();

        Self {
            client,
            base_url: base_url.into(),
            last_request: Arc::new(Mutex::new(None)),
        }
    }

    async fn rate_limit(&self) {
        let mut last = self.last_request.lock().await;
        if let Some(instant) = *last {
            let elapsed = instant.elapsed();
            if elapsed < Duration::from_millis(1000) {
                sleep(Duration::from_millis(1000) - elapsed).await;
            }
        }
        *last = Some(std::time::Instant::now());
    }
}

impl Default for MusicBrainzProvider {
    fn default() -> Self {
        Self::new()
    }
}

#[derive(Deserialize)]
struct MBReleaseSearch {
    #[serde(default)]
    releases: Vec<MBRelease>,
}

#[derive(Deserialize)]
struct MBRelease {
    id: String,
    title: String,
    #[serde(default)]
    date: Option<String>,
    #[serde(rename = "artist-credit", default)]
    artist_credit: Vec<MBArtistCredit>,
    #[serde(default)]
    media: Vec<MBMedia>,
}

#[derive(Deserialize)]
struct MBArtistCredit {
    name: String,
}

#[derive(Deserialize)]
#[allow(dead_code)]
struct MBMedia {
    #[serde(default)]
    position: Option<u32>,
    #[serde(rename = "track-count", default)]
    track_count: Option<u32>,
    #[serde(default)]
    tracks: Vec<MBTrack>,
}

#[derive(Deserialize)]
struct MBTrack {
    position: u32,
    title: String,
}

#[async_trait]
impl MetadataProvider for MusicBrainzProvider {
    fn name(&self) -> &'static str {
        "MusicBrainz"
    }

    async fn search_release(&self, query: &MetadataQuery) -> Result<Option<ReleaseMetadata>, ProviderError> {
        self.rate_limit().await;

        let lucene_query = if let Some(cat) = &query.catalog_id {
            format!("catno:\"{}\" OR (release:\"{}\" AND artist:\"{}\")", cat, query.album, query.artist)
        } else {
            format!("release:\"{}\" AND artist:\"{}\"", query.album, query.artist)
        };

        let url = format!("{}/release", self.base_url);
        let resp = self
            .client
            .get(&url)
            .query(&[("query", &lucene_query), ("fmt", &"json".to_string())])
            .send()
            .await?;

        if !resp.status().is_success() {
            return Ok(None);
        }

        let search_data: MBReleaseSearch = match resp.json().await {
            Ok(data) => data,
            Err(_) => return Ok(None),
        };

        if search_data.releases.is_empty() {
            return Ok(None);
        }

        let first = &search_data.releases[0];
        let album_artist = first
            .artist_credit
            .first()
            .map(|a| a.name.clone())
            .unwrap_or_else(|| query.artist.clone());

        // Now lookup release with media tracks if missing
        let mut all_tracks = Vec::new();
        let disc_total = first.media.len() as u32;

        for media in &first.media {
            let disc_num = media.position.unwrap_or(1);
            for t in &media.tracks {
                all_tracks.push(TrackMetadata {
                    track_number: t.position,
                    title: t.title.clone(),
                    artist: album_artist.clone(),
                    album: first.title.clone(),
                    disc_number: Some(disc_num),
                    disc_total: Some(disc_total),
                    year: first.date.clone(),
                    ..Default::default()
                });
            }
        }

        // If tracks were not inline, we fetch details with inc=recordings
        if all_tracks.is_empty() {
            self.rate_limit().await;
            let detail_url = format!("{}/release/{}", self.base_url, first.id);
            if let Ok(detail_resp) = self
                .client
                .get(&detail_url)
                .query(&[("inc", "recordings+artist-credits"), ("fmt", "json")])
                .send()
                .await
            {
                if let Ok(detail) = detail_resp.json::<MBRelease>().await {
                    let total_discs = detail.media.len() as u32;
                    for media in &detail.media {
                        let disc_num = media.position.unwrap_or(1);
                        for t in &media.tracks {
                            all_tracks.push(TrackMetadata {
                                track_number: t.position,
                                title: t.title.clone(),
                                artist: album_artist.clone(),
                                album: detail.title.clone(),
                                disc_number: Some(disc_num),
                                disc_total: Some(total_discs),
                                year: detail.date.clone(),
                                ..Default::default()
                            });
                        }
                    }
                }
            }
        }

        Ok(Some(ReleaseMetadata {
            album: first.title.clone(),
            album_artist,
            year: first.date.clone(),
            cover_url: None,
            tracks: all_tracks,
            source: self.name().to_string(),
        }))
    }
}
