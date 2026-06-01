# Business Profile v4 Review Fetching — Design

**Date:** 2026-06-01
**Status:** Approved (pending spec review)

## Problem

The homepage Google Reviews have not updated since 2026-02-17. Root cause:

1. The fetch script's primary working source is the **Places API (New)**, which
   returns at most 5 "most relevant" reviews — never the newest, never the full
   set. New reviews therefore never appear, so the weekly workflow produces no
   diff and never commits.
2. There is no evidence the automated workflow has ever committed
   (`git log --grep=automated` is empty); the only changes to
   `data/reviews.json` were manual.

The fix is to fetch from the **Google Business Profile API v4**
(`accounts.locations.reviews.list`), which returns *all* reviews for the
business, newest-first, with pagination.

References:
- https://developers.google.com/my-business/reference/rest/v4/accounts.locations.reviews
- https://developers.google.com/my-business/content/review-data

## Goals

- Fetch the complete review set from the Business Profile v4 API as the **single**
  source of truth.
- Keep the homepage showing only high-quality reviews (5-star, substantive).
- Never lose accumulated review history due to a corrupt file or transient error.
- Keep the weekly GitHub Actions workflow as the delivery mechanism.

## Non-goals

- Changing how reviews render on the homepage. The `%.200s` byte-truncation bug
  in `themes/sargent/layouts/partials/reviews.html` (breaks multibyte Spanish
  text) is real but **out of scope** for this rework; tracked separately.
- Fetching review *replies*, photos, or ratings-only reviews.

## Prerequisite: OAuth credentials (not code)

The v4 reviews endpoint requires an **OAuth 2.0 Bearer token** — a plain API key
does not authenticate it. Required, obtained once:

1. In Google Cloud Console, enable **My Business Account Management API** and
   **My Business Reviews API** (access to v4 must be granted by Google via their
   request form — the user has applied for and received this).
2. Create an **OAuth 2.0 credential of type "Desktop app"** and download the
   `client_secret_*.json` file.
3. Run `go run scripts/setup-business-profile.go /path/to/client_secret.json`.
   This opens a browser for one-time authorization and prints four values.
4. Add these as GitHub Actions repository secrets:
   - `GOOGLE_CLIENT_ID`
   - `GOOGLE_CLIENT_SECRET`
   - `GOOGLE_REFRESH_TOKEN`
   - `GOOGLE_LOCATION_NAME` (e.g. `accounts/123/locations/456`)

`setup-business-profile.go` already implements this flow and needs no changes
beyond a `gofmt` cleanup.

## Architecture

Single-source fetch pipeline in `scripts/fetch-reviews.go`:

```
env secrets ──> getAccessToken(refresh_token) ──> Bearer token
                                                      │
                                                      ▼
        fetchBusinessProfileReviews(location, token)  (paginate v4 list)
                                                      │
                                                      ▼
                          candidates []Review (all reviews, all ratings)
                                                      │
                load existing data/reviews.json ──────┤
                                                      ▼
                  filter (5-star AND >=20 words) + dedup
                                                      ▼
                       sort by date desc ──> write data/reviews.json
```

### Components

**`getAccessToken(clientID, clientSecret, refreshToken) (string, error)`**
Exchanges the refresh token at `https://oauth2.googleapis.com/token` for a
short-lived access token. Returns an error (caller exits non-zero) on failure.
*Unchanged from current code.*

**`fetchBusinessProfileReviews(locationName, accessToken) []Review`**
- `GET https://mybusiness.googleapis.com/v4/{locationName}/reviews?orderBy=updateTime desc&pageSize=50`
- Follows `nextPageToken` until empty.
- Maps each `bpReview` to `Review`:
  - `ID` = `name`
  - `Author` = `reviewer.displayName`
  - `Rating` = `starRatingToInt(starRating)` (FIVE..ONE → 5..1)
  - `Text` = `comment`
  - `Date` = `updateTime`, fallback `createTime`, fallback "" (NOT `time.Now()`)
- *Mostly unchanged; the date fallback changes from `time.Now()` to `createTime`/"".*

**Filtering & dedup (in `main`)** — unchanged semantics:
- Keep only `Rating == 5` AND `len(strings.Fields(Text)) >= 20`.
- Dedup against existing by `ID`; secondary guard by `normalizeText(Text)`.
- Guard: only write `existingIDs[r.ID]` when `r.ID != ""`.

**Sorting** — `sort.Slice` by `Date` string descending. Now meaningful because
all dates are real RFC3339 timestamps from the API.

### Code being removed

- `placesAPIResponse`, `placesAPIReview`, `extractPlacesReviews`, and the
  `GOOGLE_API_KEY` / Places fetch branch in `main`.
- The `placeID` constant (only used by Places).
- Credential check simplifies to requiring the four OAuth env vars.

### Hardening

- Package-level `http.Client{Timeout: 30 * time.Second}` used by `fetchURL` and
  `getAccessToken` (replaces `http.DefaultClient` / `http.PostForm`).
- If `data/reviews.json` exists and is non-empty but fails to unmarshal, print an
  error and `os.Exit(1)` — do not proceed with an empty `existing` set (prevents
  clobbering accumulated history).
- Check and report errors from `os.MkdirAll` / initial `os.WriteFile`.

## Error handling

| Condition | Behavior |
|---|---|
| Missing any of the 4 OAuth env vars | Print guidance, `os.Exit(0)` (no-op, so manual runs / forks don't fail CI) |
| `getAccessToken` fails | Print error, `os.Exit(1)` |
| A page fetch fails mid-pagination | Log it, keep reviews gathered so far, stop paginating |
| `reviews.json` corrupt (non-empty, unparseable) | `os.Exit(1)` — protect history |
| Zero candidates fetched | Print error, `os.Exit(1)` |
| Final marshal/write fails | Print error, `os.Exit(1)` |

## Workflow changes (`.github/workflows/hugo.yml`)

- Remove `GOOGLE_API_KEY` from the `env:` block.
- Keep the four OAuth secrets and the commit/push step (unchanged).

## Documentation changes (`README.md`)

- Replace the "Google Reviews Setup" section to describe the OAuth/Business
  Profile flow and `setup-business-profile.go`, not the API-key/Places flow.
- Remove the stale `shortcodes/google-reviews.html` reference; reviews render via
  `partials/reviews.html`.
- Add `scripts/setup-business-profile.go` to the project-structure tree.

## Testing / verification

No Go module exists, so verification is manual:

1. `gofmt -l scripts/` reports nothing.
2. `go vet scripts/fetch-reviews.go` (or `go build`) compiles cleanly.
3. Local run with real secrets exported: `go run scripts/fetch-reviews.go`
   fetches > 6 reviews, prints per-page counts, writes valid JSON, and a second
   run reports `0 new` (dedup works).
4. Manually corrupt `data/reviews.json` → run → confirms it exits 1 without
   overwriting.
5. Trigger the workflow via `workflow_dispatch` and confirm it commits.

## Decisions

- **Single source (Business Profile v4 only):** chosen over keeping Places as a
  supplement — Places caps at 5 and pollutes dates; it adds complexity for no
  benefit now that v4 access is granted.
- **Filter: 5-star AND ≥20 words (option a):** keeps homepage curated; matches
  current behavior.
- **Real timestamps over `time.Now()`:** makes newest-first sorting correct and
  dedup stable across runs.
