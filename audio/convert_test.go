package audio

import (
	"MusicConvert/api"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAlbumDirNameAddsYear(t *testing.T) {
	got := albumDirName("渡(The Crossing)", "2017")
	want := "渡(The Crossing) [2017]"
	if got != want {
		t.Fatalf("albumDirName() = %q, want %q", got, want)
	}
}

func TestAlbumDirNameDoesNotDuplicateYear(t *testing.T) {
	got := albumDirName("渡(The Crossing) [2017]", "2017")
	want := "渡(The Crossing) [2017]"
	if got != want {
		t.Fatalf("albumDirName() = %q, want %q", got, want)
	}
}

func TestSelectAlbumArtistPrefersPathOverDuetTrackArtist(t *testing.T) {
	meta := &AutoMetadata{}
	got := selectAlbumArtist(meta, "/volume4/Music/薛之谦/2022-无数/04 你不是一个人.flac", "张信哲-薛之谦", "张信哲-薛之谦")
	want := "薛之谦"
	if got != want {
		t.Fatalf("selectAlbumArtist() = %q, want %q", got, want)
	}
}

func TestSelectAlbumArtistPrefersCueArtistOverGenericPath(t *testing.T) {
	meta := &AutoMetadata{}
	got := selectAlbumArtist(meta, "/volume4/Music/import/Dylan/Bootleg Series/CD3/image.flac", "Dylan, Bob", "Bob Dylan")
	want := "Bob Dylan"
	if got != want {
		t.Fatalf("selectAlbumArtist() = %q, want %q", got, want)
	}
}

func TestNormalizeArtistNameSortName(t *testing.T) {
	got := normalizeArtistName("Dylan, Bob")
	want := "Bob Dylan"
	if got != want {
		t.Fatalf("normalizeArtistName() = %q, want %q", got, want)
	}
}

func TestPrimaryAlbumArtistUsesLeadForeignArtist(t *testing.T) {
	got := primaryAlbumArtist("Bob Dylan & The Band")
	want := "Bob Dylan"
	if got != want {
		t.Fatalf("primaryAlbumArtist() = %q, want %q", got, want)
	}
}

func TestSelectAlbumArtistKeepsEnglishAliasForTranslatedArtist(t *testing.T) {
	meta := &AutoMetadata{}
	got := selectAlbumArtist(meta, "/volume4/Music/鲍勃·迪伦-Bob Dylan/Blonde on Blonde/01.flac", "鲍勃·迪伦-Bob Dylan", "鲍勃·迪伦-Bob Dylan")
	want := "Bob Dylan"
	if got != want {
		t.Fatalf("selectAlbumArtist() = %q, want %q", got, want)
	}
}

func TestAlbumDirNameUsesLocalOriginalYear(t *testing.T) {
	year := firstYear("Blonde on Blonde", "1966-Blonde on Blonde", "2021")
	got := albumDirName("Blonde on Blonde", year)
	want := "Blonde on Blonde [1966]"
	if got != want {
		t.Fatalf("albumDirName() = %q, want %q", got, want)
	}
}

func TestAlbumDirNameNormalizesPunctuation(t *testing.T) {
	got := albumDirName("The Times They Are A‐Changin’", "1964")
	want := "The Times They Are A-Changin' [1964]"
	if got != want {
		t.Fatalf("albumDirName() = %q, want %q", got, want)
	}
	if got2 := albumDirName("The Times They Are A-Changin'", "1964"); got2 != want {
		t.Fatalf("albumDirName() = %q, want %q", got2, want)
	}
}

func TestAlbumDirNameConvertsTraditionalChinese(t *testing.T) {
	got := albumDirName("張國榮", "1989")
	want := "张国荣 [1989]"
	if got != want {
		t.Fatalf("albumDirName() = %q, want %q", got, want)
	}
}

func TestDiscDirectoryParsing(t *testing.T) {
	path := "/volume4/Music/Bob Dylan/Blonde on Blonde/CD2/06. Sad Eyed Lady.flac"
	if got, want := sourceAlbumDirName(path), "Blonde on Blonde"; got != want {
		t.Fatalf("sourceAlbumDirName() = %q, want %q", got, want)
	}
	if got, want := artistFromPath(path), "Bob Dylan"; got != want {
		t.Fatalf("artistFromPath() = %q, want %q", got, want)
	}
	if got, want := discNumberFromName("CD2"), 2; got != want {
		t.Fatalf("discNumberFromName() = %d, want %d", got, want)
	}
}

func TestVinylDiscDirectoryParsing(t *testing.T) {
	path := "/volume4/Music/Bob Dylan/Blonde on Blonde (1966)/12 Vinyl 02/Bob Dylan - Blonde on Blonde - 04 - 4th Time Around.flac"
	if got, want := sourceAlbumDirName(path), "Blonde on Blonde (1966)"; got != want {
		t.Fatalf("sourceAlbumDirName() = %q, want %q", got, want)
	}
	if got, want := artistFromPath(path), "Bob Dylan"; got != want {
		t.Fatalf("artistFromPath() = %q, want %q", got, want)
	}
	if got, want := discNumberFromName("12 Vinyl 02"), 2; got != want {
		t.Fatalf("discNumberFromName() = %d, want %d", got, want)
	}
}

func TestFirstYearIgnoresNonDigits(t *testing.T) {
	if got := firstYear("12 Vinyl 02"); got != "" {
		t.Fatalf("firstYear() = %q, want empty", got)
	}
	if got, want := firstYear("Blonde on Blonde (1966)", "2021-01-01"), "1966"; got != want {
		t.Fatalf("firstYear() = %q, want %q", got, want)
	}
}

func TestDirectoryYearPrefersReleaseFolderOverTrackOriginalYear(t *testing.T) {
	year := albumDirectoryYear(
		"1991. Bob Dyaln - The Bootleg Series Volumes 1-3 1961-1991",
		"Bob Dyaln - The Bootleg Series Volumes 1-3 1961-1991",
		"The Bootleg Series, Vols. 1-3 (Rare & Unreleased) 1961-1991",
		"1961",
	)
	got := albumDirName("The Bootleg Series, Vols. 1-3 (Rare & Unreleased) 1961-1991", year)
	want := "The Bootleg Series, Vols. 1-3 (Rare & Unreleased) 1961-1991 [1991]"
	if got != want {
		t.Fatalf("albumDirName() = %q, want %q", got, want)
	}
}

func TestDirectoryYearIgnoresMismatchedReleaseFolder(t *testing.T) {
	year := albumDirectoryYear(
		"1991. Bob Dyaln - The Bootleg Series Volumes 1-3 1961-1991",
		"Bob Dyaln - The Bootleg Series Volumes 1-3 1961-1991",
		"Blood On the Tracks",
		"1975-01-20",
		"1991",
	)
	got := albumDirName("Blood On the Tracks", year)
	want := "Blood On the Tracks [1975]"
	if got != want {
		t.Fatalf("albumDirName() = %q, want %q", got, want)
	}
}

func TestConstrainCueAlbumRejectsTrackLevelAlbum(t *testing.T) {
	contextAlbum := cueAlbumContext(
		"Bob Dyaln - The Bootleg Series Volumes 1-3 1961-1991",
		"1991. Bob Dyaln - The Bootleg Series Volumes 1-3 1961-1991",
		"The Bootleg Series Volumes 1-3",
	)
	got := constrainCueAlbumToContext("Blood On the Tracks", contextAlbum)
	want := "Bob Dyaln - The Bootleg Series Volumes 1-3 1961-1991"
	if got != want {
		t.Fatalf("constrainCueAlbumToContext() = %q, want %q", got, want)
	}
}

func TestConstrainCueAlbumKeepsSameAlbum(t *testing.T) {
	contextAlbum := "Bob Dyaln - The Bootleg Series Volumes 1-3 1961-1991"
	got := constrainCueAlbumToContext("The Bootleg Series, Vols. 1-3 (Rare & Unreleased) 1961-1991", contextAlbum)
	want := "The Bootleg Series, Vols. 1-3 (Rare & Unreleased) 1961-1991"
	if got != want {
		t.Fatalf("constrainCueAlbumToContext() = %q, want %q", got, want)
	}
}

func TestConstrainCueAlbumRejectsDifferentBootlegVolume(t *testing.T) {
	contextAlbum := "The Bootleg Series Volumes 1-3 (Rare & Unreleased) 1961-1991"
	got := constrainCueAlbumToContext("The Bootleg Series, Vol. 9: The Witmark Demos: 1962-1964", contextAlbum)
	if got != contextAlbum {
		t.Fatalf("constrainCueAlbumToContext() = %q, want %q", got, contextAlbum)
	}
}

func TestAlbumArtistFromAlbumTitleReturnsOrchestraSide(t *testing.T) {
	got := albumArtistFromAlbumTitle("戴維斯指揮阿姆斯丹大會堂管弦樂團")
	want := "阿姆斯丹大会堂管弦乐团"
	if got != want {
		t.Fatalf("albumArtistFromAlbumTitle() = %q, want %q", got, want)
	}
}

func TestCanonicalOrchestraPrefersLongerCandidate(t *testing.T) {
	got := canonicalOrchestraFromCandidates(
		"戴緈斯指揮阿姆斯丹大會堂管弦樂團",
		"戴維斯指揮阿姆斯丹堂管弦樂團",
	)
	want := "阿姆斯丹大会堂管弦乐团"
	if got != want {
		t.Fatalf("canonicalOrchestraFromCandidates() = %q, want %q", got, want)
	}
}

func TestCanonicalOrchestraPrefersChineseCandidate(t *testing.T) {
	got := canonicalOrchestraFromCandidates(
		"Royal Concertgebouw Orchestra",
		"戴維斯指揮阿姆斯丹大會堂管弦樂團",
	)
	want := "阿姆斯丹大会堂管弦乐团"
	if got != want {
		t.Fatalf("canonicalOrchestraFromCandidates() = %q, want %q", got, want)
	}
}

func TestOrchestraCandidateSplitsEnglishConductorPerformer(t *testing.T) {
	got := orchestraCandidateFromText("Claudio Abbado指挥London Symphony Orchestra")
	want := "London Symphony Orchestra"
	if got != want {
		t.Fatalf("orchestraCandidateFromText() = %q, want %q", got, want)
	}
}

func TestCanonicalOrchestraUsesLaterEqualLengthCandidate(t *testing.T) {
	got := canonicalOrchestraFromCandidates("伦郭交响乐团", "伦敦交响乐团")
	want := "伦敦交响乐团"
	if got != want {
		t.Fatalf("canonicalOrchestraFromCandidates() = %q, want %q", got, want)
	}
}

func TestPreferItunesAlbumNameRejectsDifferentBootlegVolume(t *testing.T) {
	current := "The Bootleg Series Volumes 1-3 (Rare & Unreleased) 1961-1991"
	got := preferItunesAlbumName(current, "The Bootleg Series, Vol. 9: The Witmark Demos: 1962-1964")
	if got != current {
		t.Fatalf("preferItunesAlbumName() = %q, want %q", got, current)
	}
}

func TestCueAlbumTitleAndDisc(t *testing.T) {
	title, disc := cueAlbumTitleAndDisc("The Bootleg Series Volumes 1-3 (Rare & Unreleased) 1961-1991 [Disc 3]")
	if title != "The Bootleg Series Volumes 1-3 (Rare & Unreleased) 1961-1991" || disc != 3 {
		t.Fatalf("cueAlbumTitleAndDisc() = %q, %d", title, disc)
	}
}

func TestSourceDiscTrackTitles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{
		"Bob Dylan - Blonde on Blonde - 02 - Pledging My Time.flac",
		"Bob Dylan - Blonde on Blonde - 03 - Visions of Johanna.flac",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	got := sourceDiscTrackTitles(filepath.Join(dir, "Bob Dylan - Blonde on Blonde - 03 - Visions of Johanna.flac"))
	if got[2] != "Pledging My Time" || got[3] != "Visions of Johanna" {
		t.Fatalf("sourceDiscTrackTitles() = %#v", got)
	}
}

func TestParseTrackNumberFromGenericTrackFilename(t *testing.T) {
	if got, want := parseTrackNumberFromFilename("Track13.wav"), 13; got != want {
		t.Fatalf("parseTrackNumberFromFilename() = %d, want %d", got, want)
	}
}

func TestInferOriginalTrackNumberFromGenericTitle(t *testing.T) {
	meta := &AutoMetadata{}
	if got, want := inferOriginalTrackNumber(meta, "Track13", "Track13"), 13; got != want {
		t.Fatalf("inferOriginalTrackNumber() = %d, want %d", got, want)
	}
}

func TestShouldTrustLocalMetadataForGenericTrack(t *testing.T) {
	if !shouldTrustLocalMetadata("Track13", "PHILIPS首版系列纸盒WAV", "PHCP-24002史特拉汶斯基「春之祭」。戴维斯指挥阿姆斯丹堂管弦乐团。") {
		t.Fatal("shouldTrustLocalMetadata() = false, want true")
	}
}

func TestLooksLikeGenericMusicFolderForPhilipsSource(t *testing.T) {
	if !looksLikeGenericMusicFolder("PHILIPS首版系列纸盒WAV") {
		t.Fatal("looksLikeGenericMusicFolder() = false, want true")
	}
}

func TestAlbumArtistFromAlbumTitle(t *testing.T) {
	got := albumArtistFromAlbumTitle("PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。")
	want := "阿姆斯丹堂管弦乐团"
	if got != want {
		t.Fatalf("albumArtistFromAlbumTitle() = %q, want %q", got, want)
	}
}

func TestPreferAlbumTitleArtistOverridesGenericPhilipsFolder(t *testing.T) {
	album := "PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。"
	gotAlbumArtist, gotTrackArtist := preferAlbumTitleArtist("PHILIPS首版系列纸盒WAV", "PHILIPS首版系列纸盒WAV", album)
	want := "阿姆斯丹堂管弦乐团"
	if gotAlbumArtist != want || gotTrackArtist != want {
		t.Fatalf("preferAlbumTitleArtist() = %q, %q, want %q, %q", gotAlbumArtist, gotTrackArtist, want, want)
	}
}

func TestParseClassicalAlbumMetadataFromChineseTitle(t *testing.T) {
	got := parseClassicalAlbumMetadata("PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。")
	if !got.IsClassical() {
		t.Fatal("parseClassicalAlbumMetadata().IsClassical() = false, want true")
	}
	if got.Catalog != "PHCP-24002" {
		t.Fatalf("Catalog = %q, want PHCP-24002", got.Catalog)
	}
	if got.Composer != "史特拉汶斯基" || got.Work != "春之祭" {
		t.Fatalf("Composer/Work = %q/%q, want 史特拉汶斯基/春之祭", got.Composer, got.Work)
	}
	if got.Conductor != "戴维斯" || got.Ensemble != "阿姆斯丹堂管弦乐团" {
		t.Fatalf("Conductor/Ensemble = %q/%q", got.Conductor, got.Ensemble)
	}
	if got.AlbumName("fallback") != "史特拉汶斯基: 春之祭" {
		t.Fatalf("AlbumName() = %q", got.AlbumName("fallback"))
	}
}

func TestParseClassicalAlbumMetadataNormalizesCatalogAndTraditionalText(t *testing.T) {
	got := parseClassicalAlbumMetadata("phcp-24003史特拉汶斯基「火鳥」。戴維斯指揮阿姆斯丹大會堂管弦樂團。")
	if got.Catalog != "PHCP-24003" {
		t.Fatalf("Catalog = %q, want PHCP-24003", got.Catalog)
	}
	if got.Composer != "史特拉汶斯基" || got.Work != "火鸟" {
		t.Fatalf("Composer/Work = %q/%q, want 史特拉汶斯基/火鸟", got.Composer, got.Work)
	}
	if got.Conductor != "戴维斯" || got.Ensemble != "阿姆斯丹大会堂管弦乐团" {
		t.Fatalf("Conductor/Ensemble = %q/%q", got.Conductor, got.Ensemble)
	}
	if got.AlbumName("fallback") != "史特拉汶斯基: 火鸟" {
		t.Fatalf("AlbumName() = %q", got.AlbumName("fallback"))
	}
}

func TestParseClassicalAlbumMetadataParsesColonAlbumContext(t *testing.T) {
	got := parseClassicalAlbumMetadata("phcp-24008舒曼：鋼琴協奏曲。布蘭德爾鋼琴，阿巴多指揮倫郭交響樂團。")
	if got.Catalog != "PHCP-24008" {
		t.Fatalf("Catalog = %q, want PHCP-24008", got.Catalog)
	}
	if got.Composer != "舒曼" || got.Work != "钢琴协奏曲" {
		t.Fatalf("Composer/Work = %q/%q, want 舒曼/钢琴协奏曲", got.Composer, got.Work)
	}
	if got.Conductor != "阿巴多" || got.Ensemble != "伦郭交响乐团" {
		t.Fatalf("Conductor/Ensemble = %q/%q", got.Conductor, got.Ensemble)
	}
}

func TestParseClassicalAlbumMetadataCleansNumberedHyphenContext(t *testing.T) {
	got := parseClassicalAlbumMetadata("9贝多芬- 小提琴奏鸣曲全集-谢林,海布勒")
	if got.Composer != "贝多芬" || got.Work != "小提琴奏鸣曲全集" {
		t.Fatalf("Composer/Work = %q/%q, want 贝多芬/小提琴奏鸣曲全集", got.Composer, got.Work)
	}
	if artist := normalizeArtistName("7_维瓦尔第"); artist != "维瓦尔第" {
		t.Fatalf("normalizeArtistName() = %q, want 维瓦尔第", artist)
	}
	if album := normalizeAlbumName("12首小提琴协奏曲和谐的灵感Op3"); album != "12首小提琴协奏曲和谐的灵感Op3" {
		t.Fatalf("normalizeAlbumName() = %q", album)
	}
}

func TestNormalizeAlbumNameCleansClassicalPunctuationAndRareGlyph(t *testing.T) {
	got := normalizeAlbumName("舒伯特：- 𫚭鱼")
	want := "舒伯特: 鳟鱼"
	if got != want {
		t.Fatalf("normalizeAlbumName() = %q, want %q", got, want)
	}
}

func TestParseClassicalAlbumMetadataDoesNotLetCatalogPolluteComposer(t *testing.T) {
	got := parseClassicalAlbumMetadata("phcp-24003史特拉汶斯基「火鴅」。戴緎斯指揮阿姆斯丹大會堂管弦樂坒。")
	if got.Catalog != "PHCP-24003" {
		t.Fatalf("Catalog = %q, want PHCP-24003", got.Catalog)
	}
	if strings.Contains(strings.ToLower(got.Composer), "phcp") {
		t.Fatalf("Composer = %q, should not contain catalog", got.Composer)
	}
	if got.Work != "火鸟" {
		t.Fatalf("Work = %q, want 火鸟", got.Work)
	}
}

func TestEnrichClassicalMetadataFromDiscogsRelease(t *testing.T) {
	meta := parseClassicalAlbumMetadata("phcp-24003史特拉汶斯基「火鴅」。戴緎斯指揮阿姆斯丹大會堂管弦樂坒。")
	got := enrichClassicalMetadataFromDiscogs(meta, api.DiscogsReleaseMeta{
		ID:      35094653,
		Title:   "L'Oiseau De Feu (Complete Original Version)",
		Credits: []api.DiscogsCredit{{Name: "Colin Davis", Role: "Conductor"}, {Name: "Royal Concertgebouw Orchestra", Role: "Orchestra"}},
	})
	if got.Work != "火鸟" {
		t.Fatalf("Work = %q, want 火鸟", got.Work)
	}
	if got.Conductor == "" || got.Ensemble == "" || strings.Contains(got.Ensemble, "指挥") {
		t.Fatalf("Conductor/Ensemble = %q/%q, want separate Discogs credits", got.Conductor, got.Ensemble)
	}
	if strings.Contains(got.Performer, "戴緎斯") {
		t.Fatalf("Performer = %q, should not keep dirty local conductor", got.Performer)
	}
	if got.AlbumName("fallback") != "史特拉汶斯基: 火鸟" {
		t.Fatalf("AlbumName() = %q", got.AlbumName("fallback"))
	}
}

func TestEnrichClassicalMetadataFromDiscogsKeepsValidChineseAlbum(t *testing.T) {
	meta := parseClassicalAlbumMetadata("phcp-24003史特拉汶斯基「火鳥」。戴維斯指揮阿姆斯丹大會堂管弦樂團。")
	got := enrichClassicalMetadataFromDiscogs(meta, api.DiscogsReleaseMeta{
		ID:      35094653,
		Title:   "L'Oiseau De Feu (Complete Original Version)",
		Artists: []string{"Igor Stravinsky"},
	})
	if got.Composer != "史特拉汶斯基" || got.Work != "火鸟" {
		t.Fatalf("Composer/Work = %q/%q, want Chinese local values", got.Composer, got.Work)
	}
	if got.AlbumName("fallback") != "史特拉汶斯基: 火鸟" {
		t.Fatalf("AlbumName() = %q", got.AlbumName("fallback"))
	}
}

func TestApplyClassicalMetadataTags(t *testing.T) {
	meta := parseClassicalAlbumMetadata("PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。")
	tags := map[string]string{}
	applyClassicalMetadataTags(tags, meta)
	wants := map[string]string{
		"CATALOGNUMBER": "PHCP-24002",
		"COMPOSER":      "史特拉汶斯基",
		"CONDUCTOR":     "戴维斯",
		"ORCHESTRA":     "阿姆斯丹堂管弦乐团",
		"ENSEMBLE":      "阿姆斯丹堂管弦乐团",
		"PERFORMER":     "戴维斯指挥阿姆斯丹堂管弦乐团",
		"GENRE":         "Classical",
	}
	for key, want := range wants {
		if tags[key] != want {
			t.Fatalf("tags[%s] = %q, want %q; tags=%#v", key, tags[key], want, tags)
		}
	}
}

func TestTitleFromOnlineTrackPrefixesChineseWorkForClassicalGenericTitle(t *testing.T) {
	meta := parseClassicalAlbumMetadata("PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。")
	tracks := map[int]string{8: "The Rite of Spring: Part II. The Sacrifice"}
	got := titleFromOnlineTrack(8, "Track08", tracks, meta)
	want := "春之祭-The Rite of Spring: Part II. The Sacrifice"
	if got != want {
		t.Fatalf("titleFromOnlineTrack() = %q, want %q", got, want)
	}
}

func TestTitleFromOnlineTrackBuildsBilingualClassicalTitle(t *testing.T) {
	meta := parseClassicalAlbumMetadata("PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。")
	tracks := map[int]string{4: "Dance Of The Firebird"}
	got := titleFromOnlineTrack(4, "第1场: 火鸟", tracks, meta)
	want := "第1场: 火鸟-Dance Of The Firebird"
	if got != want {
		t.Fatalf("titleFromOnlineTrack() = %q, want %q", got, want)
	}
}

func TestOutputTrackFilenameKeepsPrefixForClassicalResolvedTitle(t *testing.T) {
	meta := parseClassicalAlbumMetadata("PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。")
	got := outputTrackFilename(1, "Part I: The Adoration Of The Earth: 1. Introduction", ".flac", meta)
	want := "01. Part I- The Adoration Of The Earth- 1. Introduction.flac"
	if got != want {
		t.Fatalf("outputTrackFilename() = %q, want %q", got, want)
	}
}

func TestTitleFromOnlineTrackIgnoresGenericOnlineTitle(t *testing.T) {
	meta := parseClassicalAlbumMetadata("PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。")
	tracks := map[int]string{7: "Track07"}
	got := titleFromOnlineTrack(7, "Track07", tracks, meta)
	if got != "Track07" {
		t.Fatalf("titleFromOnlineTrack() = %q, want Track07", got)
	}
}

func TestTitleFromOnlineTrackKeepsSpecificNonClassicalTitle(t *testing.T) {
	tracks := map[int]string{1: "Online Title"}
	got := titleFromOnlineTrack(1, "Local Title", tracks, classicalMetadata{})
	if got != "Local Title" {
		t.Fatalf("titleFromOnlineTrack() = %q, want Local Title", got)
	}
}

func TestSourceDiscTrackTitlesSkipsGenericTrackNames(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"Track07.wav", "08. Real Movement.wav"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	got := sourceDiscTrackTitles(filepath.Join(dir, "Track07.wav"))
	if _, ok := got[7]; ok {
		t.Fatalf("sourceDiscTrackTitles()[7] = %q, want absent", got[7])
	}
	if got[8] != "Real Movement" {
		t.Fatalf("sourceDiscTrackTitles()[8] = %q, want Real Movement", got[8])
	}
}

func TestClassicalMetadataSearchHintsUseEnglishAliases(t *testing.T) {
	meta := parseClassicalAlbumMetadata("PHCP-24002史特拉汶斯基「春之祭」。戴維斯指揮阿姆斯丹堂管弦樂團。")
	got := strings.Join(meta.SearchHints(), "\n")
	for _, want := range []string{"Igor Stravinsky", "Le Sacre du printemps", "Colin Davis", "阿姆斯丹堂管弦乐团"} {
		if !strings.Contains(got, want) {
			t.Fatalf("SearchHints() = %q, missing %q", got, want)
		}
	}
}

func TestDiscOutputPathUsesCDSubdirForMultidisc(t *testing.T) {
	got := discOutputPath("/out/Bob Dylan/Blonde on Blonde [1966]", 2, 2)
	want := filepath.Join("/out/Bob Dylan/Blonde on Blonde [1966]", "CD2")
	if got != want {
		t.Fatalf("discOutputPath() = %q, want %q", got, want)
	}
}

func TestWriteMissingTracksReport(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"01. A.flac", "03. C.flac"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	writeMissingTracksReport(dir, 4, map[int]string{2: "B", 4: "D"}, "test")

	body, err := os.ReadFile(filepath.Join(dir, "missing_tracks.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(body)
	for _, want := range []string{"Total tracks: 4", "Present tracks: 2", "Track title source: test", "02. B", "04. D"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing report does not contain %q:\n%s", want, got)
		}
	}
}

func TestWriteMissingTracksReportRemovesFileWhenComplete(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"01. A.flac", "02. B.flac"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	reportPath := filepath.Join(dir, "missing_tracks.txt")
	if err := os.WriteFile(reportPath, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}

	writeMissingTracksReport(dir, 2, map[int]string{1: "A", 2: "B"}, "test")

	if _, err := os.Stat(reportPath); !os.IsNotExist(err) {
		t.Fatalf("missing_tracks.txt should be removed, stat err=%v", err)
	}
}

func TestWriteMissingTracksReportRecognizesClassicalFilenameWithPrefix(t *testing.T) {
	dir := t.TempDir()
	name := "01. Part I- The Adoration Of The Earth- 1. Introduction.flac"
	if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	writeMissingTracksReport(dir, 1, map[int]string{1: "Part I: The Adoration Of The Earth: 1. Introduction"}, "test")

	if _, err := os.Stat(filepath.Join(dir, "missing_tracks.txt")); !os.IsNotExist(err) {
		t.Fatalf("missing_tracks.txt should be absent, stat err=%v", err)
	}
}
