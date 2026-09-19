package audio

import (
	"testing"
)

func TestParseClassicalAlbumMetadata(t *testing.T) {
	album := "Beethoven: Symphony No. 9"
	meta := parseClassicalAlbumMetadata(album)
	if !meta.IsClassical() {
		t.Errorf("Expected %s to be recognized as classical", album)
	}
}
