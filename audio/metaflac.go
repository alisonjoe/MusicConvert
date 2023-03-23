package audio

import (
	"MusicConvert/util"
	"context"
	"fmt"
	"io"
	"log"
	"os"
)

func ReWriterMeta(ctx context.Context, in string, title string,
	num int, total int, out string) error {
	if in == "" || out == "" {
		return fmt.Errorf("in[%v] or out[%v] is nil", in, out)
	}
	// 提取flac meta
	tags := fmt.Sprintf("%s.tags", in)
	tmp := fmt.Sprintf("--export-tags-to=%s", tags)
	args := []string{tmp, in}
	if _, err := util.Run(ctx, "metaflac", args); err != nil {
		log.Fatalf("%#v", err)
		return err
	}
	// 追加 title 等到 tags文件
	f, err := os.OpenFile(tags, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModeAppend)
	if err != nil {
		return err
	}
	wireteString := fmt.Sprintf("TITLE=%s\nTRACKNUMBER=%d\nTRACKTOTAL=%d\n", title, num, total)
	_, err = io.WriteString(f, wireteString) //写入文件(字符串)
	if err != nil {
		return err
	}
	f.Close()
	//
	inTags := fmt.Sprintf("%s=%s", "--import-tags-from", tags)
	args2 := []string{inTags, out}
	if str, err := util.Run(ctx, "metaflac", args2); err != nil {
		log.Fatalf("cmd2 run fail: %v\n", str)
		return err
	}
	err = os.Remove(tags)
	if err != nil {
		log.Printf("%v 删除失败", tags)
	} else {
		log.Printf("%v 删除成功", tags)
	}
	return nil
}
