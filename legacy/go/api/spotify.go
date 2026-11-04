package api

import (
	"MusicConvert/config"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type SpotifyToken struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

var currentToken *SpotifyToken
var tokenExpiry time.Time

func getSpotifyAccessToken() (string, error) {
	if currentToken != nil && time.Now().Before(tokenExpiry) {
		return currentToken.AccessToken, nil
	}

	clientID := config.GetSpotifyID()
	clientSecret := config.GetSpotifySecret()
	if clientID == "" || clientSecret == "" {
		return "", fmt.Errorf("Spotify credentials not set")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(clientID + ":" + clientSecret))
	data := url.Values{}
	data.Set("grant_type", "client_credentials")

	req, _ := http.NewRequest("POST", "https://accounts.spotify.com/api/token", strings.NewReader(data.Encode()))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := spotifyClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("spotify token http %d: %s", resp.StatusCode, string(body))
	}

	var token SpotifyToken
	if err := json.Unmarshal(body, &token); err != nil {
		return "", err
	}
	if token.AccessToken == "" {
		return "", fmt.Errorf("spotify token response missing access_token: %s", string(body))
	}

	currentToken = &token
	tokenExpiry = time.Now().Add(time.Duration(token.ExpiresIn-60) * time.Second)
	return token.AccessToken, nil
}

var spotifyClient = &http.Client{
	Timeout: 10 * time.Second,
}

type SpotifySearchResp struct {
	Tracks struct {
		Items []struct {
			Name        string `json:"name"`
			TrackNumber int    `json:"track_number"`
			Artists     []struct {
				Name string `json:"name"`
			} `json:"artists"`
			Album struct {
				Name   string `json:"name"`
				Images []struct {
					URL string `json:"url"`
				} `json:"images"`
				ReleaseDate string `json:"release_date"`
			} `json:"album"`
		} `json:"items"`
	} `json:"tracks"`
}

type SpotifyMatchScore struct {
	Item        interface{} // 存储原始track引用
	Title       string
	Artist      string
	Album       string
	TrackNum    int
	Cover       string
	Year        string
	TitleScore  float64
	ArtistScore float64
	AlbumScore  float64
	TotalScore  float64
}

func spotifyArtistNames(artists []struct {
	Name string `json:"name"`
}) string {
	names := make([]string, 0, len(artists))
	for _, artist := range artists {
		if strings.TrimSpace(artist.Name) != "" {
			names = append(names, artist.Name)
		}
	}
	return strings.Join(names, ", ")
}

