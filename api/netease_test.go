package api

import "testing"

func TestCleanHintKeepsFullEnglishSubtitle(t *testing.T) {
	got := CleanHint("2017-渡 The Crossing")
	want := "渡(The Crossing)"
	if got != want {
		t.Fatalf("CleanHint() = %q, want %q", got, want)
	}
}

func TestPreferDetailedAlbumNameKeepsLocalCasing(t *testing.T) {
	got := PreferDetailedAlbumName("Blonde On Blonde", "Blonde on Blonde")
	want := "Blonde on Blonde"
	if got != want {
		t.Fatalf("PreferDetailedAlbumName() = %q, want %q", got, want)
	}
}

func TestSplitArtistsSplitsCompactHyphenDuet(t *testing.T) {
	got := SplitArtists("薛之谦-张靓颖")
	if len(got) < 2 || got[0] != "薛之谦" || got[1] != "张靓颖" {
		t.Fatalf("SplitArtists() = %#v", got)
	}
}

func TestAlbumTrackTitleCacheMatchesDetailedAlbumName(t *testing.T) {
	cacheAlbumTrackTitles("薛之谦", "渡", []NeteaseAlbumSong{{Name: "动物世界"}, {Name: "暧昧"}})
	got := GetAlbumTrackTitles("薛之谦", "渡(The Crossing)")
	if got[1] != "动物世界" || got[2] != "暧昧" {
		t.Fatalf("GetAlbumTrackTitles() = %#v", got)
	}
}

func TestLooksLikeRomanizedChineseQuery(t *testing.T) {
	if !looksLikeRomanizedChineseQuery("Wu Xin Shui Mian") {
		t.Fatalf("expected romanized Chinese query")
	}
	if !looksLikeRomanizedChineseQuery("Shi Hao Feng Qiu") {
		t.Fatalf("expected romanized Chinese query")
	}
	if looksLikeRomanizedChineseQuery("Back To Black Series") {
		t.Fatalf("English album title should not be treated as romanized Chinese")
	}
}

func TestASCIIAlbumNamesClose(t *testing.T) {
	if !asciiAlbumNamesClose("Cross Over", "Cross Cover") {
		t.Fatalf("expected typo-tolerant album match")
	}
	if asciiAlbumNamesClose("Cross Over", "Hot Summer") {
		t.Fatalf("unrelated album names should not match")
	}
}
