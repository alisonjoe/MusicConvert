package audio

import (
	"MusicConvert/api"
	"MusicConvert/db"
	"MusicConvert/model"
	"MusicConvert/util"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var errSkipNoTrackMetadata = errors.New("skip: no usable track metadata")

var (
	cueDiscSuffixPattern               = regexp.MustCompile(`(?i)\s*[\[\(]\s*(?:disc|disk|cd)\s*0*([0-9]+)\s*[\]\)]\s*$`)
	bootlegVolumePattern               = regexp.MustCompile(`(?i)\b(?:vol(?:ume)?s?\.?|volumes?)\s*([0-9]+)(?:\s*[-–—]\s*([0-9]+))?`)
	bootlegSeriesTitleWords            = regexp.MustCompile(`(?i)\bbootleg\b.*\bseries\b`)
	genericTrackTitlePattern           = regexp.MustCompile(`(?i)^track\s*0*([0-9]+)$`)
	catalogPrefixPattern               = regexp.MustCompile(`(?i)^\s*([A-Z]{2,}[-_ ]?\d+)\s*`)
	catalogNumberPattern               = regexp.MustCompile(`(?i)^([A-Z]{2,}[-_ ]?)(\d+)$`)
	numeroNoSpacePattern               = regexp.MustCompile(`\bNo\.\s*([0-9]+)`)
	quotedClassicalPattern             = regexp.MustCompile(`^\s*(?i:[A-Z]{2,}[-_ ]?\d+)?\s*([^「」『』《》]+)[「『《]([^」』》]+)[」』》]`)
	colonClassicalPattern              = regexp.MustCompile(`^\s*(?i:[A-Z]{2,}[-_ ]?\d+)?\s*([^:：。「」]+)[:：]\s*([^。]+)`)
	hyphenClassicalPattern             = regexp.MustCompile(`^\s*(?i:[A-Z]{2,}[-_ ]?\d+)?\s*([\p{Han}A-Za-z][^-。]+)-\s*([^。]+?(?:协奏曲|奏鸣曲|交响曲|五重奏|四重奏|三重奏|Concerto|Sonata|Symphony|Quintet|Quartet|Trio)[^。]*)`)
	conductorEnsemblePattern           = regexp.MustCompile(`([^。；;，,]+?)指挥([^。；;，,]+?(?:大?会堂管弦乐团|管弦乐团|交响乐团|音乐厅管弦乐团|樂團|乐团|交響樂團))`)
	chamberEnsemblePattern             = regexp.MustCompile(`([^。；;，,]+?(?:弦乐四重奏|弦樂四重奏|四重奏团|四重奏團|三重奏团|三重奏團|五重奏团|五重奏團|Quartet|Trio|Quintet))`)
	instrumentalistPattern             = regexp.MustCompile(`(?:^|[。；;，,])([^。；;，,「」]+?)(钢琴|小提琴|大提琴|吉他|古筝|琵琶|长笛|单簧管|双簧管|萨克斯|竖琴|风琴|管风琴)`)
	instrumentalistAfterPunctuationPat = regexp.MustCompile(`[。；;，,]([^。；;，,「」]+?)(钢琴|小提琴|大提琴|吉他|古筝|琵琶|长笛|单簧管|双簧管|萨克斯|竖琴|风琴|管风琴)`)
)

// collectAllTags merges tags from ffprobe, CUE sheet, and Netease API
func collectAllTags(meta *AutoMetadata, cueSheet *CueSheet, cueTrack *Track, neteaseYear string) map[string]string {
	tags := make(map[string]string)
	if meta != nil && meta.Format.Tags != nil {
		for k, v := range meta.Format.Tags {
			tags[strings.ToUpper(k)] = v
		}
	}
	if cueSheet != nil {
		for k, v := range cueSheet.Rem {
			if tags[k] == "" {
				tags[k] = v
			}
		}
	}
	if cueTrack != nil {
		for k, v := range cueTrack.Rem {
			tags[k] = v
		}
	}
	if neteaseYear != "" && tags["YEAR"] == "" {
		tags["YEAR"] = neteaseYear
	}
	return tags
}

func enrichWithItunesAndStandardize(tags map[string]string, itunesMeta api.ItunesMeta, trackNum, totalTracks, discNum, discCount int) {
	if tags["GENRE"] == "" && itunesMeta.Genre != "" {
		tags["GENRE"] = itunesMeta.Genre
	}
	if tags["DATE"] == "" && itunesMeta.Date != "" {
		tags["DATE"] = itunesMeta.Date
	}
	if tags["ALBUMARTIST"] == "" && itunesMeta.AlbumArtist != "" {
		tags["ALBUMARTIST"] = itunesMeta.AlbumArtist
	}

	if discCount < 1 {
		discCount = 1
	}
	if discNum < 1 {
		discNum = 1
	}

	// 关键修复：使用纯数字格式
	tags["DISCNUMBER"] = fmt.Sprintf("%d", discNum)
	tags["DISCTOTAL"] = fmt.Sprintf("%d", discCount)
	tags["TRACKNUMBER"] = fmt.Sprintf("%d", trackNum)
	tags["TRACKTOTAL"] = fmt.Sprintf("%d", totalTracks)

	// 额外写入常用别名以增加兼容性
	tags["track"] = tags["TRACKNUMBER"]
	tags["disc"] = tags["DISCNUMBER"]
	tags["TOTALTRACKS"] = tags["TRACKTOTAL"]
	tags["TOTALDISCS"] = tags["DISCTOTAL"]

	if tags["DATE"] != "" {
		if tags["YEAR"] == "" {
			tags["YEAR"] = tags["DATE"]
		}
		if tags["ORIGINALYEAR"] == "" {
			tags["ORIGINALYEAR"] = tags["DATE"]
		}
	}

	// Prefer album year (ORIGINALYEAR) for the YEAR tag when available
	if oy := tags["ORIGINALYEAR"]; oy != "" {
		// Override YEAR with ORIGINALYEAR regardless of existing value
		tags["YEAR"] = oy
	}

	// 清洗所有标签中的垃圾信息
	for k, v := range tags {
		tags[k] = api.FilterJunkTags(v)
	}

	log.Printf("[Metadata Scraping] 最终注入标签(Disc %d/%d Track %d/%d): %#v", discNum, discCount, trackNum, totalTracks, tags)
}

func outputExtFor(inputExt string) string {
	if strings.EqualFold(inputExt, ".mp3") {
		return ".mp3"
	}
	return ".flac"
}

func cueAlbumTitleAndDisc(title string) (string, int) {
	title = strings.TrimSpace(title)
	if title == "" {
		return "", 0
	}
	matches := cueDiscSuffixPattern.FindStringSubmatch(title)
	if len(matches) >= 2 {
		discNum, _ := strconv.Atoi(matches[1])
		cleanTitle := strings.TrimSpace(cueDiscSuffixPattern.ReplaceAllString(title, ""))
		return cleanTitle, discNum
	}
	return title, 0
}

func parseLeadingTrackNumber(filename string) int {
	filename = strings.TrimSpace(filename)
	var digits strings.Builder
	for _, r := range filename {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	if digits.Len() == 0 {
		return 0
	}
	if n, err := strconv.Atoi(digits.String()); err == nil && n > 0 {
		return n
	}
	return 0
}

func appendCodecArgs(args []string, outputExt string, hasCover bool) []string {
	if strings.EqualFold(outputExt, ".mp3") {
		args = append(args, "-c:a", "copy", "-id3v2_version", "3")
	} else {
		args = append(args, "-c:a", "flac")
	}
	if hasCover {
		args = append(args, "-c:v", "copy", "-disposition:v", "attached_pic")
	}
	return args
}

func looksLikeUnknownArtist(artist string) bool {
	artist = strings.TrimSpace(artist)
	return artist == "" || strings.EqualFold(artist, "Unknown Artist")
}

func artistFromPath(filePath string) string {
	absPath, _ := filepath.Abs(filePath)
	dirParts := strings.Split(absPath, string(os.PathSeparator))
	if len(dirParts) < 3 {
		return ""
	}
	artistIndex := len(dirParts) - 3
	if isDiscDir(dirParts[len(dirParts)-2]) && len(dirParts) >= 4 {
		artistIndex = len(dirParts) - 4
	}
	artist := strings.TrimSpace(dirParts[artistIndex])
	if artist == "" || strings.Contains(artist, "Volumes") || strings.Contains(artist, "Music") {
		return ""
	}
	return artist
}

func hasASCIILetter(s string) bool {
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}

func primaryAlbumArtist(candidate string) string {
	candidate = normalizeArtistName(candidate)
	parts := api.SplitArtists(candidate)
	if len(parts) == 0 {
		return strings.TrimSpace(candidate)
	}
	for _, part := range parts {
		if hasASCIILetter(part) {
			return strings.TrimSpace(part)
		}
	}
	return strings.TrimSpace(parts[0])
}

func normalizeArtistName(artist string) string {
	artist = strings.TrimSpace(artist)
	artist = cleanSourceQualifier(artist)
	artist = cleanClassicalNameNoise(artist)
	if artist == "" {
		return artist
	}
	if strings.Contains(artist, ",") && !api.ContainsChinese(artist) {
		parts := strings.SplitN(artist, ",", 2)
		last := strings.TrimSpace(parts[0])
		first := strings.TrimSpace(parts[1])
		if last != "" && first != "" {
			return strings.TrimSpace(first + " " + last)
		}
	}
	return artist
}

func selectAlbumArtist(meta *AutoMetadata, filePath, origArtist, officialArtist string) string {
	pathArtist := artistFromPath(filePath)
	var candidates []string
	if pathArtist != "" && api.ContainsChinese(pathArtist) && !looksLikeGenericMusicFolder(pathArtist) {
		candidates = append(candidates, pathArtist)
	}
	candidates = append(candidates,
		meta.GetTag("ALBUMARTIST"),
		meta.GetTag("ALBUM_ARTIST"),
		meta.GetTag("ALBUM ARTIST"),
	)
	if pathArtist != "" && !api.ContainsChinese(pathArtist) && !looksLikeGenericMusicFolder(pathArtist) {
		candidates = append(candidates, pathArtist)
	}
	candidates = append(candidates, origArtist, officialArtist)
	if pathArtist != "" && looksLikeGenericMusicFolder(pathArtist) {
		candidates = append(candidates, pathArtist)
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if looksLikeUnknownArtist(candidate) {
			continue
		}
		return primaryAlbumArtist(candidate)
	}
	return "Unknown Artist"
}

func firstYear(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if len(value) >= 4 {
			for i := 0; i <= len(value)-4; i++ {
				year := value[i : i+4]
				if year[0] >= '0' && year[0] <= '9' &&
					year[1] >= '0' && year[1] <= '9' &&
					year[2] >= '0' && year[2] <= '9' &&
					year[3] >= '0' && year[3] <= '9' &&
					year >= "1900" && year <= "2099" {
					return year
				}
			}
		}
	}
	return ""
}

func normalizeAlbumCompareKey(s string) string {
	s = strings.ToLower(normalizeAlbumName(s))
	remove := " \t\r\n　-_.·・,，.。:：;；!！?？'\"“”‘’()（）[]【】{}《》<>〈〉&"
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(remove, r) {
			return -1
		}
		return r
	}, s)
}

