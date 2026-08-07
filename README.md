███████╗ █████╗ ██████╗  ██████╗ ███████╗███╗   ██╗████████╗
██╔════╝██╔══██╗██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝
███████╗███████║██████╔╝██║  ███╗█████╗  ██╔██╗ ██║   ██║
╚════██║██╔══██║██╔══██╗██║   ██║██╔══╝  ██║╚██╗██║   ██║
███████║██║  ██║██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║
╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝

██╗   ██╗██████╗ ██╗  ██╗ ██████╗ ██╗      ███████╗████████╗███████╗██████╗ ██╗   ██╗
██║   ██║██╔══██╗██║  ██║██╔═══██╗██║      ██╔════╝╚══██╔══╝██╔════╝██╔══██╗╚██╗ ██╔╝
██║   ██║██████╔╝███████║██║   ██║██║      ███████╗   ██║   █████╗  ██████╔╝ ╚████╔╝
██║   ██║██╔═══╝ ██╔══██║██║   ██║██║      ╚════██║   ██║   ██╔══╝  ██╔══██╗  ╚██╔╝
╚██████╔╝██║     ██║  ██║╚██████╔╝███████╗ ███████║   ██║   ███████╗██║  ██║   ██║
 ╚═════╝ ╚═╝     ╚═╝  ╚═╝ ╚═════╝ ╚══════╝ ╚══════╝   ╚═╝   ╚══════╝╚═╝  ╚═╝   ╚═╝

 ██████╗ ██████╗
██╔════╝██╔═══██╗
██║     ██║   ██║
██║     ██║   ██║
╚██████╗╚██████╔╝██╗
 ╚═════╝ ╚═════╝╚═╝

# Sargent Upholstery Co. Website

## Executive Summary

This repository contains a static website for **Sargent Upholstery Co.**, Jacksonville's premier automotive upholstery shop established in 1935. The site is built with **Hugo**, a fast and flexible static site generator, and features:

- **Responsive design** showcasing upholstery services (automotive, fleet, marine, convertible tops, leather interiors, etc.)
- **Fully bilingual** — complete English/Spanish support with hreflang tags and translated content
- **Instagram gallery** — posts fetched weekly by CI and committed to the repo, rendered by a shortcode with a client-side lightbox (photos and Reels)
- **Google Reviews integration** — reviews fetched weekly via the Google Business Profile API, windowed to the last 12 months at render time
- **Custom Hugo theme** (`sargent`) with optimized layouts and partials
- **Performance-optimized** — 97/100 mobile, 100/100 desktop on Lighthouse (measured 2026-02-18)

**Tech Stack:**
- Hugo (static site generator)
- HTML5 / CSS3 / Vanilla JavaScript
- Hugo asset pipeline (CSS minification + fingerprinting, image processing with WebP + srcset)
- Instagram Graph API (official; weekly fetch committed to the repo, then build-time image processing)
- Google Business Profile API v4 (review fetching via OAuth 2.0)
- Cloudflare R2 (hosts Instagram Reels videos, served from `media.sargentupholstery.com`)
- Cloudflare Web Analytics (cookieless; beacon injected at the edge, nothing in the templates)
- Cloudflare Pages hosting with GitHub Actions for scheduled review & Instagram fetching
- Zero database requirements

---

## Quick Start

### Prerequisites
- **Hugo Extended** — required for image processing and the asset pipeline. Use the **exact** version in [`.hugo-version`](.hugo-version) (currently `0.154.5`); different versions produce different image hashes, so an unmatched version will re-fingerprint every processed asset.
  - Download: https://gohugo.io/getting-started/installing/

### Build Locally

```bash
# Clone the repository
git clone https://github.com/seanctodd/sargent-upholstery.git
cd sargent-upholstery

# Start the development server
hugo server

# Open http://localhost:1313 in your browser
```

### Build for Production

```bash
# Generate minified static HTML in the 'public/' directory
hugo --minify

# The 'public/' folder now contains your website, ready to deploy
```

---

## Deployment

The site is hosted on **Cloudflare Pages**, which builds and deploys automatically on every push to `main`.

