package api

import (
	"MusicConvert/config"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
)

var neteaseClient = &http.Client{
	Timeout: 10 * time.Second,
}

var (
	albumDetailCache = make(map[int]*NeteaseAlbumDetailResp)
	albumTrackCache  = make(map[string]map[int]string)
	albumDetailLock  sync.Mutex
	albumYearPrefix  = regexp.MustCompile(`^\s*(19|20)\d{2}\s*[-_. ]+\s*`)
)

type NeteaseSearchResp struct {
	Result struct {
		Songs []struct {
			Id      int    `json:"id"`
			Name    string `json:"name"`
			No      int    `json:"no"`
			Artists []struct {
				Id   int    `json:"id"`
				Name string `json:"name"`
			} `json:"artists"`
			Album struct {
				Id     int    `json:"id"`
				Name   string `json:"name"`
				Size   int    `json:"size"`
				Artist struct {
					Name       string   `json:"name"`
					TransNames []string `json:"transNames"`
				} `json:"artist"`
			} `json:"album"`
		} `json:"songs"`
	} `json:"result"`
}

type NeteaseArtistSearchResp struct {
	Result struct {
		Artists []struct {
			Id         int      `json:"id"`
			Name       string   `json:"name"`
			TransNames []string `json:"transNames"`
			Trans      string   `json:"trans"`
			Alias      []string `json:"alias"`
		} `json:"artists"`
	} `json:"result"`
}

type NeteaseLyricResp struct {
	Lrc struct {
		Lyric string `json:"lyric"`
	} `json:"lrc"`
	Tlyric struct {
		Lyric string `json:"lyric"`
	} `json:"tlyric"`
}

type NeteaseAlbumSong struct {
	No   int    `json:"no"`
	Name string `json:"name"`
	Id   int    `json:"id"`
}

type NeteaseAlbumDetailResp struct {
	Album struct {
		Id     int                `json:"id"`
		Name   string             `json:"name"`
		PicUrl string             `json:"picUrl"`
		Size   int                `json:"size"`
		Songs  []NeteaseAlbumSong `json:"songs"`
	} `json:"album"`
	Songs []NeteaseAlbumSong `json:"songs"`
}

type NeteaseAlbumSearchResp struct {
	Result struct {
		Albums []struct {
			Id          int    `json:"id"`
			Name        string `json:"name"`
			PicUrl      string `json:"picUrl"`
			PublishTime int64  `json:"publishTime"`
			Size        int    `json:"size"`
			Artist      struct {
				Name       string   `json:"name"`
				TransNames []string `json:"transNames"`
				Trans      string   `json:"trans"`
			} `json:"artist"`
		} `json:"albums"`
	} `json:"result"`
}

// GetAlbumDetail 获取专辑详情，包含正确的音轨号
func GetAlbumDetail(albumID int) (*NeteaseAlbumDetailResp, error) {
	if albumID <= 0 {
		return nil, fmt.Errorf("invalid album id")
	}

	albumDetailLock.Lock()
	if cached := albumDetailCache[albumID]; cached != nil {
		albumDetailLock.Unlock()
		return cached, nil
	}
	albumDetailLock.Unlock()

	apiURL := fmt.Sprintf("http://music.163.com/api/album/%d", albumID)
	body, err := doRequestWithRetry(apiURL)
	if err != nil {
		return nil, err
	}
	if strings.Contains(string(body), `"code":-462`) || strings.Contains(string(body), "请绑定手机") {
		return nil, fmt.Errorf("netease verification required for albumID=%d", albumID)
	}
	var res NeteaseAlbumDetailResp
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, err
	}
	if len(albumSongs(&res)) == 0 {
		raw := string(body)
		if len(raw) > 1200 {
			raw = raw[:1200]
		}
		log.Printf("[Netease Album Detail] Empty song list: albumID=%d album=%q size=%d albumSongs=%d topSongs=%d raw=%s",
			albumID, res.Album.Name, res.Album.Size, len(res.Album.Songs), len(res.Songs), raw)
	} else {
		log.Printf("[Netease Album Detail] albumID=%d album=%q size=%d songs=%v",
			albumID, res.Album.Name, res.Album.Size, sampleAlbumSongNames(albumSongs(&res), 20))
	}

	albumDetailLock.Lock()
	albumDetailCache[albumID] = &res
	albumDetailLock.Unlock()

	return &res, nil
}

