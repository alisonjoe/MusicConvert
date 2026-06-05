package util

import (
	"log"
	"strings"
	"sync"
	"unicode"

	"github.com/longbridgeapp/opencc"
)

var (
	t2sOnce sync.Once
	t2sConv *opencc.OpenCC
	t2sErr  error
)

func ToSimplifiedChinese(s string) string {
	if s == "" {
		return s
	}
	t2sOnce.Do(func() {
		t2sConv, t2sErr = opencc.New("t2s")
		if t2sErr != nil {
			log.Printf("[Chinese Convert] failed to initialize OpenCC t2s: %v", t2sErr)
		}
	})
	if t2sErr != nil || t2sConv == nil {
		return s
	}
	out, err := t2sConv.Convert(s)
	if err != nil {
		log.Printf("[Chinese Convert] failed to convert %q: %v", s, err)
		return s
	}
	return out
}

func NormalizeChineseForCompare(s string) string {
	if s == "" {
		return s
	}
	s = ToSimplifiedChinese(s)
	s = strings.TrimSpace(s)
	s = strings.TrimFunc(s, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r) || unicode.Is(unicode.Cf, r)
	})
	return s
}
