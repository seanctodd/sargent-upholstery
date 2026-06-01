# Instagram Gallery via Official Graph API — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the failing build-time scrape of Instagram's internal web API with a weekly GitHub Actions job that fetches posts via the official Instagram Graph API, commits images + metadata into the repo, and have Hugo render the gallery from those local files.

**Architecture:** `scripts/fetch-instagram.go` (run weekly by `.github/workflows/instagram.yml`) refreshes the long-lived token, fetches `/me/media`, downloads images to `assets/instagram/`, writes `data/instagram.json`, prunes stale images, and commits. The `instagram-gallery.html` shortcode reads `data/instagram.json` + local images and renders the existing grid + lightbox — no live API call at Cloudflare build time.

**Tech Stack:** Go (standalone script, no module), Instagram Graph API (`graph.instagram.com`), Hugo Extended (image pipeline), GitHub Actions, `gh` CLI (for token rotation).

**Testing note:** No Go module/test framework (consistent with `fetch-reviews.go`). Per-task verification is `gofmt` + `go build` for Go, and a real **Hugo build** for the template (Task 4 installs the pinned Hugo and renders the page against seed data). End-to-end live verification needs real credentials and is the manual Task 5.

**Spec:** `docs/superpowers/specs/2026-06-01-instagram-gallery-graph-api-design.md`

---

## File Structure

- **Create:** `scripts/fetch-instagram.go` — fetch/refresh/download/prune/write (Task 1)
- **Modify:** `themes/sargent/layouts/shortcodes/instagram-gallery.html` — render from local data (Task 2)
- **Create:** `.github/workflows/instagram.yml` — weekly job (Task 3)
- **Modify:** `README.md` — document the new approach (Task 3)
- **Runtime, created by the job (not committed by these tasks):** `data/instagram.json`, `assets/instagram/<id>.jpg`

---

## Task 1: Create the fetch script

**Files:**
- Create: `scripts/fetch-instagram.go`

- [ ] **Step 1: Write the complete file**

Create `scripts/fetch-instagram.go` with exactly this content:

```go
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
```

- [ ] **Step 2: Compile check**

Run: `go build -o /tmp/fetch-instagram scripts/fetch-instagram.go`
Expected: no output, exit 0.

- [ ] **Step 3: No-token no-op check**

Run: `go run scripts/fetch-instagram.go` (with `IG_ACCESS_TOKEN` unset)
Expected: prints `IG_ACCESS_TOKEN not set, skipping Instagram fetch` and exits 0. Confirm it did NOT create `data/instagram.json`: `test ! -f data/instagram.json && echo "no file (correct)"`.

- [ ] **Step 4: gofmt**

Run: `gofmt -l scripts/`
Expected: empty.

- [ ] **Step 5: Commit**

```bash
git add scripts/fetch-instagram.go
git commit -m "Add Instagram Graph API fetch script"
```

---

## Task 2: Rewrite the gallery shortcode to render from local data

**Files:**
- Modify: `themes/sargent/layouts/shortcodes/instagram-gallery.html`

The shortcode currently scrapes `i.instagram.com` at build time. Replace the data
acquisition with `site.Data.instagram` + local images; keep the grid item markup,
the lightbox markup, and the `<script>` exactly as they are.

- [ ] **Step 1: Replace lines 1–54 (the fetch + `.ig-gallery` rendering block)**

Replace everything from the top of the file through the closing `</div>` of
`<div class="ig-gallery">` (the block that currently starts at `{{- $username := ...`
and ends at the `</div>` on the line before `<div class="ig-lightbox" id="ig-lightbox">`)
with:

