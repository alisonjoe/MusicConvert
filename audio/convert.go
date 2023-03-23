package audio

import (
	"MusicConvert/db"
	"MusicConvert/model"
	"MusicConvert/util"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

func ToFLAC(ctx context.Context, outRoot string,
	ch chan model.MusicFileInfo, delCh chan string) {
	for {
		info := <-ch
		if info.FilePath == "" {
			continue
		}
		l, err := db.Query(info.FilePath)
		if err != nil {
			log.Printf("query db fail %v", err)
			continue
		}
		log.Printf("%v query db resp %#v", info.FilePath, l)
		if len(l) > 0 {
			log.Printf("%v 文件已经转码,不需要重复处理", info.FilePath)
			continue
		}
		if info.CuePath == "" {
			if err := singleConvert(info, outRoot); err != nil {
				log.Printf("info[%#v] singleConvert fail err:%v", info, err)
				continue
			}
		} else {
			if err := cueConvert(ctx, info, outRoot); err != nil {
				log.Printf("info[%#v] singleConvert split fail:%v", info, err)
				continue
			}
		}
		m := &db.MusicInfo{
			Music: info.FilePath,
			State: 1,
		}
		err = db.Insert(m)
		if err != nil {
			log.Printf("%v insert db fail %v", info.FilePath, err)
			continue
		} else {
			log.Printf("%v insert db succ", info.FilePath)
		}
		delCh <- info.FilePath
	}
}

func singleConvert(st model.MusicFileInfo, outRoot string) error {
	ctx := context.Background()
	meta, err := GetFormat(ctx, st.FilePath)
	if err != nil {
		log.Fatal(err)
		return err
	}
	basePath := fmt.Sprintf("%s/%s/%s", outRoot, meta.Format.Tags.ARTIST,
		meta.Format.Tags.ALBUM)
	err = util.MkPathIfNotExists(basePath)
	if err != nil {
		return err
	}
	var flac string
	if len(strings.TrimSpace(meta.Format.Tags.Track)) > 0 {
		flac = fmt.Sprintf("%s/%s. %s.flac", basePath, meta.Format.Tags.Track, meta.Format.Tags.TITLE)
	} else {
		flac = fmt.Sprintf("%s/%s.flac", basePath, meta.Format.Tags.TITLE)
	}

	args := []string{"-i", st.FilePath, "-map_metadata", "0", "-y", flac}
	if _, err := util.Run(ctx, "ffmpeg", args); err != nil {
		log.Fatalf("%#v", err)
		return err
	}
	log.Printf("转码成功")
	return nil
}

func cueConvertOld(ctx context.Context, st model.MusicFileInfo, outRoot string) error {
	meta, err := GetFormat(ctx, st.FilePath)
	if err != nil {
		log.Fatal(err)
		return err
	}
	basePath := fmt.Sprintf("%s/%s/%s", outRoot,
		meta.Format.Tags.ARTIST, meta.Format.Tags.ALBUM)
	err = util.MkPathIfNotExists(basePath)
	if err != nil {
		return err
	}
	args := []string{"-f", st.CuePath, "-t", "%n.%t", "-o", "flac", "-d", basePath, st.FilePath}
	if _, err := util.Run(ctx, "shnsplit", args); err != nil {
		log.Fatalf("%#v", err)
		return err
	}
	log.Printf("转码成功")
	// 复写 metadata
	n, err := GetTotalNum(ctx, st.CuePath)
	if err != nil {
		log.Fatalf("GetTotalNum fail err:%v", err)
		return err
	}
	log.Printf("GetTotalNum succ, %v", n)
	for i := 1; i <= n; i++ {
		title, err := GetTrackTitle(ctx, i, st.CuePath)
		if err != nil {
			log.Fatalf("GetTrackTitle fail err:%v", err)
			return err
		}
		fmt.Println(title)
		outFile := fmt.Sprintf("%s/%02d.%s.flac", basePath, i, title)
		if err := ReWriterMeta(ctx, st.FilePath, title, i, n, outFile); err != nil {
			log.Printf("rewriter fail[%v]\n", err)
		}
	}

	return nil
}

func cueConvert(ctx context.Context, st model.MusicFileInfo, outRoot string) error {
	meta, err := GetFormat(ctx, st.FilePath)
	if err != nil {
		log.Fatal(err)
		return err
	}
	basePath := fmt.Sprintf("%s/%s/%s", outRoot,
		meta.Format.Tags.ARTIST, meta.Format.Tags.ALBUM)
	err = util.MkPathIfNotExists(basePath)
	if err != nil {
		return err
	}
	args := []string{"-f", st.CuePath, "-t", "%t", "-o", "flac", "-d", basePath, st.FilePath}
	if _, err := util.Run(ctx, "shnsplit", args); err != nil {
		log.Fatalf("%#v", err)
		return err
	}
	log.Printf("转码成功")
	// 复写 metadata
	n, err := GetTotalNum(ctx, st.CuePath)
	if err != nil {
		log.Fatalf("GetTotalNum fail err:%v", err)
		return err
	}
	log.Printf("GetTotalNum succ, %v", n)
	for i := 1; i <= n; i++ {
		title, err := GetTrackTitle(ctx, i, st.CuePath)
		if err != nil {
			log.Fatalf("GetTrackTitle fail err:%v", err)
			return err
		}
		fmt.Println(title)
		inFile := fmt.Sprintf("%s/%s.flac", basePath, title)
		meta := inFile + ".txt"
		outFile := fmt.Sprintf("%s/%02d.%s.flac", basePath, i, strings.Replace(title, " ", "-", -1))
		// 提取整轨的 metadata
		if err := GetMetadata(ctx, st.FilePath, meta); err != nil {
			return err
		}

		// 追加 title 等到 metadata 文件
		f, err := os.OpenFile(meta, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			return err
		}
		writeString := fmt.Sprintf("TITLE=%s\nTRACKNUMBER=%d\nTRACKTOTAL=%d\n", title, i, n)
		_, err = io.WriteString(f, writeString) //写入文件(字符串)
		if err != nil {
			return err
		}
		f.Close()

		if err := CopyMeta(ctx, inFile, meta, outFile); err != nil {
			log.Printf("rewriter fail[%v]\n", err)
		}
		err = os.Remove(meta)
		if err != nil {
			log.Printf("%v 删除失败", meta)
		} else {
			log.Printf("%v 删除成功", meta)
		}
		err = os.Remove(inFile)
		if err != nil {
			log.Printf("%v 删除失败", inFile)
		} else {
			log.Printf("%v 删除成功", inFile)
		}
	}

	return nil
}
