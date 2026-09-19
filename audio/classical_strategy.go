package audio

import (
	"MusicConvert/api"
	"MusicConvert/util"
	"strings"
	"regexp"
)

type classicalMetadata struct {
	Catalog            string
	Composer           string
	Work               string
	Conductor          string
	Ensemble           string
	CanonicalOrchestra string
	Performer          string
	Instrument         string
}

var classicalAliases = map[string]string{
	"史特拉汶斯基": "Igor Stravinsky",
	"斯特拉文斯基": "Igor Stravinsky",
	"春之祭":    "Le Sacre du printemps",
	"火鸟":     "The Firebird",
	"火鳥":     "The Firebird",
	"彼得洛希卡":  "Petrushka",
	"戴维斯":    "Colin Davis",
	"戴維斯":    "Colin Davis",
}

func cleanClassicalNameNoise(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	i := 0
	for i < len(runes) && runes[i] >= '0' && runes[i] <= '9' {
		i++
	}
	j := i
	for j < len(runes) && strings.ContainsRune("_ -.", runes[j]) {
		j++
	}
	if i > 0 && j < len(runes) && runes[j] >= 0x4e00 && runes[j] <= 0x9fff && runes[j] != '首' {
		s = strings.TrimSpace(string(runes[j:]))
	}
	s = strings.Trim(s, " :：")
	return s
}

func normalizeClassicalAlbumText(album string) string {
	album = util.ToSimplifiedChinese(normalizeAlbumName(album))
	album = strings.ReplaceAll(album, "鴅", "鸟")
	album = strings.ReplaceAll(album, "𫛝", "鸟")
	album = strings.ReplaceAll(album, "鱲", "鳟")
	album = strings.ReplaceAll(album, "𫚭", "鳟")
	return album
}

