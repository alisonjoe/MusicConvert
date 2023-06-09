package util

import (
	"MusicConvert/model"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func visit(files *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Fatal(err)
		}
		// 目录
		if info.IsDir() {
			return nil
		}
		suffix := strings.ToUpper(filepath.Ext(path))
		// delete(model.NeedConvertSuffix, "."+config.GetDescType())
		// log.Printf("%v", model.NeedConvertSuffix)

		_, has := model.NeedConvertSuffix[suffix]
		if !has {
			// log.Printf("%v suffix[%v] is not match ", path, suffix)
			return nil
		}
		*files = append(*files, path)
		return nil
	}
}
func DelSrc(isDel model.SwitchState, ch chan string) {
	for {
		file := <-ch
		if isDel == model.OFF {
			log.Println("不删除源文件，如果需要删除请降 is_del_src 设置为 1")
			continue
		}
		if file != "" {
			log.Printf("wait del file:%v", file)
			// 使用 os.Remove() 删除文件
			err := os.Remove(file)
			if err != nil {
				log.Printf("%v 删除失败", file)
			} else {
				log.Printf("%v 删除成功", file)
			}
		}
	}
}

func GetAllMusic(path string, ch chan model.MusicFileInfo) {
	var files []string
	err := filepath.Walk(path, visit(&files))
	if err != nil {
		panic(err)
	}
	list := splitFiles(files)
	for _, v := range list {
		ch <- v
	}
}

func splitFiles(files []string) map[string]model.MusicFileInfo {
	mapFile := make(model.MapMusicFile)
	for _, v := range files {
		suffix := strings.ToUpper(filepath.Ext(v))
		fileprefix := strings.ToUpper(v[0 : len(v)-len(suffix)])
		f, has := mapFile[fileprefix]
		if suffix == ".CUE" {
			if has {
				f.CuePath = v
				mapFile[fileprefix] = f
			} else {
				temp := model.MusicFileInfo{CuePath: v}
				mapFile[fileprefix] = temp
			}
		} else {
			if has {
				f.FilePath = v
				f.FileSuffix = suffix
				mapFile[fileprefix] = f
			} else {
				temp := model.MusicFileInfo{FilePath: v, FileSuffix: suffix}
				mapFile[fileprefix] = temp
			}
		}
	}
	return mapFile
}

// 判断文件夹是否存在
func MkPathIfNotExists(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !os.IsNotExist(err) {
		return nil
	}
	err = os.MkdirAll(path, os.ModePerm)
	if err != nil {
		log.Fatalf("mkdir %v fail\n", path)
	}
	return err
}