```go-html-template
{{- $username := .Get "username" | default "sargentupholsteryco" -}}
{{- $count := .Get "count" | default 20 -}}
{{- $posts := site.Data.instagram -}}
<div class="ig-gallery">
{{- with $posts }}
  {{- range $i, $p := first (int $count) . }}
    {{- $img := resources.Get $p.image -}}
    {{- if $img }}
      {{- $thumb := $img.Resize "640x640 webp q85" -}}
      {{- $large := $img.Resize "1200x webp q90" -}}
      {{- $caption := $p.caption -}}
  <div class="ig-gallery-item" data-index="{{ $i }}" data-large="{{ $large.RelPermalink }}" data-caption="{{ $caption | htmlEscape }}" data-link="{{ $p.permalink }}">
    <img src="{{ $thumb.RelPermalink }}" alt="{{ with $caption }}{{ . }}{{ else }}{{ i18n "ig_alt_fallback" }} @{{ $username }}{{ end }}" loading="lazy" width="640" height="640">
  </div>
    {{- end }}
  {{- end }}
{{- else }}
  <p class="ig-gallery-error">{{ i18n "ig_error" }} <a href="https://www.instagram.com/{{ $username }}/" target="_blank" rel="noopener">@{{ $username }}</a> {{ i18n "ig_see_work" }}</p>
{{- end }}
</div>
```

Leave the rest of the file (the `<div class="ig-lightbox" id="ig-lightbox">` block
and the `<script>...</script>`) **unchanged**.

- [ ] **Step 2: Confirm no remaining reference to the old scrape**

Run: `grep -n "GetRemote\|i.instagram.com\|web_profile_info" themes/sargent/layouts/shortcodes/instagram-gallery.html`
Expected: no matches.

- [ ] **Step 3: Confirm the data + image references are present**

Run: `grep -n "site.Data.instagram\|resources.Get \$p.image" themes/sargent/layouts/shortcodes/instagram-gallery.html`
Expected: both lines match.

- [ ] **Step 4: Commit**

```bash
git add themes/sargent/layouts/shortcodes/instagram-gallery.html
git commit -m "Render Instagram gallery from local data instead of scraping"
```

(Template rendering is verified in Task 4 with a real Hugo build.)

---

## Task 3: Add the workflow and update the README

**Files:**
- Create: `.github/workflows/instagram.yml`
- Modify: `README.md`

- [ ] **Step 1: Create the workflow**

Create `.github/workflows/instagram.yml` with exactly:

```yaml
name: Fetch Instagram Posts

on:
  schedule:
    - cron: '0 10 * * 1'  # Mondays at 10:00 UTC (offset from the reviews job at 09:00)
  workflow_dispatch:

permissions:
  contents: write

defaults:
  run:
    shell: bash

jobs:
  fetch-instagram:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@v5
      - name: Fetch Instagram posts
        env:
          IG_ACCESS_TOKEN: ${{ secrets.IG_ACCESS_TOKEN }}
          GH_PAT: ${{ secrets.GH_PAT }}
        run: go run scripts/fetch-instagram.go
      - name: Commit updated gallery
        run: |
          git config user.name "github-actions[bot]"
          git config user.email "github-actions[bot]@users.noreply.github.com"
          git add assets/instagram data/instagram.json
          if ! git diff --cached --quiet; then
            git commit -m "Update Instagram gallery [automated]"
            git push
          fi
```

- [ ] **Step 2: Validate the YAML parses**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/instagram.yml')); print('yaml ok')"`
Expected: `yaml ok`.

- [ ] **Step 3: Update README Tech Stack bullet**

In `README.md`, change:
```markdown
- Instagram Web API integration (build-time image processing)
```
to:
```markdown
- Instagram Graph API (official; weekly fetch committed to the repo, then build-time image processing)
```

- [ ] **Step 4: Update the "Instagram Gallery Shortcode" notes**

In `README.md`, find the `**Features:**` / `**Note:**` block under "Instagram Gallery Shortcode" that currently reads:
```markdown
**Features:**
- Builds at **compile time** (fast static site, no runtime API calls)
- Automatic image optimization (resize to WebP format)
- Client-side lightbox with keyboard navigation (arrow keys, Escape)
- Lazy loading for performance
- Responsive grid layout

**Note:** Uses Instagram's internal web API. Instagram may change or block access; consider alternatives if fetch fails frequently.
```
and replace it with:
```markdown
**Features:**
- Posts are fetched **weekly by GitHub Actions** (`.github/workflows/instagram.yml`) via the official Instagram Graph API and committed to the repo (`data/instagram.json` + `assets/instagram/`)
- Hugo renders from those local files — **no API call at site-build time**, so the build never depends on Instagram being reachable
- Automatic image optimization (resize to WebP format)
- Client-side lightbox with keyboard navigation (arrow keys, Escape)
- Lazy loading, responsive grid layout

**Setup:** Requires a Meta app connected to the Business account and two secrets,
`IG_ACCESS_TOKEN` (long-lived token, auto-refreshed weekly) and `GH_PAT` (fine-grained
PAT with Secrets: write, so the workflow can rotate the token). See the design spec
`docs/superpowers/specs/2026-06-01-instagram-gallery-graph-api-design.md`.
```

