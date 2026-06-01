# Business Profile v4 Review Fetching Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rework `scripts/fetch-reviews.go` to fetch all reviews from the Google Business Profile v4 API as the single source, remove the Places API path, and harden data integrity so the weekly workflow keeps `data/reviews.json` fresh.

**Architecture:** A single Go script exchanges an OAuth refresh token for an access token, paginates the v4 `accounts.locations.reviews.list` endpoint, filters to 5-star/≥20-word reviews, dedups against the existing file, and writes newest-first. No Places API. No `time.Now()` fabricated dates.

**Tech Stack:** Go (standalone script, no module), Google Business Profile API v4, GitHub Actions, Hugo (consumes `data/reviews.json`).

**Testing note:** There is no Go module or test framework in this repo, and the core logic is live-API I/O requiring secrets. Per the approved spec, per-task verification is `gofmt` + `go build` (compile check); end-to-end behavior is verified by a manual run with real secrets (Task 8). This is a deliberate adaptation of the TDD default to a credentials-gated script.

**Spec:** `docs/superpowers/specs/2026-06-01-business-profile-reviews-design.md`

---

## File Structure

- **Modify:** `scripts/fetch-reviews.go` — the rework (Tasks 2–6)
- **Modify:** `scripts/setup-business-profile.go` — gofmt cleanup only (Task 1)
- **Modify:** `.github/workflows/hugo.yml` — drop `GOOGLE_API_KEY` (Task 7)
- **Modify:** `README.md` — document OAuth setup, fix stale references (Task 7)
- **Unchanged:** `themes/sargent/layouts/partials/reviews.html` (out of scope)

---

## Task 1: gofmt cleanup of setup-business-profile.go

**Files:**
- Modify: `scripts/setup-business-profile.go`

- [ ] **Step 1: Verify the formatting problem exists**

Run: `gofmt -l scripts/`
Expected: prints `scripts/setup-business-profile.go`

- [ ] **Step 2: Apply gofmt**

Run: `gofmt -w scripts/setup-business-profile.go`

This collapses the misaligned struct tags at the `Metadata` struct (around lines 204-205):
```go
			Metadata struct {
				MapsURI string `json:"mapsUri"`
				PlaceID string `json:"placeId"`
			} `json:"metadata"`
```

- [ ] **Step 3: Verify clean**

Run: `gofmt -l scripts/`
Expected: prints nothing (empty output)

- [ ] **Step 4: Commit**

```bash
git add scripts/setup-business-profile.go
git commit -m "Apply gofmt to setup-business-profile.go"
```

---

## Task 2: Add an HTTP client with a timeout

**Files:**
- Modify: `scripts/fetch-reviews.go`

Currently `fetchURL` uses `http.DefaultClient` and `getAccessToken` uses `http.PostForm`, both of which have no timeout — a hung Google endpoint blocks the CI job indefinitely.

- [ ] **Step 1: Add a package-level client**

Add immediately after the `import (...)` block (before the `const (...)` block):
```go
var httpClient = &http.Client{Timeout: 30 * time.Second}
```

- [ ] **Step 2: Route fetchURL through it**

In `fetchURL`, change:
```go
	resp, err := http.DefaultClient.Do(req)
```
to:
```go
	resp, err := httpClient.Do(req)
```

- [ ] **Step 3: Route getAccessToken through it**

In `getAccessToken`, change:
```go
	resp, err := http.PostForm("https://oauth2.googleapis.com/token", url.Values{
```
to:
```go
	resp, err := httpClient.PostForm("https://oauth2.googleapis.com/token", url.Values{
```

- [ ] **Step 4: Compile check**

Run: `go build -o /tmp/fetch-reviews scripts/fetch-reviews.go`
Expected: no output, exit 0. (`time` is still imported — used by the new client.)

- [ ] **Step 5: Commit**

```bash
git add scripts/fetch-reviews.go
git commit -m "Add 30s HTTP client timeout to review fetching"
```

---

## Task 3: Use real review timestamps instead of time.Now()

**Files:**
- Modify: `scripts/fetch-reviews.go`

`fetchBusinessProfileReviews` currently falls back to `time.Now()` when `updateTime` is empty, which corrupts newest-first sorting. Fall back to `createTime`, then to empty string.

- [ ] **Step 1: Change the date fallback**

