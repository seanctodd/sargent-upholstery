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
	placeID  = "ChIJvQmmCSS35YgR-3H9ajzGCHk"
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

// ---- Places API (New) structures ----

type placesAPIResponse struct {
	Reviews []placesAPIReview `json:"reviews"`
}

type placesAPIReview struct {
	Name                           string            `json:"name"`
	Rating                         int               `json:"rating"`
	Text                           json.RawMessage   `json:"text"`
	AuthorAttribution              map[string]string `json:"authorAttribution"`
	RelativePublishTimeDescription string            `json:"relativePublishTimeDescription"`
	GoogleMapsURI                  string            `json:"googleMapsUri"`
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

// ---- Places API extraction ----

func extractPlacesReviews(label string, data []byte) []Review {
	var probe struct {
		Reviews []json.RawMessage `json:"reviews"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		preview := string(data)
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		fmt.Fprintf(os.Stderr, "[%s] JSON parse error: %v\nBody: %s\n", label, err, preview)
		return nil
	}
	if len(probe.Reviews) == 0 {
		preview := string(data)
		if len(preview) > 300 {
			preview = preview[:300] + "..."
		}
		fmt.Fprintf(os.Stderr, "[%s] No reviews in response. Body: %s\n", label, preview)
		return nil
	}
	var resp placesAPIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var reviews []Review
	for _, r := range resp.Reviews {
		text := ""
		if len(r.Text) > 0 {
			var textObj struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(r.Text, &textObj) == nil && textObj.Text != "" {
				text = textObj.Text
			} else {
				var s string
				if json.Unmarshal(r.Text, &s) == nil {
					text = s
				}
			}
		}
		author := ""
		if r.AuthorAttribution != nil {
			author = r.AuthorAttribution["displayName"]
		}
		reviews = append(reviews, Review{
			ID:            r.Name,
			Author:        author,
			Rating:        r.Rating,
			Text:          text,
			Date:          now,
			RelativeTime:  r.RelativePublishTimeDescription,
			GoogleMapsURI: r.GoogleMapsURI,
		})
	}
	fmt.Printf("[%s] Parsed %d reviews\n", label, len(reviews))
	return reviews
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
	apiKey := os.Getenv("GOOGLE_API_KEY")
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	refreshToken := os.Getenv("GOOGLE_REFRESH_TOKEN")
	locationName := os.Getenv("GOOGLE_LOCATION_NAME")

	if apiKey == "" && (clientID == "" || clientSecret == "" || refreshToken == "" || locationName == "") {
		fmt.Println("No credentials set, skipping review fetch")
		fmt.Println("Set GOOGLE_API_KEY for Places API, or set GOOGLE_CLIENT_ID + GOOGLE_CLIENT_SECRET + GOOGLE_REFRESH_TOKEN + GOOGLE_LOCATION_NAME for Business Profile API")
		os.Exit(0)
	}

	scriptDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dataFile := filepath.Join(scriptDir, "data", "reviews.json")

	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(dataFile), 0755)
		os.WriteFile(dataFile, []byte("[]"), 0644)
	}

	var candidates []Review

	// Business Profile API — newest first, up to 50 per page (preferred when available)
	if clientID != "" && clientSecret != "" && refreshToken != "" && locationName != "" {
		fmt.Println("Using Business Profile API...")
		accessToken, err := getAccessToken(clientID, clientSecret, refreshToken)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to get access token: %v\n", err)
			fmt.Fprintln(os.Stderr, "Falling back to Places API...")
		} else {
			fmt.Println("Access token obtained")
			bpReviews := fetchBusinessProfileReviews(locationName, accessToken)
			candidates = append(candidates, bpReviews...)
		}
	}

	// Places API (New) — most relevant (fallback or supplement)
	if apiKey != "" {
		fmt.Println("Using Places API (most relevant)...")
		result := fetchURL("places-relevant",
			fmt.Sprintf("https://places.googleapis.com/v1/places/%s", placeID),
			map[string]string{
				"X-Goog-Api-Key":   apiKey,
				"X-Goog-FieldMask": "reviews",
			},
		)
		if result.err != nil {
			fmt.Fprintf(os.Stderr, "[places-relevant] Fetch error: %v\n", result.err)
		} else {
			fmt.Printf("[places-relevant] HTTP %d, %d bytes received\n", result.status, len(result.data))
			candidates = append(candidates, extractPlacesReviews("places-relevant", result.data)...)
		}
	}

	if len(candidates) == 0 {
		fmt.Fprintln(os.Stderr, "No reviews fetched from any source")
		os.Exit(1)
	}
	fmt.Printf("Total candidates before filtering: %d\n", len(candidates))

	// Load existing reviews
	var existing []Review
	if data, err := os.ReadFile(dataFile); err == nil {
		json.Unmarshal(data, &existing)
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
		existingIDs[r.ID] = true
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