- [ ] **Step 5: Update the README project-structure tree**

In the `scripts/` block of the project structure, change:
```
├── scripts/
│   ├── fetch-reviews.go        # Google Reviews fetch script (Business Profile v4)
│   └── setup-business-profile.go  # One-time OAuth setup → prints CI secrets
```
to:
```
├── scripts/
│   ├── fetch-reviews.go        # Google Reviews fetch script (Business Profile v4)
│   ├── fetch-instagram.go      # Instagram Graph API fetch script
│   └── setup-business-profile.go  # One-time OAuth setup → prints CI secrets
```
and in the `data/` block, change:
```
├── data/
│   └── reviews.json            # Google Reviews (fetched at build time)
```
to:
```
├── data/
│   ├── reviews.json            # Google Reviews (fetched by CI)
│   └── instagram.json          # Instagram posts metadata (fetched by CI)
```

- [ ] **Step 6: Commit**

```bash
git add .github/workflows/instagram.yml README.md
git commit -m "Add Instagram fetch workflow; document Graph API gallery setup"
```

---

## Task 4: Verify the shortcode with a real Hugo build

**Files:** none committed (verification only; seed artifacts are created then removed)

The agent environment has no Hugo. Install the version Cloudflare uses (0.147.8,
per the README) and render the gallery page against seed data to prove the
rewritten template compiles and emits the grid. Then verify the empty-data
fallback. Clean up so the working tree is unchanged.

- [ ] **Step 1: Install pinned Hugo Extended**

```bash
cd /tmp
curl -sL -o hugo.tar.gz https://github.com/gohugoio/hugo/releases/download/v0.147.8/hugo_extended_0.147.8_linux-amd64.tar.gz
tar xzf hugo.tar.gz hugo
/tmp/hugo version
```
Expected: prints `hugo v0.147.8 ... extended`.

- [ ] **Step 2: Create seed data + a seed image (reusing an existing repo photo)**

```bash
cd /home/possum/Projects/sargent-upholstery
mkdir -p assets/instagram
cp assets/images/old/2021_03_09_12_19_29.jpg assets/instagram/seedtest.jpg
cat > data/instagram.json <<'JSON'
[
  {
    "id": "seedtest",
    "caption": "Seed caption for build verification",
    "permalink": "https://www.instagram.com/p/SEED/",
    "timestamp": "2026-05-30T12:00:00+0000",
    "image": "instagram/seedtest.jpg",
    "mediaType": "IMAGE"
  }
]
JSON
```

- [ ] **Step 3: Build and assert the gallery rendered**

```bash
/tmp/hugo --minify --quiet
grep -c 'ig-gallery-item' public/gallery/index.html
grep -o 'instagram/[^"]*\.webp' public/gallery/index.html | head -2
```
Expected: the first `grep -c` prints `1` (one gallery item), and the second prints
at least one processed `.webp` path (the resized thumbnail). No Hugo errors.

- [ ] **Step 4: Verify the empty-data fallback still builds**

```bash
rm data/instagram.json
/tmp/hugo --minify --quiet
grep -c 'ig-gallery-error' public/gallery/index.html
```
Expected: prints `1` (the fallback message renders) and the build exits 0.

- [ ] **Step 5: Clean up all verification artifacts**

```bash
rm -f assets/instagram/seedtest.jpg
rm -rf public resources
rmdir assets/instagram 2>/dev/null || true
git status --porcelain
```
Expected: `git status --porcelain` prints NOTHING (working tree clean — no seed
files, no build output left, `data/instagram.json` was not committed). If anything
shows, remove it before proceeding.

- [ ] **Step 6: No commit**

This task makes no commit. If Steps 3 and 4 passed and Step 5 shows a clean tree,
the template is verified.

---

