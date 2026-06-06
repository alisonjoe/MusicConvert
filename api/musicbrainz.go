package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

var (
	mbTransport = &http.Transport{
		DisableKeepAlives:     true,
		ForceAttemptHTTP2:     false,
		ResponseHeaderTimeout: 10 * time.Second,
	}
	mbClient = &http.Client{
		Timeout:   30 * time.Second,
		Transport: mbTransport,
	}
	lastMBRequest time.Time
	mbLock        sync.Mutex
)

func normalizeMBQueryText(s string) string {
	replacer := strings.NewReplacer(
		"‐", "-",
		"‑", "-",
		"‒", "-",
		"–", "-",
		"—", "-",
		"―", "-",
		"‘", "'",
		"’", "'",
	)
	return strings.TrimSpace(replacer.Replace(s))
}

func musicBrainzGetJSON(apiURL string, out any, attempts int) bool {
	for i := 0; i < attempts; i++ {
		rateLimitMB()
		log.Printf("[MusicBrainz API Request] URL: %s (Attempt %d)", apiURL, i+1)

		req, _ := http.NewRequest("GET", apiURL, nil)
		req.Header.Set("User-Agent", "MusicConvertTool/1.2.0 ( https://github.com/alisonjoe/MusicConvert )")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Connection", "close")
		req.Close = true

		resp, err := mbClient.Do(req)
		if err != nil {
			log.Printf("[MusicBrainz API Error] request failed: %v, retrying...", err)
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			log.Printf("[MusicBrainz API Error] read failed: status=%d bytes=%d err=%v, retrying...", resp.StatusCode, len(body), readErr)
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			continue
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Printf("[MusicBrainz API Error] http status=%d body=%s", resp.StatusCode, string(body))
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
			continue
		}
		if err := json.Unmarshal(body, out); err != nil {
			log.Printf("[MusicBrainz API Error] json decode failed: bytes=%d err=%v body_prefix=%q", len(body), err, string(body[:min(len(body), 200)]))
			return false
		}
		return true
	}
	return false
}

func rateLimitMB() {
	mbLock.Lock()
	defer mbLock.Unlock()
	elapsed := time.Since(lastMBRequest)
	if elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}
	lastMBRequest = time.Now()
}

type MBArtist struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Aliases []struct {
		Name    string `json:"name"`
		Locale  string `json:"locale"`
		Primary bool   `json:"primary"`
	} `json:"aliases"`
}

type MBSearchResp struct {
	Artists []MBArtist `json:"artists"`
}

type MBRecording struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	ArtistCredit []struct {
		Artist struct {
			Name string `json:"name"`
		} `json:"artist"`
	} `json:"artist-credit"`
	Releases []struct {
		Title      string `json:"title"`
		Date       string `json:"date"`
		TrackCount int    `json:"track-count"`
		Media      []struct {
			Track []struct {
				Number string `json:"number"`
			} `json:"track"`
		} `json:"media"`
	} `json:"releases"`
}

type MBRecordingSearchResp struct {
	Recordings []MBRecording `json:"recordings"`
}

type MBReleaseSearchResp struct {
	Releases []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		Date  string `json:"date"`
	} `json:"releases"`
}

type MBReleaseLookupResp struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Media []struct {
		Tracks []struct {
			Position  int    `json:"position"`
			Number    string `json:"number"`
			Title     string `json:"title"`
			Recording struct {
				Title string `json:"title"`
			} `json:"recording"`
		} `json:"tracks"`
	} `json:"media"`
}

