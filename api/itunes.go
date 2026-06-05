package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"time"
)

var itunesClient = &http.Client{
	Timeout: 10 * time.Second,
}

type ItunesMeta struct {
	Genre          string
	AlbumArtist    string
	CollectionName string
	Date           string
	DiscTotal      int
}

type ItunesSearchResp struct {
	Results []struct {
		PrimaryGenreName string `json:"primaryGenreName"`
		ArtistName       string `json:"artistName"`
		CollectionName   string `json:"collectionName"`
		ReleaseDate      string `json:"releaseDate"`
		DiscCount        int    `json:"discCount"`
	} `json:"results"`
}

// FetchItunesMeta queries Apple iTunes API for rich album metadata
func FetchItunesMeta(artist, album string) ItunesMeta {
	if artist == "" || album == "" {
		return ItunesMeta{DiscTotal: 1}
	}
	return fetchItunesMetaForQuery(fmt.Sprintf("%s %s", artist, album), fmt.Sprintf("artist=%q album=%q", artist, album))
}

func FetchItunesMetaByQuery(query string) ItunesMeta {
	if query == "" {
		return ItunesMeta{DiscTotal: 1}
	}
	return fetchItunesMetaForQuery(query, fmt.Sprintf("query=%q", query))
}

func fetchItunesMetaForQuery(query, contextLabel string) ItunesMeta {
	meta := ItunesMeta{DiscTotal: 1} // 默认至少 1 碟
	apiURL := fmt.Sprintf("https://itunes.apple.com/search?term=%s&entity=album&limit=10", url.QueryEscape(query))
	log.Printf("[iTunes API Request] URL: %s", apiURL)

	resp, err := itunesClient.Get(apiURL)
	if err != nil {
		log.Printf("[iTunes API Error] %v", err)
		return meta
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[iTunes API Error] http status=%d", resp.StatusCode)
		return meta
	}

	var res ItunesSearchResp
	if err := json.NewDecoder(resp.Body).Decode(&res); err == nil && len(res.Results) > 0 {
		log.Printf("[iTunes Candidates] count=%d for %s", len(res.Results), contextLabel)
		for i, r := range res.Results {
			log.Printf("[iTunes Candidate %02d] artist=%q album=%q genre=%q release=%q disc_count=%d",
				i+1, r.ArtistName, r.CollectionName, r.PrimaryGenreName, r.ReleaseDate, r.DiscCount)
		}
		r := res.Results[0]
		meta.Genre = r.PrimaryGenreName
		meta.AlbumArtist = r.ArtistName
		meta.CollectionName = r.CollectionName
		meta.DiscTotal = r.DiscCount
		if len(r.ReleaseDate) >= 4 {
			meta.Date = r.ReleaseDate[:4] // 提取 YYYY
		}
	} else if err != nil {
		log.Printf("[iTunes API Error] decode failed: %v", err)
	} else {
		log.Printf("[iTunes Candidates] none for %s", contextLabel)
	}
	return meta
}