func looksLikeClassicalWork(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	for _, term := range []string{"协奏曲", "奏鸣曲", "交响曲", "五重奏", "四重奏", "三重奏", "concerto", "sonata", "symphony", "quintet", "quartet", "trio"} {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}

func parseClassicalAlbumMetadata(album string) classicalMetadata {
	album = normalizeClassicalAlbumText(album)
	var meta classicalMetadata
	if matches := catalogPrefixPattern.FindStringSubmatch(album); len(matches) >= 2 && !isDiscDir(matches[1]) {
		meta.Catalog = strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(matches[1]), "_", "-"))
	}
	if matches := quotedClassicalPattern.FindStringSubmatch(album); len(matches) >= 3 {
		meta.Composer = strings.Trim(strings.TrimSpace(matches[1]), " -_.。")
		meta.Work = strings.TrimSpace(matches[2])
		if meta.Catalog != "" {
			meta.Composer = strings.TrimSpace(catalogPrefixPattern.ReplaceAllString(meta.Composer, ""))
		}
	} else if matches := colonClassicalPattern.FindStringSubmatch(album); len(matches) >= 3 && (meta.Catalog != "" || looksLikeClassicalWork(matches[2])) {
		meta.Composer = strings.Trim(strings.TrimSpace(matches[1]), " -_.。")
		meta.Work = strings.TrimSpace(matches[2])
		if meta.Catalog != "" {
			meta.Composer = strings.TrimSpace(catalogPrefixPattern.ReplaceAllString(meta.Composer, ""))
		}
	} else if matches := hyphenClassicalPattern.FindStringSubmatch(album); len(matches) >= 3 {
		meta.Composer = strings.Trim(strings.TrimSpace(matches[1]), " -_.。")
		meta.Work = strings.TrimSpace(matches[2])
		if idx := strings.Index(meta.Work, "-"); idx >= 0 {
			if performer := strings.TrimSpace(meta.Work[idx+1:]); performer != "" {
				meta.Performer = performer
			}
			meta.Work = strings.TrimSpace(meta.Work[:idx])
		}
		if meta.Catalog != "" {
			meta.Composer = strings.TrimSpace(catalogPrefixPattern.ReplaceAllString(meta.Composer, ""))
		}
	}
	meta.Composer = cleanClassicalNameNoise(meta.Composer)
	meta.Work = cleanClassicalNameNoise(meta.Work)
	if idx := strings.Index(meta.Work, "-"); idx > 0 && looksLikeClassicalWork(meta.Work[:idx]) {
		if meta.Performer == "" {
			meta.Performer = strings.TrimSpace(meta.Work[idx+1:])
		}
		meta.Work = strings.TrimSpace(meta.Work[:idx])
	}
	if strings.Contains(meta.Work, "鳟鱼") && strings.Contains(album, "钢琴五重奏") {
		meta.Work = "鳟鱼钢琴五重奏"
	}
	if matches := conductorEnsemblePattern.FindStringSubmatch(album); len(matches) >= 3 {
		meta.Conductor = strings.TrimSpace(matches[1])
		meta.Ensemble = strings.TrimSpace(matches[2])
		meta.Performer = strings.TrimSpace(meta.Conductor + "指挥" + meta.Ensemble)
	}
	if meta.Ensemble == "" {
		if matches := chamberEnsemblePattern.FindStringSubmatch(album); len(matches) >= 2 {
			meta.Ensemble = strings.TrimSpace(matches[1])
		} else if candidate := orchestraCandidateFromText(album); candidate != "" {
			meta.Ensemble = candidate
		}
	}
	instrumentMatches := instrumentalistAfterPunctuationPat.FindAllStringSubmatch(album, -1)
	if len(instrumentMatches) == 0 {
		instrumentMatches = instrumentalistPattern.FindAllStringSubmatch(album, -1)
	}
	for _, matches := range instrumentMatches {
		if len(matches) < 3 {
			continue
		}
		performer := cleanClassicalNameNoise(matches[1])
		if performer != "" && performer != meta.Composer && !strings.Contains(performer, meta.Work) {
			if meta.Performer == "" || strings.Contains(meta.Performer, "指挥") {
				meta.Performer = performer
			}
			meta.Instrument = strings.TrimSpace(matches[2])
		}
	}
	return meta
}

func (m classicalMetadata) IsClassical() bool {
	return m.Catalog != "" || m.Composer != "" || m.Work != "" || m.Conductor != "" || m.Ensemble != "" || m.Instrument != ""
}

func (m classicalMetadata) OrchestraName() string {
	if m.CanonicalOrchestra != "" {
		return m.CanonicalOrchestra
	}
	if m.Ensemble != "" {
		return m.Ensemble
	}
	if m.Performer != "" {
		return orchestraCandidateFromText(m.Performer)
	}
	return ""
}

func (m classicalMetadata) AlbumName(fallback string) string {
	base := ""
	if m.Composer != "" && m.Work != "" {
		base = m.Composer + ": " + m.Work
	} else if m.Work != "" {
		base = m.Work
	} else {
		base = fallback
	}
	if disambiguator := m.AlbumDisambiguator(); disambiguator != "" && !strings.Contains(base, disambiguator) {
		base = strings.TrimSpace(base + " - " + disambiguator)
	}
	return normalizeAlbumName(base)
}