func albumSongs(detail *NeteaseAlbumDetailResp) []NeteaseAlbumSong {
	if detail == nil {
		return nil
	}
	if len(detail.Album.Songs) > 0 {
		return detail.Album.Songs
	}
	return detail.Songs
}

func sampleAlbumSongNames(songs []NeteaseAlbumSong, limit int) []string {
	if limit <= 0 || limit > len(songs) {
		limit = len(songs)
	}
	names := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		names = append(names, fmt.Sprintf("%02d:%s(no=%d,id=%d)", i+1, songs[i].Name, songs[i].No, songs[i].Id))
	}
	return names
}

func albumTrackCacheKeys(artist, album string) []string {
	canonicalArtist := normalizeSongName(GetArtistCanonicalName(artist))
	rawArtist := normalizeSongName(artist)
	cleanAlbum := normalizeSongName(CleanHint(album))
	rawAlbum := normalizeSongName(album)
	keys := []string{
		canonicalArtist + "|" + cleanAlbum,
		canonicalArtist + "|" + rawAlbum,
		rawArtist + "|" + cleanAlbum,
		rawArtist + "|" + rawAlbum,
		cleanAlbum,
		rawAlbum,
	}
	seen := make(map[string]struct{})
	var unique []string
	for _, key := range keys {
		if key == "" || key == "|" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		unique = append(unique, key)
	}
	return unique
}

func cacheAlbumTrackTitles(artist, album string, songs []NeteaseAlbumSong) {
	if len(songs) == 0 {
		return
	}
	tracks := make(map[int]string)
	for i, song := range songs {
		if strings.TrimSpace(song.Name) != "" {
			tracks[i+1] = song.Name
		}
	}
	if len(tracks) == 0 {
		return
	}
	albumDetailLock.Lock()
	for _, key := range albumTrackCacheKeys(artist, album) {
		albumTrackCache[key] = tracks
	}
	albumDetailLock.Unlock()
}

func cachedAlbumTrackTitles(artist, album string) map[int]string {
	albumDetailLock.Lock()
	defer albumDetailLock.Unlock()
	for _, key := range albumTrackCacheKeys(artist, album) {
		if tracks := albumTrackCache[key]; len(tracks) > 0 {
			return tracks
		}
	}
	return nil
}

func GetAlbumTrackTitlesWithSource(artist, album string) (map[int]string, string) {
	tracks := make(map[int]string)
	if overrideTracks, ok := config.LookupTrackOverrideList(artist, album); ok {
		log.Printf("[Album Tracks] Using track_overrides for %s / %s", artist, album)
		for i, title := range overrideTracks {
			tracks[i+1] = title
		}
		return tracks, "track_overrides"
	}
	if isOpenAPIEnabled() {
		if openAPITracks, source, err := GetOpenAPIAlbumTrackTitles(artist, album); err == nil && len(openAPITracks) > 0 {
			return openAPITracks, source
		} else if err != nil {
			log.Printf("[Album Tracks] Netease OpenAPI failed for %s / %s: %v", artist, album, err)
		}
	}
	if cached := cachedAlbumTrackTitles(artist, album); len(cached) > 0 {
		log.Printf("[Album Tracks] Using cached track titles for %s / %s", artist, album)
		return cached, "cache"
	}

	albumID := FindAlbumID(artist, album)
	if albumID <= 0 {
		return tracks, "empty: album id not found"
	}
	legacyAuth := "anonymous"
	if config.GetNeteaseCookie() != "" {
		legacyAuth = "cookie"
	}
	log.Printf("[Album Tracks] Using legacy Netease API (%s) for %s / %s (albumID=%d)", legacyAuth, artist, album, albumID)
	detail, err := GetAlbumDetail(albumID)
	if err != nil {
		log.Printf("[Album Tracks] Failed to fetch album detail for %s / %s: %v", artist, album, err)
		return tracks, "empty: " + err.Error()
	}
	songs := albumSongs(detail)
	cacheAlbumTrackTitles(artist, album, songs)
	for i, song := range songs {
		if strings.TrimSpace(song.Name) != "" {
			tracks[i+1] = song.Name
		}
	}
	if len(tracks) == 0 {
		return tracks, "empty: album detail has no songs"
	}
	return tracks, "netease"
}