func FetchSpotifyMeta(title, artist, albumHint string) (string, string, string, string, string, int) {
	token, err := getSpotifyAccessToken()
	if err != nil {
		log.Printf("[Spotify API Error] Token failure: %v", err)
		return "", "", "", "", "", 0
	}

	// Spotify query: combining track and artist for precision
	query := fmt.Sprintf("track:%s artist:%s", title, artist)
	if albumHint != "" && albumHint != "Unknown Album" {
		query += fmt.Sprintf(" album:%s", albumHint)
	}

	apiURL := "https://api.spotify.com/v1/search?q=" + url.QueryEscape(query) + "&type=track&limit=15"
	log.Printf("[Spotify API Request] URL: %s", apiURL)

	req, _ := http.NewRequest("GET", apiURL, nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := spotifyClient.Do(req)
	if err != nil {
		log.Printf("[Spotify API Error] Request failure: %v", err)
		return "", "", "", "", "", 0
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[Spotify API Error] Search http %d: %s", resp.StatusCode, string(body))
		return "", "", "", "", "", 0
	}
	if strings.Contains(string(body), "premium subscription required") {
		log.Printf("[Spotify Warning] 您当前的 Spotify 账号不是 Premium 会员，搜索接口受限。程序将自动切换至网易云搜刮模式。")
		return "", "", "", "", "", 0
	}

	var res SpotifySearchResp
	if err := json.Unmarshal(body, &res); err == nil && len(res.Tracks.Items) > 0 {
		log.Printf("[Spotify Candidates] count=%d for query=%q", len(res.Tracks.Items), query)
		for i, item := range res.Tracks.Items {
			if i >= 10 {
				log.Printf("[Spotify Candidates] ... %d more", len(res.Tracks.Items)-i)
				break
			}
			log.Printf("[Spotify Candidate %02d] artist=%q title=%q album=%q track=%d release=%q images=%d",
				i+1, spotifyArtistNames(item.Artists), item.Name, item.Album.Name, item.TrackNumber, item.Album.ReleaseDate, len(item.Album.Images))
		}
		targetTitle := strings.ToLower(title)
		targetArtist := strings.ToLower(artist)
		hint := strings.ToLower(albumHint)
		hasHint := (hint != "" && hint != "unknown album")

		var bestMatch *SpotifyMatchScore

		for _, item := range res.Tracks.Items {
			sTitle := strings.ToLower(item.Name)
			sAlbum := strings.ToLower(item.Album.Name)

			// 标题匹配度
			titleScore := 0.0
			if sTitle == targetTitle {
				titleScore = 1.0
			} else if strings.Contains(sTitle, targetTitle) || strings.Contains(targetTitle, sTitle) {
				titleScore = 0.7
			}

			// 艺术家匹配度
			artistScore := 0.0
			for _, a := range item.Artists {
				aName := strings.ToLower(a.Name)
				if aName == targetArtist {
					artistScore = 1.0
					break
				} else if strings.Contains(aName, targetArtist) || strings.Contains(targetArtist, aName) {
					if artistScore < 0.8 {
						artistScore = 0.8
					}
				}
			}

			// 专辑匹配度
			albumScore := 0.0
			if hasHint {
				if sAlbum == hint {
					albumScore = 1.0
				} else if strings.Contains(sAlbum, hint) {
					albumScore = 0.7
				} else if strings.Contains(hint, sAlbum) {
					albumScore = 0.5
				}
			}

			// 只有标题和艺术家都有匹配才考虑
			if titleScore < 0.5 || artistScore < 0.5 {
				continue
			}

			// 计算总分
			totalScore := 0.0
			if hasHint {
				totalScore = titleScore*0.35 + artistScore*0.35 + albumScore*0.30
			} else {
				totalScore = titleScore*0.5 + artistScore*0.5
			}

			cover := ""
			if len(item.Album.Images) > 0 {
				cover = item.Album.Images[0].URL
			}
			year := ""
			if len(item.Album.ReleaseDate) >= 4 {
				year = item.Album.ReleaseDate[:4]
			}

			current := &SpotifyMatchScore{
				Title:       item.Name,
				Artist:      item.Artists[0].Name,
				Album:       item.Album.Name,
				TrackNum:    item.TrackNumber,
				Cover:       cover,
				Year:        year,
				TitleScore:  titleScore,
				ArtistScore: artistScore,
				AlbumScore:  albumScore,
				TotalScore:  totalScore,
			}

			if current.TrackNum <= 0 {
				current.TrackNum = 1
			}

			// 选择得分最高的
			if bestMatch == nil || current.TotalScore > bestMatch.TotalScore {
				bestMatch = current
			}
		}

		if bestMatch != nil {
			log.Printf("[Spotify Match] Title: %.2f, Artist: %.2f, Album: %.2f, Total: %.2f -> %s - %s / %s",
				bestMatch.TitleScore, bestMatch.ArtistScore, bestMatch.AlbumScore, bestMatch.TotalScore,
				bestMatch.Artist, bestMatch.Title, bestMatch.Album)
			return bestMatch.Artist, bestMatch.Album, bestMatch.Title, bestMatch.Cover, bestMatch.Year, bestMatch.TrackNum
		}
	}

	log.Printf("[Spotify Match Failure] No suitable results for: %s", query)
	return "", "", "", "", "", 0
}

func TestSpotifySearch() error {
	artist, album, title, _, year, track := FetchSpotifyMeta("Pledging My Time", "Bob Dylan", "Blonde on Blonde")
	if artist == "" {
		return fmt.Errorf("spotify search returned no match")
	}
	log.Printf("[Spotify Test] OK: %s - %s / %s year=%s track=%d", artist, title, album, year, track)
	return nil
}