// FetchMBMetadata 深度抓取 MusicBrainz 元数据
func FetchMBMetadata(title, artist, albumHint string) (string, string, string, string, string, int) {
	if title == "" {
		return "", "", "", "", "", 0
	}
	title = normalizeMBQueryText(title)
	artist = normalizeMBQueryText(artist)
	albumHint = normalizeMBQueryText(albumHint)

	query := fmt.Sprintf("recording:\"%s\"", title)
	if artist != "" && artist != "Unknown Artist" {
		query += fmt.Sprintf(" AND artist:\"%s\"", artist)
	}
	if albumHint != "" && albumHint != "Unknown Album" {
		query += fmt.Sprintf(" AND release:\"%s\"", albumHint)
	}

	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/recording/?query=%s&fmt=json&limit=10", url.QueryEscape(query))

	type MBScore struct {
		Artist      string
		Album       string
		Title       string
		Year        string
		TrackNum    string
		TrackTotal  int
		TitleScore  float64
		ArtistScore float64
		AlbumScore  float64
		TotalScore  float64
	}

	var bestMatch *MBScore
	targetTitle := strings.ToLower(title)
	targetArtist := strings.ToLower(artist)
	hasArtist := targetArtist != "" && targetArtist != "unknown artist"
	hint := strings.ToLower(albumHint)
	hasHint := (hint != "" && hint != "unknown album")

	var res MBRecordingSearchResp
	if musicBrainzGetJSON(apiURL, &res, 3) && len(res.Recordings) > 0 {
		log.Printf("[MusicBrainz Candidates] recordings=%d for title=%q artist=%q album=%q", len(res.Recordings), title, artist, albumHint)
		candidateNo := 0
		for recordingIndex, r := range res.Recordings {
			rTitle := strings.ToLower(normalizeMBQueryText(r.Title))

			// 标题匹配度
			titleScore := 0.0
			if rTitle == targetTitle {
				titleScore = 1.0
			} else if strings.Contains(rTitle, targetTitle) || strings.Contains(targetTitle, rTitle) {
				titleScore = 0.7
			}

			// 艺术家匹配度
			artistScore := 0.0
			mArtist := ""
			if len(r.ArtistCredit) > 0 {
				mArtist = r.ArtistCredit[0].Artist.Name
				mArtistLower := strings.ToLower(normalizeMBQueryText(mArtist))
				if !hasArtist && mArtistLower != "" {
					artistScore = 1.0
				} else if mArtistLower == targetArtist {
					artistScore = 1.0
				} else if strings.Contains(mArtistLower, targetArtist) || strings.Contains(targetArtist, mArtistLower) {
					artistScore = 0.8
				}
			}

			log.Printf("[MusicBrainz Recording %02d] artist=%q title=%q releases=%d title_score=%.2f artist_score=%.2f",
				recordingIndex+1, mArtist, r.Title, len(r.Releases), titleScore, artistScore)

			// 遍历所有release，寻找最佳匹配
			for _, rel := range r.Releases {
				relAlbum := strings.ToLower(normalizeMBQueryText(rel.Title))
				mTrackNum := ""
				if len(rel.Media) > 0 && len(rel.Media[0].Track) > 0 {
					mTrackNum = rel.Media[0].Track[0].Number
				}

				// 专辑匹配度
				albumScore := 0.0
				if hasHint {
					if relAlbum == hint {
						albumScore = 1.0
					} else if strings.Contains(relAlbum, hint) {
						albumScore = 0.7
					} else if strings.Contains(hint, relAlbum) {
						albumScore = 0.5
					}
				}
				if hasHint && albumScore < 0.8 {
					if candidateNo < 10 {
						candidateNo++
						log.Printf("[MusicBrainz Candidate %02d] skip album_mismatch artist=%q title=%q release=%q date=%q track_total=%d track=%q scores title=%.2f artist=%.2f album=%.2f",
							candidateNo, mArtist, r.Title, rel.Title, rel.Date, rel.TrackCount, mTrackNum, titleScore, artistScore, albumScore)
					}
					continue
				}

				// 计算总分
				totalScore := 0.0
				if hasHint && !hasArtist {
					totalScore = titleScore*0.55 + albumScore*0.45
				} else if hasHint {
					totalScore = titleScore*0.35 + artistScore*0.35 + albumScore*0.30
				} else {
					totalScore = titleScore*0.5 + artistScore*0.5
				}

				// 提取其他信息
				mYear := ""
				if len(rel.Date) >= 4 {
					mYear = rel.Date[:4]
				}
				if candidateNo < 10 {
					candidateNo++
					log.Printf("[MusicBrainz Candidate %02d] artist=%q title=%q release=%q date=%q track_total=%d media=%d track=%q scores title=%.2f artist=%.2f album=%.2f total=%.2f",
						candidateNo, mArtist, r.Title, rel.Title, rel.Date, rel.TrackCount, len(rel.Media), mTrackNum, titleScore, artistScore, albumScore, totalScore)
				}
				if titleScore < 0.5 || (hasArtist && artistScore < 0.5) {
					continue
				}

				current := &MBScore{
					Artist:      mArtist,
					Album:       rel.Title,
					Title:       r.Title,
					Year:        mYear,
					TrackNum:    mTrackNum,
					TrackTotal:  rel.TrackCount,
					TitleScore:  titleScore,
					ArtistScore: artistScore,
					AlbumScore:  albumScore,
					TotalScore:  totalScore,
				}

				if bestMatch == nil || current.TotalScore > bestMatch.TotalScore {
					bestMatch = current
				}
			}
			if candidateNo == 10 {
				log.Printf("[MusicBrainz Candidates] logging limited to first 10 expanded candidates")
				candidateNo++
			}
		}
	}
	if bestMatch != nil {
		trackNum := 0
		if n, err := strconv.Atoi(bestMatch.TrackNum); err == nil {
			trackNum = n
		}
		if trackNum <= 0 {
			trackNum = 1
		}
		log.Printf("[MusicBrainz Match] Title: %.2f, Artist: %.2f, Album: %.2f, Total: %.2f -> %s - %s / %s",
			bestMatch.TitleScore, bestMatch.ArtistScore, bestMatch.AlbumScore, bestMatch.TotalScore,
			bestMatch.Artist, bestMatch.Title, bestMatch.Album)
		return bestMatch.Artist, bestMatch.Album, bestMatch.Title, bestMatch.Year, bestMatch.TrackNum, bestMatch.TrackTotal
	}
	return "", "", "", "", "", 0
}

