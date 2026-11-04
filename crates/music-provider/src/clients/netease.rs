use crate::core::error::ProviderError;
use crate::core::query::MetadataQuery;
use crate::core::traits::MetadataProvider;
use async_trait::async_trait;
use music_core::{ReleaseMetadata, TrackMetadata};
use reqwest::header::{HeaderMap, HeaderValue, COOKIE, REFERER, USER_AGENT};
use reqwest::Client;
use serde::Deserialize;
use std::time::Duration;

pub struct NeteaseProvider {
    client: Client,
    base_url: String,
}

impl NeteaseProvider {
    pub fn new(cookie: Option<&str>) -> Self {
        Self::with_base_url("https://music.163.com/api", cookie)
    }

    pub fn with_base_url(base_url: impl Into<String>, cookie: Option<&str>) -> Self {
        let mut headers = HeaderMap::new();
        headers.insert(
            USER_AGENT,
            HeaderValue::from_static(
                "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
            ),
        );
        headers.insert(REFERER, HeaderValue::from_static("https://music.163.com/"));
        if let Some(c) = cookie {
            if !c.trim().is_empty() {
                if let Ok(val) = HeaderValue::from_str(c) {
                    headers.insert(COOKIE, val);
                }
            }
        }

        let client = Client::builder()
            .default_headers(headers)
            .timeout(Duration::from_secs(15))
            .build()
            .unwrap_or_default();

        Self {
            client,
            base_url: base_url.into(),
        }
    }
}

#[derive(Deserialize)]
struct NeteaseAlbumSearchResp {
    #[serde(default)]
    result: Option<NeteaseAlbumSearchResult>,
}

#[derive(Deserialize)]
struct NeteaseAlbumSearchResult {
    #[serde(default)]
    albums: Vec<NeteaseSearchAlbum>,
}

#[derive(Deserialize)]
#[allow(dead_code)]
struct NeteaseSearchAlbum {
    id: i64,
    name: String,
    artist: Option<NeteaseArtistSimple>,
    #[serde(default)]
    size: usize,
    #[serde(rename = "picUrl", default)]
    pic_url: Option<String>,
}

#[derive(Deserialize)]
struct NeteaseArtistSimple {
    name: String,
}

#[derive(Deserialize)]
struct NeteaseAlbumDetailResp {
    #[serde(default)]
    album: Option<NeteaseAlbumDetail>,
    #[serde(default)]
    songs: Vec<NeteaseSong>,
}

#[derive(Deserialize)]
struct NeteaseAlbumDetail {
    name: String,
    artist: Option<NeteaseArtistSimple>,
    #[serde(rename = "picUrl", default)]
    pic_url: Option<String>,
    #[serde(rename = "publishTime", default)]
    publish_time: Option<i64>,
}

#[derive(Deserialize)]
struct NeteaseSong {
    name: String,
    no: u32,
    #[serde(default)]
    cd: Option<String>,
}

#[async_trait]
impl MetadataProvider for NeteaseProvider {
    fn name(&self) -> &'static str {
        "NetEase"
    }

    async fn search_release(
        &self,
        query: &MetadataQuery,
    ) -> Result<Option<ReleaseMetadata>, ProviderError> {
        let search_text = format!("{} {}", query.album, query.artist);
        let url = format!("{}/search/get/web", self.base_url);

        let resp = self
            .client
            .post(&url)
            .form(&[
                ("s", search_text.as_str()),
                ("type", "10"), // 10 = Album
                ("offset", "0"),
                ("limit", "5"),
            ])
            .send()
            .await?;

        if !resp.status().is_success() {
            return Ok(None);
        }

        let search_data: NeteaseAlbumSearchResp = match resp.json().await {
            Ok(d) => d,
            Err(_) => return Ok(None),
        };

        let albums = match search_data.result {
            Some(res) if !res.albums.is_empty() => res.albums,
            _ => return Ok(None),
        };

        let target_album_id = albums[0].id;
        let detail_url = format!("{}/album/{}", self.base_url, target_album_id);
        let detail_resp = self.client.get(&detail_url).send().await?;

        if !detail_resp.status().is_success() {
            return Ok(None);
        }

        let detail_data: NeteaseAlbumDetailResp = match detail_resp.json().await {
            Ok(d) => d,
            Err(_) => return Ok(None),
        };

        let album_info = match detail_data.album {
            Some(a) => a,
            None => return Ok(None),
        };

        let album_artist = album_info
            .artist
            .map(|a| a.name)
            .unwrap_or_else(|| query.artist.clone());

        let year = album_info.publish_time.map(|ts| {
            let secs = ts / 1000;
            chrono::DateTime::from_timestamp(secs, 0)
                .map(|dt| dt.format("%Y").to_string())
                .unwrap_or_default()
        });

        let mut tracks = Vec::new();
        for (idx, song) in detail_data.songs.iter().enumerate() {
            let disc_num = song
                .cd
                .as_deref()
                .and_then(|s| s.parse::<u32>().ok())
                .unwrap_or(1);
            let track_no = if song.no > 0 {
                song.no
            } else {
                (idx + 1) as u32
            };
            tracks.push(TrackMetadata {
                track_number: track_no,
                title: song.name.clone(),
                artist: album_artist.clone(),
                album: album_info.name.clone(),
                disc_number: Some(disc_num),
                disc_total: None,
                year: year.clone(),
                ..Default::default()
            });
        }

        Ok(Some(ReleaseMetadata {
            album: album_info.name,
            album_artist,
            year,
            cover_url: album_info.pic_url,
            tracks,
            source: self.name().to_string(),
        }))
    }
}
