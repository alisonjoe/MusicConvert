package util

import (
	"MusicConvert/model"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func visit(files *[]string) filepath.WalkFunc {
	return func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Printf("visit %v error: %v", path, err)
			return nil
		}
		// 目录
		if info.IsDir() {
			return nil
		}
		suffix := strings.ToUpper(filepath.Ext(path))

		_, has := model.NeedConvertSuffix[suffix]
		if !has {
			return nil
		}
		*files = append(*files, path)
		return nil
	}
}

// GetMusicFileInfoForFile takes a single file path and returns MusicFileInfo.
// If the file is a music file, it attempts to find a corresponding .cue file.
// If the file is a .cue file, it attempts to find a corresponding music file.
func GetMusicFileInfoForFile(filePath string) (model.MusicFileInfo, error) {
	fileInfo := model.MusicFileInfo{}
	ext := filepath.Ext(filePath)
	suffix := strings.ToUpper(ext)
	fileprefix := strings.TrimSuffix(filePath, ext)

	if suffix == ".CUE" {
		fileInfo.CuePath = filePath
		// Try to find a matching music file
		for k := range model.NeedConvertSuffix {
			if k == ".CUE" {
				continue
			}
			musicFilePath := fileprefix + strings.ToLower(k) // Assuming lowercased suffix for search
			if _, err := os.Stat(musicFilePath); err == nil {
				fileInfo.FilePath = musicFilePath
				fileInfo.FileSuffix = strings.ToUpper(filepath.Ext(musicFilePath))
				break
			}
		}
	} else {
		_, has := model.NeedConvertSuffix[suffix]
		if has {
			fileInfo.FilePath = filePath
			fileInfo.FileSuffix = suffix
			// Try to find a matching .cue file
			cuePath := fileprefix + ".cue"
			if _, err := os.Stat(cuePath); err == nil {
				fileInfo.CuePath = cuePath
			}
		} else {
			return model.MusicFileInfo{}, fmt.Errorf("unsupported file type: %s", filePath)
		}
	}
	return fileInfo, nil
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
				log.Printf("%v 删除失败: %v", file, err)
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
		log.Printf("GetAllMusic walk error: %v", err)
	}
	list := splitFiles(files)
	for _, v := range list {
		ch <- v
	}
}

func splitFiles(files []string) map[string]model.MusicFileInfo {
	mapFile := make(model.MapMusicFile)
	for _, v := range files {
		ext := filepath.Ext(v)
		suffix := strings.ToUpper(ext)
		fileprefix := strings.ToUpper(strings.TrimSuffix(v, ext))
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

// SanitizeFilename removes invalid characters from a string to make it safe for use as a filename.
func SanitizeFilename(filename string) string {
	// Replace common invalid characters with a hyphen
	filename = strings.ReplaceAll(filename, "/", "-")
	filename = strings.ReplaceAll(filename, "\\", "-")
	filename = strings.ReplaceAll(filename, ":", "-")
	filename = strings.ReplaceAll(filename, "*", "-")
	filename = strings.ReplaceAll(filename, "?", "-")
	filename = strings.ReplaceAll(filename, "\"", "-")
	filename = strings.ReplaceAll(filename, "<", "-")
	filename = strings.ReplaceAll(filename, ">", "-")
	filename = strings.ReplaceAll(filename, "|", "-")

	// Replace multiple spaces with a single space, then trim leading/trailing spaces
	filename = strings.Join(strings.Fields(filename), " ")
	filename = strings.TrimSpace(filename)

	return filename
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
		log.Printf("mkdir %v fail: %v\n", path, err)
	}
	return err
}

// GetUniquePath 检查路径是否冲突，如果冲突则生成一个新的唯一路径 (e.g. file (1).flac)
func GetUniquePath(path string) string {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return path
	}
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	for i := 1; ; i++ {
		newPath := fmt.Sprintf("%s (%d)%s", base, i, ext)
		if _, err := os.Stat(newPath); os.IsNotExist(err) {
			return newPath
		}
	}
}
