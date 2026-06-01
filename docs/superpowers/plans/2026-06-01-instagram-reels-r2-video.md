# Instagram Reels Playback via Cloudflare R2 — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Instagram Reels play in the gallery lightbox via native `<video>`, with the MP4s hosted on Cloudflare R2 (out of git) and uploaded by the weekly CI job.

**Architecture:** `fetch-instagram.go` additionally downloads each video post's MP4 into a gitignored `reels/` dir and records an R2 URL in the manifest; a CI step `aws s3 sync`s `reels/` to R2 (with `--delete` pruning) before the commit step; the shortcode renders a ▶ badge on video tiles and swaps the lightbox `<img>` for a native `<video>` when a post has a video. No third-party JS; videos never enter git.

**Tech Stack:** Go (stdlib-only script), Instagram Graph API, Cloudflare R2 (S3 API via preinstalled AWS CLI), Hugo Extended, GitHub Actions.

**Testing note:** No Go module/test framework; verification is `gofmt` + `go build` for Go and a real **Hugo build** for the template (Task 4 renders a seed manifest containing a video entry). The R2 upload + live playback are verified at go-live with real credentials (Task 5).

**Spec:** `docs/superpowers/specs/2026-06-01-instagram-reels-r2-video-design.md`

---

## File Structure

- **Modify:** `scripts/fetch-instagram.go` — download MP4s to `reels/`, add `video` field (Task 1)
- **Modify:** `themes/sargent/layouts/shortcodes/instagram-gallery.html` — ▶ badge + `<video>` lightbox (Task 2)
- **Modify:** `assets/css/style.css` — play badge + lightbox video styles (Task 2)
- **Modify:** `.github/workflows/instagram.yml` — `aws s3 sync` step (Task 3)
- **Modify:** `.gitignore` — ignore `reels/` (Task 3)

---

## Task 1: Download video MP4s and record R2 URLs

**Files:**
- Modify: `scripts/fetch-instagram.go`

- [ ] **Step 1: Add the `Video` field to the `Post` struct**

Replace:
```go
type Post struct {
	ID        string `json:"id"`
	Caption   string `json:"caption"`
	Permalink string `json:"permalink"`
	Timestamp string `json:"timestamp"`
	Image     string `json:"image"`
	MediaType string `json:"mediaType"`
}
```
with:
```go
type Post struct {
	ID        string `json:"id"`
	Caption   string `json:"caption"`
	Permalink string `json:"permalink"`
	Timestamp string `json:"timestamp"`
	Image     string `json:"image"`
	MediaType string `json:"mediaType"`
	Video     string `json:"video,omitempty"`
}
```

- [ ] **Step 2: Add the media base URL constant**

Replace:
```go
const (
	mediaLimit = 20
	graphBase  = "https://graph.instagram.com"
)
```
with:
```go
const (
	mediaLimit   = 20
	graphBase    = "https://graph.instagram.com"
	mediaBaseURL = "https://media.sargentupholstery.com"
)
```

- [ ] **Step 3: Recreate a fresh `reels/` dir each run (after the imgDir block in `main`)**

Find:
```go
	imgDir := filepath.Join("assets", "instagram")
	if err := os.MkdirAll(imgDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir error: %v\n", err)
		os.Exit(1)
	}
```
and insert immediately AFTER it:
```go
	// reels/ holds the MP4s this run; recreate it fresh so it mirrors the current
	// feed exactly (makes the CI `aws s3 sync --delete` correct). It is .gitignored.
	reelsDir := "reels"
	if err := os.RemoveAll(reelsDir); err != nil {
		fmt.Fprintf(os.Stderr, "could not clear %s: %v\n", reelsDir, err)
		os.Exit(1)
	}
	if err := os.MkdirAll(reelsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir %s error: %v\n", reelsDir, err)
		os.Exit(1)
	}
```

- [ ] **Step 4: Download the MP4 for video posts and set the `Video` URL**

