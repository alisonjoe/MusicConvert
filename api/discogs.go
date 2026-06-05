package api

import (
	"MusicConvert/config"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	discogsClient    = &http.Client{Timeout: 20 * time.Second}
	trackNumberRE    = regexp.MustCompile(`\d+`)
	discogsCache     = make(map[string]DiscogsReleaseMeta)
	discogsCacheLock sync.Mutex
)

type DiscogsSearchResp struct {
	Results []DiscogsSearchResult `json:"results"`
}

type DiscogsSearchResult struct {
	ID          int      `json:"id"`
	Title       string   `json:"title"`
	Catno       string   `json:"catno"`
	ResourceURL string   `json:"resource_url"`
	Format      []string `json:"format"`
	Label       []string `json:"label"`
	Year        string   `json:"year"`
}

type DiscogsReleaseResp struct {
	ID           int             `json:"id"`
	Title        string          `json:"title"`
	Tracklist    []DiscogsTrack  `json:"tracklist"`
	Artists      []DiscogsArtist `json:"artists"`
	ExtraArtists []DiscogsArtist `json:"extraartists"`
	Labels       []struct {
		Name   string `json:"name"`
		Catno  string `json:"catno"`
		Entity string `json:"entity_type_name"`
	} `json:"labels"`
}

type DiscogsArtist struct {
	Name string `json:"name"`
	Role string `json:"role"`
}

type DiscogsTrack struct {
	Position  string         `json:"position"`
	Type      string         `json:"type_"`
	Title     string         `json:"title"`
	Duration  string         `json:"duration"`
	SubTracks []DiscogsTrack `json:"sub_tracks"`
}

type DiscogsReleaseMeta struct {
	ID      int
	Title   string
	Artists []string
	Credits []DiscogsCredit
	Tracks  map[int]string
}

type DiscogsCredit struct {
	Name string
	Role string
}

func discogsGetJSON(apiURL string, out any) error {
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		if err := discogsGetJSONOnce(apiURL, out); err != nil {
			lastErr = err
			log.Printf("[Discogs] request failed (attempt %d/2): %v", attempt, err)
			if attempt < 2 {
				time.Sleep(time.Duration(attempt) * time.Second)
			}
			continue
		}
		return nil
	}
	return lastErr
}

func discogsGetJSONOnce(apiURL string, out any) error {
	token := config.GetDiscogsToken()
	if token == "" {
		return fmt.Errorf("discogs token not configured")
	}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "MusicConvert/1.0")
	req.Header.Set("Authorization", "Discogs token="+token)
	resp, err := discogsClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discogs http %d: %s", resp.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("discogs decode failed: %w", err)
	}
	return nil
}

func FetchDiscogsReleaseTrackTitles(catalog string, hints []string) (map[int]string, string) {
	meta, source := FetchDiscogsReleaseMeta(catalog, hints)
	return meta.Tracks, source
}

