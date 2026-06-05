package config

import (
	"MusicConvert/model"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"gopkg.in/yaml.v2"
)

var cfg = &Config{}

type Config struct {
	// IsSplit 整轨是否切割, cue 文件配合
	IsSplit model.SwitchState `yaml:"is_split"`
	// IsDelSrc 转换格式成功后, 是否删掉源文件
	IsDelSrc model.SwitchState `yaml:"is_del_src"`
	SrcPath  string            `yaml:"src_path"`
	// DescType 目标编码
	DescType string `yaml:"desc_type"`
	// DescPath 目标文件存储位置, 默认和源文件一致
	DescPath string `yaml:"desc_path"`
	// OutputRoot 输出目录
	OutputRoot string `yaml:"output_root"`
	// Spotify 凭证
	SpotifyID      string `yaml:"spotify_client_id"`
	SpotifySecret  string `yaml:"spotify_client_secret"`
	SpotifyEnabled bool   `yaml:"spotify_enabled"`
	SpotifyPremium bool   `yaml:"spotify_premium"` // Deprecated: use spotify_enabled.
	DiscogsToken   string `yaml:"discogs_token"`
	NeteaseCookie  string `yaml:"netease_cookie"`
	// Netease OpenAPI 凭证。AuthMode=openapi 时优先使用，失败后回退旧接口/Cookie。
	NeteaseAuthMode     string `yaml:"netease_auth_mode"`
	NeteaseAppID        string `yaml:"netease_app_id"`
	NeteaseAppSecret    string `yaml:"netease_app_secret"`
	NeteasePrivateKey   string `yaml:"netease_private_key"`
	NeteaseAccessToken  string `yaml:"netease_access_token"`
	NeteaseRefreshToken string `yaml:"netease_refresh_token"`
	NeteaseDevice       string `yaml:"netease_device"`
	// TrackOverrides 手工曲序兜底，key 支持 "歌手|专辑" 或 "专辑"
	TrackOverrides map[string][]string `yaml:"track_overrides"`
}

func GetSpotifyID() string {
	return cfg.SpotifyID
}

func GetSpotifySecret() string {
	return cfg.SpotifySecret
}

func IsSpotifyEnabled() bool {
	return cfg.SpotifyEnabled || cfg.SpotifyPremium
}

func GetDiscogsToken() string {
	return strings.TrimSpace(cfg.DiscogsToken)
}

func GetNeteaseCookie() string {
	return cfg.NeteaseCookie
}

func GetNeteaseAuthMode() string {
	return strings.ToLower(strings.TrimSpace(cfg.NeteaseAuthMode))
}

func GetNeteaseAppID() string {
	return strings.TrimSpace(cfg.NeteaseAppID)
}

func GetNeteaseAppSecret() string {
	return strings.TrimSpace(cfg.NeteaseAppSecret)
}

func GetNeteasePrivateKey() string {
	return strings.TrimSpace(cfg.NeteasePrivateKey)
}

func GetNeteaseAccessToken() string {
	return strings.TrimSpace(cfg.NeteaseAccessToken)
}

func GetNeteaseRefreshToken() string {
	return strings.TrimSpace(cfg.NeteaseRefreshToken)
}

func GetNeteaseDevice() string {
	device := strings.TrimSpace(cfg.NeteaseDevice)
	if device != "" {
		return device
	}
	return `{"deviceType":"andrwear","os":"otos","appVer":"0.1","channel":"hm","model":"kys","deviceId":"musicconvert001","brand":"hm","osVer":"8.1.0"}`
}

func Summary() string {
	return fmt.Sprintf("src_path=%q output_root=%q desc_type=%q spotify_enabled=%t discogs_configured=%t netease_mode=%q netease_app=%t netease_cookie=%t track_overrides=%d",
		cfg.SrcPath,
		cfg.OutputRoot,
		cfg.DescType,
		IsSpotifyEnabled() && cfg.SpotifyID != "" && cfg.SpotifySecret != "",
		GetDiscogsToken() != "",
		GetNeteaseAuthMode(),
		cfg.NeteaseAppID != "" && cfg.NeteasePrivateKey != "",
		cfg.NeteaseCookie != "",
		len(cfg.TrackOverrides),
	)
}

func HasNeteaseOpenAPIConfig() bool {
	return cfg.NeteaseAppID != "" && cfg.NeteasePrivateKey != ""
}

func normalizeTrackOverrideKey(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	remove := " \t\r\n　-_.·・,，.。:：;；!！?？'\"“”‘’()（）[]【】{}《》<>〈〉"
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(remove, r) {
			return -1
		}
		return r
	}, s)
}

func LookupTrackOverride(artist, album, title string) (int, int, bool) {
	if cfg.TrackOverrides == nil {
		return 0, 0, false
	}
	keys := []string{
		artist + "|" + album,
		album,
	}
	target := normalizeTrackOverrideKey(title)
	for _, key := range keys {
		tracks, ok := cfg.TrackOverrides[key]
		if !ok {
			continue
		}
		for i, track := range tracks {
			normalizedTrack := normalizeTrackOverrideKey(track)
			if normalizedTrack == target || strings.Contains(normalizedTrack, target) || strings.Contains(target, normalizedTrack) {
				return i + 1, len(tracks), true
			}
		}
	}
	return 0, 0, false
}

func LookupTrackOverrideList(artist, album string) ([]string, bool) {
	if cfg.TrackOverrides == nil {
		return nil, false
	}
	keys := []string{
		artist + "|" + album,
		album,
	}
	for _, key := range keys {
		if tracks, ok := cfg.TrackOverrides[key]; ok && len(tracks) > 0 {
			return tracks, true
		}
	}
	return nil, false
}

func TrackOverrideSummary() string {
	if len(cfg.TrackOverrides) == 0 {
		return "0"
	}
	return strconv.Itoa(len(cfg.TrackOverrides))
}

func GetDefaultConfig() *Config {
	return &Config{
		IsSplit:  model.OFF,
		IsDelSrc: model.OFF,
		SrcPath:  "./",
		DescType: "FLAC",
		DescPath: "",
	}
}

func Init(configPath string) (*Config, error) {

	if configPath == "" {
		return GetDefaultConfig(), nil
	}
	//读取yaml文件到缓存中
	s, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	//yaml文件内容影射到结构体中
	err = yaml.Unmarshal(s, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config file %s: %w", configPath, err)
	}
	return cfg, nil
}

func GetDescType() string {
	return cfg.DescType
}

func GetOutputRoot() string {
	return cfg.OutputRoot
}
func GetIsDelSrc() model.SwitchState {
	return cfg.IsDelSrc
}