In `fetchBusinessProfileReviews`, replace:
```go
		for _, r := range resp.Reviews {
			date := r.UpdateTime
			if date == "" {
				date = time.Now().UTC().Format(time.RFC3339)
			}
			all = append(all, Review{
				ID:     r.Name,
				Author: r.Reviewer.DisplayName,
				Rating: starRatingToInt(r.StarRating),
				Text:   r.Comment,
				Date:   date,
			})
		}
```
with:
```go
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
```

- [ ] **Step 2: Compile check**

Run: `go build -o /tmp/fetch-reviews scripts/fetch-reviews.go`
Expected: no output, exit 0.

- [ ] **Step 3: Commit**

```bash
git add scripts/fetch-reviews.go
git commit -m "Use real review timestamps (updateTime/createTime) not time.Now()"
```

---

## Task 4: Remove the Places API path

**Files:**
- Modify: `scripts/fetch-reviews.go`

Make Business Profile v4 the single source. Delete all Places API code and the `GOOGLE_API_KEY` branch.

- [ ] **Step 1: Delete the Places API structs**

Remove this entire block (the `// ---- Places API (New) structures ----` section):
```go
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
```

- [ ] **Step 2: Delete the placeID constant**

Change:
```go
const (
	placeID  = "ChIJvQmmCSS35YgR-3H9ajzGCHk"
	minWords = 20
)
```
to:
```go
const (
	minWords = 20
)
```

- [ ] **Step 3: Delete the extractPlacesReviews function**

Remove the entire `// ---- Places API extraction ----` section — the whole `func extractPlacesReviews(label string, data []byte) []Review { ... }` (lines ~119-178), including its section comment.

- [ ] **Step 4: Simplify the credential check in main**

Replace:
```go
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
```
with:
```go
	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")
	refreshToken := os.Getenv("GOOGLE_REFRESH_TOKEN")
	locationName := os.Getenv("GOOGLE_LOCATION_NAME")

	if clientID == "" || clientSecret == "" || refreshToken == "" || locationName == "" {
		fmt.Println("Business Profile API credentials not set, skipping review fetch")
		fmt.Println("Set GOOGLE_CLIENT_ID + GOOGLE_CLIENT_SECRET + GOOGLE_REFRESH_TOKEN + GOOGLE_LOCATION_NAME")
		os.Exit(0)
	}
```

- [ ] **Step 5: Remove the Places fetch branch and simplify the Business Profile branch**

Replace this block in `main`:
```go
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
```
with:
```go
	fmt.Println("Using Business Profile API...")
	accessToken, err := getAccessToken(clientID, clientSecret, refreshToken)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get access token: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Access token obtained")
	candidates := fetchBusinessProfileReviews(locationName, accessToken)
```

- [ ] **Step 6: Compile check**

Run: `go build -o /tmp/fetch-reviews scripts/fetch-reviews.go`
Expected: no output, exit 0. If the compiler reports an unused import, see Step 7.

- [ ] **Step 7: Confirm imports are still all used**

After removing Places code, these imports must all still be referenced: `encoding/json` (BP parsing), `fmt`, `io` (fetchURL/getAccessToken bodies), `net/http`, `net/url` (getAccessToken), `os`, `path/filepath`, `sort`, `strings`, `time` (httpClient timeout). They are. If `go build` in Step 6 passed, no import change is needed. Do NOT remove any import that compiles.

- [ ] **Step 8: Verify gofmt**

Run: `gofmt -l scripts/`
Expected: prints nothing.

- [ ] **Step 9: Commit**

```bash
git add scripts/fetch-reviews.go
git commit -m "Remove Places API path; Business Profile v4 is the sole review source"
```

---

## Task 5: Harden data integrity (corrupt-file guard, dedup guard, init errors)

**Files:**
- Modify: `scripts/fetch-reviews.go`

- [ ] **Step 1: Check errors on first-run file creation**

Replace:
```go
	if _, err := os.Stat(dataFile); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(dataFile), 0755)
		os.WriteFile(dataFile, []byte("[]"), 0644)
	}
```
with:
```go
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
```

- [ ] **Step 2: Fail loudly on a corrupt existing file**

Replace:
```go
	// Load existing reviews
	var existing []Review
	if data, err := os.ReadFile(dataFile); err == nil {
		json.Unmarshal(data, &existing)
	}
	fmt.Printf("Existing reviews in file: %d\n", len(existing))
```
with:
```go
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
```