func albumCompareTokens(s string) map[string]bool {
	s = strings.ToLower(normalizeAlbumName(s))
	fields := strings.FieldsFunc(s, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r >= 128)
	})
	stop := map[string]bool{
		"": true, "a": true, "an": true, "the": true, "and": true, "of": true,
		"bob": true, "dylan": true, "dyaln": true, "vol": true, "vols": true,
	}
	tokens := make(map[string]bool)
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if stop[field] || len([]rune(field)) < 2 {
			continue
		}
		if _, err := strconv.Atoi(field); err == nil {
			continue
		}
		tokens[field] = true
	}
	return tokens
}

func bootlegSeriesVolumeSignature(s string) string {
	s = normalizeAlbumName(s)
	if !bootlegSeriesTitleWords.MatchString(s) {
		return ""
	}
	matches := bootlegVolumePattern.FindStringSubmatch(s)
	if len(matches) < 2 {
		return ""
	}
	start := strings.TrimLeft(matches[1], "0")
	if start == "" {
		start = "0"
	}
	end := start
	if len(matches) >= 3 && strings.TrimSpace(matches[2]) != "" {
		end = strings.TrimLeft(matches[2], "0")
		if end == "" {
			end = "0"
		}
	}
	return start + "-" + end
}

func albumNamesSimilar(a, b string) bool {
	ka := normalizeAlbumCompareKey(a)
	kb := normalizeAlbumCompareKey(b)
	if ka == "" || kb == "" {
		return false
	}
	aBootlegVolume := bootlegSeriesVolumeSignature(a)
	bBootlegVolume := bootlegSeriesVolumeSignature(b)
	if aBootlegVolume != "" && bBootlegVolume != "" && aBootlegVolume != bBootlegVolume {
		return false
	}
	if strings.Contains(ka, kb) || strings.Contains(kb, ka) {
		return true
	}
	ta := albumCompareTokens(a)
	tb := albumCompareTokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return false
	}
	overlap := 0
	for token := range ta {
		if tb[token] {
			overlap++
		}
	}
	return overlap >= 2
}

func preferItunesAlbumName(currentAlbum, itunesAlbum string) string {
	if strings.TrimSpace(itunesAlbum) == "" {
		return currentAlbum
	}
	if currentAlbum == "" || currentAlbum == "Unknown Album" || albumNamesSimilar(currentAlbum, itunesAlbum) {
		return normalizeAlbumName(itunesAlbum)
	}
	log.Printf("[iTunes Album Context] Rejecting iTunes album %q; keeping source album context %q", itunesAlbum, currentAlbum)
	return currentAlbum
}

func preferItunesAlbumArtist(currentArtist, itunesArtist string) string {
	currentArtist = strings.TrimSpace(currentArtist)
	itunesArtist = strings.TrimSpace(itunesArtist)
	if itunesArtist == "" {
		return currentArtist
	}
	if currentArtist != "" && api.ContainsChinese(currentArtist) && !api.ContainsChinese(itunesArtist) {
		return currentArtist
	}
	return simplifyMetadataText(primaryAlbumArtist(itunesArtist))
}

func constrainCueAlbumToContext(officialAlbum, contextAlbum string) string {
	if contextAlbum == "" || officialAlbum == "" || officialAlbum == "Unknown Album" {
		return officialAlbum
	}
	if albumNamesSimilar(officialAlbum, contextAlbum) {
		return officialAlbum
	}
	log.Printf("[CUE Album Context] Rejecting track-level album %q; using source album context %q", officialAlbum, contextAlbum)
	return contextAlbum
}

func albumDirName(album, year string) string {
	album = normalizeAlbumName(album)
	year = firstYear(year)
	if year == "" || album == "" || strings.Contains(album, "["+year+"]") {
		return album
	}
	return fmt.Sprintf("%s [%s]", album, year)
}

func looksLikeGarbageOutputName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return true
	}
	if regexp.MustCompile(`(?i)^[a-z0-9]{5,8}~[a-z0-9]$`).MatchString(name) {
		return true
	}
	for _, r := range name {
		if r < 32 || r == 127 {
			return true
		}
	}
	return false
}

func validateOutputIdentity(artist, album string) error {
	if looksLikeUnknownArtist(artist) || looksLikeGarbageOutputName(artist) {
		return fmt.Errorf("%w: invalid output artist=%q album=%q", errSkipNoTrackMetadata, artist, album)
	}
	if album == "" || album == "Unknown Album" || looksLikeGarbageOutputName(album) {
		return fmt.Errorf("%w: invalid output album=%q artist=%q", errSkipNoTrackMetadata, album, artist)
	}
	return nil
}

