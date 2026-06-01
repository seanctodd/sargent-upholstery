# Instagram Gallery via Official Graph API — Design

**Date:** 2026-06-01
**Status:** Approved (pending spec review)

## Problem

The homepage/gallery Instagram feed is blank — it shows the "Could not load
Instagram posts" fallback with zero thumbnails on the live site.

Root cause (verified 2026-06-01):

- `themes/sargent/layouts/shortcodes/instagram-gallery.html` scrapes Instagram's
  **internal** web API (`i.instagram.com/api/v1/users/web_profile_info`) at Hugo
  build time via `resources.GetRemote`.
- That endpoint serves residential IPs (a local `curl` with the shortcode's exact
  headers returns **HTTP 200, 12 posts**) but is **blocked from datacenter/cloud
  IP ranges** — which is where the site builds (Cloudflare Pages).
- So the fetch errors, `try` swallows it, and the `{{ else }}` branch renders the
  fallback. Confirmed: live `/gallery/` shows the fallback, 0 thumbnails.

This is architectural, not a header tweak — Instagram actively blocks datacenter
IPs and rewrites this private endpoint. The fix is to use the **official
Instagram Graph API** with an authenticated token, and to decouple the fetch from
the Cloudflare build.

## Goals

- Display the account's recent Instagram posts using the official, supported API.
- Keep the existing grid + lightbox UI, the build-time image processing, and the
  no-third-party-JS / performance profile unchanged.
- Make the gallery resilient: a failed fetch leaves the last good gallery in
  place rather than blanking it.
- Keep the access token alive automatically (no manual 60-day renewal).

## Non-goals

- Redesigning the gallery grid/lightbox markup or CSS (reused as-is).
- Video playback (show the poster image linking to Instagram, as today).
- Fetching post comments, likes, or insights.
- Meta App Review (the app runs in development mode against the account's own
  media — see Prerequisites).

## Decisions

- **Fetch-and-commit, not build-time fetch.** A weekly GitHub Actions job fetches
  from Instagram and commits images + metadata into the repo; Hugo renders only
  local files. Mirrors the existing `fetch-reviews.go` pipeline and removes
  Instagram from the Cloudflare build path entirely. Chosen over a Cloudflare
  build-time fetch because (a) the build would again depend on a live external
  call, and (b) token persistence on Cloudflare is not automatable.
- **Commit image bytes, not URLs.** Instagram CDN `media_url`s are signed and
  expire, so storing URLs would break between the weekly fetch and a later
  Cloudflare build. Committing the downloaded bytes makes the build fully
  self-contained.
- **Automated token refresh.** The weekly job refreshes the long-lived token
  (valid 60 days) every run and writes the new value back to the
  `IG_ACCESS_TOKEN` secret via `gh secret set`, authorized by a fine-grained PAT.
- **Default 20 posts**, matching the current shortcode default.
- **Instagram API with Instagram Login** (`graph.instagram.com`), not the
  Facebook-Login Graph API — the account is a Business account and can authorize
  directly without a linked Facebook Page.

## Prerequisite: Meta app + token (one-time, manual)

The Graph API needs an OAuth Bearer token; this is set up once.

1. Create a **Meta app** (business type), add the **Instagram** product, and
   connect @sargentupholsteryco (Business account) under Instagram API setup with
   Instagram Login. The app may stay in **development mode** — accessing the
   connected account's own media does not require App Review.
2. In the app dashboard, generate an access token for the connected account,
   then exchange it for a **long-lived** token (60-day) via a single call to
   `https://graph.instagram.com/access_token?grant_type=ig_exchange_token`.
3. Add GitHub Actions repository secrets:
   - `IG_ACCESS_TOKEN` — the long-lived token (rotated automatically thereafter).
   - `GH_PAT` — a fine-grained Personal Access Token scoped to this repo with
     **Secrets: write**, so the workflow may update `IG_ACCESS_TOKEN`.

A helper, `scripts/setup-instagram.go`, may automate step 2 (exchange + print),
but steps 1 and 3 are manual in the Meta and GitHub dashboards.

## Architecture

```
Weekly GitHub Actions job (scripts/fetch-instagram.go)
  1. refresh token:  GET graph.instagram.com/refresh_access_token
                       ?grant_type=ig_refresh_token&access_token=$IG_ACCESS_TOKEN
                     -> new 60-day token -> `gh secret set IG_ACCESS_TOKEN` (uses GH_PAT)
  2. fetch media:    GET graph.instagram.com/me/media
                       ?fields=id,caption,media_type,media_url,thumbnail_url,permalink,timestamp
                       &limit=20&access_token=<token>
  3. per post:       choose image URL (media_url for IMAGE/CAROUSEL_ALBUM,
                       thumbnail_url for VIDEO); download bytes -> assets/instagram/<id>.jpg
  4. write metadata: data/instagram.json = [{id, caption, permalink, timestamp,
                       image: "instagram/<id>.jpg", mediaType}]
  5. prune:          delete assets/instagram/*.jpg whose id is not in the latest set
  6. commit:         git add assets/instagram data/instagram.json; commit if changed
                              │
                              ▼
Hugo build (Cloudflare) — instagram-gallery.html
  read data/instagram.json -> for each: resources.Get "instagram/<id>.jpg"
    -> .Resize "640x640 webp q85" (thumb) and "1200x webp q90" (large)
    -> render existing .ig-gallery-item grid + existing lightbox JS (unchanged)
  if data/instagram.json is empty/missing -> existing fallback message
```

