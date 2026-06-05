package api

import "testing"

func TestParseWikipediaClassicalStructureSplitsFirebirdSceneTitles(t *testing.T) {
	extract := `== 分析 ==
=== 架構 ===
1.序曲
第1景
2.魔王卡什伊的魔法庭園
3.火鳥登場,伊凡·沙列維奇追逐而至
4.火鳥之舞
5.伊凡·沙列維奇捕獲火鳥
6.火鳥的哀求
7.十三位迷人公主的登場
8.公主們的金蘋果遊戲
9.伊凡·沙列維奇突然出現
10.公主們的霍洛沃德舞(輪旋舞)
11.拂曉
12.魔鐘響起,卡什伊的妖怪侍衛登場,捕獲伊凡王子
13.不死卡什伊的登場-與伊凡·沙列維奇的對話-公主們的求情
14.火鳥的出現
15.火鳥魔咒下卡什伊的侍衛之舞
16.卡什伊所有臣屬的地獄之舞
17.火鳥的催眠曲
18.卡什伊的覺醒-卡什伊的死亡-深沉的黑暗
第2景
19.卡什伊咒語被破,宮殿消失,石化騎士復活,全體感恩
== 參考資料 ==`
	got := parseWikipediaClassicalStructure(extract)
	if len(got) != 22 {
		t.Fatalf("len = %d, want 22; tracks=%#v", len(got), got)
	}
	wants := map[int]string{
		1:  "引子",
		4:  "第1场: 火鸟之舞",
		13: "第1场: 不死卡斯切伊的登场",
		14: "第1场: 與伊凡王子的對話",
		20: "第1场: 卡斯切伊的觉醒",
		22: "第2场: 卡斯切伊咒語被破,宮殿消失,石化騎士復活,全體感恩",
	}
	for track, want := range wants {
		if got[track] != want {
			t.Fatalf("track %d = %q, want %q; tracks=%#v", track, got[track], want, got)
		}
	}
}
