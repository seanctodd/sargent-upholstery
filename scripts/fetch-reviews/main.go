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
	"strconv"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 30 * time.Second}

const (
	minWords = 20
	// defaultMaxRemovals caps how many reviews a single run may tombstone.
	// Deletions are rare and arrive one at a time; a run proposing more than
	// this is far more likely to be an API anomaly than genuine churn.
	defaultMaxRemovals = 5
)

type Review struct {
	ID     string `json:"id"`
	Author string `json:"author"`
	Rating int    `json:"rating"`
	Text   string `json:"text"`
	Date   string `json:"date"`
	// Removed is the UTC date (YYYY-MM-DD) on which this review stopped being
	// displayable -- deleted from Google, or edited below the display bar.
	// Records are tombstoned rather than deleted so nothing is ever lost and a
	// false positive can be undone by clearing this field. reviews.html filters
	// them out with `where ... "removed" nil`.
	Removed string `json:"removed,omitempty"`
}

// qualifies reports whether a review meets the bar for display on the site.
func qualifies(r Review) bool {
	return r.Rating == 5 && len(strings.Fields(r.Text)) >= minWords
}

// ---- Business Profile API structures ----

type bpReviewsResponse struct {
	Reviews       []bpReview `json:"reviews"`
	NextPageToken string     `json:"nextPageToken"`
	// TotalReviewCount is the total across ALL pages, not just this one. It is
	// what lets us prove a fetch was complete before trusting an absence to
	// mean "deleted" rather than "not retrieved".
	TotalReviewCount int     `json:"totalReviewCount"`
	AverageRating    float64 `json:"averageRating"`
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
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("token read error (HTTP %d): %w", resp.StatusCode, err)
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("token parse error (HTTP %d): %w; body: %.300s", resp.StatusCode, err, body)
	}
	if tok.Error != "" {
		return "", fmt.Errorf("token error (HTTP %d): %s — %s", resp.StatusCode, tok.Error, tok.Description)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("token endpoint HTTP %d: %.300s", resp.StatusCode, body)
	}
	return tok.AccessToken, nil
}

// fetchOutcome carries the raw (unfiltered) reviews plus enough metadata to
// decide whether an absence can be trusted.
type fetchOutcome struct {
	reviews  []Review // every review returned, at any star rating
	total    int      // totalReviewCount reported by the API (0 if absent)
	complete bool     // true only if every page was fetched without error
}

func fetchBusinessProfileReviews(locationName, accessToken string) fetchOutcome {
	out := fetchOutcome{}
	var all []Review
	pageToken := ""
	page := 1

	for {
		apiURL := fmt.Sprintf("https://mybusiness.googleapis.com/v4/%s/reviews?orderBy=update_time+desc&pageSize=50", locationName)
		if pageToken != "" {
			apiURL += "&pageToken=" + url.QueryEscape(pageToken)
		}

		result := fetchURL(fmt.Sprintf("business-profile-p%d", page), apiURL, map[string]string{
			"Authorization": "Bearer " + accessToken,
		})
		if result.err != nil {
			fmt.Fprintf(os.Stderr, "[business-profile-p%d] Fetch error: %v\n", page, result.err)
			if page > 1 {
				fmt.Printf("::warning::Reviews fetch stopped early at page %d (partial result): %v\n", page, result.err)
			}
			break
		}
		fmt.Printf("[business-profile-p%d] HTTP %d, %d bytes received\n", page, result.status, len(result.data))

		var resp bpReviewsResponse
		if err := json.Unmarshal(result.data, &resp); err != nil {
			fmt.Fprintf(os.Stderr, "[business-profile-p%d] Parse error: %v\n", page, err)
			if page > 1 {
				fmt.Printf("::warning::Reviews fetch stopped early at page %d (parse error, partial result): %v\n", page, err)
			}
			break
		}
		fmt.Printf("[business-profile-p%d] Parsed %d reviews\n", page, len(resp.Reviews))
		if resp.TotalReviewCount > out.total {
			out.total = resp.TotalReviewCount
		}

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
			// Ran out of pages with no error: this is the only path that yields
			// a provably complete fetch.
			out.complete = true
			break
		}
		pageToken = resp.NextPageToken
		page++
	}

	out.reviews = all
	return out
}

