package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

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

// Places API (New) response structures
type newAPIResponse struct {
	Reviews []newAPIReview `json:"reviews"`
}

type newAPIReview struct {
	Name                           string            `json:"name"`
	Rating                         int               `json:"rating"`
	Text                           json.RawMessage   `json:"text"`
	AuthorAttribution              map[string]string `json:"authorAttribution"`
	RelativePublishTimeDescription string            `json:"relativePublishTimeDescription"`
	GoogleMapsURI                  string            `json:"googleMapsUri"`
}

// Legacy API response structures
type legacyAPIResponse struct {
	Result struct {
		Reviews []legacyAPIReview `json:"reviews"`
	} `json:"result"`
}

type legacyAPIReview struct {
	AuthorName              string `json:"author_name"`
	Rating                  int    `json:"rating"`
	Text                    string `json:"text"`
	RelativeTimeDescription string `json:"relative_time_description"`
	Time                    int64  `json:"time"`
}

func fetchURL(url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func extractNewAPI(data []byte) []Review {
	var resp newAPIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var reviews []Review
	for _, r := range resp.Reviews {
		// text can be {"text": "..."} or a plain string
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
	return reviews
}

func extractLegacy(data []byte) []Review {
	var resp legacyAPIResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil
	}
	var reviews []Review
	for _, r := range resp.Result.Reviews {
		id := ""
		if r.Time != 0 {
			id = fmt.Sprintf("legacy-%d-%s", r.Time, r.AuthorName)
		}
		date := time.Now().UTC().Format(time.RFC3339)
		if r.Time != 0 {
			date = time.Unix(r.Time, 0).UTC().Format(time.RFC3339)
		}
		reviews = append(reviews, Review{
			ID:           id,
			Author:       r.AuthorName,
			Rating:       r.Rating,
			Text:         r.Text,
			Date:         date,
			RelativeTime: r.RelativeTimeDescription,
		})
	}
	return reviews
}

func extractReviews(data []byte) []Review {
	// Try new API format first — check for authorAttribution or name in first review
	var probe struct {
		Reviews []json.RawMessage `json:"reviews"`
		Result  json.RawMessage   `json:"result"`
	}
	if json.Unmarshal(data, &probe) != nil {
		return nil
	}
	if len(probe.Reviews) > 0 {
		var first map[string]json.RawMessage
		if json.Unmarshal(probe.Reviews[0], &first) == nil {
			if _, ok := first["authorAttribution"]; ok {
				return extractNewAPI(data)
			}
			if _, ok := first["name"]; ok {
				return extractNewAPI(data)
			}
		}
	}
	if probe.Result != nil {
		return extractLegacy(data)
	}
	return nil
}

func normalizeText(text string) string {
	return strings.Join(strings.Fields(strings.ToLower(text)), " ")
}

func main() {
	apiKey := os.Getenv("GOOGLE_API_KEY")
	if apiKey == "" {
		fmt.Println("GOOGLE_API_KEY not set, skipping review fetch")
		os.Exit(0)
	}

	// Resolve data file path relative to script location
	scriptDir, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	dataFile := filepath.Join(scriptDir, "data", "reviews.json")

	// Initialize data file if missing
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(dataFile), 0755)
		os.WriteFile(dataFile, []byte("[]"), 0644)
	}

	// Fetch from 3 endpoints concurrently
	type fetchResult struct {
		data []byte
		err  error
	}
	results := make([]fetchResult, 3)
	var wg sync.WaitGroup

	wg.Add(3)

	// Places API (New)
	go func() {
		defer wg.Done()
		data, err := fetchURL(
			fmt.Sprintf("https://places.googleapis.com/v1/places/%s", placeID),
			map[string]string{
				"X-Goog-Api-Key":   apiKey,
				"X-Goog-FieldMask": "reviews",
			},
		)
		results[0] = fetchResult{data, err}
	}()

	// Legacy API — most relevant
	go func() {
		defer wg.Done()
		data, err := fetchURL(
			fmt.Sprintf("https://maps.googleapis.com/maps/api/place/details/json?place_id=%s&fields=reviews&reviews_sort=most_relevant&key=%s", placeID, apiKey),
			nil,
		)
		results[1] = fetchResult{data, err}
	}()

	// Legacy API — newest
	go func() {
		defer wg.Done()
		data, err := fetchURL(
			fmt.Sprintf("https://maps.googleapis.com/maps/api/place/details/json?place_id=%s&fields=reviews&reviews_sort=newest&key=%s", placeID, apiKey),
			nil,
		)
		results[2] = fetchResult{data, err}
	}()

	wg.Wait()

	// Extract reviews from all responses
	var candidates []Review
	for _, r := range results {
		if r.err != nil || r.data == nil {
			continue
		}
		candidates = append(candidates, extractReviews(r.data)...)
	}

	// Load existing reviews
	var existing []Review
	if data, err := os.ReadFile(dataFile); err == nil {
		json.Unmarshal(data, &existing)
	}

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
	newCount := 0
	for _, r := range candidates {
		if r.Rating != 5 {
			continue
		}
		if len(strings.Fields(r.Text)) < minWords {
			continue
		}
		if r.ID != "" && existingIDs[r.ID] {
			continue
		}
		norm := normalizeText(r.Text)
		if existingTexts[norm] {
			continue
		}
		existing = append(existing, r)
		existingIDs[r.ID] = true
		existingTexts[norm] = true
		newCount++
	}

	// Sort by date descending
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].Date > existing[j].Date
	})

	// Write back
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
