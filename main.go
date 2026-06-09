package main

import (
	"MusicConvert/api"
	"MusicConvert/audio"
	"MusicConvert/config"
	"MusicConvert/db"
	"MusicConvert/model"
	"MusicConvert/util"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"github.com/fsnotify/fsnotify"
)

var convertChan = make(chan model.MusicFileInfo, 1000)
var delChan = make(chan string, 1000)

const buildVersion = "metadata-fix-v4-smb-safe-paths"

const (
	logMaxBytes = 100 * 1024 * 1024
	logMaxFiles = 5
)

type rotatingLogWriter struct {
	mu       sync.Mutex
	path     string
	maxBytes int64
	maxFiles int
	file     *os.File
	size     int64
}

func newRotatingLogWriter(path string, maxBytes int64, maxFiles int) (*rotatingLogWriter, error) {
	w := &rotatingLogWriter{path: path, maxBytes: maxBytes, maxFiles: maxFiles}
	if err := w.open(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *rotatingLogWriter) open() error {
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return err
	}
	w.file = file
	w.size = info.Size()
	return nil
}

func (w *rotatingLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	if w.maxBytes > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingLogWriter) rotate() error {
	if w.file != nil {
		w.file.Close()
		w.file = nil
	}
	if w.maxFiles > 0 {
		os.Remove(fmt.Sprintf("%s.%d", w.path, w.maxFiles))
		for i := w.maxFiles - 1; i >= 1; i-- {
			oldPath := fmt.Sprintf("%s.%d", w.path, i)
			newPath := fmt.Sprintf("%s.%d", w.path, i+1)
			if _, err := os.Stat(oldPath); err == nil {
				os.Rename(oldPath, newPath)
			}
		}
		if _, err := os.Stat(w.path); err == nil {
			os.Rename(w.path, fmt.Sprintf("%s.1", w.path))
		}
	}
	return w.open()
}

func (w *rotatingLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

func initLogFile() io.Closer {
	logDir := "logs"
	if err := os.MkdirAll(logDir, 0755); err != nil {
		log.Printf("Warning: Failed to create log directory %s: %v", logDir, err)
		return nil
	}
	logPath := "logs/MusicConvert.log"
	writer, err := newRotatingLogWriter(logPath, logMaxBytes, logMaxFiles)
	if err != nil {
		log.Printf("Warning: Failed to open log file %s: %v", logPath, err)
		return nil
	}
	log.SetOutput(io.MultiWriter(os.Stdout, writer))
	log.Printf("Log file: %s (rotate at %d MB, keep %d files)", logPath, logMaxBytes/1024/1024, logMaxFiles)
	return writer
}

func CheckEnvironment() {
	tools := []string{"ffmpeg", "ffprobe", "metaflac"}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			log.Printf("Warning: Required tool '%s' not found in PATH. Some features will fail.", tool)
		} else {
			log.Printf("Tool check: '%s' found.", tool)
		}
	}
}

func main() {
	rescan := flag.Bool("rescan", false, "delete conversion records under configured src_path before processing")
	rescanAll := flag.Bool("rescan-all", false, "delete all conversion records before processing")
	testSpotify := flag.Bool("test-spotify", false, "test Spotify API credentials and search, then exit")
	flag.Parse()

	logFile := initLogFile()
	if logFile != nil {
		defer logFile.Close()
	}

	log.Printf("MusicConvert build: %s", buildVersion)
	CheckEnvironment()
	conf, err := config.Init("config.yaml")
	if err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}
	conf.SrcPath = util.ResolveExistingPath(conf.SrcPath)
	log.Printf("config: %s", config.Summary())
	log.Printf("Netease auth mode: %s (openapi app configured: %t, cookie configured: %t)",
		config.GetNeteaseAuthMode(), config.HasNeteaseOpenAPIConfig(), config.GetNeteaseCookie() != "")
	if *testSpotify {
		if err := api.TestSpotifySearch(); err != nil {
			log.Fatalf("Spotify test failed: %v", err)
		}
		return
	}

	ctx := context.Background()
	// 初始化数据库
	db.Init()
	if *rescanAll {
		rows, err := db.DeleteAll()
		if err != nil {
			log.Fatalf("Failed to clear conversion records: %v", err)
		}
		log.Printf("Rescan all enabled: cleared %d conversion records", rows)
	} else if *rescan {
		rows, err := db.DeleteByMusicPrefix(conf.SrcPath)
		if err != nil {
			log.Fatalf("Failed to clear conversion records under %s: %v", conf.SrcPath, err)
		}
		log.Printf("Rescan enabled: cleared %d conversion records under %s", rows, conf.SrcPath)
	}
	// 监控是否有需要删除文件
	go util.DelSrc(conf.IsDelSrc, delChan)
	// 处理转码
	go audio.ToFLAC(ctx, conf.OutputRoot, convertChan, delChan)

	// --- New fsnotify logic ---
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("Failed to create watcher: %v", err)
	}
	defer watcher.Close()

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Only care about new files or renamed files (which can be new files)
				// or existing files being modified (e.g., download complete)
				if event.Op&fsnotify.Create == fsnotify.Create ||
					event.Op&fsnotify.Write == fsnotify.Write ||
					event.Op&fsnotify.Rename == fsnotify.Rename {
					// Check if it's a file and not a directory
					fileInfo, err := os.Stat(event.Name)
					if err != nil {
						log.Printf("Error stating file %s: %v", event.Name, err)
						continue
					}
					if !fileInfo.IsDir() {
						// 简单的防抖：大文件在拷贝过程中会连续触发 Write，
						// 这里可以稍微判断一下或者完全交由队列处理，转码失败会自动重试。
						// 为了避免拷贝到一半就被处理，简单粗暴休眠一下并检查大小是否还变化
						log.Printf("Detected new or modified file: %s", event.Name)
						musicFileInfo, err := util.GetMusicFileInfoForFile(event.Name)
						if err != nil {
							// log.Printf("Failed to get music file info for %s: %v", event.Name, err)
							continue
						}
						// Only send to channel if it's a valid music or cue file and has a FilePath
						if musicFileInfo.FilePath != "" {
							convertChan <- musicFileInfo
						}
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Printf("Watcher error: %v", err)
			}
		}
	}()

	err = watcher.Add(conf.SrcPath)
	if err != nil {
		log.Printf("Warning: Failed to add watch path %s: %v (fsnotify limits might be reached). Will only process existing files.", conf.SrcPath, err)
	} else {
		log.Printf("Watching directory: %s", conf.SrcPath)
	}

	// Process existing files once at startup
	log.Printf("Processing existing files in %s...", conf.SrcPath)
	util.GetAllMusic(conf.SrcPath, convertChan)

	// --- Graceful shutdown ---
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("MusicConvert is running. Press Ctrl+C to stop.")
	<-sigs
	log.Printf("Shutting down...")
}