**Cloudflare Pages build settings:**
- **Build command:** `hugo --minify`
- **Build output directory:** `public`
- **Environment variable:** `HUGO_VERSION` = the value in [`.hugo-version`](.hugo-version) (currently `0.154.5`)
- **Build image:** v2 or v3 (both Ubuntu 22.04). Hugo Extended 0.154.5 needs `GLIBC_2.34` / `GLIBCXX_3.4.29`, which Ubuntu 20.04 (build image v1) does not provide.

> ⚠️ **`HUGO_VERSION` cannot be set from the repository.** Cloudflare Pages reads build environment variables from the dashboard only (**Workers & Pages → the project → Settings → Environment variables**). `.hugo-version` is the source of truth for humans and local tooling; **when you change it, update the dashboard variable in the same session or the two will drift.**
>
> If `HUGO_VERSION` is unset, Cloudflare falls back to the build image default (v3 → `0.147.7`), which predates the `minify.tdewolff.css.version` key in `hugo.toml` and would silently change the minified CSS.

### Asset caching

`static/_headers` splits assets into two groups, and the split matters:

- **Fingerprinted** (`/css/*`, `/js/*`, `/img/*`, `/instagram/*`) — Hugo puts a content hash in the filename, so the URL changes whenever the bytes do. Cached `immutable` for a year.
- **Copied verbatim from `static/`** (`/images/*`, `/fonts/*`) — fixed URLs, so the filename does *not* change with the content. Never `immutable`: heroes get a week because swapping one keeps its filename, and the font gets a year because a pinned subset has no reason to change.

> The logos live at `/img/*` rather than `/images/*` specifically so the immutable rule can cover them without also matching the hand-made hero variants. Cloudflare merges overlapping rules into a comma-joined `Cache-Control`.

> ⚠️ Never mark a non-fingerprinted asset `immutable`. It tells browsers not to revalidate *even on an explicit reload*, so a bad file stays pinned for the full TTL with no way to push a fix. This is not hypothetical: `logo.svg` and `js/main.js` were served that way, and corrected versions of both were invisible to anyone who had already loaded the site. **If an asset needs to be updatable, put it in `themes/sargent/assets/` and pipe it through `fingerprint`** rather than shortening its TTL.

> ⚠️ Cloudflare Pages **merges every matching rule** and joins duplicate headers with a comma, so two rules matching one file emit a malformed `Cache-Control`. Each path must match exactly one `Cache-Control` rule — CI asserts this.

**Domain redirect (`www` → apex):** handled by a zone-level **Redirect Rule** (Cloudflare dashboard → **Rules → Redirect Rules**), a `301` matching hostname `www.sargentupholstery.com` and preserving path + query string.

> ⚠️ **`static/_redirects` cannot express the `www` redirect.** Pages `_redirects` matches *relative paths* only — it cannot see hostnames. A file attempting the `www` → apex redirect shipped from the GitHub Pages migration until 2026-07-31; every line was rejected (`Parsed 0 valid redirect rules` in the build log) and `www` returned HTTP 522 the whole time. Hostname redirects belong in a Redirect Rule.

`static/_redirects` does exist, for path-level rules only. It currently holds one: `/security.txt` → `/.well-known/security.txt` (`301`), so scanners that probe the document root reach the canonical file instead of a second copy that could drift. Pages consults `_redirects` only when no static asset matches the request path, so a rule can never shadow a real file.

### Security headers, crawling, and disclosure

