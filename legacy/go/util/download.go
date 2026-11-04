package util

import (
	"io"
	"net/http"
	"os"
	"time"
)

var downloadClient = &http.Client{
	Timeout: 20 * time.Second,
}

func DownloadCover(url, destPath string) error {
	if url == "" {
		return nil
	}
	resp, err := downloadClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