## Task 5: One-time credential setup + go-live (manual, requires Meta access)

**Files:** none (operator-run; needs the Instagram/Meta account and repo admin)

This cannot be done by an agent — it needs the Business account login and GitHub
repo admin. The operator runs it.

- [ ] **Step 1: Create and configure the Meta app**

In developers.facebook.com → Create App (Business type) → add the **Instagram**
product → under **Instagram → API setup with Instagram login**, connect
@sargentupholsteryco (Business). The app may stay in **Development** mode for
accessing its own media (no App Review needed). Note the app's Instagram app ID
and app secret.

- [ ] **Step 2: Mint a long-lived token**

In the API setup panel, generate an access token for the connected account
(grant `instagram_business_basic`). It is short-lived; exchange it for a 60-day
token with one call (replace `APP_SECRET` and `SHORT_TOKEN`):
```bash
curl -s "https://graph.instagram.com/access_token?grant_type=ig_exchange_token&client_secret=APP_SECRET&access_token=SHORT_TOKEN"
```
Copy the `access_token` from the JSON response — this is `IG_ACCESS_TOKEN`.

- [ ] **Step 3: Create the fine-grained PAT**

github.com → Settings → Developer settings → Fine-grained tokens → Generate.
Scope it to **only** the `seanctodd/sargent-upholstery` repo, with repository
permission **Secrets: Read and write**. Copy the token — this is `GH_PAT`.

- [ ] **Step 4: Add both secrets**

github.com/seanctodd/sargent-upholstery → Settings → Secrets and variables →
Actions → New repository secret: add `IG_ACCESS_TOKEN` and `GH_PAT`.

- [ ] **Step 5: Local end-to-end test (optional but recommended)**

```bash
export IG_ACCESS_TOKEN=...   # the long-lived token
go run scripts/fetch-instagram.go
ls assets/instagram/ | head
python3 -c "import json;d=json.load(open('data/instagram.json'));print(len(d),'posts; first:',d[0]['permalink'])"
```
Expected: images downloaded, `data/instagram.json` has up to 20 posts. (Token
rotation is skipped locally unless `GH_PAT` is also exported — that's fine.)
If you ran this, commit the result: `git add assets/instagram data/instagram.json && git commit -m "Seed Instagram gallery" && git push`.

- [ ] **Step 6: Trigger the workflow**

GitHub → Actions → "Fetch Instagram Posts" → Run workflow. Confirm: the fetch step
prints `Token refreshed` and `IG_ACCESS_TOKEN secret updated`, posts are committed,
and the Cloudflare rebuild shows photos on `/gallery/`.

---

## Self-Review

- **Spec coverage:** single-source Graph API fetch (Task 1) ✓; fetch-and-commit / no build-time API call (Tasks 1+2) ✓; commit image bytes + prune (Task 1) ✓; automated token refresh + `gh secret set` via `GH_PAT` (Task 1, workflow Task 3) ✓; render from `data/instagram.json` keeping grid + lightbox + i18n keys (Task 2) ✓; resilient fallback when data missing (Task 2, verified Task 4 Step 4) ✓; separate `instagram.yml` workflow (Task 3) ✓; README updated (Task 3) ✓; default 20 posts (`mediaLimit`, `count="20"`) ✓; prerequisites/setup (Task 5) ✓; verification = gofmt+build+real Hugo render (Tasks 1,4) ✓. The optional `setup-instagram.go` helper from the spec is intentionally replaced by the one-line `curl` exchange in Task 5 Step 2 (YAGNI) — noted here as a deliberate deviation.
- **Placeholder scan:** none — every code/step shows full content; the only `...` are operator-supplied secret values in Task 5 commands, which is correct.
- **Type consistency:** `Post` JSON keys (`image`, `caption`, `permalink`, `timestamp`, `mediaType`, `id`) match the shortcode's `$p.image`, `$p.caption`, `$p.permalink`. `mediaItem` JSON tags match the Graph API field set requested in `fetchMedia`. `httpClient`, `refreshToken`, `persistToken`, `fetchMedia`, `downloadFile` are all defined in Task 1 and referenced consistently. The image path written (`instagram/<id>.jpg`) is what `resources.Get` resolves under `assets/`.