- [ ] **Step 3: Guard the dedup map against empty IDs**

In the filter/dedup loop, replace:
```go
		existing = append(existing, r)
		existingIDs[r.ID] = true
		existingTexts[norm] = true
		newCount++
```
with:
```go
		existing = append(existing, r)
		if r.ID != "" {
			existingIDs[r.ID] = true
		}
		existingTexts[norm] = true
		newCount++
```

- [ ] **Step 4: Compile check**

Run: `go build -o /tmp/fetch-reviews scripts/fetch-reviews.go`
Expected: no output, exit 0.

- [ ] **Step 5: Verify the corrupt-file guard manually**

```bash
cp data/reviews.json /tmp/reviews.backup.json
printf 'this is not json' > data/reviews.json
GOOGLE_CLIENT_ID=x GOOGLE_CLIENT_SECRET=x GOOGLE_REFRESH_TOKEN=x GOOGLE_LOCATION_NAME=x go run scripts/fetch-reviews.go; echo "exit=$?"
cp /tmp/reviews.backup.json data/reviews.json
```
Expected: prints the "not valid JSON" / "Refusing to overwrite" error and `exit=1`. (The dummy creds fail at token exchange, but the corrupt-file check runs first and exits before that — confirm the JSON error is what you see.) The final `cp` restores the real file.

- [ ] **Step 6: Verify gofmt and commit**

```bash
gofmt -l scripts/   # expect empty
git add scripts/fetch-reviews.go
git commit -m "Harden review fetch: guard corrupt file, init errors, empty IDs"
```

---

## Task 6: Confirm sorting and pagination are intact

**Files:**
- Read-only review of `scripts/fetch-reviews.go` (no edit expected)

This task verifies the pieces the spec relies on but that earlier tasks did not touch, so a reviewer doesn't assume they were forgotten.

- [ ] **Step 1: Confirm the pagination loop**

Confirm `fetchBusinessProfileReviews` still builds the URL with `pageSize=50` and `orderBy`, and loops on `resp.NextPageToken` until empty. Spec note: `orderBy` value is non-critical because Step 2 sorts client-side; leave the existing string as-is.

- [ ] **Step 2: Confirm client-side sort**

Confirm `main` still contains:
```go
	sort.Slice(existing, func(i, j int) bool {
		return existing[i].Date > existing[j].Date
	})
```
This now sorts on real RFC3339 timestamps (Task 3), so newest-first is correct.

- [ ] **Step 3: No commit**

No code change in this task. If both checks pass, proceed. If either is missing, it was lost in an earlier edit — restore it before continuing.

---

## Task 7: Update workflow and README

**Files:**
- Modify: `.github/workflows/hugo.yml`
- Modify: `README.md`

- [ ] **Step 1: Drop GOOGLE_API_KEY from the workflow**

In `.github/workflows/hugo.yml`, remove this line from the `env:` block:
```yaml
          GOOGLE_API_KEY: ${{ secrets.GOOGLE_API_KEY }}
```
Leave the four OAuth secrets and the commit/push step unchanged.

- [ ] **Step 2: Rewrite the README "Google Reviews Setup" section**

Replace the section that currently reads:
```markdown
### Google Reviews Setup

The workflow fetches reviews from the Google Places API. To configure:
1. Create a Google API key with Places API access
2. Add it as a repository secret named `GOOGLE_API_KEY`
3. Reviews are saved to `data/reviews.json` and displayed on the homepage
```
with:
```markdown
### Google Reviews Setup

The workflow fetches reviews from the **Google Business Profile API v4**
(`accounts.locations.reviews`), which returns the full set of reviews newest-first.
This API uses **OAuth 2.0**, not a plain API key. One-time setup:

1. In Google Cloud Console, enable the **My Business Account Management API** and
   **My Business Reviews API** (v4 access must be granted by Google via their
   access-request form).
2. Create an **OAuth 2.0 credential of type "Desktop app"** and download the
   `client_secret_*.json` file.
3. Run the setup helper, which authorizes in your browser and prints the secrets:
   ```bash
   go run scripts/setup-business-profile.go /path/to/client_secret.json
   ```
4. Add the four printed values as GitHub Actions repository secrets:
   `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REFRESH_TOKEN`,
   `GOOGLE_LOCATION_NAME`.

Reviews are filtered to 5-star with 20+ words, saved to `data/reviews.json`, and
displayed on the homepage.
```