Replace:
```go
		keep[fileName] = true
		posts = append(posts, Post{
			ID:        it.ID,
			Caption:   it.Caption,
			Permalink: it.Permalink,
			Timestamp: it.Timestamp,
			Image:     rel,
			MediaType: it.MediaType,
		})
```
with:
```go
		keep[fileName] = true
		post := Post{
			ID:        it.ID,
			Caption:   it.Caption,
			Permalink: it.Permalink,
			Timestamp: it.Timestamp,
			Image:     rel,
			MediaType: it.MediaType,
		}
		// For video posts, also download the MP4 locally; CI uploads reels/ to R2.
		// On failure the post still renders as its poster image (Video stays empty).
		if it.MediaType == "VIDEO" && it.MediaURL != "" {
			mp4 := filepath.Join(reelsDir, it.ID+".mp4")
			if err := downloadFile(it.MediaURL, mp4); err != nil {
				fmt.Fprintf(os.Stderr, "warning %s: video download failed, showing poster only: %v\n", it.ID, err)
			} else {
				post.Video = mediaBaseURL + "/reels/" + it.ID + ".mp4"
			}
		}
		posts = append(posts, post)
```

- [ ] **Step 5: Compile + format checks**

Run: `go build -o /tmp/fetch-instagram scripts/fetch-instagram.go` (expect exit 0)
Run: `gofmt -l scripts/` (expect empty)

- [ ] **Step 6: No-token no-op still works**

Run (IG_ACCESS_TOKEN unset): `go run scripts/fetch-instagram.go`
Expected: prints the skip message, exits 0, creates neither `data/instagram.json` nor `reels/`.
Confirm: `test ! -d reels && test ! -f data/instagram.json && echo ok`

- [ ] **Step 7: Commit**

```bash
git add scripts/fetch-instagram.go
git commit -m "fetch-instagram: download Reels MP4s to reels/ and record R2 URLs"
```

---

## Task 2: Render the play badge and lightbox video

**Files:**
- Modify: `themes/sargent/layouts/shortcodes/instagram-gallery.html`
- Modify: `assets/css/style.css`

- [ ] **Step 1: Grid item — add the video class, `data-video`, and ▶ badge**

In `instagram-gallery.html`, replace:
```go-html-template
  <div class="ig-gallery-item" data-index="{{ $i }}" data-large="{{ $large.RelPermalink }}" data-caption="{{ $caption | htmlEscape }}" data-link="{{ $p.permalink }}">
    <img src="{{ $thumb.RelPermalink }}" alt="{{ with $caption }}{{ . }}{{ else }}{{ i18n "ig_alt_fallback" }} @{{ $username }}{{ end }}" loading="lazy" width="640" height="640">
  </div>
```
with:
```go-html-template
  <div class="ig-gallery-item{{ with $p.video }} is-video{{ end }}" data-index="{{ $i }}" data-large="{{ $large.RelPermalink }}" data-caption="{{ $caption | htmlEscape }}" data-link="{{ $p.permalink }}"{{ with $p.video }} data-video="{{ . }}"{{ end }}>
    <img src="{{ $thumb.RelPermalink }}" alt="{{ with $caption }}{{ . }}{{ else }}{{ i18n "ig_alt_fallback" }} @{{ $username }}{{ end }}" loading="lazy" width="640" height="640">
    {{- with $p.video }}<span class="ig-play-badge" aria-hidden="true"></span>{{ end }}
  </div>
```

- [ ] **Step 2: Lightbox markup — add the `<video>` element**

Replace:
```html
    <img class="ig-lightbox-img" src="" alt="">
```
with:
```html
    <img class="ig-lightbox-img" src="" alt="">
    <video class="ig-lightbox-video" controls preload="none" playsinline style="display:none"></video>
```

- [ ] **Step 3: JS — add the video reference**

Replace:
```javascript
  var lbImg = lightbox.querySelector('.ig-lightbox-img');
```
with:
```javascript
  var lbImg = lightbox.querySelector('.ig-lightbox-img');
  var lbVideo = lightbox.querySelector('.ig-lightbox-video');
```

