package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	mediaLimit = 20
	graphBase  = "https://graph.instagram.com"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

// Post is the metadata written to data/instagram.json (consumed by the shortcode).
type Post struct {
	ID        string `json:"id"`
	Caption   string `json:"caption"`
	Permalink string `json:"permalink"`
	Timestamp string `json:"timestamp"`
	Image     string `json:"image"`
	MediaType string `json:"mediaType"`
}

// mediaItem mirrors one entry of the Graph API /me/media response.
type mediaItem struct {
	ID           string `json:"id"`
	Caption      string `json:"caption"`
	MediaType    string `json:"media_type"`
	MediaURL     string `json:"media_url"`
	ThumbnailURL string `json:"thumbnail_url"`
	Permalink    string `json:"permalink"`
	Timestamp    string `json:"timestamp"`
}

type mediaResponse struct {
	Data  []mediaItem `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// refreshToken exchanges the current long-lived token for a fresh 60-day one.
func refreshToken(token string) (string, error) {
	u := fmt.Sprintf("%s/refresh_access_token?grant_type=ig_refresh_token&access_token=%s",
		graphBase, url.QueryEscape(token))
	resp, err := httpClient.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	var r struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", err
	}
	if r.AccessToken == "" {
		return "", fmt.Errorf("no access_token in refresh response: %s", body)
	}
	return r.AccessToken, nil
}

// persistToken updates the IG_ACCESS_TOKEN GitHub Actions secret via the gh CLI,
// authorized by GH_PAT. Requires gh to be installed (it is on GitHub runners).
func persistToken(token string) error {
	if os.Getenv("GH_PAT") == "" {
		return fmt.Errorf("GH_PAT not set")
	}
	cmd := exec.Command("gh", "secret", "set", "IG_ACCESS_TOKEN")
	cmd.Stdin = strings.NewReader(token)
	cmd.Env = append(os.Environ(), "GH_TOKEN="+os.Getenv("GH_PAT"))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, out)
	}
	return nil
}

// fetchMedia returns the most recent media items (newest first, as the API orders them).
func fetchMedia(token string) ([]mediaItem, error) {
	fields := "id,caption,media_type,media_url,thumbnail_url,permalink,timestamp"
	u := fmt.Sprintf("%s/me/media?fields=%s&limit=%d&access_token=%s",
		graphBase, fields, mediaLimit, url.QueryEscape(token))
	resp, err := httpClient.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}
	var mr mediaResponse
	if err := json.Unmarshal(body, &mr); err != nil {
		return nil, err
	}
	if mr.Error != nil {
		return nil, fmt.Errorf("API error: %s", mr.Error.Message)
	}
	return mr.Data, nil
}

// downloadFile writes the body of url to dst.
func downloadFile(fileURL, dst string) error {
	resp, err := httpClient.Get(fileURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func main() {
	token := os.Getenv("IG_ACCESS_TOKEN")
	if token == "" {
		fmt.Println("IG_ACCESS_TOKEN not set, skipping Instagram fetch")
		fmt.Println("Set IG_ACCESS_TOKEN (and GH_PAT for automated token rotation)")
		os.Exit(0)
	}

	// 1. Refresh + persist token. Non-fatal: the current token is valid up to 60 days.
	if newTok, err := refreshToken(token); err != nil {
		fmt.Fprintf(os.Stderr, "Token refresh failed (continuing with current token): %v\n", err)
	} else {
		token = newTok
		fmt.Println("Token refreshed")
		if err := persistToken(newTok); err != nil {
			fmt.Fprintf(os.Stderr, "Could not persist refreshed token (rotation skipped): %v\n", err)
		} else {
			fmt.Println("IG_ACCESS_TOKEN secret updated")
		}
	}

	// 2. Fetch media.
	items, err := fetchMedia(token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Media fetch error: %v\n", err)
		os.Exit(1)
	}
	if len(items) == 0 {
		fmt.Fprintln(os.Stderr, "No media returned; leaving existing gallery untouched")
		os.Exit(1)
	}
	fmt.Printf("Fetched %d media items\n", len(items))

	imgDir := filepath.Join("assets", "instagram")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir error: %v\n", err)
		os.Exit(1)
	}

	// 3. Download images + build metadata.
	var posts []Post
	keep := map[string]bool{}
	for _, it := range items {
		imgURL := it.MediaURL
		if it.MediaType == "VIDEO" {
			imgURL = it.ThumbnailURL
		}
		if imgURL == "" {
			fmt.Fprintf(os.Stderr, "skipping %s: no image url\n", it.ID)
			continue
		}
		fileName := it.ID + ".jpg"
		rel := filepath.ToSlash(filepath.Join("instagram", fileName))
		dst := filepath.Join("assets", "instagram", fileName)
		if err := downloadFile(imgURL, dst); err != nil {
			// Keep the previously committed image for this post if we already have it.
			if _, statErr := os.Stat(dst); statErr != nil {
				fmt.Fprintf(os.Stderr, "skipping %s: download failed and no cached image: %v\n", it.ID, err)
				continue
			}
			fmt.Fprintf(os.Stderr, "warning %s: download failed, keeping cached image: %v\n", it.ID, err)
		}
		keep[fileName] = true
		posts = append(posts, Post{
			ID:        it.ID,
			Caption:   it.Caption,
			Permalink: it.Permalink,
			Timestamp: it.Timestamp,
			Image:     rel,
			MediaType: it.MediaType,
		})
	}
	if len(posts) == 0 {
		fmt.Fprintln(os.Stderr, "No images available; leaving existing gallery untouched")
		os.Exit(1)
	}

	// 4. Prune images no longer referenced.
	entries, _ := os.ReadDir(imgDir)
	for _, e := range entries {
		if !keep[e.Name()] {
			if err := os.Remove(filepath.Join(imgDir, e.Name())); err == nil {
				fmt.Printf("pruned %s\n", e.Name())
			}
		}
	}

	// 5. Write metadata.
	out, err := json.MarshalIndent(posts, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal error: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll("data", 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir data error: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join("data", "instagram.json"), append(out, '\n'), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Wrote data/instagram.json with %d posts\n", len(posts))
}
