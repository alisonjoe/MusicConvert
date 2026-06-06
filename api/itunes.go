package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
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

func itunesContextArtistAlbum(contextLabel string) (string, string) {
	artist, album := "", ""
	for _, part := range []string{"artist=", "album="} {
		idx := strings.Index(contextLabel, part)
		if idx < 0 {
			continue
		}
		start := idx + len(part)
		if start < len(contextLabel) && contextLabel[start] == '"' {
			start++
			end := strings.Index(contextLabel[start:], "\"")
			if end >= 0 {
				if part == "artist=" {
					artist = contextLabel[start : start+end]
				} else {
					album = contextLabel[start : start+end]
				}
			}
		}
	}
	return artist, album
}

func itunesCandidateScore(queryArtist, queryAlbum, candidateArtist, candidateAlbum string) int {
	qa := normalizeSongName(queryArtist)
	qalb := normalizeSongName(queryAlbum)
	ca := normalizeSongName(candidateArtist)
	calb := normalizeSongName(candidateAlbum)
	if qalb == "" || calb == "" {
		return 0
	}
	score := 0
	if calb == qalb {
		score += 60
	} else if strings.HasPrefix(calb, qalb) && (strings.Contains(calb, "deluxe") || strings.Contains(calb, "edition")) {
		score += 45
	} else {
		return 0
	}
	if qa != "" && ca == qa {
		score += 40
	}
	return score
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
		queryArtist, queryAlbum := itunesContextArtistAlbum(contextLabel)
		bestIndex := -1
		bestScore := 0
		for i, r := range res.Results {
			score := itunesCandidateScore(queryArtist, queryAlbum, r.ArtistName, r.CollectionName)
			log.Printf("[iTunes Candidate %02d] artist=%q album=%q genre=%q release=%q disc_count=%d score=%d",
				i+1, r.ArtistName, r.CollectionName, r.PrimaryGenreName, r.ReleaseDate, r.DiscCount, score)
			if score > bestScore {
				bestScore = score
				bestIndex = i
			}
		}
		if bestIndex < 0 {
			log.Printf("[iTunes Candidates] no context match for %s", contextLabel)
			return meta
		}
		r := res.Results[bestIndex]
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
