package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var wikipediaClient = &http.Client{Timeout: 15 * time.Second}

func FetchWikipediaClassicalTrackTitles(work, composer string) (map[int]string, string) {
	work = strings.TrimSpace(work)
	composer = strings.TrimSpace(composer)
	if work == "" {
		return nil, "empty: wikipedia work missing"
	}
	queries := []string{strings.TrimSpace(work + " " + composer + " 曲目"), strings.TrimSpace(work + " " + composer), work}
	for _, query := range queries {
		if query == "" {
			continue
		}
		title := wikipediaSearchTitle(query)
		if title == "" {
			continue
		}
		extract := wikipediaPageExtract(title)
		tracks := parseWikipediaClassicalStructure(extract)
		if len(tracks) > 0 {
			log.Printf("[Wikipedia Tracks] Using zh.wikipedia page %q for %q (%d tracks)", title, query, len(tracks))
			return tracks, "wikipedia:" + title
		}
	}
	return nil, "empty: wikipedia tracks not found"
}

func wikipediaSearchTitle(query string) string {
	apiURL := "https://zh.wikipedia.org/w/api.php?action=opensearch&namespace=0&limit=1&format=json&search=" + url.QueryEscape(query)
	body, err := wikipediaGet(apiURL)
	if err != nil {
		log.Printf("[Wikipedia Tracks] search failed for %q: %v", query, err)
		return ""
	}
	var res []json.RawMessage
	if err := json.Unmarshal(body, &res); err != nil || len(res) < 2 {
		return ""
	}
	var titles []string
	if err := json.Unmarshal(res[1], &titles); err != nil || len(titles) == 0 {
		return ""
	}
	return strings.TrimSpace(titles[0])
}

func wikipediaPageExtract(title string) string {
	values := url.Values{}
	values.Set("action", "query")
	values.Set("prop", "extracts")
	values.Set("explaintext", "1")
	values.Set("redirects", "1")
	values.Set("format", "json")
	values.Set("titles", title)
	body, err := wikipediaGet("https://zh.wikipedia.org/w/api.php?" + values.Encode())
	if err != nil {
		log.Printf("[Wikipedia Tracks] extract failed for %q: %v", title, err)
		return ""
	}
	var res struct {
		Query struct {
			Pages map[string]struct {
				Extract string `json:"extract"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(body, &res); err != nil {
		return ""
	}
	for _, page := range res.Query.Pages {
		return page.Extract
	}
	return ""
}

func wikipediaGet(apiURL string) ([]byte, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "MusicConvertTool/1.2.0 ( https://github.com/alisonjoe/MusicConvert )")
	resp, err := wikipediaClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("wikipedia http %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func parseWikipediaClassicalStructure(extract string) map[int]string {
	lines := strings.Split(extract, "\n")
	inStructure := false
	currentScene := ""
	tracks := make(map[int]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.Contains(line, "架構") || strings.Contains(line, "结构") || strings.Contains(line, "曲目") {
			inStructure = true
			continue
		}
		if !inStructure {
			continue
		}
		if strings.HasPrefix(line, "==") || strings.HasPrefix(line, "參考") || strings.HasPrefix(line, "参考") || strings.HasPrefix(line, "參見") || strings.HasPrefix(line, "参见") {
			if len(tracks) > 0 {
				break
			}
			continue
		}
		if scene := wikipediaSceneLabel(line); scene != "" {
			currentScene = scene
			continue
		}
		parts := numberedWikipediaLineParts(line, currentScene)
		for _, part := range parts {
			tracks[len(tracks)+1] = part
		}
	}
	if len(tracks) == 0 {
		return nil
	}
	return tracks
}

var wikipediaTrackLinePattern = regexp.MustCompile(`^\s*\d+[.．、]\s*(.+)$`)

func wikipediaSceneLabel(line string) string {
	title := strings.TrimSpace(line)
	if matches := wikipediaTrackLinePattern.FindStringSubmatch(line); len(matches) >= 2 {
		title = strings.TrimSpace(matches[1])
	}
	if strings.HasPrefix(title, "第") && (strings.HasSuffix(title, "景") || strings.HasSuffix(title, "场")) {
		return strings.TrimSuffix(strings.TrimSuffix(title, "景"), "场") + "场"
	}
	return ""
}

func numberedWikipediaLineParts(line, scene string) []string {
	matches := wikipediaTrackLinePattern.FindStringSubmatch(line)
	if len(matches) < 2 {
		return nil
	}
	title := strings.TrimSpace(matches[1])
	if title == "" || wikipediaSceneLabel(line) != "" {
		return nil
	}
	title = strings.ReplaceAll(title, "：", ":")
	title = strings.ReplaceAll(title, "，", ",")
	title = strings.ReplaceAll(title, "－", "-")
	title = strings.ReplaceAll(title, "—", "-")
	var parts []string
	for _, part := range strings.Split(title, "-") {
		part = cleanWikipediaTrackTitle(part)
		if part != "" {
			if len(parts) > 0 && strings.Contains(part, "黑暗") {
				parts[len(parts)-1] += "；" + part
				continue
			}
			if scene != "" && part != "引子" && !strings.HasPrefix(part, scene) {
				part = scene + ": " + part
			}
			parts = append(parts, part)
		}
	}
	return parts
}

func cleanWikipediaTrackTitle(title string) string {
	title = strings.TrimSpace(title)
	title = strings.Trim(title, " .。;；,，")
	title = strings.ReplaceAll(title, "伊凡·沙列維奇", "伊凡王子")
	title = strings.ReplaceAll(title, "伊凡·沙列维奇", "伊凡王子")
	title = strings.ReplaceAll(title, "卡什伊", "卡斯切伊")
	title = strings.ReplaceAll(title, "火鳥", "火鸟")
	title = strings.ReplaceAll(title, "登場", "登场")
	title = strings.ReplaceAll(title, "遊戲", "游戏")
	title = strings.ReplaceAll(title, "覺醒", "觉醒")
	title = strings.ReplaceAll(title, "覺", "觉")
	title = strings.ReplaceAll(title, "處", "处")
	if title == "序曲" {
		return "引子"
	}
	return title
}