func GetAlbumTrackTitles(artist, album string) map[int]string {
	tracks, _ := GetAlbumTrackTitlesWithSource(artist, album)
	return tracks
}

func doRequestWithRetry(apiURL string) ([]byte, error) {
	var body []byte
	var err error
	for i := 0; i < 3; i++ {
		req, reqErr := http.NewRequest("GET", apiURL, nil)
		if reqErr != nil {
			return nil, reqErr
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125.0 Safari/537.36")
		req.Header.Set("Referer", "https://music.163.com/")
		req.Header.Set("Accept", "application/json, text/plain, */*")
		if cookie := strings.TrimSpace(config.GetNeteaseCookie()); cookie != "" {
			req.Header.Set("Cookie", cookie)
		}

		resp, reqErr := neteaseClient.Do(req)
		if reqErr != nil {
			err = reqErr
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		body, err = ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			continue
		}
		if strings.Contains(string(body), "操作频繁") {
			err = fmt.Errorf("netease rate limited")
			time.Sleep(time.Duration(i+2) * time.Second)
			continue
		}
		if strings.Contains(string(body), `"code":-462`) || strings.Contains(string(body), "请绑定手机") {
			err = fmt.Errorf("netease verification required")
			time.Sleep(time.Duration(i+1) * time.Second)
			continue
		}
		return body, nil
	}
	return nil, err
}

func containsChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

func ContainsChinese(s string) bool {
	return containsChinese(s)
}

func GetAlbumTrackCount(albumID int) int {
	if albumID <= 0 {
		return 0
	}
	apiURL := fmt.Sprintf("http://music.163.com/api/album/%d", albumID)
	body, err := doRequestWithRetry(apiURL)
	if err != nil {
		return 0
	}
	var res struct {
		Album struct {
			Size int `json:"size"`
		} `json:"album"`
	}
	if err := json.Unmarshal(body, &res); err == nil {
		return res.Album.Size
	}
	return 0
}

func FindAlbumID(artist, album string) int {
	album = CleanHint(album)
	if album == "" || album == "Unknown Album" {
		return 0
	}
	query := album
	if artist != "" && artist != "Unknown Artist" {
		query = artist + " " + album
	}
	apiURL := fmt.Sprintf("http://music.163.com/api/search/get/web?s=%s&type=10&offset=0&limit=10", url.QueryEscape(query))
	body, err := doRequestWithRetry(apiURL)
	if err != nil {
		return 0
	}
	var res NeteaseAlbumSearchResp
	if err := json.Unmarshal(body, &res); err != nil {
		return 0
	}
	targetAlbum := normalizeSongName(album)
	targetArtist := normalizeSongName(artist)
	for _, a := range res.Result.Albums {
		albumName := normalizeSongName(a.Name)
		artistName := normalizeSongName(a.Artist.Name)
		if albumName != targetAlbum && !strings.Contains(albumName, targetAlbum) && !strings.Contains(targetAlbum, albumName) {
			continue
		}
		if targetArtist == "" || artistName == "" || artistName == targetArtist || strings.Contains(artistName, targetArtist) || strings.Contains(targetArtist, artistName) {
			return a.Id
		}
	}
	return 0
}

func FilterJunkTags(val string) string {
	junks := []string{"http", "www", ".com", ".cn", "分享", "下载", "群", "公众号", "制作", "【", "】", "letians"}
	v := strings.ToLower(val)
	for _, junk := range junks {
		if strings.Contains(v, junk) {
			return ""
		}
	}
	return val
}

func FormatName(original string, searchType int) string {
	original = strings.TrimSpace(original)
	if original == "" {
		return "Unknown"
	}
	if containsChinese(original) {
		return original
	}
	apiURL := fmt.Sprintf("http://music.163.com/api/search/get/web?s=%s&type=%d&offset=0&limit=1", url.QueryEscape(original), searchType)
	body, err := doRequestWithRetry(apiURL)
	if err != nil {
		return original
	}
	var res struct {
		Result struct {
			Artists []struct {
				Name       string   `json:"name"`
				TransNames []string `json:"transNames"`
			} `json:"artists"`
		} `json:"result"`
	}
	if err := json.Unmarshal(body, &res); err == nil && len(res.Result.Artists) > 0 {
		a := res.Result.Artists[0]
		if containsChinese(a.Name) {
			return a.Name
		}
		if len(a.TransNames) > 0 {
			return fmt.Sprintf("%s-%s", a.TransNames[0], a.Name)
		}
	}
	return original
}

func SplitArtists(artistStr string) []string {
	separators := []string{" / ", "/", " & ", "&", " 、 ", "、", " vs ", " VS ", " feat. ", " ft. ", " - ", "-", " feat ", " ft "}
	res := []string{artistStr}
	for _, sep := range separators {
		var nextRes []string
		for _, s := range res {
			parts := strings.Split(s, sep)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					nextRes = append(nextRes, p)
				}
			}
		}
		res = nextRes
	}
	return res
}

