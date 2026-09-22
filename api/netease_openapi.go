package api

import (
	"MusicConvert/config"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const neteaseOpenAPIBase = "https://openapi.music.163.com"

var (
	openAPIToken       string
	openAPITokenFailed bool
	openAPITokenLock   sync.Mutex
)

type openAPIArtist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type openAPIAlbum struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Artists []openAPIArtist `json:"artists"`
	PicUrl  string          `json:"picUrl"`
}

type openAPIAlbumSearchResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Records []openAPIAlbum `json:"records"`
	} `json:"data"`
}

type openAPISong struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Artists []openAPIArtist `json:"artists"`
	PicUrl  string          `json:"picUrl"`
	Album   openAPIAlbum    `json:"album"`
}

type openAPIAlbumSongListResp struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type openAPITokenResp struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		AccessToken string `json:"accessToken"`
	} `json:"data"`
}

func isOpenAPIEnabled() bool {
	if config.GetNeteaseAccessToken() != "" {
		return true
	}
	if config.GetNeteaseCookie() != "" {
		return false
	}
	mode := config.GetNeteaseAuthMode()
	return mode == "openapi" || (mode == "" && config.GetNeteaseAppID() != "" && config.GetNeteasePrivateKey() != "")
}

func importNeteasePrivateKey(raw string) (*rsa.PrivateKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("netease private key is empty")
	}
	if !strings.Contains(raw, "BEGIN") {
		cleaned := strings.Join(strings.Fields(raw), "")
		var b strings.Builder
		b.WriteString("-----BEGIN PRIVATE KEY-----\n")
		for i := 0; i < len(cleaned); i += 64 {
			end := i + 64
			if end > len(cleaned) {
				end = len(cleaned)
			}
			b.WriteString(cleaned[i:end])
			b.WriteByte('\n')
		}
		b.WriteString("-----END PRIVATE KEY-----")
		raw = b.String()
	}
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM private key")
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, fmt.Errorf("private key is not RSA")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func signOpenAPIParams(params map[string]string) (string, error) {
	var keys []string
	for key, value := range params {
		if key == "sign" || value == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var pairs []string
	for _, key := range keys {
		pairs = append(pairs, key+"="+params[key])
	}
	content := strings.Join(pairs, "&")
	privateKey, err := importNeteasePrivateKey(config.GetNeteasePrivateKey())
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(content))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, sum[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(signature), nil
}

func openAPIRequest(path string, biz map[string]any, accessToken string) ([]byte, error) {
	if !isOpenAPIEnabled() {
		return nil, fmt.Errorf("netease openapi disabled")
	}
	bizBytes, err := json.Marshal(biz)
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"appId":      config.GetNeteaseAppID(),
		"signType":   "RSA_SHA256",
		"timestamp":  strconv.FormatInt(time.Now().UnixMilli(), 10),
		"bizContent": string(bizBytes),
		"device":     config.GetNeteaseDevice(),
	}
	if accessToken != "" {
		params["accessToken"] = accessToken
	}
	if secret := config.GetNeteaseAppSecret(); secret != "" {
		params["appSecret"] = secret
	}
	sign, err := signOpenAPIParams(params)
	if err != nil {
		return nil, err
	}
	values := url.Values{}
	for key, value := range params {
		values.Set(key, value)
	}
	values.Set("sign", sign)
	reqURL := neteaseOpenAPIBase + path + "?" + values.Encode()
	resp, err := neteaseClient.Get(reqURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("netease openapi http %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func getOpenAPIAccessToken() (string, error) {
	if token := config.GetNeteaseAccessToken(); token != "" {
		log.Printf("[Netease OpenAPI] Using configured access token")
		return token, nil
	}
	openAPITokenLock.Lock()
	defer openAPITokenLock.Unlock()
	if openAPIToken != "" {
		log.Printf("[Netease OpenAPI] Using cached anonymous access token")
		return openAPIToken, nil
	}
	if openAPITokenFailed {
		return "", fmt.Errorf("netease openapi anonymous token previously failed")
	}
	log.Printf("[Netease OpenAPI] Requesting anonymous access token")
	body, err := openAPIRequest("/openapi/music/basic/oauth2/login/anonymous", map[string]any{
		"clientId": config.GetNeteaseAppID(),
	}, "")
	if err != nil {
		openAPITokenFailed = true
		return "", err
	}
	var res openAPITokenResp
	if err := json.Unmarshal(body, &res); err != nil {
		return "", err
	}
	if res.Code != 200 || res.Data.AccessToken == "" {
		openAPITokenFailed = true
		return "", fmt.Errorf("netease openapi anonymous token failed: code=%d message=%s body=%s", res.Code, res.Message, string(body))
	}
	openAPIToken = res.Data.AccessToken
	return openAPIToken, nil
}

func searchOpenAPIAlbumID(artist, album string) (string, string, string, error) {
	token, err := getOpenAPIAccessToken()
	if err != nil {
		return "", "", "", err
	}
	keyword := strings.TrimSpace(artist + " " + album)
	log.Printf("[Netease OpenAPI] Searching album: %s", keyword)
	body, err := openAPIRequest("/openapi/music/basic/search/album/get/v2", map[string]any{
		"keyword": keyword,
		"limit":   20,
		"offset":  0,
	}, token)
	if err != nil {
		return "", "", "", err
	}
	var res openAPIAlbumSearchResp
	if err := json.Unmarshal(body, &res); err != nil {
		return "", "", "", err
	}
	if res.Code != 200 {
		return "", "", "", fmt.Errorf("netease openapi album search failed: code=%d message=%s", res.Code, res.Message)
	}
	targetAlbum := normalizeSongName(album)
	targetArtist := normalizeSongName(artist)
	for _, record := range res.Data.Records {
		recordAlbum := normalizeSongName(record.Name)
		if recordAlbum != targetAlbum && !strings.Contains(recordAlbum, targetAlbum) && !strings.Contains(targetAlbum, recordAlbum) {
			continue
		}
		if targetArtist == "" || len(record.Artists) == 0 {
			return record.ID, record.Name, record.PicUrl, nil
		}
		for _, a := range record.Artists {
			recordArtist := normalizeSongName(a.Name)
			if recordArtist == targetArtist || strings.Contains(recordArtist, targetArtist) || strings.Contains(targetArtist, recordArtist) {
				return record.ID, record.Name, record.PicUrl, nil
			}
		}
	}
	if len(res.Data.Records) > 0 {
		return res.Data.Records[0].ID, res.Data.Records[0].Name, res.Data.Records[0].PicUrl, nil
	}
	return "", "", "", fmt.Errorf("netease openapi album not found: %s / %s", artist, album)
}

func fetchOpenAPIAlbumSongs(albumID string) ([]openAPISong, error) {
	token, err := getOpenAPIAccessToken()
	if err != nil {
		return nil, err
	}
	log.Printf("[Netease OpenAPI] Fetching album song list: %s", albumID)
	body, err := openAPIRequest("/openapi/music/basic/album/song/list/get/v2", map[string]any{
		"albumId":     albumID,
		"qualityFlag": true,
	}, token)
	if err != nil {
		return nil, err
	}
	var res openAPIAlbumSongListResp
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("netease openapi album songs failed: code=%d message=%s", res.Code, res.Message)
	}
	var songs []openAPISong
	if err := json.Unmarshal(res.Data, &songs); err == nil {
		return songs, nil
	}
	var wrapped struct {
		Records []openAPISong `json:"records"`
	}
	if err := json.Unmarshal(res.Data, &wrapped); err != nil {
		return nil, err
	}
	return wrapped.Records, nil
}

func GetOpenAPIAlbumTrackTitles(artist, album string) (map[int]string, string, error) {
	albumID, albumName, _, err := searchOpenAPIAlbumID(artist, album)
	if err != nil {
		return nil, "", err
	}
	songs, err := fetchOpenAPIAlbumSongs(albumID)
	if err != nil {
		return nil, "", err
	}
	tracks := make(map[int]string)
	for i, song := range songs {
		if strings.TrimSpace(song.Name) != "" {
			tracks[i+1] = song.Name
		}
	}
	if len(tracks) == 0 {
		return nil, "", fmt.Errorf("netease openapi album songs empty: %s", albumID)
	}
	log.Printf("[Netease OpenAPI] album=%s id=%s tracks=%d", albumName, albumID, len(tracks))
	return tracks, "openapi", nil
}

func TestSearch(artist, album string) (string, string, string, error) {
    return searchOpenAPIAlbumID(artist, album)
}
