package api

import "testing"

func TestItunesCandidateScoreRejectsContainingAlbumMismatch(t *testing.T) {
	if got := itunesCandidateScore("Hurts", "Desire", "Kelly Howell", "Fulfill Your Heart's Desire"); got != 0 {
		t.Fatalf("itunesCandidateScore() = %d, want 0", got)
	}
}

func TestItunesCandidateScoreAcceptsExactAndDeluxe(t *testing.T) {
	if got := itunesCandidateScore("Hurts", "Desire", "Hurts", "Desire"); got <= 0 {
		t.Fatalf("exact score = %d, want > 0", got)
	}
	if got := itunesCandidateScore("Hurts", "Surrender", "Hurts", "Surrender (Deluxe)"); got <= 0 {
		t.Fatalf("deluxe score = %d, want > 0", got)
	}
}

func TestItunesCandidateScoreAcceptsChineseAlbumWithSharedEnglishToken(t *testing.T) {
	got := itunesCandidateScore("张国荣", "Back To Black Series - Salute", "Leslie Cheung", "復黑王: Salute")
	if got <= 0 {
		t.Fatalf("score = %d, want > 0", got)
	}
}