func GetArtistCanonicalName(artist string) string {
	artist = strings.TrimSpace(artist)
	if artist == "" {
		return "Unknown Artist"
	}
	if containsChinese(artist) {
		return artist
	}
	primary := SplitArtists(artist)[0]
	apiURL := fmt.Sprintf("http://music.163.com/api/search/get/web?s=%s&type=100&offset=0&limit=5", url.QueryEscape(primary))
	body, err := doRequestWithRetry(apiURL)
	if err == nil {
		var res NeteaseArtistSearchResp
		if err := json.Unmarshal(body, &res); err == nil && len(res.Result.Artists) > 0 {
			a := res.Result.Artists[0]
			trans := ""
			if len(a.TransNames) > 0 {
				trans = a.TransNames[0]
			}
			if trans == "" {
				trans = a.Trans
			}
			if trans != "" && containsChinese(trans) {
				return fmt.Sprintf("%s-%s", trans, a.Name)
			}
		}
	}
	return artist
}

func CleanHint(hint string) string {
	hint = strings.TrimSpace(hint)
	hint = albumYearPrefix.ReplaceAllString(hint, "")
	// 移除末尾的 (2021) 或 [2021] 格式
	if idx := strings.LastIndex(hint, "("); idx > 0 {
		hint = hint[:idx]
	}
	if idx := strings.LastIndex(hint, "["); idx > 0 {
		hint = hint[:idx]
	}
	if idx := strings.Index(hint, " "); idx > 0 && containsChinese(hint[:idx]) && !containsChinese(hint[idx+1:]) {
		hint = strings.TrimSpace(hint[:idx]) + "(" + strings.TrimSpace(hint[idx+1:]) + ")"
	}
	return strings.TrimSpace(hint)
}

func PreferDetailedAlbumName(apiAlbum, hint string) string {
	apiAlbum = CleanHint(apiAlbum)
	hint = CleanHint(hint)
	if hint == "" || hint == "Unknown Album" {
		return apiAlbum
	}
	if apiAlbum == "" || apiAlbum == "Unknown Album" {
		return hint
	}
	if normalizeSongName(apiAlbum) == normalizeSongName(hint) {
		return hint
	}
	if strings.Contains(hint, "(") && normalizeSongName(apiAlbum) != normalizeSongName(hint) && strings.Contains(normalizeSongName(hint), normalizeSongName(apiAlbum)) {
		return hint
	}
	return apiAlbum
}

