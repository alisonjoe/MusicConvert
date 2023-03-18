package main

import (
	"MusicConvert/config"
	"MusicConvert/model"
	"MusicConvert/util"
	"log"
	"time"
)

var convertChan = make(chan model.MusicFileInfo)
var delChan = make(chan string)

func main() {
	conf := config.Init("config.yaml")
	log.Printf("config is %#v", conf)
	// 处理转码
	go util.DelSrc(delChan)
	go util.ToFLAC(convertChan, delChan)
	// 监控是否有需要删除文件
	// 监控是否有需要转码文件
	for {
		util.GetAllMusic(conf.SrcPath, convertChan)
		time.Sleep(10 * time.Second)
		log.Printf("没有新文件需要转码")
	}

}