func normalizeAlbumName(album string) string {
	replacer := strings.NewReplacer(
		"‐", "-",
		"‑", "-",
		"‒", "-",
		"–", "-",
		"—", "-",
		"―", "-",
		"‘", "'",
		"’", "'",
		"“", "\"",
		"”", "\"",
		"N°", "No.",
		"n°", "No.",
		"№", "No.",
		"𫚭", "鳟",
		"鱲", "鳟",
	)
	album = util.ToSimplifiedChinese(strings.TrimSpace(replacer.Replace(album)))
	album = strings.ReplaceAll(album, "𫚭", "鳟")
	album = strings.ReplaceAll(album, "鱲", "鳟")
	album = cleanClassicalNameNoise(album)
	album = strings.ReplaceAll(album, ":-", ": ")
	album = strings.ReplaceAll(album, ": -", ": ")
	album = strings.ReplaceAll(album, "：-", ": ")
	album = strings.ReplaceAll(album, "： -", ": ")
	album = strings.Join(strings.Fields(album), " ")
	album = numeroNoSpacePattern.ReplaceAllString(album, "No. $1")
	return strings.TrimSpace(album)
}

func simplifyMetadataText(s string) string {
	return normalizeAlbumName(s)
}

func simplifyMetadataTags(tags map[string]string) {
	for _, key := range []string{
		"ARTIST", "ALBUM", "TITLE", "ALBUMARTIST", "ALBUM_ARTIST", "ALBUM ARTIST",
		"GENRE", "COMPOSER", "LYRICIST", "PERFORMER", "PUBLISHER", "LABEL",
	} {
		if tags[key] != "" {
			tags[key] = simplifyMetadataText(tags[key])
		}
	}
}

func albumOutputPath(outRoot, artist, album, year string) string {
	return filepath.Join(outRoot, util.SanitizeFilename(artist), util.SanitizeFilename(albumDirName(album, year)))
}

func discOutputPath(albumPath string, discNum, discCount int) string {
	if discCount > 1 || discNum > 1 {
		return filepath.Join(albumPath, fmt.Sprintf("CD%d", discNum))
	}
	return albumPath
}

func parseOutputTrackNumber(name string) int {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	return parseLeadingTrackNumber(base)
}

func outputTrackFilename(trackNum int, title, ext string, classicalMeta classicalMetadata) string {
	return fmt.Sprintf("%02d. %s%s", trackNum, util.SanitizeFilename(title), ext)
}

func parseTrackNumberFromFilename(name string) int {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	if n := parseLeadingTrackNumber(base); n > 0 {
		return n
	}
	if matches := genericTrackTitlePattern.FindStringSubmatch(base); len(matches) >= 2 {
		if n, err := strconv.Atoi(matches[1]); err == nil && n > 0 {
			return n
		}
	}
	parts := strings.Split(base, " - ")
	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if n, err := strconv.Atoi(part); err == nil && n > 0 && n < 1000 {
			return n
		}
	}
	return 0
}

func looksLikeGenericTrackTitle(title string) bool {
	return genericTrackTitlePattern.MatchString(strings.TrimSpace(title))
}

func shouldTrustLocalMetadata(title, artist, album string) bool {
	if looksLikeGenericTrackTitle(title) {
		return true
	}
	album = strings.ToLower(album)
	artist = strings.ToLower(artist)
	return strings.Contains(album, "phcp-") ||
		strings.Contains(album, "philips") ||
		strings.Contains(artist, "philips") ||
		strings.Contains(artist, "首版系列")
}

func albumArtistFromAlbumTitle(album string) string {
	album = normalizeClassicalAlbumText(album)
	return orchestraCandidateFromText(album)
}

func localizedArtistName(name string) string {
	name = strings.TrimSpace(normalizeArtistName(name))
	if name == "" {
		return ""
	}
	if api.ContainsChinese(name) {
		return simplifyMetadataText(name)
	}
	if chinese, _ := api.FetchMBArtistName(name); strings.TrimSpace(chinese) != "" {
		return simplifyMetadataText(chinese)
	}
	canonical := api.GetArtistCanonicalName(name)
	if api.ContainsChinese(canonical) {
		parts := api.SplitArtists(canonical)
		if len(parts) > 0 && api.ContainsChinese(parts[0]) {
			return simplifyMetadataText(parts[0])
		}
		return simplifyMetadataText(canonical)
	}
	return name
}

func preferAlbumTitleArtist(finalMainArtist, officialArtist, officialAlbum string) (string, string) {
	albumArtist := albumArtistFromAlbumTitle(officialAlbum)
	if albumArtist == "" {
		return finalMainArtist, officialArtist
	}
	if looksLikeUnknownArtist(finalMainArtist) || looksLikeGenericMusicFolder(finalMainArtist) {
		finalMainArtist = albumArtist
	}
	if looksLikeUnknownArtist(officialArtist) || looksLikeGenericMusicFolder(officialArtist) {
		officialArtist = albumArtist
	}
	return finalMainArtist, officialArtist
}

func containsKana(s string) bool {
	for _, r := range s {
		if (r >= 0x3040 && r <= 0x30ff) || (r >= 0x31f0 && r <= 0x31ff) {
			return true
		}
	}
	return false
}

