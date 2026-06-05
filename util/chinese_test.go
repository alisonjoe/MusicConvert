package util

import "testing"

func TestToSimplifiedChinese(t *testing.T) {
	got := ToSimplifiedChinese("張國榮")
	want := "张国荣"
	if got != want {
		t.Fatalf("ToSimplifiedChinese() = %q, want %q", got, want)
	}
}

func TestNormalizeChineseForCompareKeepsModifiers(t *testing.T) {
	got := NormalizeChineseForCompare("\u200b  戴維斯指揮阿姆斯丹大會堂管弦樂團  \ufeff")
	want := "戴维斯指挥阿姆斯丹大会堂管弦乐团"
	if got != want {
		t.Fatalf("NormalizeChineseForCompare() = %q, want %q", got, want)
	}
}
