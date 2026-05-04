package main

import (
	"bytes"
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


type fetchResult struct {
	label  string
	data   []byte
	status int
	err    error
}

func fetchPost(label, url string, headers map[string]string, body string) fetchResult {
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(body))
	if err != nil {
		return fetchResult{label: label, err: err}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fetchResult{label: label, err: err}
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fetchResult{label: label, status: resp.StatusCode, err: err}
	}
	result := fetchResult{label: label, data: respBody, status: resp.StatusCode}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		preview := string(respBody)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		result.err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, preview)
	}
	return result
}

func fetchURL(label, url string, headers map[string]string) fetchResult {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fetchResult{label: label, err: err}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
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
		// Truncate error body to 500 chars to keep logs readable
		preview := string(body)
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		result.err = fmt.Errorf("HTTP %d: %s", resp.StatusCode, preview)
	}
	return result
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

func extractReviews(label string, data []byte) []Review {
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
	reviews := extractNewAPI(data)
	fmt.Printf("[%s] Parsed %d reviews\n", label, len(reviews))
	return reviews
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

	// Fetch from 2 endpoints concurrently (legacy Places API disabled on this project)
	rawResults := make([]fetchResult, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	getHeaders := map[string]string{
		"X-Goog-Api-Key":   apiKey,
		"X-Goog-FieldMask": "reviews",
	}
	postHeaders := map[string]string{
		"X-Goog-Api-Key":   apiKey,
		"X-Goog-FieldMask": "reviews",
		"Content-Type":     "application/json",
	}
	placeURL := fmt.Sprintf("https://places.googleapis.com/v1/places/%s", placeID)

	// Places API (New) — most relevant (GET)
	go func() {
		defer wg.Done()
		rawResults[0] = fetchURL("places-relevant", placeURL, getHeaders)
	}()

	// Places API (New) — newest (POST with body, reviewSortOrder not supported as URL param)
	go func() {
		defer wg.Done()
		rawResults[1] = fetchPost(
			"places-newest",
			placeURL,
			postHeaders,
			`{"reviewSortOrder":"NEWEST"}`,
		)
	}()

	wg.Wait()

	// Log fetch results and extract reviews
	var candidates []Review
	fetchOK := 0
	for _, r := range rawResults {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "[%s] Fetch error: %v\n", r.label, r.err)
			continue
		}
		fmt.Printf("[%s] HTTP %d, %d bytes received\n", r.label, r.status, len(r.data))
		fetchOK++
		extracted := extractReviews(r.label, r.data)
		candidates = append(candidates, extracted...)
	}

	if fetchOK == 0 {
		fmt.Fprintln(os.Stderr, "All endpoints failed — no reviews fetched")
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

	// Filter and deduplicate with per-reason counts
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
