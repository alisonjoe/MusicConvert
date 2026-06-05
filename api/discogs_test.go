package api

import "testing"

func TestDiscogsTrackPosition(t *testing.T) {
	tests := map[string]int{
		"7":     7,
		"CD1-7": 7,
		"1.10":  10,
		"A1":    1,
		"":      0,
	}
	for input, want := range tests {
		if got := discogsTrackPosition(input); got != want {
			t.Fatalf("discogsTrackPosition(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestFlattenDiscogsTracklistSkipsHeadingsAndUsesSubTracks(t *testing.T) {
	got := flattenDiscogsTracklist([]DiscogsTrack{
		{Type: "heading", Title: "Le Sacre du Printemps"},
		{Type: "track", Position: "1", Title: "Introduction"},
		{Type: "track", Position: "2", Title: "The Augurs Of Spring"},
		{Type: "track", Position: "3", Title: "Part II", SubTracks: []DiscogsTrack{
			{Type: "track", Position: "3", Title: "Mystic Circles"},
			{Type: "track", Position: "4", Title: "Glorification"},
		}},
	})
	wants := map[int]string{
		1: "Le Sacre du Printemps: Introduction",
		2: "Le Sacre du Printemps: The Augurs Of Spring",
		3: "Part II: Mystic Circles",
		4: "Part II: Glorification",
	}
	for pos, want := range wants {
		if got[pos] != want {
			t.Fatalf("flattenDiscogsTracklist()[%d] = %q, want %q; got=%#v", pos, got[pos], want, got)
		}
	}
}