- [ ] **Step 3: Fix the stale shortcode reference in the project tree**

In the README "Project Structure" block, change:
```
│   │   └── shortcodes/         # Custom shortcodes
│   │       ├── img.html               # Responsive image shortcode (WebP + srcset)
│   │       ├── instagram-gallery.html
│   │       └── google-reviews.html
```
to:
```
│   │   └── shortcodes/         # Custom shortcodes
│   │       ├── img.html               # Responsive image shortcode (WebP + srcset)
│   │       └── instagram-gallery.html
```
(Reviews render via `partials/reviews.html`, not a shortcode.)

- [ ] **Step 4: Add the setup script to the project tree**

In the same block, change:
```
├── scripts/
│   └── fetch-reviews.go        # Google Reviews fetch script
```
to:
```
├── scripts/
│   ├── fetch-reviews.go        # Google Reviews fetch script (Business Profile v4)
│   └── setup-business-profile.go  # One-time OAuth setup → prints CI secrets
```

- [ ] **Step 5: Update the Deployment bullet that names the secret**

In the "GitHub Actions" deployment paragraph, change:
```markdown
1. Fetches reviews via `scripts/fetch-reviews.go` (requires `GOOGLE_API_KEY` secret)
```
to:
```markdown
1. Fetches reviews via `scripts/fetch-reviews.go` (requires the four `GOOGLE_*` OAuth secrets)
```

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/hugo.yml README.md
git commit -m "Drop GOOGLE_API_KEY; document Business Profile OAuth setup in README"
```

---

## Task 8: End-to-end manual verification (requires real secrets)

**Files:** none (verification only)

This task needs the four real OAuth secrets and cannot be run by an agent without them. The person with credentials runs it.

- [ ] **Step 1: Export real secrets locally**

```bash
export GOOGLE_CLIENT_ID=...
export GOOGLE_CLIENT_SECRET=...
export GOOGLE_REFRESH_TOKEN=...
export GOOGLE_LOCATION_NAME=accounts/123/locations/456
```
(If you don't have `GOOGLE_REFRESH_TOKEN` / `GOOGLE_LOCATION_NAME` yet, run
`go run scripts/setup-business-profile.go /path/to/client_secret.json` first.)

- [ ] **Step 2: Run the fetch**

Run: `go run scripts/fetch-reviews.go`
Expected: "Access token obtained", one or more `[business-profile-pN] Parsed M reviews` lines, a candidate count, a filter summary, and `Reviews: X new, Y total` with Y > 6.

- [ ] **Step 3: Validate the output file**

Run: `python3 -c "import json;d=json.load(open('data/reviews.json'));print(len(d),'reviews; first date',d[0]['date'])"`
Expected: a count, and a real ISO timestamp (not today's run time) as the first/newest date.

- [ ] **Step 4: Verify dedup on a second run**

Run: `go run scripts/fetch-reviews.go`
Expected: `Reviews: 0 new, Y total` (same Y) — nothing re-added.

- [ ] **Step 5: Commit the refreshed data**

```bash
git add data/reviews.json
git commit -m "Refresh Google reviews from Business Profile API"
```

- [ ] **Step 6: Verify in CI**

Push, then trigger the workflow manually (Actions → "Fetch Google Reviews" →
"Run workflow"). Confirm it succeeds and, if there are new reviews, commits them.

---

## Self-Review

- **Spec coverage:** single-source v4 fetch (Task 4) ✓; OAuth prerequisite documented (Task 7) ✓; 5-star/≥20-word filter kept (unchanged, confirmed Task 6 context) ✓; real timestamps + correct sort (Tasks 3, 6) ✓; corrupt-file guard, timeout, dedup/init hardening (Tasks 2, 5) ✓; workflow drops API key (Task 7) ✓; README updated incl. stale-shortcode + setup-script tree fixes (Task 7) ✓; template UTF-8 bug explicitly out of scope ✓; manual verification path (Task 8) ✓.
- **Placeholder scan:** none — every code step shows full before/after content.
- **Type consistency:** `Review`, `bpReview` (`UpdateTime`/`CreateTime`/`Comment`/`Reviewer.DisplayName`/`StarRating`/`Name`), `starRatingToInt`, `fetchBusinessProfileReviews`, `getAccessToken`, `fetchURL`, `httpClient`, `normalizeText` all referenced consistently with their existing/added definitions.
