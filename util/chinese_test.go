package util

import "testing"

func TestToSimplifiedChinese(t *testing.T) {
	got := ToSimplifiedChinese("張國榮")
	want := "张国荣"
	if got != want {
		t.Fatalf("ToSimplifiedChinese() = %q, want %q", got, want)
	}
}