- **`static/_headers`** carries the security headers alongside the caching rules: HSTS, `X-Frame-Options: DENY`, `nosniff`, a referrer policy, a `Permissions-Policy` denying geolocation/mic/camera, and the CSP.
- **The CSP keeps `'unsafe-inline'` in `script-src` for Cloudflare, not for this site.** Under Bot Fight Mode, Cloudflare injects an inline JavaScript Detections bootstrap into every HTML response at the edge and that injection cannot be disabled; removing `'unsafe-inline'` silently breaks bot detection and logs a CSP violation on every page load (verified live on 2026-07-31, then reverted). The supported fix is a per-request nonce, which a static Pages site cannot generate without a Function. **The site's own JavaScript is still never inline** — `main.js` and `gallery.js` are fingerprinted assets — and that should stay true: it is better for caching and it is what lets `tests/*.test.js` exercise the real files.
- **`static/robots.txt`** allows the whole tree and deliberately does **not** block AI crawlers (GPTBot, ClaudeBot, CCBot, Google-Extended, …) — being citable when someone asks an assistant about Jacksonville upholstery is a discovery channel. It lists the sitemap *index*, which fans out to `/en/sitemap.xml` and `/es/sitemap.xml` and stays correct if a language is added. robots.txt is advisory and is never an access control.
- **`static/.well-known/security.txt`** publishes the RFC 9116 disclosure contact (`security@sargentupholstery.com`). ⚠️ **It expires** — currently `2027-08-01`, the RFC's one-year cap. Past that date tooling reports it as stale, so bump the date and re-confirm the mailbox routes at least annually.

## Continuous Integration

`.github/workflows/ci.yml` runs on every push and pull request to `main`:

| Job | What it checks |
|---|---|
| **Hugo build** | Installs the exact binary from `.hugo-version` (same release Cloudflare downloads), builds, and **fails on any `WARN`** — this is what catches deprecated config keys. Then asserts key artefacts exist, that empty taxonomy pages have not returned to the sitemap, and that no built path matches more than one `_headers` `Cache-Control` rule. |
| **JS behaviour tests** | Every `tests/*.test.js` under jsdom, each run against the real source file. Currently `lite-youtube.test.js` (14 assertions — the YouTube facade's keyboard handling, re-activation guard, and label fallbacks) and `gallery.test.js` (18 assertions — the Instagram lightbox's open/close, keyboard navigation and wrap-around, video vs. photo tiles, and its no-op on pages with no gallery). |
| **Go scripts** | `gofmt`, `go vet ./...`, `go build ./...`, and `go test ./...` — covering `reconcile()` in `fetch-reviews` (the logic that decides when a review stops being displayed) and the `ours()` file filter in `fetch-instagram` (which decides what the prune step is allowed to delete). |

> `--panicOnWarning` is deliberately **not** used: it does not fail on config deprecations (it logs them and exits `0`). The build log is grepped for `^WARN` instead.
>
> jsdom is installed with `npm install --no-save`, so no `package.json` or lockfile enters the repo — a plain `hugo` build remains the only requirement to work on the site.

Run the same checks locally:

```bash
hugo --gc --printPathWarnings          # must print no WARN lines
npm install --no-save jsdom && for t in tests/*.test.js; do node "$t"; done
gofmt -l . && go vet ./... && go test ./...
```

Two more workflows run on a weekly schedule, both on Mondays and offset so they
do not race for a commit:

| Workflow | Schedule | What it does |
|---|---|---|
| `reviews.yml` | Mon 09:00 UTC | Runs `scripts/fetch-reviews` (needs the four `GOOGLE_*` OAuth secrets), commits `data/reviews.json` |
| `instagram.yml` | Mon 10:00 UTC | Runs `scripts/fetch-instagram`, syncs new Reels to R2, commits `data/instagram.json` + `assets/instagram/` |

Each commit triggers a Cloudflare Pages rebuild automatically. Both can also be
run manually from the Actions tab; `reviews.yml` takes a **`dry_run`** checkbox
that reports what would change without writing the file or committing.