func (m classicalMetadata) AlbumDisambiguator() string {
	var parts []string
	add := func(part string) {
		part = strings.TrimSpace(part)
		if part == "" || part == m.Composer || part == m.Work {
			return
		}
		for _, existing := range parts {
			if existing == part {
				return
			}
		}
		parts = append(parts, part)
	}
	if m.Performer != "" && !strings.Contains(m.Performer, "指挥") {
		for _, part := range strings.FieldsFunc(m.Performer, func(r rune) bool { return r == ',' || r == '，' || r == '/' || r == '、' }) {
			add(part)
		}
	}
	if len(parts) > 0 {
		add(m.Conductor)
	}
	if len(parts) == 0 && m.Ensemble != "" && m.Conductor == "" {
		add(m.Ensemble)
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func (m classicalMetadata) PreferredArtist(fallback string) string {
	if orchestra := m.OrchestraName(); orchestra != "" {
		return orchestra
	}
	if m.Composer != "" {
		return m.Composer
	}
	return fallback
}

func classicalAlias(value string) string {
	value = normalizeAlbumName(value)
	for key, alias := range classicalAliases {
		if strings.Contains(value, normalizeAlbumName(key)) {
			return alias
		}
	}
	return ""
}

func (m classicalMetadata) SearchHints() []string {
	var hints []string
	add := func(parts ...string) {
		var cleaned []string
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				cleaned = append(cleaned, part)
			}
		}
		if len(cleaned) > 0 {
			hints = append(hints, strings.Join(cleaned, " "))
		}
	}
	composerAlias := classicalAlias(m.Composer)
	workAlias := classicalAlias(m.Work)
	conductorAlias := classicalAlias(m.Conductor)
	ensembleAlias := classicalAlias(m.OrchestraName())
	add(m.Catalog, composerAlias, workAlias, conductorAlias, ensembleAlias)
	add(composerAlias, workAlias, conductorAlias)
	add(workAlias, conductorAlias, ensembleAlias)
	add(m.Catalog, m.Composer, m.Work, m.Performer, m.OrchestraName())
	return hints
}

func classicalWorkFromDiscogsTitle(title string) string {
	title = normalizeAlbumName(title)
	title = strings.TrimSpace(regexp.MustCompile(`\s*\([^)]*\)\s*$`).ReplaceAllString(title, ""))
	return title
}

func classicalComposerFromDiscogsArtists(artists []string) string {
	for _, artist := range artists {
		name := strings.TrimSpace(localizedArtistName(artist))
		if name != "" {
			return name
		}
	}
	return ""
}

func localizedClassicalWorkName(work string) string {
	work = normalizeAlbumName(work)
	if work == "" || api.ContainsChinese(work) {
		return work
	}
	lower := strings.ToLower(work)
	if strings.Contains(lower, "sonata") && strings.Contains(lower, "violin") && strings.Contains(lower, "piano") {
		return combinedLocalizedTitle("小提琴奏鸣曲全集", work)
	}
	return work
}

func chineseClassicalWorkPrefix(work string) string {
	work = normalizeAlbumName(work)
	if !api.ContainsChinese(work) {
		return ""
	}
	if idx := strings.Index(work, "-"); idx > 0 {
		prefix := strings.TrimSpace(work[:idx])
		if api.ContainsChinese(prefix) {
			return prefix
		}
	}
	return work
}

func orchestraCandidateFromText(text string) string {
	text = normalizeClassicalAlbumText(text)
	if text == "" {
		return ""
	}
	if matches := conductorEnsemblePattern.FindStringSubmatch(text); len(matches) >= 3 {
		return strings.TrimSpace(matches[2])
	}
	lower := strings.ToLower(text)
	if idx := strings.LastIndex(text, "指挥"); idx >= 0 {
		right := strings.TrimSpace(text[idx+len("指挥"):])
		rightLower := strings.ToLower(right)
		if strings.Contains(rightLower, "orchestra") ||
			strings.Contains(rightLower, "philharmonic") ||
			strings.Contains(rightLower, "symphony") ||
			strings.Contains(rightLower, "ensemble") {
			return right
		}
	}
	if strings.Contains(lower, "orchestra") ||
		strings.Contains(lower, "philharmonic") ||
		strings.Contains(lower, "symphony") ||
		strings.Contains(lower, "ensemble") ||
		strings.Contains(text, "管弦乐团") ||
		strings.Contains(text, "管弦樂團") ||
		strings.Contains(text, "交响乐团") ||
		strings.Contains(text, "交響樂團") ||
		strings.Contains(text, "乐团") ||
		strings.Contains(text, "樂團") {
		return strings.TrimSpace(text)
	}
	return ""
}