func isLikelyPinyinTitle(title string) bool {
	title = strings.ToLower(normalizeAlbumName(title))
	title = regexp.MustCompile(`\([^)]*\)`).ReplaceAllString(title, " ")
	words := strings.FieldsFunc(title, func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || r == '\'')
	})
	if len(words) < 2 {
		return false
	}
	pinyin := map[string]bool{
		"a": true, "ai": true, "an": true, "ang": true, "ao": true, "ba": true, "bai": true, "ban": true, "bang": true, "bei": true, "ben": true, "bi": true, "bian": true, "bie": true, "bin": true, "bo": true, "bu": true,
		"ca": true, "cai": true, "can": true, "cang": true, "ce": true, "cen": true, "cha": true, "chai": true, "chan": true, "chang": true, "che": true, "chen": true, "cheng": true, "chi": true, "chong": true, "chou": true, "chu": true, "chuan": true, "chuang": true, "chui": true, "chun": true, "ci": true, "cong": true, "cu": true, "cui": true,
		"da": true, "dai": true, "dan": true, "dao": true, "de": true, "deng": true, "di": true, "dian": true, "ding": true, "dong": true, "dou": true, "du": true, "dui": true, "duo": true,
		"e": true, "en": true, "er": true, "fa": true, "fan": true, "fang": true, "fei": true, "fen": true, "feng": true, "fo": true, "fu": true,
		"ga": true, "gai": true, "gan": true, "gang": true, "gao": true, "ge": true, "gen": true, "geng": true, "gong": true, "gou": true, "gu": true, "gua": true, "guai": true, "guan": true, "guang": true, "gui": true, "gun": true, "guo": true,
		"ha": true, "hai": true, "han": true, "hang": true, "hao": true, "he": true, "hen": true, "heng": true, "hong": true, "hou": true, "hu": true, "hua": true, "huai": true, "huan": true, "huang": true, "hui": true, "hun": true, "huo": true,
		"ji": true, "jia": true, "jian": true, "jiang": true, "jiao": true, "jie": true, "jin": true, "jing": true, "jiong": true, "jiu": true, "ju": true, "juan": true, "jue": true, "jun": true,
		"ka": true, "kai": true, "kan": true, "kang": true, "kao": true, "ke": true, "ken": true, "kong": true, "kou": true, "ku": true, "kua": true, "kuai": true, "kuang": true, "kui": true, "kun": true, "kuo": true,
		"la": true, "lai": true, "lan": true, "lang": true, "lao": true, "le": true, "lei": true, "leng": true, "li": true, "lia": true, "lian": true, "liang": true, "liao": true, "lie": true, "lin": true, "ling": true, "liu": true, "lo": true, "long": true, "lou": true, "lu": true, "luan": true, "lun": true, "luo": true,
		"ma": true, "mai": true, "man": true, "mang": true, "mao": true, "me": true, "mei": true, "men": true, "meng": true, "mi": true, "mian": true, "miao": true, "mie": true, "min": true, "ming": true, "miu": true, "mo": true, "mou": true, "mu": true,
		"na": true, "nai": true, "nan": true, "nang": true, "nao": true, "ne": true, "nei": true, "nen": true, "neng": true, "ni": true, "nian": true, "niang": true, "niao": true, "nie": true, "nin": true, "ning": true, "niu": true, "nong": true, "nu": true, "nuan": true, "nuo": true,
		"o": true, "ou": true, "pa": true, "pai": true, "pan": true, "pang": true, "pao": true, "pei": true, "pen": true, "peng": true, "pi": true, "pian": true, "piao": true, "pie": true, "pin": true, "ping": true, "po": true, "pu": true,
		"qi": true, "qia": true, "qian": true, "qiang": true, "qiao": true, "qie": true, "qin": true, "qing": true, "qiu": true, "qu": true, "quan": true, "que": true, "qun": true,
		"ran": true, "rang": true, "rao": true, "re": true, "ren": true, "reng": true, "ri": true, "rong": true, "rou": true, "ru": true, "ruan": true, "rui": true, "run": true, "ruo": true,
		"sa": true, "sai": true, "san": true, "sang": true, "sao": true, "se": true, "sen": true, "sha": true, "shai": true, "shan": true, "shang": true, "shao": true, "she": true, "shen": true, "sheng": true, "shi": true, "shou": true, "shu": true, "shua": true, "shuai": true, "shuan": true, "shuang": true, "shui": true, "shun": true, "shuo": true, "si": true, "song": true, "sou": true, "su": true, "sui": true, "sun": true, "suo": true,
		"ta": true, "tai": true, "tan": true, "tang": true, "tao": true, "te": true, "teng": true, "ti": true, "tian": true, "tiao": true, "tie": true, "ting": true, "tong": true, "tou": true, "tu": true, "tuan": true, "tui": true, "tun": true, "tuo": true,
		"wa": true, "wai": true, "wan": true, "wang": true, "wei": true, "wen": true, "weng": true, "wo": true, "wu": true, "xi": true, "xia": true, "xian": true, "xiang": true, "xiao": true, "xie": true, "xin": true, "xing": true, "xiong": true, "xiu": true, "xu": true, "xuan": true, "xue": true, "xun": true,
		"ya": true, "yan": true, "yang": true, "yao": true, "ye": true, "yi": true, "yin": true, "ying": true, "yo": true, "yong": true, "you": true, "yu": true, "yuan": true, "yue": true, "yun": true,
		"za": true, "zai": true, "zan": true, "zang": true, "zao": true, "ze": true, "zei": true, "zen": true, "zeng": true, "zha": true, "zhai": true, "zhan": true, "zhang": true, "zhao": true, "zhe": true, "zhen": true, "zheng": true, "zhi": true, "zhong": true, "zhou": true, "zhu": true, "zhua": true, "zhuai": true, "zhuan": true, "zhuang": true, "zhui": true, "zhun": true, "zhuo": true, "zi": true, "zong": true, "zou": true, "zu": true, "zuan": true, "zui": true, "zun": true, "zuo": true,
	}
	matched := 0
	for _, word := range words {
		word = strings.Trim(word, "'")
		if pinyin[word] {
			matched++
		}
	}
	return float64(matched)/float64(len(words)) >= 0.6
}

func titleFromOnlineTrack(trackNum int, currentTitle string, trackTitles map[int]string, classicalMeta classicalMetadata) string {
	onlineTitle := strings.TrimSpace(trackTitles[trackNum])
	currentTitle = normalizeAlbumName(currentTitle)
	onlineTitle = normalizeAlbumName(onlineTitle)
	if onlineTitle == "" || looksLikeGenericTrackTitle(onlineTitle) {
		return currentTitle
	}
	if classicalMeta.IsClassical() {
		if currentTitle == "" || looksLikeGenericTrackTitle(currentTitle) {
			if shouldPrefixChineseWork(classicalMeta, onlineTitle) {
				return combinedLocalizedTitle(chineseClassicalWorkPrefix(classicalMeta.Work), onlineTitle)
			}
			return onlineTitle
		}
		currentHasChinese := api.ContainsChinese(currentTitle)
		onlineHasChinese := api.ContainsChinese(onlineTitle)
		switch {
		case currentHasChinese && !onlineHasChinese:
			return combinedLocalizedTitle(currentTitle, onlineTitle)
		case !currentHasChinese && onlineHasChinese:
			return combinedLocalizedTitle(onlineTitle, currentTitle)
		case currentTitle == onlineTitle:
			return currentTitle
		case strings.Contains(onlineTitle, currentTitle) || strings.Contains(currentTitle, onlineTitle):
			return combinedLocalizedTitle(currentTitle, onlineTitle)
		case len([]rune(onlineTitle)) >= len([]rune(currentTitle)):
			return onlineTitle
		default:
			return currentTitle
		}
	}
	if looksLikeGenericTrackTitle(currentTitle) {
		return onlineTitle
	}
	return currentTitle
}

func relatedCatalogs(catalog string) []string {
	catalog = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(catalog), "_", "-"))
	if catalog == "" {
		return nil
	}
	return []string{catalog}
}

func relatedMetadataCatalogs(catalog string) []string {
	catalogs := relatedCatalogs(catalog)
	if len(catalogs) == 0 {
		return nil
	}
	seen := map[string]bool{catalogs[0]: true}
	matches := catalogNumberPattern.FindStringSubmatch(catalogs[0])
	if len(matches) >= 3 {
		prefix := strings.ReplaceAll(matches[1], " ", "")
		numText := matches[2]
		if n, err := strconv.Atoi(numText); err == nil {
			for delta := 1; delta <= 3; delta++ {
				if n-delta <= 0 {
					break
				}
				candidate := fmt.Sprintf("%s%0*d", prefix, len(numText), n-delta)
				if !seen[candidate] {
					seen[candidate] = true
					catalogs = append(catalogs, candidate)
				}
			}
		}
	}
	return catalogs
}

func albumTrackTitlesForConversion(filePath, artist, album string, classicalMeta classicalMetadata) (map[int]string, string) {
	if !classicalMeta.IsClassical() {
		if tracks, source := api.GetAlbumTrackTitlesWithSource(artist, album); len(tracks) > 0 {
			return remapTrackTitlesForSource(filePath, tracks), source
		}
	}
	if sourceTracks := sourceDiscTrackTitles(filePath); len(sourceTracks) > 0 {
		return sourceTracks, "source_dir"
	}
	if tracks, source := localizedClassicalTrackTitles(classicalMeta); len(tracks) > 0 {
		return remapTrackTitlesForSource(filePath, tracks), source
	}
	if classicalMeta.IsClassical() && hasChineseClassicalContext(classicalMeta) {
		if tracks, source := api.GetAlbumTrackTitlesWithSource(artist, album); len(tracks) > 0 {
			return remapTrackTitlesForSource(filePath, tracks), source
		}
	}
	if classicalMeta.IsClassical() {
		for _, catalog := range relatedCatalogs(classicalMeta.Catalog) {
			if tracks, source := api.FetchMBReleaseTrackTitles(catalog, ""); len(tracks) > 0 {
				return remapTrackTitlesForSource(filePath, tracks), source
			}
		}
		for _, queryHint := range classicalMeta.SearchHints() {
			if tracks, source := api.FetchMBReleaseTrackTitles("", queryHint); len(tracks) > 0 {
				return remapTrackTitlesForSource(filePath, tracks), source
			}
		}
	}
	if tracks, source := api.GetAlbumTrackTitlesWithSource(artist, album); len(tracks) > 0 {
		return remapTrackTitlesForSource(filePath, tracks), source
	}
	return nil, "empty"
}

