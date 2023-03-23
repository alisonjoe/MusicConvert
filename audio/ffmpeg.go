package audio

import (
	"MusicConvert/util"
	"context"
	"fmt"
	"log"
)

func GetMetadata(ctx context.Context, in string, out string) error {
	if in == "" || out == "" {
		return fmt.Errorf("in[%v] or out[%v] is nil", in, out)
	}
	args := []string{"-i", in, "-f", "ffmetadata", out}
	if _, err := util.Run(ctx, "ffmpeg", args); err != nil {
		log.Fatalf("%#v", err)
		return err
	}

	return nil
}

func CopyMeta(ctx context.Context, in string, meta string, out string) error {
	if in == "" || out == "" || meta == "" {
		return fmt.Errorf("in[%v] or out[%v] is nil", in, out)
	}
	args := []string{"-i", in, "-i", meta, "-map_metadata", "1", "-codec", "copy", out}
	if str, err := util.Run(ctx, "ffmpeg", args); err != nil {
		log.Fatalf("copy meta fail %v err: %#v", str, err)
		return err
	}

	return nil
}