func canonicalOrchestraFromCandidates(candidates ...string) string {
	best := ""
	bestLen := -1
	bestChinese := ""
	bestChineseLen := -1
	for _, candidate := range candidates {
		candidate = orchestraCandidateFromText(candidate)
		candidate = util.NormalizeChineseForCompare(candidate)
		candidate = normalizeAlbumName(candidate)
		if candidate == "" {
			continue
		}
		length := len([]rune(candidate))
		if api.ContainsChinese(candidate) && length >= bestChineseLen {
			bestChinese = candidate
			bestChineseLen = length
		}
		if length >= bestLen {
			best = candidate
			bestLen = length
		}
	}
	if bestChinese != "" {
		return bestChinese
	}
	return best
}

func resolveCanonicalClassicalMetadata(meta classicalMetadata, candidates ...string) classicalMetadata {
	orchestra := canonicalOrchestraFromCandidates(append(candidates, meta.CanonicalOrchestra, meta.Ensemble, meta.Performer)...)
	if orchestra == "" {
		return meta
	}
	meta.CanonicalOrchestra = orchestra
	meta.Ensemble = orchestra
	if meta.Conductor != "" {
		meta.Performer = strings.TrimSpace(meta.Conductor + "指挥" + orchestra)
	}
	return meta
}

func preferLocalClassicalText(local, external string) string {
	local = strings.TrimSpace(local)
	external = strings.TrimSpace(external)
	if local == "" {
		return external
	}
	if external == "" {
		return local
	}
	if api.ContainsChinese(local) && (classicalAlias(local) != "" || !api.ContainsChinese(external)) {
		return local
	}
	return external
}

func preferClassicalCreditName(local, external string) string {
	local = strings.TrimSpace(local)
	external = strings.TrimSpace(external)
	if external == "" {
		return local
	}
	localized := localizedArtistName(external)
	if api.ContainsChinese(local) && !api.ContainsChinese(localized) {
		return local
	}
	return localized
}

func hasChineseClassicalContext(meta classicalMetadata) bool {
	return api.ContainsChinese(meta.Composer) || api.ContainsChinese(meta.Work) || api.ContainsChinese(meta.AlbumName(""))
}

func enrichClassicalMetadataFromDiscogs(meta classicalMetadata, release api.DiscogsReleaseMeta) classicalMetadata {
	if release.ID <= 0 {
		return meta
	}
	if work := classicalWorkFromDiscogsTitle(release.Title); work != "" {
		meta.Work = preferLocalClassicalText(meta.Work, localizedClassicalWorkName(work))
	}
	if composer := classicalComposerFromDiscogsArtists(release.Artists); composer != "" {
		meta.Composer = preferLocalClassicalText(meta.Composer, composer)
	}
	discogsConductor := ""
	discogsEnsemble := ""
	var discogsPerformers []string
	for _, credit := range release.Credits {
		role := strings.ToLower(credit.Role)
		if discogsConductor == "" && strings.Contains(role, "conduct") {
			if name := strings.TrimSpace(normalizeArtistName(credit.Name)); name != "" {
				discogsConductor = name
			}
		}
		if discogsEnsemble == "" && (strings.Contains(role, "orchestra") || strings.Contains(role, "quartet") || strings.Contains(role, "trio") || strings.Contains(role, "quintet") || strings.Contains(role, "ensemble")) {
			if name := strings.TrimSpace(normalizeArtistName(credit.Name)); name != "" {
				discogsEnsemble = name
			}
		}
		if strings.Contains(role, "piano") || strings.Contains(role, "violin") || strings.Contains(role, "cello") || strings.Contains(role, "flute") || strings.Contains(role, "clarinet") {
			if name := strings.TrimSpace(normalizeArtistName(credit.Name)); name != "" {
				discogsPerformers = append(discogsPerformers, name)
			}
		}
	}
	for _, artist := range release.Artists {
		if discogsEnsemble == "" {
			lowerArtist := strings.ToLower(artist)
			if name := strings.TrimSpace(normalizeArtistName(artist)); name != "" && (strings.Contains(lowerArtist, "orchestra") || strings.Contains(lowerArtist, "quartet") || strings.Contains(lowerArtist, "trio") || strings.Contains(lowerArtist, "quintet")) {
				discogsEnsemble = name
			}
		}
	}
	if discogsConductor != "" {
		meta.Conductor = preferClassicalCreditName(meta.Conductor, discogsConductor)
	}
	if discogsEnsemble != "" {
		meta.Ensemble = preferClassicalCreditName(meta.Ensemble, discogsEnsemble)
	}
	if meta.Performer == "" && len(discogsPerformers) > 0 {
		meta.Performer = strings.Join(discogsPerformers, ", ")
	}
	if meta.Conductor != "" && meta.Ensemble != "" {
		meta.Performer = meta.Conductor + "指挥" + meta.Ensemble
	}
	return meta
}