- [ ] **Step 4: JS — replace `show()` to handle video vs image**

Replace the entire existing `function show(index) { ... }` block with:
```javascript
  function show(index) {
    if (index < 0) index = items.length - 1;
    if (index >= items.length) index = 0;
    current = index;
    var item = items[index];
    var video = item.getAttribute('data-video');
    lbVideo.pause();
    if (video) {
      lbImg.style.display = 'none';
      lbVideo.poster = item.getAttribute('data-large');
      lbVideo.src = video;
      lbVideo.style.display = '';
    } else {
      lbVideo.removeAttribute('src');
      lbVideo.load();
      lbVideo.style.display = 'none';
      lbImg.src = item.getAttribute('data-large');
      lbImg.alt = item.getAttribute('data-caption') || '{{ i18n "ig_post_alt" }}';
      lbImg.style.display = '';
    }
    var caption = item.getAttribute('data-caption');
    var link = item.getAttribute('data-link');
    lbCaption.textContent = caption || '{{ i18n "ig_view" }}';
    lbCaption.href = link;
    lbCaptionWrap.style.display = '';
    lightbox.classList.add('active');
    document.body.style.overflow = 'hidden';
    closeBtn.focus();
  }
```

- [ ] **Step 5: JS — pause video on close**

Replace the entire existing `function close() { ... }` block with:
```javascript
  function close() {
    lbVideo.pause();
    lightbox.classList.remove('active');
    document.body.style.overflow = '';
    if (triggerEl) {
      triggerEl.focus();
      triggerEl = null;
    }
  }
```

- [ ] **Step 6: CSS — play badge (in `assets/css/style.css`)**

Find:
```css
.ig-gallery-item:hover img {
  transform: scale(1.05);
  opacity: 0.85;
}
```
and insert immediately AFTER it:
```css

.ig-play-badge {
  position: absolute;
  top: 50%;
  left: 50%;
  transform: translate(-50%, -50%);
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: rgba(0, 0, 0, 0.55);
  pointer-events: none;
}

.ig-play-badge::after {
  content: "";
  position: absolute;
  top: 50%;
  left: 54%;
  transform: translate(-50%, -50%);
  border-style: solid;
  border-width: 9px 0 9px 15px;
  border-color: transparent transparent transparent #fff;
}
```

- [ ] **Step 7: CSS — lightbox video**

Find:
```css
.ig-lightbox-img {
  max-width: 100%;
  max-height: 75vh;
  object-fit: contain;
  border-radius: 4px;
}
```
and insert immediately AFTER it:
```css

.ig-lightbox-video {
  max-width: 100%;
  max-height: 80vh;
  border-radius: 4px;
  background: #000;
}
```

- [ ] **Step 8: Sanity greps + commit**

Run: `grep -c 'ig-lightbox-video' themes/sargent/layouts/shortcodes/instagram-gallery.html` (expect 2: the markup + the JS var)
Run: `grep -c 'ig-play-badge' assets/css/style.css` (expect 2: the rule + the `::after`)
```bash
git add themes/sargent/layouts/shortcodes/instagram-gallery.html assets/css/style.css
git commit -m "gallery: play badge on video tiles, native <video> in lightbox"
```
(Rendering is verified for real in Task 4.)

---

## Task 3: CI upload step and gitignore

**Files:**
- Modify: `.github/workflows/instagram.yml`
- Modify: `.gitignore`

- [ ] **Step 1: Ignore `reels/`**

Append a line to `.gitignore`:
```
reels/
```
Verify: `mkdir -p reels && touch reels/x.mp4 && git check-ignore reels/x.mp4 && rm -rf reels` → prints `reels/x.mp4`.

- [ ] **Step 2: Add the R2 upload step to the workflow**

