package audio

import (
	"testing"
)

func TestDiscNumberFromName(t *testing.T) {
	if n := discNumberFromName("CD1"); n != 1 {
		t.Errorf("Expected CD1 to return 1, got %d", n)
	}
	if n := discNumberFromName("Disc 2"); n != 2 {
		t.Errorf("Expected Disc 2 to return 2, got %d", n)
	}
}