func FetchDiscogsReleaseMeta(catalog string, hints []string) (DiscogsReleaseMeta, string) {
	catalog = strings.TrimSpace(catalog)
	if catalog == "" || config.GetDiscogsToken() == "" {
		return DiscogsReleaseMeta{}, ""
	}
	cacheKey := strings.ToUpper(catalog)
	discogsCacheLock.Lock()
	if cached := discogsCache[cacheKey]; cached.ID > 0 {
		discogsCacheLock.Unlock()
		return cached, "discogs:" + strconv.Itoa(cached.ID)
	}
	discogsCacheLock.Unlock()

	values := url.Values{}
	values.Set("type", "release")
	values.Set("catno", catalog)
	values.Set("per_page", "10")
	apiURL := "https://api.discogs.com/database/search?" + values.Encode()
	var search DiscogsSearchResp
	if err := discogsGetJSON(apiURL, &search); err != nil {
		log.Printf("[Discogs] Search failed for catno=%s: %v", catalog, err)
		return DiscogsReleaseMeta{}, ""
	}
	if len(search.Results) == 0 {
		log.Printf("[Discogs] No release candidates for catno=%s", catalog)
		return DiscogsReleaseMeta{}, ""
	}
	best := chooseDiscogsCandidate(search.Results, hints)
	if best.ID <= 0 {
		return DiscogsReleaseMeta{}, ""
	}
	releaseURL := fmt.Sprintf("https://api.discogs.com/releases/%d", best.ID)
	var release DiscogsReleaseResp
	if err := discogsGetJSON(releaseURL, &release); err != nil {
		log.Printf("[Discogs] Release fetch failed id=%d: %v", best.ID, err)
		return DiscogsReleaseMeta{}, ""
	}
	tracks := flattenDiscogsTracklist(release.Tracklist)
	if len(tracks) == 0 {
		log.Printf("[Discogs] Release id=%d has no usable tracklist", best.ID)
		return DiscogsReleaseMeta{}, ""
	}
	artists := make([]string, 0, len(release.Artists))
	for _, artist := range release.Artists {
		if strings.TrimSpace(artist.Name) != "" {
			artists = append(artists, strings.TrimSpace(artist.Name))
		}
	}
	credits := make([]DiscogsCredit, 0, len(release.ExtraArtists))
	for _, artist := range release.ExtraArtists {
		if strings.TrimSpace(artist.Name) != "" {
			credits = append(credits, DiscogsCredit{Name: strings.TrimSpace(artist.Name), Role: strings.TrimSpace(artist.Role)})
		}
	}
	log.Printf("[Discogs] Matched release id=%d title=%q catno=%q tracks=%d", best.ID, release.Title, best.Catno, len(tracks))
	meta := DiscogsReleaseMeta{ID: best.ID, Title: release.Title, Artists: artists, Credits: credits, Tracks: tracks}
	discogsCacheLock.Lock()
	discogsCache[cacheKey] = meta
	discogsCacheLock.Unlock()
	return meta, "discogs:" + strconv.Itoa(best.ID)
}

func chooseDiscogsCandidate(results []DiscogsSearchResult, hints []string) DiscogsSearchResult {
	bestScore := -1
	var best DiscogsSearchResult
	for _, candidate := range results {
		text := strings.ToLower(candidate.Title + " " + strings.Join(candidate.Label, " ") + " " + strings.Join(candidate.Format, " "))
		score := 0
		for _, hint := range hints {
			hint = strings.ToLower(strings.TrimSpace(hint))
			if hint == "" {
				continue
			}
			for _, part := range strings.Fields(hint) {
				if len(part) >= 4 && strings.Contains(text, part) {
					score++
				}
			}
		}
		if strings.Contains(text, "classical") {
			score++
		}
		if score > bestScore {
			bestScore = score
			best = candidate
		}
	}
	return best
}

func flattenDiscogsTracklist(tracklist []DiscogsTrack) map[int]string {
	tracks := make(map[int]string)
	next := 1
	var heading string
	for _, track := range tracklist {
		next = appendDiscogsTrack(tracks, next, heading, track)
		if strings.EqualFold(track.Type, "heading") && strings.TrimSpace(track.Title) != "" {
			heading = strings.TrimSpace(track.Title)
		}
	}
	return tracks
}

func appendDiscogsTrack(tracks map[int]string, next int, heading string, track DiscogsTrack) int {
	trackType := strings.ToLower(strings.TrimSpace(track.Type))
	if trackType == "heading" {
		return next
	}
	if len(track.SubTracks) > 0 {
		parent := strings.TrimSpace(track.Title)
		if parent == "" {
			parent = heading
		}
		for _, sub := range track.SubTracks {
			next = appendDiscogsTrack(tracks, next, parent, sub)
		}
		return next
	}
	title := strings.TrimSpace(track.Title)
	if title == "" {
		return next
	}
	if heading != "" && !strings.Contains(title, heading) {
		title = heading + ": " + title
	}
	position := discogsTrackPosition(track.Position)
	if position <= 0 || tracks[position] != "" {
		position = next
	}
	tracks[position] = title
	if position >= next {
		return position + 1
	}
	return next + 1
}

func discogsTrackPosition(position string) int {
	position = strings.TrimSpace(position)
	if position == "" {
		return 0
	}
	matches := trackNumberRE.FindAllString(position, -1)
	if len(matches) == 0 {
		return 0
	}
	last := matches[len(matches)-1]
	n, err := strconv.Atoi(last)
	if err != nil {
		return 0
	}
	return n
}