func IdentifyFullContext(title, artist, albumHint string) (string, string, string, int, int) {
	albumHint = CleanHint(albumHint)
	preferredAlbumID := FindAlbumID(artist, albumHint)
	query := fmt.Sprintf("%s %s", title, artist)
	if albumHint != "" && albumHint != "Unknown Album" {
		query += " " + albumHint
	}

	// 第一次搜索：使用完整查询（包括专辑）
	officialArtist, officialAlbum, officialTitle, trackNo, total := searchSongAndMatch(query, title, artist, albumHint, preferredAlbumID)
	if officialAlbum != "" && officialAlbum != "Unknown Album" {
		return officialArtist, officialAlbum, officialTitle, trackNo, total
	}

	// 第二次搜索：只使用标题和艺术家（可能专辑名称不准确）
	if !strings.Contains(title, "Unknown") {
		officialArtist, officialAlbum, officialTitle, trackNo, total = searchSongAndMatch(title, title, artist, "", preferredAlbumID)
		if officialAlbum != "" && officialAlbum != "Unknown Album" {
			return officialArtist, officialAlbum, officialTitle, trackNo, total
		}
	}

	// Fallback: 使用原始信息，但标准化艺术家名称
	canonicalArtist := GetArtistCanonicalName(artist)
	if albumHint == "" || albumHint == "Unknown Album" {
		// 尝试搜索艺术家来获取专辑信息
		return canonicalArtist, "Unknown Album", title, 1, 0
	}
	return canonicalArtist, albumHint, title, 1, 0
}

type MatchScore struct {
	Artist      string
	Album       string
	Title       string
	AlbumID     int
	TrackNo     int
	Total       int
	TitleScore  float64
	ArtistScore float64
	AlbumScore  float64
	TotalScore  float64
}

func normalizeSongName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	remove := " \t\r\n　-_.·・,，.。:：;；!！?？'\"“”‘’()（）[]【】{}《》<>〈〉"
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(remove, r) {
			return -1
		}
		return r
	}, s)
}

func resolveAlbumTrackNumber(albumID int, artist, album, title string, fallbackTrack, fallbackTotal int) (int, int) {
	trackNo := fallbackTrack
	total := fallbackTotal
	if albumID <= 0 {
		if trackNo <= 0 {
			trackNo = 1
		}
		return trackNo, total
	}

	detail, err := GetAlbumDetail(albumID)
	songs := albumSongs(detail)
	if err != nil || detail == nil || len(songs) == 0 {
		log.Printf("[Netease Track Fix] Album detail unavailable for albumID=%d title=%s: %v", albumID, title, err)
		if trackNo <= 0 {
			trackNo = 1
		}
		return trackNo, total
	}

	if len(songs) > 0 {
		total = len(songs)
		cacheAlbumTrackTitles(artist, album, songs)
	}
	targetTitle := normalizeSongName(title)
	for i, song := range songs {
		songName := normalizeSongName(song.Name)
		if songName == targetTitle || strings.Contains(songName, targetTitle) || strings.Contains(targetTitle, songName) {
			trackNo = i + 1
			log.Printf("[Netease Track Fix] Got track number from album order: %d (api no: %d, fallback: %d)", trackNo, song.No, fallbackTrack)
			return trackNo, total
		}
	}

	log.Printf("[Netease Track Fix] Title not found in album detail: albumID=%d title=%s songs=%v", albumID, title, sampleAlbumSongNames(songs, 20))

	if trackNo <= 0 {
		trackNo = 1
	}
	return trackNo, total
}