**Live site:** https://sargentupholstery.com/

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
   go run ./scripts/setup-business-profile /path/to/client_secret.json
   ```
4. Add the four printed values as GitHub Actions repository secrets:
   `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REFRESH_TOKEN`,
   `GOOGLE_LOCATION_NAME`.

Reviews are filtered to 5-star with 20+ words, saved to `data/reviews.json`, and
displayed on the homepage.

#### Recency window

`data/reviews.json` is a **complete archive** — every qualifying review back to
2016. That is deliberate: reconciliation matches stored records against the full
API response, so trimming the file would break removal detection.

The homepage instead windows at render time. `themes/sargent/layouts/partials/reviews.html`:

```go-html-template
{{ $windowMonths := -12 }}
{{ $cutoff := (now.AddDate 0 $windowMonths 0).Format "2006-01-02" }}
```

Change that one number to widen or narrow it — nothing is discarded either way,
and it takes effect on the next build. Pool sizes *as of 2026-08-07* (these
shrink and grow on their own as the window slides — re-count before relying on
them):

| Window | Reviews available |
|---|---|
| 3 months | 10 |
| 6 months | 20 |
| 12 months (current) | 55 |
| all time | 126 |

("Available" means displayable, so tombstoned records are excluded — the file
holds 127 records, one of which is tombstoned.)

Four are shuffled onto the page each build. If the window ever holds fewer than
four, it falls back to the four most recent regardless of age, so the grid never
renders half-empty.

> The cutoff is computed at **build** time, so it only advances when the site
> rebuilds. The weekly review and Instagram jobs commit often enough to keep it
> sliding.

#### Removals (reconciliation)

A review that is deleted on Google, or edited below the display bar (under 5
stars, or shortened under 20 words), is **tombstoned** — the record keeps its
history and gains a `"removed": "YYYY-MM-DD"` field. `reviews.html` filters
those out with `where ... "removed" nil`. Nothing is ever deleted from the file,
so a false positive is undone by clearing the field.

Removals are far riskier than additions: an absent review may simply mean the
fetch failed halfway. Three gates must all pass before anything is tombstoned:

1. every page fetched without error, **and**
2. the API reported `totalReviewCount`, **and**
3. the number fetched equals that total.

If any gate fails the run still adds new reviews but skips removals entirely and
emits a CI warning. A fourth guard caps the blast radius: a run proposing more
than `REVIEWS_MAX_REMOVALS` (default 5) changes nothing and lists what it would
have removed, so an API anomaly cannot quietly wipe the archive.

Content of a still-qualifying review is **never** rewritten — edits upstream do
not change text already captured.

| Variable | Purpose |
|---|---|
| `REVIEWS_DRY_RUN` | Set to anything to report changes without writing the file |
| `REVIEWS_MAX_REMOVALS` | Override the default cap of 5 removals per run |

Preview what a run would do without touching anything:

```bash
REVIEWS_DRY_RUN=1 go run ./scripts/fetch-reviews
```

---

## Project Structure

```
sargent-upholstery/
├── assets/
│   ├── css/style.css           # Main stylesheet (processed via Hugo pipes)
│   ├── images/old/             # History page photos (Hugo-processed to WebP)
│   └── instagram/              # Instagram post posters (committed by CI)
├── content/                    # Markdown pages — every page has a .es.md twin
│   ├── _index.md               # Homepage
│   ├── gallery.md              # Gallery with Instagram integration
│   ├── contact.md
│   ├── estimate.md
│   ├── faq.md
│   ├── our-history.md
│   ├── privacy-policy.md
│   └── service/                # Service detail pages
│       ├── _index.md
│       ├── automotive.md
│       ├── fleet-services.md
│       └── marine.md
├── i18n/                       # UI string translations (en.yaml, es.yaml)
├── data/
│   ├── reviews.json            # Google Reviews (fetched by CI)
│   └── instagram.json          # Instagram posts metadata (fetched by CI)
├── go.mod                      # Module for the scripts (the site needs no Go)
├── scripts/                    # One package per command: `go run ./scripts/<name>`
│   ├── fetch-reviews/          # Google Reviews fetch (Business Profile v4)
│   │   ├── main.go
│   │   └── reconcile_test.go   # Tests for the review removal logic
│   ├── fetch-instagram/        # Instagram Graph API fetch
│   │   ├── main.go
│   │   └── prune_test.go       # Guards which files the prune step may delete
│   └── setup-business-profile/ # One-time OAuth setup → prints CI secrets
├── reels/                      # Instagram Reels MP4 staging (gitignored; synced to R2)
├── docs/superpowers/           # Design specs & implementation plans
├── static/
│   ├── favicon.ico
│   ├── fonts/                  # Self-hosted Work Sans woff2
│   ├── _headers                # Cloudflare Pages security + caching headers
│   ├── _redirects              # Path-only redirects (/security.txt → canonical)
│   ├── robots.txt              # Crawl directives
│   ├── .well-known/
│   │   └── security.txt        # RFC 9116 disclosure contact (has an expiry date)
│   └── images/
│       ├── heroes/             # Hero images (WebP: 640w/768w/1024w + full 1920w)
│       └── non-oval-logo-color.png
├── themes/sargent/             # Custom Hugo theme
│   ├── layouts/
│   │   ├── index.html          # Homepage layout
│   │   ├── 404.html
│   │   ├── _default/           # Default templates (single, list, baseof)
│   │   ├── partials/           # nav, footer, head, schema, reviews
│   │   │   └── hero-srcset.html       # Hero width variants; one source of truth
│   │   │                              # for the <img> and the <head> preload
│   │   └── shortcodes/         # Custom shortcodes
│   │       ├── img.html               # Responsive image shortcode (WebP + srcset)
│   │       └── instagram-gallery.html
│   └── assets/                 # Piped through Hugo and fingerprinted
│       ├── js/main.js          # Lite YouTube facade
│       ├── js/gallery.js       # Instagram lightbox (external, so the CSP and
│       │                       # the jsdom tests can both reach it)
│       └── img/                # logo.svg (footer + favicon), non-oval-logo-white.svg (nav)
├── tests/                      # jsdom behaviour tests; CI runs every *.test.js
│   ├── lite-youtube.test.js    # YouTube facade
│   └── gallery.test.js         # Instagram lightbox
├── .github/workflows/
│   ├── ci.yml                  # Build + tests on every push / PR
│   ├── reviews.yml             # Weekly Google Reviews fetch
│   └── instagram.yml           # Weekly Instagram gallery fetch
├── .hugo-version               # Pinned Hugo version (mirror into Cloudflare)
├── hugo.toml                   # Hugo configuration
└── README.md
```

---

## Performance

The site is optimized for Core Web Vitals. The figures below were measured on
**2026-02-18** and have not been re-run since; they predate the Google Analytics
removal and the 768w hero, both of which should only have helped. Re-measure
before quoting them.

| Metric | Mobile | Desktop |
|---|---|---|
| Performance | 97 | 100 |
| FCP | 0.8s | 0.2s |
| LCP | 2.3s | 0.5s |
| TBT | 0ms | 0ms |
| CLS | 0 | 0 |

Key optimizations:
- **Lite YouTube facade** — YouTube iframe loads only on click, eliminating ~500KB of third-party JS
- **Responsive hero images** — WebP with 640w/768w/1024w/1920w srcset variants, preloaded in `<head>`. The widths live in one place, `partials/hero-srcset.html`; 768w exists because a ~412px viewport at DPR 1.75 needs ~763px and would otherwise pull the 1024w file for ~21 KB it cannot display
- **Hugo image processing** — history page photos auto-resized to WebP with srcset (17 MB → ~1.5 MB)
- **Self-hosted fonts** — Work Sans woff2 served from same origin with `<link rel="preload">`
- **SVGO-optimized SVG logos** — nav logo reduced from 53 KB to 11 KB
- **CSS minification + fingerprinting** — via Hugo asset pipeline
- **HTML minification** — enabled via Hugo config
- **Explicit image dimensions** — width/height on all `<img>` elements to prevent CLS
- **`fetchpriority="high"`** — on hero images for faster LCP discovery
- **Cloudflare Pages caching** — immutable cache headers for static assets, no-cache for HTML

> **Note:** No analytics script is loaded from the templates. Traffic is measured by Cloudflare Web Analytics, whose beacon Cloudflare injects at the edge. Everything else (fonts, lightbox, the YouTube facade, Reels video) is first-party or loads no JS until interaction.
>
> Google Analytics was removed on 2026-07-31: `gtag.js` was 167 KiB (70 KiB unused) — nearly twice the weight of the entire first-party page — and the only source of long main-thread tasks. Cloudflare Web Analytics covers pageviews, referrers and geography at a fraction of the cost and needs no cookie banner.

---

## Instagram Gallery Shortcode

The `instagram-gallery` shortcode automatically fetches Instagram posts and displays them in an interactive lightbox gallery.

**Usage in Markdown:**
```markdown
{{< instagram-gallery count="20" username="sargentupholsteryco" >}}
```

**Parameters:**
- `count` (default: `20`) — Number of posts to display
- `username` (default: `sargentupholsteryco`) — used only for image alt-text fallback and the fallback profile link; the posts come from the account tied to `IG_ACCESS_TOKEN`, not this value

**Features:**
- Posts are fetched **weekly by GitHub Actions** (`.github/workflows/instagram.yml`) via the official Instagram Graph API and committed to the repo (`data/instagram.json` + `assets/instagram/`)
- Hugo renders from those local files — **no API call at site-build time**, so the build never depends on Instagram being reachable
- Automatic image optimization — tiles are `Fill`ed to the square the grid expects, not `Resize`d (which warped the 16:9 and 9:16 video posters)
- Client-side lightbox with keyboard navigation (arrow keys, Escape), from the fingerprinted `themes/sargent/assets/js/gallery.js`. It is kept free of template values so one file serves both locales — the two translated strings arrive as `data-` attributes — and covered by `tests/gallery.test.js`
- Lazy loading, responsive grid layout

**Video (Reels):** Video posts also have their MP4 downloaded into `reels/` (gitignored)
and uploaded to a **Cloudflare R2** bucket (`sargent-media`) via `aws s3 sync` in the
workflow. They're served from `https://media.sargentupholstery.com/reels/` and played in
the lightbox with a native `<video>` element (no third-party player). The committed grid
only ever holds static poster images, so videos never enter git.

