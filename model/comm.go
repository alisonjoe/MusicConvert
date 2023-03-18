package model

// MusicType 音乐编码格式
type MusicType int

const (
	WAV  MusicType = 0
	FLAC MusicType = 1
	APE  MusicType = 2
	DFF  MusicType = 3
	DSF  MusicType = 4
	ALAC MusicType = 5
	WV   MusicType = 6 // WavPack
	WMA  MusicType = 7 // WMA 9.0+ 开始为无损
)

var NeedConvertSuffix = map[string]struct{}{
	".FLAC": {},
	".APE":  {},
	".WAV":  {},
	".CUE":  {},
}

type SwitchState int

const (
	OFF SwitchState = 0 // 开关关闭
	ON  SwitchState = 1 // 开关开启
)

type MusicFileInfo struct {
	CuePath    string
	FilePath   string
	FileSuffix string
}

type MapMusicFile map[string]MusicFileInfo