func writeMissingTracksReport(albumDir string, totalTracks int, trackTitles map[int]string, trackTitleSource string) {
	if totalTracks <= 0 {
		return
	}
	entries, err := os.ReadDir(albumDir)
	if err != nil {
		log.Printf("[Missing Tracks] Failed to read album dir %s: %v", albumDir, err)
		return
	}
	present := make(map[int]string)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".flac" && ext != ".mp3" {
			continue
		}
		trackNo := parseOutputTrackNumber(entry.Name())
		if trackNo > 0 {
			present[trackNo] = entry.Name()
			continue
		}
		base := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		for i, title := range trackTitles {
			if title == "" {
				continue
			}
			if base == util.SanitizeFilename(title) {
				present[i] = entry.Name()
				break
			}
		}
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Total tracks: %d", totalTracks))
	lines = append(lines, fmt.Sprintf("Present tracks: %d", len(present)))
	lines = append(lines, fmt.Sprintf("Track title source: %s", trackTitleSource))
	lines = append(lines, "")
	lines = append(lines, "Missing tracks:")
	missing := 0
	for i := 1; i <= totalTracks; i++ {
		if _, ok := present[i]; ok {
			continue
		}
		missing++
		if title := strings.TrimSpace(trackTitles[i]); title != "" {
			lines = append(lines, fmt.Sprintf("%02d. %s", i, title))
		} else {
			lines = append(lines, fmt.Sprintf("%02d. <unknown>", i))
		}
	}
	if missing == 0 {
		reportPath := filepath.Join(albumDir, "missing_tracks.txt")
		if err := os.Remove(reportPath); err != nil && !os.IsNotExist(err) {
			log.Printf("[Missing Tracks] Failed to remove complete report %s: %v", reportPath, err)
			return
		}
		log.Printf("[Missing Tracks] Complete album dir %s; removed missing_tracks.txt if present (title source: %s)", albumDir, trackTitleSource)
		return
	}
	lines = append(lines, "")

	reportPath := filepath.Join(albumDir, "missing_tracks.txt")
	if err := os.WriteFile(reportPath, []byte(strings.Join(lines, "\n")), 0644); err != nil {
		log.Printf("[Missing Tracks] Failed to write report %s: %v", reportPath, err)
		return
	}
	log.Printf("[Missing Tracks] Updated %s (%d missing, title source: %s)", reportPath, missing, trackTitleSource)
}

func applySwinsianTags(ctx context.Context, flacFile string, trackNum, totalTracks, discNum, discCount int, year string) {
	if discCount < 1 {
		discCount = 1
	}
	if discNum < 1 {
		discNum = 1
	}
	args := []string{
		"--remove-tag=TRACKNUMBER", "--remove-tag=TRACKTOTAL",
		"--remove-tag=DISCNUMBER", "--remove-tag=DISCTOTAL",
		"--remove-tag=track", "--remove-tag=disc",
		"--remove-tag=TOTALTRACKS", "--remove-tag=TOTALDISCS",
		fmt.Sprintf("--set-tag=TRACKNUMBER=%d", trackNum),
		fmt.Sprintf("--set-tag=TRACKTOTAL=%d", totalTracks),
		fmt.Sprintf("--set-tag=DISCNUMBER=%d", discNum),
		fmt.Sprintf("--set-tag=DISCTOTAL=%d", discCount),
		fmt.Sprintf("--set-tag=track=%d", trackNum),
		fmt.Sprintf("--set-tag=TOTALTRACKS=%d", totalTracks),
		fmt.Sprintf("--set-tag=disc=%d", discNum),
		fmt.Sprintf("--set-tag=TOTALDISCS=%d", discCount),
	}
	if year != "" {
		args = append(args, fmt.Sprintf("--set-tag=ORIGINALYEAR=%s", year))
	}
	args = append(args, flacFile)
	util.Run(ctx, "metaflac", args)
}

func buildMetadataArgs(ffmpegArgs []string, tags map[string]string) []string {
	for k, v := range tags {
		if v != "" {
			ffmpegArgs = append(ffmpegArgs, "-metadata", fmt.Sprintf("%s=%s", k, v))
		}
	}
	return ffmpegArgs
}

func ToFLAC(ctx context.Context, outRoot string, ch chan model.MusicFileInfo, delCh chan string) {
	for {
		info := <-ch
		if info.FilePath == "" {
			continue
		}
		l, _ := db.Query(info.FilePath)
		if len(l) > 0 {
			continue
		}

		taskCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)

		var err error
		if info.CuePath == "" {
			err = singleConvert(taskCtx, info, outRoot)
		} else {
			err = cueConvert(taskCtx, info, outRoot)
		}

		if err == nil {
			db.Insert(&db.MusicInfo{Music: info.FilePath, State: 1})
			delCh <- info.FilePath
		} else if errors.Is(err, errSkipNoTrackMetadata) {
			log.Printf("Skipped %s: no usable track metadata found", info.FilePath)
			db.Insert(&db.MusicInfo{Music: info.FilePath, State: 1})
		} else {
			log.Printf("Failed to process %s: %v", info.FilePath, err)
		}
		cancel()
	}
}