In `.github/workflows/instagram.yml`, find:
```yaml
      - name: Fetch Instagram posts
        env:
          IG_ACCESS_TOKEN: ${{ secrets.IG_ACCESS_TOKEN }}
          GH_PAT: ${{ secrets.GH_PAT }}
        run: go run scripts/fetch-instagram.go
```
and insert this NEW step immediately AFTER it (before "Commit updated gallery"):
```yaml
      - name: Upload Reels to R2
        env:
          AWS_ACCESS_KEY_ID: ${{ secrets.AWS_ACCESS_KEY_ID }}
          AWS_SECRET_ACCESS_KEY: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          AWS_DEFAULT_REGION: auto
        run: |
          if [ ! -d reels ]; then
            echo "No reels/ directory (fetch skipped); nothing to upload."
            exit 0
          fi
          aws s3 sync reels "s3://sargent-media/reels/" \
            --endpoint-url "https://${{ secrets.R2_ACCOUNT_ID }}.r2.cloudflarestorage.com" \
            --content-type video/mp4 \
            --cache-control "public, max-age=604800, immutable" \
            --delete
```
Leave "Commit updated gallery" unchanged. It must run AFTER this step so manifest video URLs are only committed once the MP4s are in R2; if the sync fails (e.g. missing creds), the job fails here and nothing is committed.

- [ ] **Step 3: Validate YAML + commit**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/instagram.yml')); print('yaml ok')"` (expect `yaml ok`)
```bash
git add .github/workflows/instagram.yml .gitignore
git commit -m "ci: sync Reels to R2 before committing; gitignore reels/"
```

---

## Task 4: Verify rendering with a real Hugo build

**Files:** none committed (verification only; seed artifacts created then removed)

- [ ] **Step 1: Ensure pinned Hugo is available**

```bash
if [ ! -x /tmp/hugo ]; then cd /tmp && curl -sL -o hugo.tar.gz https://github.com/gohugoio/hugo/releases/download/v0.147.8/hugo_extended_0.147.8_linux-amd64.tar.gz && tar xzf hugo.tar.gz hugo; fi
/tmp/hugo version
```
Expected: `hugo v0.147.8 ... extended`.

- [ ] **Step 2: Seed a manifest with one VIDEO and one IMAGE entry + a poster image**

```bash
cd /home/possum/Projects/sargent-upholstery
mkdir -p assets/instagram
cp assets/images/old/2021_03_09_12_19_29.jpg assets/instagram/seedtest.jpg
cat > data/instagram.json <<'JSON'
[
  {"id":"seedvid","caption":"Seed video","permalink":"https://www.instagram.com/p/SEEDV/","timestamp":"2026-05-28T12:00:00+0000","image":"instagram/seedtest.jpg","mediaType":"VIDEO","video":"https://media.sargentupholstery.com/reels/seedvid.mp4"},
  {"id":"seedimg","caption":"Seed image","permalink":"https://www.instagram.com/p/SEEDI/","timestamp":"2026-05-27T12:00:00+0000","image":"instagram/seedtest.jpg","mediaType":"IMAGE"}
]
JSON
```

- [ ] **Step 3: Build and assert video + image render correctly**

```bash
/tmp/hugo --minify --quiet && echo "build OK"
echo "gallery items: $(grep -o 'data-index=' public/gallery/index.html | wc -l)"          # expect 2
echo "play badges:   $(grep -o 'ig-play-badge' public/gallery/index.html | wc -l)"        # expect 1 (video only)
echo "data-video on video tile: $(grep -c 'data-video=\"https://media.sargentupholstery.com/reels/seedvid.mp4\"' public/gallery/index.html)"  # expect 1
echo "lightbox <video> present: $(grep -c 'class=\"ig-lightbox-video\"' public/gallery/index.html)"  # expect 1
echo "fallback error: $(grep -c 'ig-gallery-error' public/gallery/index.html)"            # expect 0
```
Expected: build OK; 2 items; exactly 1 play badge and 1 `data-video` (the video tile); the image tile has neither; the lightbox `<video>` element is present; no fallback.

- [ ] **Step 4: Clean up verification artifacts**

```bash
rm -f assets/instagram/seedtest.jpg data/instagram.json
rm -rf public resources
rmdir assets/instagram 2>/dev/null || true
git status --porcelain
```
Expected: `git status --porcelain` prints NOTHING (clean tree; seed manifest/image not committed). If anything shows, remove it.

