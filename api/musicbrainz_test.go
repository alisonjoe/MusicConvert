package api

import "testing"

func TestLikelyChineseAliasRejectsJapaneseKana(t *testing.T) {
	if isLikelyChineseText("アムステルダム・コンセルトヘボウ管弦楽団") {
		t.Fatal("Japanese alias with kanji should not be treated as Chinese")
	}
	if !isLikelyChineseText("阿姆斯特丹皇家音乐厅管弦乐团") {
		t.Fatal("Chinese alias should be accepted")
	}
}

func TestPreferredChineseAliasRequiresChineseLocale(t *testing.T) {
	if isPreferredChineseAlias("阿姆斯特丹皇家音乐厅管弦乐团", "ja") {
		t.Fatal("Chinese text with non-Chinese locale should not be preferred")
	}
	if !isPreferredChineseAlias("阿姆斯特丹皇家音乐厅管弦乐团", "zh-Hans") {
		t.Fatal("Chinese text with zh locale should be preferred")
	}
}
