# Instagram Reels Playback via Cloudflare R2 — Design

**Date:** 2026-06-01
**Status:** Approved (pending spec review)

## Problem

The Instagram gallery shows poster thumbnails for every post, including Reels
(~65% of the feed). Videos don't play on the site — clicking a Reel just shows
its still image. We want Reels to actually play in the gallery lightbox.

Constraints (from the existing site's design): keep **no third-party JavaScript**
and the Lighthouse performance profile; keep the fetch scripts **stdlib-only Go**
(no `go.mod`/SDK); and **do not bloat git** with video binaries. Instagram CDN
video URLs are signed and expire, so URLs can't simply be referenced later — the
MP4 bytes must be hosted somewhere stable.

## Decision summary (from research, high confidence)

- **Host on Cloudflare R2**, not Stream. R2 serves a plain MP4 into a native
  `<video>` element with no third-party JS/iframe, is **free at this scale**
  (~52 MB vs R2's 10 GB free storage; egress always free), and uploads fit the
  stdlib-only Go constraint. Stream would require an iframe/hls.js player
  (third-party JS) and bills per stored + delivered minute.
- **Upload from CI with the AWS CLI** (preinstalled on `ubuntu-latest`) against
  R2's S3-compatible endpoint, using `aws s3 sync --delete` (uploads new files
  and prunes dropped ones in one command). The Go script stays stdlib-only and
  only downloads MP4s locally; no SigV4/SDK code.
- **Serve via custom domain `media.sargentupholstery.com`** connected to the
  bucket — stable URLs, CDN caching, no rate limits, correct HTTP Range support
  for scrubbing. (R2's `r2.dev` URL is dev-only/rate-limited.)
- **Skip ffmpeg `+faststart`** for v1 (YAGNI — click-to-open tolerates a brief
  load). Can add later if first-frame latency is noticeable.

## Goals

- Reels play inline in the lightbox via a native `<video>` (no third-party JS).
- Video files live in R2, never in git; grid load performance is unchanged.
- Weekly job keeps R2 in sync with the current feed (uploads new, prunes old).
- A failed video upload/download degrades gracefully to the poster image.

## Non-goals

- Autoplaying videos in the grid (would tank performance/bandwidth).
- Adaptive bitrate / transcoding (unnecessary for short clips).
- ffmpeg re-muxing (deferred).
- Changing image-post behavior (unchanged).

## Prerequisite: R2 setup (one-time, operator — guided)

1. Create R2 bucket **`sargent-media`**.
2. Create an R2 **API token** (Object Read & Write, scoped to the bucket) →
   yields **Access Key ID**, **Secret Access Key**, and the **account ID**.
3. Connect custom domain **`media.sargentupholstery.com`** to the bucket
   (R2 → bucket → Settings → Custom Domains). The domain is already a Cloudflare
   zone, so this is a proxied CNAME, active in minutes.
4. Add a Cache/Compression rule to **disable HTTP compression for `video/mp4`**
   on that hostname, so byte-range requests return 206 and the scrubber works.
5. GitHub Actions repository secrets:
   - `AWS_ACCESS_KEY_ID` = R2 token Access Key ID
   - `AWS_SECRET_ACCESS_KEY` = R2 token Secret Access Key
   - `R2_ACCOUNT_ID` = Cloudflare account ID (used to build the S3 endpoint)

   (Bucket name `sargent-media`, region `auto`, and base URL
   `https://media.sargentupholstery.com` are non-secret and live in code/workflow.)

## Architecture

```
Weekly job (.github/workflows/instagram.yml):

 1. go run scripts/fetch-instagram.go
      posters (committed):  IMAGE → media_url ;  VIDEO → thumbnail_url
                            → assets/instagram/<id>.jpg   (Hugo-processed, as today)
      reels (NOT committed): for VIDEO posts, download media_url (mp4)
                            → reels/<id>.mp4   (reels/ recreated fresh each run)
      manifest: data/instagram.json entries get, for VIDEO posts:
                "video": "https://media.sargentupholstery.com/reels/<id>.mp4"

 2. aws s3 sync ./reels s3://sargent-media/reels/ --delete \
      --endpoint-url https://${R2_ACCOUNT_ID}.r2.cloudflarestorage.com \
      --content-type video/mp4 \
      --cache-control "public, max-age=604800, immutable"
      (env: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_DEFAULT_REGION=auto)
      --delete prunes R2 objects for posts that dropped out of the feed.

 3. commit assets/instagram + data/instagram.json   (reels/ is .gitignored)

Hugo build (Cloudflare) — instagram-gallery.html:
   grid: poster <img> + ▶ play-badge overlay on posts that have a video
   lightbox: if item has video → <video controls preload="none" poster=<large.webp>
             src=<r2 url> playsinline> ; else <img> (as today)
```

### Manifest schema (`data/instagram.json`)

Add an optional `video` field (present only for VIDEO posts whose MP4 uploaded):

```json
{
  "id": "1810...", "caption": "...", "permalink": "https://...",
  "timestamp": "2026-05-28T...", "image": "instagram/1810....jpg",
  "mediaType": "VIDEO",
  "video": "https://media.sargentupholstery.com/reels/1810....mp4"
}
```

Image posts (and any video whose MP4 download failed) simply omit `video`.

### Components

**`scripts/fetch-instagram.go`** (modify)
- Add `const mediaBaseURL = "https://media.sargentupholstery.com"`.
- Recreate a local `reels/` dir fresh each run (`os.RemoveAll` then `MkdirAll`) so
  it mirrors exactly the current feed (makes `aws s3 sync --delete` correct even
  on repeated local runs).
- For each VIDEO post: after saving the poster, also download `media_url` (the MP4)
  to `reels/<id>.mp4` (reuse the existing atomic `downloadFile`). On success, set
  the post's `Video` field to `mediaBaseURL + "/reels/" + id + ".mp4"`. On failure,
  log and leave `Video` empty (post still renders as its poster).
- `Post` struct gains `Video string `json:"video,omitempty"``.
- Image posts and posters are unchanged.

**`themes/sargent/layouts/shortcodes/instagram-gallery.html`** (modify)
- Grid item: when `$p.video` is non-empty, add a `is-video` class / ▶ badge element
  and a `data-video="{{ $p.video }}"` attribute. Poster `<img>` is unchanged.
- Lightbox JS: on open, if the item has `data-video`, build a
  `<video controls preload="none" playsinline poster=<data-large> src=<data-video>>`
  in place of the `<img>`; on close/navigate, pause and clear it. Keep the existing
  `<img>` path for image posts.
- CSS: a ▶ badge overlay; lightbox sizing that accommodates portrait (9:16) video.

**`.github/workflows/instagram.yml`** (modify)
- After the fetch step, add the `aws s3 sync` step (env from the 3 secrets,
  `AWS_DEFAULT_REGION: auto`, endpoint built from `R2_ACCOUNT_ID`).
- The commit step is unchanged (commits posters + manifest; `reels/` is ignored).

**`.gitignore`** (modify) — add `reels/`.

## Error handling

| Condition | Behavior |
|---|---|
| MP4 download fails for a post | Log to stderr; leave `video` empty; post still renders as its poster image. Non-fatal. |
| No videos in feed | Fine — `reels/` is empty; `aws s3 sync --delete` empties the R2 prefix; gallery is all images. |
| `aws s3 sync` step fails (bad creds/network) | The workflow step fails (surfaces the problem); posters + manifest from the fetch step are NOT committed because the commit step runs after — so a failed sync doesn't publish video URLs pointing at missing objects. |
| Fetch step itself fails | As today (`os.Exit(1)`), nothing committed. |

Ordering guarantee: **sync must run before the commit step**, so manifest video
URLs are only committed once the corresponding objects are uploaded.

## Testing / verification

No Go module; verification is `gofmt` + `go build`, plus a real Hugo build.

1. `gofmt -l scripts/` clean; `go build` compiles.
2. Local run with a real `IG_ACCESS_TOKEN`: confirms `reels/<id>.mp4` files are
   downloaded and `data/instagram.json` video entries get the
   `media.sargentupholstery.com/reels/...` URL.
3. Hugo build (pinned 0.147.8) against a seed manifest containing one `video`
   entry: grid shows the ▶ badge; lightbox markup produces a `<video>` with the
   correct `src`/`poster`; an image-only entry still renders `<img>`. Fallback
   (no manifest) still builds.
4. `aws s3 sync` is verified at go-live with real R2 credentials (operator):
   objects appear in the bucket, `media.sargentupholstery.com/reels/<id>.mp4`
   returns the video with `Accept-Ranges: bytes` / 206 on range requests.
5. Live check: a Reel plays in the lightbox on the deployed site.

## Rollout

Code merges first (gallery keeps working — video URLs only appear once R2 is
populated). Then operator does the R2 setup + secrets, and the next workflow run
(or a manual local seed) uploads the MP4s and commits the manifest with video URLs.
