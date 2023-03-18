package util

import (
	"MusicConvert/model"
	"log"
	"os/exec"
	"path/filepath"
)

func ToFLAC(ch chan model.MusicFileInfo, delCh chan string) {
	for {
		info := <-ch
		if info.FilePath == "" {
			continue
		}
		suffix := filepath.Ext(info.FilePath)
		filePrefix := info.FilePath[0 : len(info.FilePath)-len(suffix)]
		flac := filePrefix + ".flac"

		args := []string{info.FilePath, "-f", "-o", flac}
		cmd := exec.Command("flac", args...)
		if err := cmd.Run(); err != nil {
			panic(err)
		}
		log.Printf("转码成功")
		delCh <- info.FilePath
	}
}