**Setup:** Requires a Meta app connected to the Business account plus these GitHub Actions
secrets:
- `IG_ACCESS_TOKEN` — long-lived Instagram token (auto-refreshed weekly by the workflow)
- `GH_PAT` — fine-grained PAT with **Secrets: write**, so the workflow can rotate `IG_ACCESS_TOKEN`
- `AWS_ACCESS_KEY_ID` + `AWS_SECRET_ACCESS_KEY` — Cloudflare R2 S3-API token (Reels upload)
- `R2_ACCOUNT_ID` — Cloudflare account ID, used in the R2 S3 endpoint URL

See the design specs in `docs/superpowers/specs/` (Graph API gallery + R2 video).

---

## Development

### Run Local Server
```bash
hugo server -D    # Include drafts
hugo server       # Production mode
```

Visit http://localhost:1313

### Create New Page
```bash
hugo new content/new-page.md
```

### Edit Theme
Theme files are in `themes/sargent/`. Modify:
- `layouts/` — HTML templates
- `assets/js/` — JavaScript (`main.js`, `gallery.js`), minified and fingerprinted by Hugo

Site-wide CSS is in `assets/css/style.css` (processed through Hugo's asset pipeline for minification and fingerprinting).

Changes auto-reload in dev server. After touching either JS file, run its jsdom test — both load the real source, so a rename means updating the test's path too.

### Editing content
Every page is bilingual: `content/foo.md` and `content/foo.es.md`. Add or edit them in pairs, and put UI strings (buttons, headings shared across pages) in `i18n/en.yaml` + `i18n/es.yaml` rather than hard-coding them in templates.

---

## Dependencies

- **Hugo Extended** — exact version pinned in [`.hugo-version`](.hugo-version) (for image processing and the asset pipeline). This is the only thing needed to build the site: no npm manifest, no lockfile, no database.
- **Go** — version pinned in [`go.mod`](go.mod) (currently 1.24). Needed only to run or test the `scripts/` fetchers, never to build the site.
- **Node.js + jsdom** — needed only to run `tests/*.test.js`. jsdom is installed with `npm install --no-save`, so no `package.json` or lockfile enters the repo.

---

## License

Repository contents (c) Sargent Upholstery Co. All rights reserved.

---

## Contact

**Sargent Upholstery Co.**
- Address: 44 E 1st St, Jacksonville, FL 32206
- Phone: (904) 355-2529
- Email: sales@sargentupholstery.com
- Instagram: [@sargentupholsteryco](https://instagram.com/sargentupholsteryco)
- Facebook: [Sargent Upholstery](https://facebook.com/sargentupholstery)