func singleConvert(ctx context.Context, st model.MusicFileInfo, outRoot string) error {
	var chain *ProviderChain
	meta, err := GetFormat(ctx, st.FilePath)
	if err != nil {
		return err
	}

	origArtist := meta.GetTag("ARTIST")
	origAlbum := meta.GetTag("ALBUM")
	origTitle := meta.GetTag("TITLE")
	filename := strings.TrimSuffix(filepath.Base(st.FilePath), filepath.Ext(st.FilePath))
	sourceAlbumDir := sourceAlbumDirName(st.FilePath)

	if origArtist == "" || origTitle == "" {
		parts := strings.Split(filename, "-")
		if len(parts) >= 2 {
			if origArtist == "" {
				origArtist = strings.TrimSpace(parts[0])
			}
			if origTitle == "" {
				origTitle = strings.TrimSpace(parts[1])
			}
		} else {
			if origTitle == "" {
				origTitle = filename
			}
		}
		origTitle = strings.TrimLeft(origTitle, "0123456789. ")
		if origArtist == "" || origArtist == "Unknown Artist" {
			absPath, _ := filepath.Abs(st.FilePath)
			dirParts := strings.Split(absPath, string(os.PathSeparator))
			if len(dirParts) >= 3 {
				parentDir := sourceAlbumDir
				grandDir := artistFromPath(st.FilePath)
				if !strings.Contains(grandDir, "Volumes") && !strings.Contains(grandDir, "Music") {
					if origArtist == "" || origArtist == "Unknown Artist" {
						origArtist = grandDir
					}
					if origAlbum == "" || origAlbum == "Unknown Album" {
						origAlbum = parentDir
					}
				}
			}
		}
	}
	if origArtist == "" {
		origArtist = "Unknown Artist"
	}
	origArtist = normalizeArtistName(origArtist)
	pathArtist := normalizeArtistName(artistFromPath(st.FilePath))
	if api.ContainsChinese(pathArtist) && !looksLikeGenericMusicFolder(pathArtist) && !api.ContainsChinese(origArtist) {
		origArtist = pathArtist
	}
	if looksLikeGenericMusicFolder(origArtist) {
		origArtist = "Unknown Artist"
	}
	if origTitle == "" {
		origTitle = "Unknown Title"
	}

	log.Printf("[Metadata Source] Local: %s - %s - %s", origArtist, origAlbum, origTitle)

	// 尝试从原始元数据、文件名、通用标题中提取音轨号。
	originalTrackNum := inferOriginalTrackNumber(meta, origTitle, filename)

	// 清理并规范化专辑提示
	albumHint := api.CleanHint(origAlbum)
	if isDiscDir(albumHint) && sourceAlbumDir != "" && !looksLikeGenericMusicFolder(sourceAlbumDir) {
		albumHint = api.CleanHint(sourceAlbumDir)
	}
	if sourceCatalog(st.FilePath) != "" && looksLikeGenericMusicFolder(albumHint) {
		albumHint = sourceCatalog(st.FilePath)
	}
	if isCatalogOnlyDir(albumHint) && sourceAlbumDir != "" && !looksLikeGenericMusicFolder(sourceAlbumDir) {
		albumHint = strings.TrimSpace(albumHint + " " + api.CleanHint(sourceAlbumDir))
	}
	if albumHint == "" || albumHint == "Unknown Album" {
		albumHint = api.CleanHint(sourceAlbumDir)
	}

	officialArtist, officialAlbum, officialTitle, trackNum, trackCount := origArtist, "Unknown Album", origTitle, originalTrackNum, 0
	coverUrl, neteaseYear := "", ""

	if shouldTrustLocalMetadata(origTitle, origArtist, albumHint) {
		log.Printf("[Metadata Winner] Source: Local. Data: %s - %s / %s", origArtist, origTitle, albumHint)
		officialArtist, officialAlbum, officialTitle = origArtist, albumHint, origTitle
		if sourceTracks := sourceDiscTrackCount(st.FilePath); sourceTracks > 0 {
			trackCount = sourceTracks
		}
		goto PROCESS_PATH
	}

	chain = NewProviderChain()
	if meta, ok := chain.Fetch(origTitle, origArtist, albumHint); ok {
		officialArtist, officialAlbum, officialTitle = meta.Artist, meta.Album, meta.Title
		coverUrl, neteaseYear = meta.CoverURL, meta.Year
		if meta.TrackTotal > 0 {
			trackCount = meta.TrackTotal
		}
		trackNum = normalizeTrackNumber(meta.TrackNum, originalTrackNum)
	}

PROCESS_PATH:
	officialAlbum = api.PreferDetailedAlbumName(officialAlbum, albumHint)
	// 专辑fallback策略：优先使用目录提示，其次使用API返回值，最后使用歌曲标题
	if officialAlbum == "" || officialAlbum == "Unknown Album" {
		if albumHint != "" && albumHint != "Unknown Album" {
			officialAlbum = albumHint
			log.Printf("[Album Fallback] Using directory hint: %s", officialAlbum)
		} else {
			officialAlbum = officialTitle
			log.Printf("[Album Fallback] Using title as album: %s", officialAlbum)
		}
	}
	officialAlbum = normalizeAlbumName(officialAlbum)
	classicalMeta := parseClassicalAlbumMetadata(officialAlbum)
	if classicalMeta.Catalog == "" {
		classicalMeta.Catalog = sourceCatalog(st.FilePath)
	}
	var discogsMeta api.DiscogsReleaseMeta
	var discogsSource string
	if classicalMeta.IsClassical() {
		discogsMeta, discogsSource = fetchDiscogsMetaForClassical(classicalMeta)
		classicalMeta = enrichClassicalMetadataFromDiscogs(classicalMeta, discogsMeta)
		officialAlbum = classicalMeta.AlbumName(officialAlbum)
		log.Printf("[Classical Metadata] composer=%q work=%q conductor=%q ensemble=%q performer=%q catalog=%q",
			classicalMeta.Composer, classicalMeta.Work, classicalMeta.Conductor, classicalMeta.Ensemble, classicalMeta.Performer, classicalMeta.Catalog)
	}
	officialArtist = simplifyMetadataText(officialArtist)
	officialTitle = simplifyMetadataText(officialTitle)
	finalMainArtist := selectAlbumArtist(meta, st.FilePath, origArtist, officialArtist)
	finalMainArtist = simplifyMetadataText(finalMainArtist)
	finalMainArtist, officialArtist = preferAlbumTitleArtist(finalMainArtist, officialArtist, officialAlbum)
	if classicalMeta.IsClassical() {
		classicalMeta = resolveCanonicalClassicalMetadata(classicalMeta,
			officialAlbum, origAlbum, sourceAlbumDir, officialArtist, finalMainArtist,
			classicalMeta.Ensemble, classicalMeta.Performer)
		finalMainArtist = classicalMeta.PreferredArtist(finalMainArtist)
		officialArtist = classicalMeta.PreferredArtist(officialArtist)
	}
	log.Printf("[Album Artist] Using album artist: %s (track artist: %s)", finalMainArtist, officialArtist)
	discNum := discNumberFromSource(meta, st.FilePath)
	sourceDiscs := sourceDiscCount(st.FilePath)
	if sourceDiscs > 1 || discNum > 1 {
		trackNum = originalTrackNum
		if sourceTracks := sourceDiscTrackCount(st.FilePath); sourceTracks > 0 {
			trackCount = sourceTracks
		}
	}
	trackNum = normalizeTrackNumber(trackNum, originalTrackNum)
	trackCount = normalizeTrackTotal(trackCount, trackNum)

	itunesMeta := api.ItunesMeta{DiscTotal: 1}
	if !classicalMeta.IsClassical() {
		fetchedItunes := api.FetchItunesMeta(finalMainArtist, officialAlbum)
		if fetchedItunes.CollectionName == "" || albumNamesSimilar(officialAlbum, fetchedItunes.CollectionName) {
			itunesMeta = fetchedItunes
		} else {
			log.Printf("[iTunes Album Context] Rejecting iTunes album %q; keeping source album context %q", fetchedItunes.CollectionName, officialAlbum)
		}
	}
	if itunesMeta.AlbumArtist != "" && !classicalMeta.IsClassical() {
		finalMainArtist = preferItunesAlbumArtist(finalMainArtist, itunesMeta.AlbumArtist)
	}
	if (looksLikeUnknownArtist(officialArtist) || (api.ContainsChinese(finalMainArtist) && !api.ContainsChinese(officialArtist))) && !looksLikeUnknownArtist(finalMainArtist) {
		officialArtist = finalMainArtist
	}
	if itunesMeta.CollectionName != "" {
		if api.ContainsChinese(finalMainArtist) && api.ContainsChinese(itunesMeta.CollectionName) && !api.ContainsChinese(officialAlbum) {
			officialAlbum = normalizeAlbumName(itunesMeta.CollectionName)
		} else {
			officialAlbum = preferItunesAlbumName(officialAlbum, itunesMeta.CollectionName)
		}
	}
	discCount := itunesMeta.DiscTotal
	if discCount < sourceDiscs {
		discCount = sourceDiscs
	}
	if discCount < discNum {
		discCount = discNum
	}
	if discCount < 1 {
		discCount = 1
	}
	if err := validateOutputIdentity(finalMainArtist, officialAlbum); err != nil {
		return err
	}
	dirYear := albumDirectoryYear(sourceAlbumDir, origAlbum, officialAlbum, itunesMeta.Date, neteaseYear, meta.GetTag("RELEASEDATE"), meta.GetTag("DATE"), meta.GetTag("YEAR"), meta.GetTag("ORIGINALDATE"), meta.GetTag("ORIGINALYEAR"))
	albumPath := albumOutputPath(outRoot, finalMainArtist, officialAlbum, dirYear)
	basePath := discOutputPath(albumPath, discNum, discCount)
	log.Printf("[Output Path] %s", basePath)
	util.MkPathIfNotExists(basePath)
	var trackTitles map[int]string
	trackTitleSource := ""
	if len(discogsMeta.Tracks) > 0 && !hasChineseClassicalContext(classicalMeta) {
		trackTitles, trackTitleSource = remapTrackTitlesForSource(st.FilePath, discogsMeta.Tracks), discogsSource
	} else {
		trackTitles, trackTitleSource = albumTrackTitlesForConversion(st.FilePath, finalMainArtist, officialAlbum, classicalMeta)
		if len(trackTitles) == 0 && len(discogsMeta.Tracks) > 0 {
			trackTitles, trackTitleSource = remapTrackTitlesForSource(st.FilePath, discogsMeta.Tracks), discogsSource
		}
	}
	if len(trackTitles) > trackCount {
		trackCount = len(trackTitles)
	}
	resolvedTitle := titleFromOnlineTrack(trackNum, officialTitle, trackTitles, classicalMeta)
	if resolvedTitle != officialTitle {
		log.Printf("[Track Title] %02d %q -> %q (source: %s)", trackNum, officialTitle, resolvedTitle, trackTitleSource)
		officialTitle = resolvedTitle
	}
	if looksLikeGenericTrackTitle(officialTitle) {
		return fmt.Errorf("%w: title=%q album=%q artist=%q", errSkipNoTrackMetadata, officialTitle, officialAlbum, finalMainArtist)
	}
	if !classicalMeta.IsClassical() && (api.ContainsChinese(finalMainArtist) || api.ContainsChinese(officialAlbum)) && !api.ContainsChinese(officialTitle) && isLikelyPinyinTitle(officialTitle) {
		return fmt.Errorf("%w: pinyin title=%q album=%q artist=%q", errSkipNoTrackMetadata, officialTitle, officialAlbum, finalMainArtist)
	}

	coverPath := ""
	if coverUrl != "" {
		tmpPath := filepath.Join(albumPath, "folder.jpg")
		if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
			if dlErr := util.DownloadCover(coverUrl, tmpPath); dlErr == nil {
				coverPath = tmpPath
			}
		} else {
			coverPath = tmpPath
		}
	}

	ext := outputExtFor(st.FileSuffix)
	targetPath := filepath.Join(basePath, outputTrackFilename(trackNum, officialTitle, ext, classicalMeta))
	outFile := util.GetUniquePath(targetPath)

	allTags := collectAllTags(meta, nil, nil, neteaseYear)
	allTags["ARTIST"] = officialArtist
	allTags["ALBUM"] = officialAlbum
	allTags["TITLE"] = officialTitle
	allTags["ALBUMARTIST"] = finalMainArtist
	applyClassicalMetadataTags(allTags, classicalMeta)
	simplifyMetadataTags(allTags)

	enrichWithItunesAndStandardize(allTags, itunesMeta, trackNum, trackCount, discNum, discCount)

	args := []string{"-i", st.FilePath}
	hasCover := false
	if coverPath != "" {
		if _, err := os.Stat(coverPath); err == nil {
			args = append(args, "-i", coverPath, "-map", "0:a:0", "-map", "1:0")
			hasCover = true
		} else {
			args = append(args, "-map", "0:a:0")
		}
	} else {
		args = append(args, "-map", "0:a:0")
	}
	args = appendCodecArgs(args, ext, hasCover)
	args = append(args, "-map_metadata", "-1")
	args = buildMetadataArgs(args, allTags)

	lyrics := api.GetSongLyrics(officialTitle, finalMainArtist)
	if lyrics != "" {
		os.WriteFile(strings.TrimSuffix(outFile, ext)+".lrc", []byte(lyrics), 0644)
		args = append(args, "-metadata", "LYRICS="+lyrics)
	}
	args = append(args, "-y", outFile)

	if _, err := util.Run(ctx, "ffmpeg", args); err != nil {
		return err
	}
	if ext == ".flac" {
		applySwinsianTags(ctx, outFile, trackNum, trackCount, discNum, discCount, dirYear)
	}
	writeMissingTracksReport(basePath, trackCount, trackTitles, trackTitleSource)
	return nil
}