### Components

**`scripts/fetch-instagram.go`** (new, run by CI)
- Reads env: `IG_ACCESS_TOKEN` (required), `GH_PAT` (for the secret update step).
- Refreshes the token; on success persists it via `gh secret set IG_ACCESS_TOKEN`
  (invoked as a subprocess with `GH_TOKEN=$GH_PAT`). A refresh failure is logged
  and non-fatal — the current token is still valid for up to 60 days.
- Fetches `/me/media` with `limit=20`, following the documented field set.
- Downloads each chosen image to `assets/instagram/<id>.jpg`. Skips posts with no
  usable image URL.
- Writes `data/instagram.json` (array, newest-first as returned by the API).
- Prunes `assets/instagram/*.jpg` not referenced by the new metadata.
- Exit 0 only after writing valid metadata + images; non-zero on hard failures
  (no token, media fetch error, zero posts) so CI surfaces the problem. Mirrors
  `fetch-reviews.go` conventions (30s HTTP client timeout, stderr errors).

**`scripts/setup-instagram.go`** (new, one-time helper)
- Given a short-lived token + app secret, exchanges for a long-lived token and
  prints it for pasting into `IG_ACCESS_TOKEN`. Optional convenience; the same
  exchange can be done with one `curl`.

**`themes/sargent/layouts/shortcodes/instagram-gallery.html`** (modify)
- Remove the `resources.GetRemote` scrape of the internal API.
- Read `site.Data.instagram` (from `data/instagram.json`).
- For each entry: `resources.Get` the local image, process to webp thumb/large,
  render the existing `.ig-gallery-item` markup (same `data-*` attributes).
- Keep the existing lightbox markup + `<script>` verbatim.
- Keep the `count` param to cap how many render (default 20); `username` stays for
  the fallback link.
- If `site.Data.instagram` is empty → render the existing fallback message.

**`.github/workflows/instagram.yml`** (new — separate from the "Fetch Google
Reviews" workflow for clarity)
- A job that runs `go run scripts/fetch-instagram.go` with env `IG_ACCESS_TOKEN`
  and `GH_PAT`, then commits changed `assets/instagram/` and `data/instagram.json`.
  Weekly schedule + `workflow_dispatch`, like reviews. Needs `permissions:
  contents: write`; the `GH_PAT` (not the default `GITHUB_TOKEN`) is what allows
  the in-job `gh secret set` to update `IG_ACCESS_TOKEN`.

### Data shape: `data/instagram.json`

```json
[
  {
    "id": "17900000000000000",
    "caption": "Reupholstered bench seat ...",
    "permalink": "https://www.instagram.com/p/ABC123/",
    "timestamp": "2026-05-30T14:02:11+0000",
    "image": "instagram/17900000000000000.jpg",
    "mediaType": "IMAGE"
  }
]
```

## Error handling

| Condition | Behavior |
|---|---|
| `IG_ACCESS_TOKEN` unset | Print guidance, `os.Exit(0)` (no-op; forks/manual runs don't fail) |
| Token refresh fails | Log to stderr, continue with current token (still valid up to 60 days) |
| `gh secret set` fails / `GH_PAT` unset | Log warning, continue (token rotation skipped this run) |
| Media fetch errors (non-2xx) | Print body, `os.Exit(1)` — do not overwrite existing data/images |
| Zero posts returned | Print error, `os.Exit(1)` — preserve last good gallery |
| Single image download fails | Skip that post, keep going; keep its existing committed image if present |
| `data/instagram.json` empty at build | Shortcode renders the existing fallback message |

The combination means: a bad run never blanks the gallery — Hugo keeps rendering
the last committed images and metadata.

## Repo growth

Steady-state ~20 images (~a few MB). Pruning removes images for posts that drop
out of the latest 20, so the working tree stays bounded. (Git history retains old
blobs; acceptable for this volume.)

## Testing / verification

No Go module exists; verification is `gofmt` + `go build` (compile) plus manual
runs, consistent with the reviews work.

1. `gofmt -l scripts/` clean; `go build` of both scripts compiles.
2. Local run with a real `IG_ACCESS_TOKEN` exported: `go run scripts/fetch-instagram.go`
   writes `data/instagram.json` with ~20 entries and downloads matching images
   into `assets/instagram/`.
3. `hugo` build (or `hugo server`) renders the grid from local files with zero
   network calls to Instagram; lightbox still opens/navigates.
4. Empty/removed `data/instagram.json` → fallback message renders, build succeeds.
5. Re-run after deleting one image → it is re-downloaded; an id removed from the
   feed → its image is pruned.
6. Trigger the workflow via `workflow_dispatch`; confirm it refreshes the token
   (secret `IG_ACCESS_TOKEN` updated), commits changes, and the token rotation
   step succeeds with `GH_PAT`.
