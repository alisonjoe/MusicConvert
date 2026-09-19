package audio

import (
	"MusicConvert/api"
	"MusicConvert/model"
	"MusicConvert/util"
	"path/filepath"
	"strconv"
	"os"
	"strings"
	"sort"
	"regexp"
)

func discNumberFromName(name string) int {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, prefix := range []string{"cd", "disc", "disk"} {
		if strings.HasPrefix(name, prefix) {
			rest := strings.TrimSpace(strings.TrimPrefix(name, prefix))
			rest = strings.Trim(rest, " -_.")
			if n := parseLeadingTrackNumber(rest); n > 0 {
				return n
			}
		}
	}
	if strings.Contains(name, "vinyl") || strings.Contains(name, "cd") || strings.Contains(name, "disc") || strings.Contains(name, "disk") {
		fields := strings.FieldsFunc(name, func(r rune) bool {
			return r < '0' || r > '9'
		})
		for i := len(fields) - 1; i >= 0; i-- {
			if n, err := strconv.Atoi(fields[i]); err == nil && n > 0 && n < 100 {
				return n
			}
		}
	}
	return 0
}

func isDiscDir(name string) bool {
	return discNumberFromName(name) > 0
}

func isCatalogOnlyDir(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || isDiscDir(name) {
		return false
	}
	return strings.TrimSpace(catalogPrefixPattern.ReplaceAllString(name, "")) == ""
}

func sourceAlbumDirName(filePath string) string {
	parent := filepath.Base(filepath.Dir(filePath))
	if isDiscDir(parent) || isCatalogOnlyDir(parent) {
		grandParent := filepath.Base(filepath.Dir(filepath.Dir(filePath)))
		if grandParent != "." && grandParent != string(os.PathSeparator) && !looksLikeGenericMusicFolder(grandParent) {
			return grandParent
		}
	}
	return parent
}

func catalogSiblingDirs(filePath string) []string {
	parent := filepath.Base(filepath.Dir(filePath))
	if !isCatalogOnlyDir(parent) {
		return nil
	}
	albumDir := filepath.Dir(filepath.Dir(filePath))
	entries, err := os.ReadDir(albumDir)
	if err != nil {
		return nil
	}
	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && isCatalogOnlyDir(entry.Name()) {
			dirs = append(dirs, entry.Name())
		}
	}
	sort.Strings(dirs)
	return dirs
}

func catalogDiscNumberFromSource(filePath string) int {
	parent := filepath.Base(filepath.Dir(filePath))
	for i, dir := range catalogSiblingDirs(filePath) {
		if dir == parent {
			return i + 1
		}
	}
	return 0
}

func sourceCatalog(filePath string) string {
	parent := filepath.Base(filepath.Dir(filePath))
	if isCatalogOnlyDir(parent) {
		return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(parent), "_", "-"))
	}
	return ""
}

func discNumberFromSource(meta *AutoMetadata, filePath string) int {
	for _, key := range []string{"DISCNUMBER", "DISC", "disc"} {
		if n := parseDiscNumberTag(meta.GetTag(key)); n > 0 {
			return n
		}
	}
	if n := parseDiscNumberTag(filepath.Base(filepath.Dir(filePath))); n > 0 && !isCatalogOnlyDir(filepath.Base(filepath.Dir(filePath))) {
		return n
	}
	if n := catalogDiscNumberFromSource(filePath); n > 0 {
		return n
	}
	return 1
}

func sourceDiscCount(filePath string) int {
	parent := filepath.Base(filepath.Dir(filePath))
	if isCatalogOnlyDir(parent) {
		if dirs := catalogSiblingDirs(filePath); len(dirs) > 0 {
			return len(dirs)
		}
		return 1
	}
	if !isDiscDir(parent) {
		return 1
	}
	albumDir := filepath.Dir(filepath.Dir(filePath))
	entries, err := os.ReadDir(albumDir)
	if err != nil {
		return 1
	}
	maxDisc := 1
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if n := discNumberFromName(entry.Name()); n > maxDisc {
			maxDisc = n
		}
	}
	return maxDisc
}

func sourceDiscTrackCount(filePath string) int {
	return trackCountInDir(filepath.Dir(filePath))
}

func trackCountInDir(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToUpper(filepath.Ext(entry.Name()))
		if _, ok := model.NeedConvertSuffix[ext]; ok && ext != ".CUE" {
			count++
		}
	}
	return count
}

func sourceDiscTrackOffset(filePath string) int {
	parent := filepath.Base(filepath.Dir(filePath))
	albumDir := filepath.Dir(filepath.Dir(filePath))
	var siblings []string
	if isCatalogOnlyDir(parent) {
		siblings = catalogSiblingDirs(filePath)
	} else if isDiscDir(parent) {
		entries, err := os.ReadDir(albumDir)
		if err != nil {
			return 0
		}
		for _, entry := range entries {
			if entry.IsDir() && isDiscDir(entry.Name()) {
				siblings = append(siblings, entry.Name())
			}
		}
		sort.Slice(siblings, func(i, j int) bool { return discNumberFromName(siblings[i]) < discNumberFromName(siblings[j]) })
	}
	if len(siblings) == 0 {
		return 0
	}
	offset := 0
	for _, sibling := range siblings {
		if sibling == parent {
			return offset
		}
		offset += trackCountInDir(filepath.Join(albumDir, sibling))
	}
	return 0
}