func searchSongAndMatch(query, targetTitle, targetArtist, albumHint string, preferredAlbumID int) (string, string, string, int, int) {
	albumHint = CleanHint(albumHint)
	apiURL := fmt.Sprintf("http://music.163.com/api/search/get/web?s=%s&type=1&offset=0&limit=15", url.QueryEscape(query))
	body, err := doRequestWithRetry(apiURL)
	if err != nil {
		return "", "", "", 0, 0
	}
	var res NeteaseSearchResp
	if err := json.Unmarshal(body, &res); err != nil || len(res.Result.Songs) == 0 {
		return "", "", "", 0, 0
	}

	targetTitle = strings.ToLower(targetTitle)
	targetArtist = strings.ToLower(targetArtist)
	hint := strings.ToLower(albumHint)

	var bestMatch *MatchScore
	hasHint := (hint != "" && hint != "unknown album")

	for _, s := range res.Result.Songs {
		songName := strings.ToLower(s.Name)
		albumName := strings.ToLower(s.Album.Name)

		// 计算标题匹配度
		titleScore := 0.0
		if songName == targetTitle {
			titleScore = 1.0
		} else if strings.Contains(songName, targetTitle) || strings.Contains(targetTitle, songName) {
			titleScore = 0.7
		}

		// 计算艺术家匹配度
		artistScore := 0.0
		for _, a := range s.Artists {
			aName := strings.ToLower(a.Name)
			if aName == targetArtist {
				artistScore = 1.0
				break
			} else if strings.Contains(aName, targetArtist) || strings.Contains(targetArtist, aName) {
				if artistScore < 0.8 {
					artistScore = 0.8
				}
			}
			// 同时检查主艺术家
			primary := strings.ToLower(SplitArtists(targetArtist)[0])
			if aName == primary {
				artistScore = 0.95
			}
		}

		// 专辑匹配 - 增加权重
		albumScore := 0.0
		if hasHint {
			if albumName == hint {
				albumScore = 1.0 // 精确匹配最高分
			} else if strings.Contains(albumName, hint) {
				albumScore = 0.6
			} else if strings.Contains(hint, albumName) {
				albumScore = 0.5
			}
		}
		if hasHint && albumScore < 0.8 {
			log.Printf("[Netease Match Skip] album mismatch: title=%s candidateAlbum=%s hint=%s score=%.2f", s.Name, s.Album.Name, albumHint, albumScore)
			continue
		}

		// 只有标题和艺术家都有匹配才考虑
		if titleScore < 0.5 || artistScore < 0.5 {
			continue
		}

		// 计算总分 - 根据是否有专辑提示调整权重
		totalScore := 0.0
		if hasHint {
			// 有专辑提示时，专辑匹配权重提高
			totalScore = titleScore*0.35 + artistScore*0.35 + albumScore*0.30
		} else {
			totalScore = titleScore*0.5 + artistScore*0.5
		}
		if preferredAlbumID > 0 && s.Album.Id == preferredAlbumID {
			totalScore += 0.2
			albumScore = 1.0
		}

		current := &MatchScore{
			Title:       s.Name,
			Artist:      s.Album.Artist.Name,
			Album:       s.Album.Name,
			AlbumID:     s.Album.Id,
			TrackNo:     s.No,
			Total:       s.Album.Size,
			TitleScore:  titleScore,
			ArtistScore: artistScore,
			AlbumScore:  albumScore,
			TotalScore:  totalScore,
		}

		// 设置albumArtist
		if current.Artist == "" && len(s.Artists) > 0 {
			current.Artist = s.Artists[0].Name
		}
		if current.Total <= 0 {
			current.Total = GetAlbumTrackCount(s.Album.Id)
		}
		if current.TrackNo <= 0 {
			current.TrackNo = 1
		}

		// 选择得分最高的
		if bestMatch == nil || current.TotalScore > bestMatch.TotalScore {
			bestMatch = current
		}
	}
	if bestMatch != nil {
		if overrideTrack, overrideTotal, ok := config.LookupTrackOverride(bestMatch.Artist, bestMatch.Album, bestMatch.Title); ok {
			log.Printf("[Track Override] %s / %s / %s -> Track #%d/%d", bestMatch.Artist, bestMatch.Album, bestMatch.Title, overrideTrack, overrideTotal)
			return GetArtistCanonicalName(bestMatch.Artist), bestMatch.Album, bestMatch.Title, overrideTrack, overrideTotal
		}

		albumID := bestMatch.AlbumID
		if preferredAlbumID > 0 {
			albumID = preferredAlbumID
		}
		correctTrackNo, total := resolveAlbumTrackNumber(albumID, bestMatch.Artist, bestMatch.Album, bestMatch.Title, bestMatch.TrackNo, bestMatch.Total)

		log.Printf("[Netease Match] Title: %.2f, Artist: %.2f, Album: %.2f, Total: %.2f -> %s - %s / %s (Track #%d/%d)",
			bestMatch.TitleScore, bestMatch.ArtistScore, bestMatch.AlbumScore, bestMatch.TotalScore,
			bestMatch.Artist, bestMatch.Title, bestMatch.Album, correctTrackNo, total)
		return GetArtistCanonicalName(bestMatch.Artist), bestMatch.Album, bestMatch.Title, correctTrackNo, total
	}

	return "", "", "", 0, 0
}

