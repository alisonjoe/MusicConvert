package main

import (
	"MusicConvert/audio"
	"MusicConvert/config"
	"MusicConvert/db"
	"MusicConvert/model"
	"MusicConvert/util"
	"context"
	"log"
	"time"
)

var convertChan = make(chan model.MusicFileInfo)
var delChan = make(chan string)

func main() {
	conf := config.Init("config.yaml")
	log.Printf("config is %#v", conf)

	ctx := context.Background()
	// 初始化数据库
	db.Init()
	// 监控是否有需要删除文件
	go util.DelSrc(config.GetIsDelSrc(), delChan)
	// 处理转码
	go audio.ToFLAC(ctx, config.GetOutputRoot(), convertChan, delChan)
	// 监控是否有需要转码文件
	for {
		util.GetAllMusic(conf.SrcPath, convertChan)
		time.Sleep(1000 * time.Second)
		log.Printf("没有新文件需要转码")
	}

}