func applyClassicalMetadataTags(tags map[string]string, meta classicalMetadata) {
	if !meta.IsClassical() {
		return
	}
	if meta.Catalog != "" {
		tags["CATALOGNUMBER"] = meta.Catalog
	}
	if meta.Composer != "" {
		tags["COMPOSER"] = meta.Composer
	}
	if meta.Conductor != "" {
		tags["CONDUCTOR"] = meta.Conductor
	}
	if orchestra := meta.OrchestraName(); orchestra != "" {
		tags["ORCHESTRA"] = orchestra
		tags["ENSEMBLE"] = orchestra
	}
	if meta.Performer != "" {
		tags["PERFORMER"] = meta.Performer
	}
	if meta.Instrument != "" {
		tags["GROUPING"] = meta.Instrument
	}
	if tags["GENRE"] == "" {
		tags["GENRE"] = "Classical"
	}
}

func combinedLocalizedTitle(chinese, original string) string {
	chinese = normalizeAlbumName(chinese)
	original = normalizeAlbumName(original)
	if original == "" {
		return chinese
	}
	if chinese == "" || strings.EqualFold(chinese, original) {
		return original
	}
	if strings.Contains(original, chinese) || strings.Contains(chinese, original) {
		return original
	}
	return chinese + "-" + original
}

func shouldPrefixChineseWork(classicalMeta classicalMetadata, title string) bool {
	if chineseClassicalWorkPrefix(classicalMeta.Work) == "" {
		return false
	}
	if !api.ContainsChinese(title) || containsKana(title) {
		return true
	}
	return false
}

func fetchDiscogsMetaForClassical(classicalMeta classicalMetadata) (api.DiscogsReleaseMeta, string) {
	if !classicalMeta.IsClassical() {
		return api.DiscogsReleaseMeta{}, ""
	}
	primary := ""
	if catalogs := relatedCatalogs(classicalMeta.Catalog); len(catalogs) > 0 {
		primary = catalogs[0]
	}
	for _, catalog := range relatedMetadataCatalogs(classicalMeta.Catalog) {
		meta, source := api.FetchDiscogsReleaseMeta(catalog, classicalMeta.SearchHints())
		if meta.ID > 0 {
			if primary != "" && catalog != primary {
				meta.Tracks = nil
				source += ":metadata"
			}
			return meta, source
		}
	}
	return api.DiscogsReleaseMeta{}, ""
}

func localizedClassicalTrackTitles(classicalMeta classicalMetadata) (map[int]string, string) {
	if !classicalMeta.IsClassical() || !hasChineseClassicalContext(classicalMeta) {
		return nil, ""
	}
	tracks, source := api.FetchWikipediaClassicalTrackTitles(classicalMeta.Work, classicalMeta.Composer)
	for track, title := range tracks {
		tracks[track] = simplifyMetadataText(title)
	}
	return tracks, source
}