func GetTranslatedArtistAlbum(artist, album string) (string, string, string, string, string) {
	canonical := GetArtistCanonicalName(artist)
	albumName := album
	coverUrl := ""
	yearStr := ""

	albumID := FindAlbumID(artist, album)
	if albumID <= 0 && canonical != artist {
		albumID = FindAlbumID(canonical, album)
	}
	if albumID <= 0 {
		log.Printf("[Album Cover] No validated album match for %s / %s; skip cover fallback", artist, album)
		return canonical, canonical, albumName, coverUrl, yearStr
	}

	detail, err := GetAlbumDetail(albumID)
	if err != nil {
		log.Printf("[Album Cover] Failed to fetch validated album detail for %s / %s (albumID=%d): %v", artist, album, albumID, err)
		return canonical, canonical, albumName, coverUrl, yearStr
	}
	albumName = PreferDetailedAlbumName(detail.Album.Name, album)
	coverUrl = detail.Album.PicUrl

	apiURL := fmt.Sprintf("http://music.163.com/api/search/get/web?s=%s&type=10&offset=0&limit=10", url.QueryEscape(strings.TrimSpace(artist+" "+album)))
	body, err := doRequestWithRetry(apiURL)
	if err == nil {
		var res NeteaseAlbumSearchResp
		if err := json.Unmarshal(body, &res); err == nil {
			for _, a := range res.Result.Albums {
				if a.Id == albumID && a.PublishTime > 0 {
					yearStr = fmt.Sprintf("%d", time.Unix(a.PublishTime/1000, 0).Year())
					break
				}
			}
		}
	}
	return canonical, canonical, albumName, coverUrl, yearStr
}

func GetSongLyrics(title, artist string) string {
	query := fmt.Sprintf("%s %s", title, artist)
	apiURL := fmt.Sprintf("http://music.163.com/api/search/get/web?s=%s&type=1&offset=0&limit=1", url.QueryEscape(query))
	body, err := doRequestWithRetry(apiURL)
	if err != nil {
		return ""
	}
	var res NeteaseSearchResp
	if err := json.Unmarshal(body, &res); err == nil && len(res.Result.Songs) > 0 {
		lyricURL := fmt.Sprintf("http://music.163.com/api/song/lyric?id=%d&lv=1&kv=1&tv=-1", res.Result.Songs[0].Id)
		lBody, lErr := doRequestWithRetry(lyricURL)
		if lErr == nil {
			var lRes NeteaseLyricResp
			if err := json.Unmarshal(lBody, &lRes); err == nil {
				orig := strings.TrimSpace(lRes.Lrc.Lyric)
				trans := strings.TrimSpace(lRes.Tlyric.Lyric)
				if trans != "" && orig != "" {
					return orig + "\n" + trans
				}
				return orig
			}
		}
	}
	return ""
}
