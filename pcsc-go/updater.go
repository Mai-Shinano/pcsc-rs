package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func update() error {
	resp, err := http.Get("https://api.github.com/repos/eoeo-org/pcsc-rs/releases/latest")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned status %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return err
	}

	if release.TagName == gitDescribe {
		return nil
	}

	osName := runtime.GOOS
	arch := runtime.GOARCH
	for _, asset := range release.Assets {
		name := strings.ToLower(asset.Name)
		if strings.Contains(name, osName) && strings.Contains(name, arch) {
			u, err := url.Parse(asset.BrowserDownloadURL)
			if err != nil {
				continue
			}
			host := strings.ToLower(u.Host)
			if host != "github.com" && !strings.HasSuffix(host, ".githubusercontent.com") {
				continue
			}
			return downloadAndReplace(asset.BrowserDownloadURL)
		}
	}

	return nil
}

func downloadAndReplace(downloadURL string) error {
	log.Printf("Downloading update from %s", downloadURL)

	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	tmpPath := exePath + ".new"
	f, err := os.Create(tmpPath)
	if err != nil {
		return err
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return err
	}
	f.Close()

	// Atomic-ish replace: rename current → .old, rename .new → current
	oldPath := exePath + ".old"
	os.Remove(oldPath)

	if err := os.Rename(exePath, oldPath); err != nil {
		os.Remove(tmpPath)
		return err
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		os.Rename(oldPath, exePath) // rollback
		return err
	}

	os.Remove(oldPath)
	log.Println("Updated successfully")

	switch os.Getenv("PCSC_UPDATED") {
	case "restart":
		cmd := exec.Command(exePath)
		if err := cmd.Start(); err != nil {
			log.Printf("Failed to restart: %v", err)
		}
		os.Exit(0)
	case "terminate":
		os.Exit(0)
	}

	return nil
}