- [ ] **Step 5: No commit** — verification only.

---

## Task 5: One-time R2 setup + go-live (manual, requires Cloudflare + repo admin)

**Files:** none (operator-run).

- [ ] **Step 1: Create the R2 bucket** named `sargent-media` (Cloudflare dashboard → R2).

- [ ] **Step 2: Create an R2 API token** (R2 → Manage R2 API Tokens → Create) with **Object Read & Write** scoped to `sargent-media`. Note the **Access Key ID**, **Secret Access Key**, and your **account ID**.

- [ ] **Step 3: Connect the custom domain** `media.sargentupholstery.com` to the bucket (R2 → bucket → Settings → Custom Domains → Add). Wait for status **Active**.

- [ ] **Step 4: Disable compression for video** so Range/seeking works: add a Cloudflare **Compression Rule** (or Transform/Cache rule) on `media.sargentupholstery.com` setting compression off for `Media Type contains video/mp4` (or matching `*.mp4`).

- [ ] **Step 5: Add GitHub Actions secrets** (Settings → Secrets and variables → Actions):
  - `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` (the R2 token), `R2_ACCOUNT_ID` (account ID).

- [ ] **Step 6: Seed now (optional, fastest) or run the workflow.**

  Local seed (with `.env` loaded and AWS creds exported):
  ```bash
  cd /home/possum/Projects/sargent-upholstery
  set -a; source .env; set +a
  export AWS_ACCESS_KEY_ID=... AWS_SECRET_ACCESS_KEY=... AWS_DEFAULT_REGION=auto
  go run scripts/fetch-instagram.go
  aws s3 sync reels "s3://sargent-media/reels/" \
    --endpoint-url "https://<ACCOUNT_ID>.r2.cloudflarestorage.com" \
    --content-type video/mp4 --cache-control "public, max-age=604800, immutable" --delete
  git add assets/instagram data/instagram.json && git commit -m "Seed Reels video gallery" && git push
  ```
  Or just trigger **Actions → "Fetch Instagram Posts" → Run workflow**.

- [ ] **Step 7: Verify live.** Confirm `https://media.sargentupholstery.com/reels/<id>.mp4` returns the video (and `curl -sI -H 'Range: bytes=0-1' <url>` returns `206`), and a Reel plays in the lightbox on `https://sargentupholstery.com/gallery/`.

---

## Self-Review

- **Spec coverage:** R2 host + native `<video>` no-JS (Task 2) ✓; MP4 download to gitignored `reels/` + manifest `video` URL (Task 1, Task 3 gitignore) ✓; `aws s3 sync --delete` before commit, ordering guarantee (Task 3) ✓; custom domain baked into `mediaBaseURL` (Task 1) ✓; play badge + portrait-friendly lightbox video CSS (Task 2) ✓; graceful degradation on video download failure (Task 1 Step 4) ✓; skip ffmpeg (not present anywhere) ✓; secrets `AWS_ACCESS_KEY_ID`/`AWS_SECRET_ACCESS_KEY`/`R2_ACCOUNT_ID` (Task 3, Task 5) ✓; setup incl. compression rule (Task 5 Step 4) ✓; verification gofmt+build+real Hugo render of a video entry (Tasks 1, 4) ✓.
- **Placeholder scan:** none — all code/steps are complete; `...` only appears in Task 5 operator commands (real secret values / account ID the operator supplies), which is correct.
- **Type consistency:** `Post.Video` (`json:"video,omitempty"`) is written by Task 1 and read by the shortcode as `$p.video`/`data-video` (Task 2) and rendered into `lbVideo.src` (Task 2 JS); `mediaBaseURL` + `/reels/<id>.mp4` matches the object key `reels/<id>.mp4` uploaded by `aws s3 sync reels s3://sargent-media/reels/` (Task 3) served at `media.sargentupholstery.com/reels/<id>.mp4` (Task 5). Consistent end to end.