// reconcile tombstones reviews that are gone from Google, or that no longer
// meet the display bar. It never deletes a record and never rewrites the
// content of a review that still qualifies.
//
// maxRemovals caps the blast radius: if a run would tombstone more than this,
// nothing is changed and skipped is true. That guards against the case where
// the API silently stops returning older reviews -- which would otherwise look
// like a mass deletion and be committed automatically.
func reconcile(existing []Review, fetched []Review, today string, maxRemovals int) (out []Review, tombstoned []Review, skipped bool) {
	byID := make(map[string]Review, len(fetched))
	byText := make(map[string]Review, len(fetched))
	for _, r := range fetched {
		if r.ID != "" {
			byID[r.ID] = r
		}
		if r.Text != "" {
			byText[normalizeText(r.Text)] = r
		}
	}

	var idx []int
	for i, r := range existing {
		// Already tombstoned, or nothing to match on -- leave alone. A review
		// that cannot be looked up cannot be proven absent.
		if r.Removed != "" || (r.ID == "" && r.Text == "") {
			continue
		}
		// Match by ID, then fall back to normalised text -- the same pairing the
		// add path uses for dedup. Review IDs are NOT stable across data
		// sources: records captured while the site used the Places API carry
		// places/... ids that the Business Profile API will never return, so an
		// ID-only match would tombstone every one of them as a false positive.
		current, present := byID[r.ID]
		if !present && r.Text != "" {
			current, present = byText[normalizeText(r.Text)]
		}
		if !present || !qualifies(current) {
			idx = append(idx, i)
		}
	}

	// Work on a copy so a skipped run leaves the caller's slice untouched.
	out = make([]Review, len(existing))
	copy(out, existing)

	if len(idx) > maxRemovals {
		for _, i := range idx {
			tombstoned = append(tombstoned, existing[i])
		}
		return out, tombstoned, true
	}
	for _, i := range idx {
		out[i].Removed = today
		tombstoned = append(tombstoned, out[i])
	}
	return out, tombstoned, false
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

	// REVIEWS_DRY_RUN reports what would change without touching the file --
	// use it the first time reconciliation runs against real data.
	dryRun := os.Getenv("REVIEWS_DRY_RUN") != ""
	if dryRun {
		fmt.Println("DRY RUN: no file will be written")
	}
	maxRemovals := defaultMaxRemovals
	if v := os.Getenv("REVIEWS_MAX_REMOVALS"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			fmt.Fprintf(os.Stderr, "Invalid REVIEWS_MAX_REMOVALS %q: want a non-negative integer\n", v)
			os.Exit(1)
		}
		maxRemovals = n
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
	} else {
		fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", dataFile, err)
		os.Exit(1)
	}
	fmt.Printf("Existing reviews in file: %d\n", len(existing))

	fmt.Println("Using Business Profile API...")
	accessToken, err := getAccessToken(clientID, clientSecret, refreshToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get access token: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Access token obtained")
	outcome := fetchBusinessProfileReviews(locationName, accessToken)
	candidates := outcome.reviews

	if len(candidates) == 0 {
		fmt.Fprintln(os.Stderr, "No reviews fetched from Business Profile API")
		os.Exit(1)
	}
	fmt.Printf("Total candidates before filtering: %d (API reports %d total)\n",
		len(candidates), outcome.total)

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

	// ---- Reconciliation ----
	//
	// Additions above are always safe. Removals are not: an absent review might
	// mean "deleted from Google" or merely "not retrieved". Only tombstone when
	// the fetch is provably complete.
	removedCount := 0
	switch {
	case !outcome.complete:
		fmt.Printf("::warning::Reviews fetch was incomplete (a page failed); " +
			"skipping removal checks. New reviews were still added.\n")
	case outcome.total == 0:
		fmt.Printf("::warning::API did not report totalReviewCount; cannot prove the " +
			"fetch was complete, so skipping removal checks.\n")
	case len(candidates) != outcome.total:
		fmt.Printf("::warning::Fetched %d reviews but API reports %d total; "+
			"skipping removal checks.\n", len(candidates), outcome.total)
	default:
		today := time.Now().UTC().Format("2006-01-02")
		updated, tombstoned, skipped := reconcile(existing, candidates, today, maxRemovals)
		if skipped {
			fmt.Printf("::warning::%d reviews would be marked removed, which exceeds the "+
				"limit of %d. Nothing was changed. Review the list below and, if it is "+
				"correct, re-run with REVIEWS_MAX_REMOVALS set higher.\n",
				len(tombstoned), maxRemovals)
			for _, r := range tombstoned {
				fmt.Printf("  would remove: %s (%s, %s)\n", r.ID, r.Author, r.Date)
			}
		} else {
			existing = updated
			removedCount = len(tombstoned)
			for _, r := range tombstoned {
				fmt.Printf("  marked removed: %s (%s, %s)\n", r.ID, r.Author, r.Date)
			}
		}
	}

	// Sort by date descending
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].Date > existing[j].Date
	})

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
		os.Exit(1)
	}
	if dryRun {
		fmt.Printf("DRY RUN: would write %d reviews (%d new, %d newly removed); "+
			"no file was modified\n", len(existing), newCount, removedCount)
		return
	}
	if err := os.WriteFile(dataFile, out, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Reviews: %d new, %d total\n", newCount, len(existing))
}
