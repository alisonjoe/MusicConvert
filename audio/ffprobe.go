package audio

import (
	"MusicConvert/util"
	"context"
	"encoding/json"
	"strings"
)

type AutoMetadata struct {
	Format struct {
		Filename       string `json:"filename"`
		NbStreams      int    `json:"nb_streams"`
		NbPrograms     int    `json:"nb_programs"`
		FormatName     string `json:"format_name"`
		FormatLongName string `json:"format_long_name"`
		StartTime      string `json:"start_time"`
		Duration       string `json:"duration"`
		Size           string `json:"size"`
		BitRate        string `json:"bit_rate"`
		ProbeScore     int    `json:"probe_score"`
		Tags           struct {
			ALBUM   string `json:"ALBUM"`
			ARTIST  string `json:"ARTIST"`
			Comment string `json:"comment"`
			ENCODER string `json:"ENCODER"`
			TITLE   string `json:"TITLE"`
			Track   string `json:"track"`
			DATE    string `json:"DATE"`
		} `json:"tags"`
	} `json:"format"`
}

func GetFormat(ctx context.Context, file string) (*AutoMetadata, error) {
	args := []string{"-v", "quiet", "-print_format", "json", "-show_format", file}
	resp, err := util.Run(context.Background(), "ffprobe", args)
	if err != nil {
		return nil, err
	}
	r := new(AutoMetadata)
	if err := json.Unmarshal([]byte(resp), &r); err != nil {
		return nil, err
	}
	r.Format.Tags.ARTIST = strings.Replace(r.Format.Tags.ARTIST, "/", "-", -1)
	r.Format.Tags.ALBUM = strings.Replace(r.Format.Tags.ALBUM, "/", "-", -1)
	r.Format.Tags.TITLE = strings.Replace(r.Format.Tags.TITLE, "/", "-", -1)
	r.Format.Tags.ARTIST = strings.Replace(r.Format.Tags.ARTIST, " ", ".", -1)

	return r, nil
}
