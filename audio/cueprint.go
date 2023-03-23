package audio

import (
	"MusicConvert/util"
	"context"
	"fmt"
	"log"
	"strconv"
)

func GetTotalNum(ctx context.Context, cue string) (int, error) {
	// cueprint -n 1 -t '%N' all.cue
	args := []string{"-n", "1", "-t", "%N", cue}
	str, err := util.Run(ctx, "cueprint", args)
	if err != nil {
		log.Fatalf("cmd2 run fail: %v\n", str)
		return 0, err
	}

	i, err := strconv.Atoi(str)
	if err != nil {
		log.Fatalf("cmd2 run fail: %v\n", str)
		return 0, err
	}
	return i, nil
}

func GetTrackTitle(ctx context.Context, num int, cue string) (string, error) {
	// cueprint -n 1 -t '%t' all.cue
	args := []string{"-n", fmt.Sprintf("%d", num), "-t", "%t", cue}
	str, err := util.Run(ctx, "cueprint", args)
	if err != nil {
		log.Fatalf("cmd2 run fail: %v\n", str)
		return "", err
	}

	return str, nil
}
