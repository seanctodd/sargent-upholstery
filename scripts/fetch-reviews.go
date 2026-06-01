package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

const (
	minWords = 20
)

type Review struct {
	ID            string `json:"id"`
	Author        string `json:"author"`
	Rating        int    `json:"rating"`
	Text          string `json:"text"`
	Date          string `json:"date"`
	RelativeTime  string `json:"relativeTime"`
	GoogleMapsURI string `json:"googleMapsUri"`
}

// ---- Business Profile API structures ----

type bpReviewsResponse struct {
	Reviews       []bpReview `json:"reviews"`
	NextPageToken string     `json:"nextPageToken"`
}

type bpReview struct {
	Name     string `json:"name"`
	ReviewID string `json:"reviewId"`
	Reviewer struct {
		DisplayName string `json:"displayName"`
	} `json:"reviewer"`
	StarRating string `json:"starRating"`
	Comment    string `json:"comment"`
	CreateTime string `json:"createTime"`
	UpdateTime string `json:"updateTime"`
}

func starRatingToInt(s string) int {
	switch s {
	case "FIVE":
		return 5
	case "FOUR":
		return 4
	case "THREE":
		return 3
	case "TWO":
		return 2
	case "ONE":
		return 1
	default:
		return 0
	}
}

// ---- HTTP helpers ----

type fetchResult struct {
	label  string
	data   []byte
	status int
	err    error
}

func fetchURL(label, url string, headers map[string]string) fetchResult {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fetchResult{label: label, err: err}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return fetchResult{label: label, err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fetchResult{label: label, status: resp.StatusCode, err: err}
	}
	result := fetchResult{label: label, data: body, status: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		result.err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, preview)
	}
	return result
}

// ---- Business Profile API ----

func getAccessToken(clientID, clientSecret, refreshToken string) (string, error) {
	resp, err := httpClient.PostForm("https://oauth2.googleapis.com/token", url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	})
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("token parse error: %w", err)
	}
	if tok.Error != "" {
		return "", fmt.Errorf("token error: %s — %s", tok.Error, tok.Description)
	}
	return tok.AccessToken, nil
}

func fetchBusinessProfileReviews(locationName, accessToken string) []Review {
	var all []Review
	pageToken := ""
	page := 1

	for {
		apiURL := fmt.Sprintf("https://mybusiness.googleapis.com/v4/%s/reviews?orderBy=update_time+desc&pageSize=50", locationName)
		if pageToken != "" {
			apiURL += "&pageToken=" + pageToken
		}

		result := fetchURL(fmt.Sprintf("business-profile-p%d", page), apiURL, map[string]string{
			"Authorization": "Bearer " + accessToken,
		})
		if result.err != nil {
			fmt.Fprintf(os.Stderr, "[business-profile-p%d] Fetch error: %v\n", page, result.err)
			break
		}
		fmt.Printf("[business-profile-p%d] HTTP %d, %d bytes received\n", page, result.status, len(result.data))

		var resp bpReviewsResponse
		if err := json.Unmarshal(result.data, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "[business-profile-p%d] Parse error: %v\n", page, err)
			break
		}
		fmt.Printf("[business-profile-p%d] Parsed %d reviews\n", page, len(resp.Reviews))

		for _, r := range resp.Reviews {
			date := r.UpdateTime
			if date == "" {
				date = r.CreateTime
			}
			all = append(all, Review{
				ID:     r.Name,
				Author: r.Reviewer.DisplayName,
				Rating: starRatingToInt(r.StarRating),
				Text:   r.Comment,
				Date:   date,
			})
		}

		if resp.NextPageToken == "" {
			break
		}
		pageToken = resp.NextPageToken
		page++
	}

	return all
}

// ---- Shared helpers ----

func normalizeText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

// ---- Main ----

func main() {
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	refreshToken := os.Getenv("GOOGLE_REFRESH_TOKEN")
	locationName := os.Getenv("GOOGLE_LOCATION_NAME")

	if clientID == "" || clientSecret == "" || refreshToken == "" || locationName == "" {
		fmt.Println("Business Profile API credentials not set, skipping review fetch")
		fmt.Println("Set GOOGLE_CLIENT_ID + GOOGLE_CLIENT_SECRET + GOOGLE_REFRESH_TOKEN + GOOGLE_LOCATION_NAME")
		os.Exit(0)
	}

	scriptDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dataFile := filepath.Join(scriptDir, "data", "reviews.json")

	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(dataFile), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating data dir: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(dataFile, []byte("[]"), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating %s: %v\n", dataFile, err)
			os.Exit(1)
		}
	}

	fmt.Println("Using Business Profile API...")
	accessToken, err := getAccessToken(clientID, clientSecret, refreshToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get access token: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Access token obtained")
	candidates := fetchBusinessProfileReviews(locationName, accessToken)

	if len(candidates) == 0 {
		fmt.Fprintln(os.Stderr, "No reviews fetched from any source")
		os.Exit(1)
	}
	fmt.Printf("Total candidates before filtering: %d\n", len(candidates))

	// Load existing reviews. A non-empty but unparseable file means something is
	// wrong — refuse to proceed rather than silently overwrite accumulated history.
	var existing []Review
	if data, err := os.ReadFile(dataFile); err == nil {
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := json.Unmarshal(data, &existing); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %s is not valid JSON: %v\n", dataFile, err)
				fmt.Fprintln(os.Stderr, "Refusing to overwrite to protect existing reviews.")
				os.Exit(1)
			}
		}
	}
	fmt.Printf("Existing reviews in file: %d\n", len(existing))

	// Build dedup sets
	existingIDs := make(map[string]bool)
	existingTexts := make(map[string]bool)
	for _, r := range existing {
		if r.ID != "" {
			existingIDs[r.ID] = true
		}
		if r.Text != "" {
			existingTexts[normalizeText(r.Text)] = true
		}
	}

	// Filter and deduplicate
	skippedRating, skippedShort, skippedDup, newCount := 0, 0, 0, 0
	for _, r := range candidates {
		if r.Rating != 5 {
			skippedRating++
			continue
		}
		if len(strings.Fields(r.Text)) < minWords {
			skippedShort++
			continue
		}
		if r.ID != "" && existingIDs[r.ID] {
			skippedDup++
			continue
		}
		norm := normalizeText(r.Text)
		if existingTexts[norm] {
			skippedDup++
			continue
		}
		existing = append(existing, r)
		if r.ID != "" {
			existingIDs[r.ID] = true
		}
		existingTexts[norm] = true
		newCount++
	}

	fmt.Printf("Filtered out: %d not 5-star, %d too short (<%d words), %d duplicates\n",
		skippedRating, skippedShort, minWords, skippedDup)

	// Sort by date descending
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].Date > existing[j].Date
	})

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(dataFile, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Reviews: %d new, %d total\n", newCount, len(existing))
}
