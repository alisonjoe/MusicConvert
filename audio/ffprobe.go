package audio

import (
	"MusicConvert/util"
	"context"
	"encoding/json"
	"strings"
)

type AutoMetadata struct {
	Format struct {
		Filename       string            `json:"filename"`
		NbStreams      int               `json:"nb_streams"`
		NbPrograms     int               `json:"nb_programs"`
		FormatName     string            `json:"format_name"`
		FormatLongName string            `json:"format_long_name"`
		StartTime      string            `json:"start_time"`
		Duration       string            `json:"duration"`
		Size           string            `json:"size"`
		BitRate        string            `json:"bit_rate"`
		ProbeScore     int               `json:"probe_score"`
		Tags           map[string]string `json:"tags"`
	} `json:"format"`
}

func (m *AutoMetadata) GetTag(key string) string {
	if m.Format.Tags == nil {
		return ""
	}
	key = strings.ToLower(key)
	for k, v := range m.Format.Tags {
		if strings.ToLower(k) == key {
			return v
		}
	}
	return ""
}

func GetFormat(ctx context.Context, file string) (*AutoMetadata, error) {
	args := []string{"-v", "quiet", "-print_format", "json", "-show_format", file}
	resp, err := util.Run(ctx, "ffprobe", args)
	if err != nil {
		return nil, err
	}
	r := new(AutoMetadata)
	if err := json.Unmarshal([]byte(resp), &r); err != nil {
		return nil, err
	}

	return r, nil
}