func FetchMBReleaseTrackTitles(catalog, queryHint string) (map[int]string, string) {
	catalog = normalizeMBQueryText(catalog)
	queryHint = normalizeMBQueryText(queryHint)
	queries := make([]string, 0, 2)
	if catalog != "" {
		queries = append(queries, fmt.Sprintf("catno:\"%s\"", catalog))
	}
	if queryHint != "" {
		queries = append(queries, fmt.Sprintf("release:\"%s\"", queryHint))
	}
	for _, query := range queries {
		tracks, source := fetchMBReleaseTrackTitlesByQuery(query)
		if len(tracks) > 0 {
			return tracks, source
		}
	}
	return nil, ""
}

func fetchMBReleaseTrackTitlesByQuery(query string) (map[int]string, string) {
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/release/?query=%s&fmt=json&limit=5", url.QueryEscape(query))
	var search MBReleaseSearchResp
	if !musicBrainzGetJSON(apiURL, &search, 3) || len(search.Releases) == 0 {
		return nil, ""
	}
	for _, release := range search.Releases {
		tracks, ok := fetchMBReleaseTracksByID(release.ID)
		if ok && len(tracks) > 0 {
			log.Printf("[MusicBrainz Release Tracks] source=query:%s release=%q id=%s tracks=%d", query, release.Title, release.ID, len(tracks))
			return tracks, "musicbrainz:" + release.ID
		}
	}
	return nil, ""
}

func fetchMBReleaseTracksByID(releaseID string) (map[int]string, bool) {
	if releaseID == "" {
		return nil, false
	}
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/release/%s?inc=media+recordings&fmt=json", url.PathEscape(releaseID))
	var release MBReleaseLookupResp
	if !musicBrainzGetJSON(apiURL, &release, 3) {
		return nil, false
	}
	tracks := make(map[int]string)
	index := 1
	for _, medium := range release.Media {
		for _, track := range medium.Tracks {
			title := strings.TrimSpace(track.Title)
			if title == "" {
				title = strings.TrimSpace(track.Recording.Title)
			}
			if title == "" {
				index++
				continue
			}
			position := track.Position
			if position <= 0 {
				if n, err := strconv.Atoi(track.Number); err == nil {
					position = n
				}
			}
			if position <= 0 {
				position = index
			}
			tracks[position] = title
			index++
		}
	}
	return tracks, len(tracks) > 0
}

func FetchMBArtistName(query string) (string, string) {
	if query == "" {
		return "", ""
	}
	query = normalizeMBQueryText(query)
	apiURL := fmt.Sprintf("https://musicbrainz.org/ws/2/artist/?query=artist:\"%s\"&fmt=json", url.QueryEscape(query))
	var res MBSearchResp
	if musicBrainzGetJSON(apiURL, &res, 2) && len(res.Artists) > 0 {
		a := res.Artists[0]
		chinese, original := "", a.Name
		for _, alias := range a.Aliases {
			if isPreferredChineseAlias(alias.Name, alias.Locale) {
				chinese = alias.Name
				break
			}
		}
		if chinese == "" {
			for _, alias := range a.Aliases {
				if isLikelyChineseText(alias.Name) {
					chinese = alias.Name
					break
				}
			}
		}
		if containsChinese(original) {
			chinese = original
			for _, alias := range a.Aliases {
				if !containsChinese(alias.Name) && alias.Name != "" && !containsJapaneseKana(alias.Name) {
					original = alias.Name
					break
				}
			}
		}
		return chinese, original
	}
	return "", ""
}

func isPreferredChineseAlias(name, locale string) bool {
	locale = strings.ToLower(strings.TrimSpace(locale))
	return isLikelyChineseText(name) && (locale == "zh" || strings.HasPrefix(locale, "zh-") || locale == "cmn")
}

func isLikelyChineseText(name string) bool {
	return containsChinese(name) && !containsJapaneseKana(name)
}

func containsJapaneseKana(s string) bool {
	for _, r := range s {
		if unicode.In(r, unicode.Hiragana, unicode.Katakana) {
			return true
		}
	}
	return false
}
