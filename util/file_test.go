package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveExistingPathFoldsCombiningAccent(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "Celine Dion")
	if err := os.Mkdir(existing, 0755); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "Ce\u0301line Dion")
	got := ResolveExistingPath(input)
	if got != existing {
		t.Fatalf("ResolveExistingPath() = %q, want %q", got, existing)
	}
}