func remapTrackTitlesForSource(filePath string, tracks map[int]string) map[int]string {
	if len(tracks) == 0 {
		return tracks
	}
	offset := sourceDiscTrackOffset(filePath)
	if offset <= 0 {
		return tracks
	}
	count := sourceDiscTrackCount(filePath)
	if count <= 0 {
		return tracks
	}
	remapped := make(map[int]string)
	for i := 1; i <= count; i++ {
		if title := strings.TrimSpace(tracks[offset+i]); title != "" {
			remapped[i] = title
		}
	}
	if len(remapped) == 0 {
		return nil
	}
	return remapped
}

func sourceDiscTrackTitles(filePath string) map[int]string {
	entries, err := os.ReadDir(filepath.Dir(filePath))
	if err != nil {
		return nil
	}
	tracks := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToUpper(filepath.Ext(entry.Name()))
		if _, ok := model.NeedConvertSuffix[ext]; !ok || ext == ".CUE" {
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		trackNo := parseTrackNumberFromFilename(entry.Name())
		if trackNo <= 0 {
			continue
		}
		title := strings.TrimSpace(base)
		if idx := strings.LastIndex(title, " - "); idx >= 0 {
			title = strings.TrimSpace(title[idx+3:])
		}
		title = strings.TrimLeft(title, "0123456789. -_")
		if title != "" && !looksLikeGenericTrackTitle(title) {
			tracks[trackNo] = util.ToSimplifiedChinese(title)
		}
	}
	if len(tracks) == 0 {
		return nil
	}
	return tracks
}

func looksLikeGenericMusicFolder(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return true
	}
	if !api.ContainsChinese(name) && !strings.Contains(name, " ") && !strings.Contains(name, ",") {
		return true
	}
	exactGenericTerms := []string{"import", "album", "music", "disc", "disk", "cd", "vinyl"}
	for _, term := range exactGenericTerms {
		if name == term {
			return true
		}
	}
	for _, term := range []string{"bootleg", "series"} {
		if strings.Contains(name, term) {
			return true
		}
	}
	if strings.Contains(name, "philips") || strings.Contains(name, "首版系列") || strings.Contains(name, "纸盒") {
		return true
	}
	return false
}

func cleanSourceQualifier(s string) string {
	s = strings.TrimSpace(s)
	for _, marker := range []string{"（qobuz）", "(qobuz)", "（Qobuz）", "(Qobuz)"} {
		s = strings.ReplaceAll(s, marker, "")
	}
	// Discogs-style artist disambiguators, e.g. "Indecision (2)", are not part of the artist name.
	s = regexp.MustCompile(`\s*[（(]\d+[）)]\s*$`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

func albumDirectoryYear(sourceAlbumDir, originalAlbum, finalAlbum string, fallbackValues ...string) string {
	sourceYear := ""
	if albumNamesSimilar(sourceAlbumDir, finalAlbum) || albumNamesSimilar(originalAlbum, finalAlbum) {
		sourceYear = firstYear(sourceAlbumDir, originalAlbum)
	}
	values := []string{sourceYear}
	values = append(values, fallbackValues...)
	return firstYear(values...)
}

func cueAlbumContext(albumHint, sourceAlbumDir, cueTitle string) string {
	for _, candidate := range []string{albumHint, sourceAlbumDir, cueTitle} {
		candidate = api.CleanHint(candidate)
		if candidate != "" && candidate != "Unknown Album" {
			return candidate
		}
	}
	return ""
}

func parseTrackNumberTag(trackTag string) int {
	if idx := strings.Index(trackTag, "/"); idx > 0 {
		trackTag = trackTag[:idx]
	}
	trackTag = strings.TrimSpace(trackTag)
	if n, err := strconv.Atoi(trackTag); err == nil && n > 0 {
		return n
	}
	return 0
}

func parseDiscNumberTag(discTag string) int {
	if idx := strings.Index(discTag, "/"); idx > 0 {
		discTag = discTag[:idx]
	}
	discTag = strings.TrimSpace(discTag)
	if n, err := strconv.Atoi(discTag); err == nil && n > 0 {
		return n
	}
	return 0
}

func normalizeTrackNumber(trackNum, fallback int) int {
	if fallback > 1 && trackNum <= 1 {
		return fallback
	}
	if trackNum > 0 {
		return trackNum
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func normalizeTrackTotal(totalTracks, fallback int) int {
	if totalTracks > 0 {
		return totalTracks
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}

func inferOriginalTrackNumber(meta *AutoMetadata, title, filename string) int {
	for _, key := range []string{"TRACKNUMBER", "TRACK", "track"} {
		if trackTag := meta.GetTag(key); trackTag != "" {
			if n := parseTrackNumberTag(trackTag); n > 0 {
				return n
			}
		}
	}
	for _, candidate := range []string{filename, title} {
		if n := parseLeadingTrackNumber(candidate); n > 0 {
			return n
		}
		if n := parseTrackNumberFromFilename(candidate); n > 0 {
			return n
		}
	}
	return 0
}