func cueConvert(ctx context.Context, st model.MusicFileInfo, outRoot string) error {
	var chain *ProviderChain
	meta, err := GetFormat(ctx, st.FilePath)
	if err != nil {
		return err
	}
	cueSheet, err := ParseCue(st.CuePath)
	if err != nil {
		return err
	}
	origArtist := meta.GetTag("ARTIST")
	if origArtist == "" && cueSheet.Performer != "" {
		origArtist = cueSheet.Performer
	}
	origArtist = normalizeArtistName(origArtist)

	officialArtist, officialAlbum, coverUrl, neteaseYear := "", "", "", ""
	absPath, _ := filepath.Abs(st.FilePath)
	dirParts := strings.Split(absPath, string(os.PathSeparator))
	sourceAlbumDir := sourceAlbumDirName(st.FilePath)
	cueAlbumTitle, cueDiscNum := cueAlbumTitleAndDisc(cueSheet.Title)

	// 清理并规范化专辑提示
	albumHint := api.CleanHint(cueAlbumTitle)
	if albumHint == "" || albumHint == "Unknown Album" {
		albumHint = api.CleanHint(meta.GetTag("ALBUM"))
	}
	if (albumHint == "" || albumHint == "Unknown Album") && len(dirParts) >= 2 {
		albumHint = api.CleanHint(sourceAlbumDir)
	}
	// 如果CUE中有TITLE，也作为参考
	if (albumHint == "" || albumHint == "Unknown Album") && cueSheet.Title != "" {
		albumHint = api.CleanHint(cueSheet.Title)
	}

	log.Printf("[CUE Processing] Artist: %s, Album Hint: %s, Track Count: %d", origArtist, albumHint, len(cueSheet.Tracks))

	if len(cueSheet.Tracks) > 0 {
		firstTrack := cueSheet.Tracks[0].Title

		chain = NewProviderChain()
		if meta, ok := chain.Fetch(firstTrack, origArtist, albumHint); ok {
			officialArtist, officialAlbum = meta.Artist, meta.Album
			coverUrl, neteaseYear = meta.CoverURL, meta.Year
		}
	}

	contextAlbum := cueAlbumContext(albumHint, sourceAlbumDir, cueAlbumTitle)
	officialAlbum = constrainCueAlbumToContext(officialAlbum, contextAlbum)
	officialAlbum = api.PreferDetailedAlbumName(officialAlbum, albumHint)
	// 专辑fallback策略：优先使用CUE标题，其次使用目录提示，最后使用第一首歌名
	if officialAlbum == "" || officialAlbum == "Unknown Album" {
		if cueAlbumTitle != "" {
			officialAlbum = api.CleanHint(cueAlbumTitle)
			log.Printf("[CUE Album Fallback] Using CUE title: %s", officialAlbum)
		} else if albumHint != "" && albumHint != "Unknown Album" {
			officialAlbum = albumHint
			log.Printf("[CUE Album Fallback] Using directory hint: %s", officialAlbum)
		} else if len(cueSheet.Tracks) > 0 {
			officialAlbum = cueSheet.Tracks[0].Title
			log.Printf("[CUE Album Fallback] Using first track title: %s", officialAlbum)
		}
	}
	officialAlbum = normalizeAlbumName(officialAlbum)
	classicalMeta := parseClassicalAlbumMetadata(officialAlbum)
	var discogsMeta api.DiscogsReleaseMeta
	var discogsSource string
	if classicalMeta.IsClassical() {
		discogsMeta, discogsSource = fetchDiscogsMetaForClassical(classicalMeta)
		classicalMeta = enrichClassicalMetadataFromDiscogs(classicalMeta, discogsMeta)
		officialAlbum = classicalMeta.AlbumName(officialAlbum)
		log.Printf("[Classical Metadata] composer=%q work=%q conductor=%q ensemble=%q performer=%q catalog=%q",
			classicalMeta.Composer, classicalMeta.Work, classicalMeta.Conductor, classicalMeta.Ensemble, classicalMeta.Performer, classicalMeta.Catalog)
	}
	officialArtist = simplifyMetadataText(officialArtist)
	finalMainArtist := selectAlbumArtist(meta, st.FilePath, origArtist, officialArtist)
	finalMainArtist = simplifyMetadataText(finalMainArtist)
	finalMainArtist, officialArtist = preferAlbumTitleArtist(finalMainArtist, officialArtist, officialAlbum)
	if classicalMeta.IsClassical() {
		var orchestraCandidates []string
		orchestraCandidates = append(orchestraCandidates,
			officialAlbum, cueAlbumTitle, albumHint, sourceAlbumDir,
			officialArtist, finalMainArtist, origArtist, cueSheet.Performer,
			classicalMeta.Ensemble, classicalMeta.Performer,
		)
		for _, track := range cueSheet.Tracks {
			orchestraCandidates = append(orchestraCandidates, track.Performer)
		}
		classicalMeta = resolveCanonicalClassicalMetadata(classicalMeta, orchestraCandidates...)
		finalMainArtist = classicalMeta.PreferredArtist(finalMainArtist)
		officialArtist = classicalMeta.PreferredArtist(officialArtist)
	}
	log.Printf("[Album Artist] Using album artist: %s (track artist: %s)", finalMainArtist, officialArtist)
	discNum := discNumberFromSource(meta, st.FilePath)
	if cueDiscNum > 0 {
		discNum = cueDiscNum
	}
	sourceDiscs := sourceDiscCount(st.FilePath)
	itunesMeta := api.ItunesMeta{DiscTotal: 1}
	if !classicalMeta.IsClassical() {
		fetchedItunes := api.FetchItunesMeta(finalMainArtist, officialAlbum)
		if fetchedItunes.CollectionName == "" || albumNamesSimilar(officialAlbum, fetchedItunes.CollectionName) {
			itunesMeta = fetchedItunes
		} else {
			log.Printf("[iTunes Album Context] Rejecting iTunes album %q; keeping source album context %q", fetchedItunes.CollectionName, officialAlbum)
		}
	}
	if itunesMeta.AlbumArtist != "" && !classicalMeta.IsClassical() {
		finalMainArtist = preferItunesAlbumArtist(finalMainArtist, itunesMeta.AlbumArtist)
	}
	if (looksLikeUnknownArtist(officialArtist) || (api.ContainsChinese(finalMainArtist) && !api.ContainsChinese(officialArtist))) && !looksLikeUnknownArtist(finalMainArtist) {
		officialArtist = finalMainArtist
	}
	if itunesMeta.CollectionName != "" {
		if api.ContainsChinese(finalMainArtist) && api.ContainsChinese(itunesMeta.CollectionName) && !api.ContainsChinese(officialAlbum) {
			officialAlbum = normalizeAlbumName(itunesMeta.CollectionName)
		} else {
			officialAlbum = preferItunesAlbumName(officialAlbum, itunesMeta.CollectionName)
		}
	}
	discCount := itunesMeta.DiscTotal
	if discCount < sourceDiscs {
		discCount = sourceDiscs
	}
	if discCount < discNum {
		discCount = discNum
	}
	if discCount < 1 {
		discCount = 1
	}
	if err := validateOutputIdentity(finalMainArtist, officialAlbum); err != nil {
		return err
	}
	dirYear := albumDirectoryYear(sourceAlbumDir, cueAlbumTitle, officialAlbum, cueSheet.Rem["DATE"], cueSheet.Rem["YEAR"], itunesMeta.Date, neteaseYear, cueSheet.Rem["RELEASEDATE"], meta.GetTag("RELEASEDATE"), meta.GetTag("DATE"), meta.GetTag("YEAR"), cueSheet.Rem["ORIGINALDATE"], cueSheet.Rem["ORIGINALYEAR"], meta.GetTag("ORIGINALDATE"), meta.GetTag("ORIGINALYEAR"))
	albumPath := albumOutputPath(outRoot, finalMainArtist, officialAlbum, dirYear)
	basePath := discOutputPath(albumPath, discNum, discCount)
	log.Printf("[Output Path] %s", basePath)
	util.MkPathIfNotExists(basePath)
	var onlineTrackTitles map[int]string
	onlineTrackTitleSource := ""
	if len(discogsMeta.Tracks) > 0 && !hasChineseClassicalContext(classicalMeta) {
		onlineTrackTitles, onlineTrackTitleSource = discogsMeta.Tracks, discogsSource
	} else {
		onlineTrackTitles, onlineTrackTitleSource = albumTrackTitlesForConversion(st.FilePath, finalMainArtist, officialAlbum, classicalMeta)
		if len(onlineTrackTitles) == 0 && len(discogsMeta.Tracks) > 0 {
			onlineTrackTitles, onlineTrackTitleSource = discogsMeta.Tracks, discogsSource
		}
	}

	coverPath := ""
	if coverUrl != "" {
		tmpPath := filepath.Join(albumPath, "folder.jpg")
		if _, err := os.Stat(tmpPath); os.IsNotExist(err) {
			if dlErr := util.DownloadCover(coverUrl, tmpPath); dlErr == nil {
				coverPath = tmpPath
			}
		} else {
			coverPath = tmpPath
		}
	}

	n := len(cueSheet.Tracks)
	cueTrackTitles := make(map[int]string)
	for i, track := range cueSheet.Tracks {
		title := track.Title
		if title == "" {
			title = fmt.Sprintf("Track %02d", track.TrackNum)
		}
		title = simplifyMetadataText(title)
		resolvedTitle := titleFromOnlineTrack(track.TrackNum, title, onlineTrackTitles, classicalMeta)
		if resolvedTitle != title {
			log.Printf("[Track Title] %02d %q -> %q (source: %s)", track.TrackNum, title, resolvedTitle, onlineTrackTitleSource)
			title = resolvedTitle
		}
		cueTrackTitles[track.TrackNum] = title
		outFile := util.GetUniquePath(filepath.Join(basePath, outputTrackFilename(track.TrackNum, title, ".flac", classicalMeta)))
		allTags := collectAllTags(meta, cueSheet, &track, neteaseYear)
		trackArtist := finalMainArtist
		if track.Performer != "" {
			trackArtist = api.GetArtistCanonicalName(track.Performer)
		}
		trackArtist = simplifyMetadataText(trackArtist)
		allTags["ARTIST"] = trackArtist
		allTags["ALBUM"] = officialAlbum
		allTags["TITLE"] = title
		allTags["ALBUMARTIST"] = finalMainArtist
		applyClassicalMetadataTags(allTags, classicalMeta)
		simplifyMetadataTags(allTags)
		enrichWithItunesAndStandardize(allTags, itunesMeta, track.TrackNum, n, discNum, discCount)

		args := []string{"-i", st.FilePath}
		hasCover := false
		if coverPath != "" {
			if _, err := os.Stat(coverPath); err == nil {
				args = append(args, "-i", coverPath, "-map", "0:a:0", "-map", "1:0")
				hasCover = true
			} else {
				args = append(args, "-map", "0:a:0")
			}
		} else {
			args = append(args, "-map", "0:a:0")
		}
		args = append(args, "-ss", fmt.Sprintf("%f", track.Index01Seconds))
		if i < n-1 {
			args = append(args, "-to", fmt.Sprintf("%f", cueSheet.Tracks[i+1].Index01Seconds))
		}
		args = appendCodecArgs(args, ".flac", hasCover)
		args = append(args, "-map_metadata", "-1")
		args = buildMetadataArgs(args, allTags)
		lyrics := api.GetSongLyrics(title, finalMainArtist)
		if lyrics != "" {
			os.WriteFile(strings.TrimSuffix(outFile, ".flac")+".lrc", []byte(lyrics), 0644)
			args = append(args, "-metadata", "LYRICS="+lyrics)
		}
		args = append(args, "-y", outFile)
		if _, err := util.Run(ctx, "ffmpeg", args); err == nil {
			applySwinsianTags(ctx, outFile, track.TrackNum, n, discNum, discCount, dirYear)
		} else {
			return fmt.Errorf("convert cue track %02d failed: %w", track.TrackNum, err)
		}
	}
	reportSource := "cue"
	if len(onlineTrackTitles) > 0 {
		reportSource = onlineTrackTitleSource
	}
	writeMissingTracksReport(basePath, n, cueTrackTitles, reportSource)
	return nil
}
