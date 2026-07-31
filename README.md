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
- **Instagram integration shortcode** that fetches and displays recent Instagram posts with a client-side lightbox gallery
- **Google Reviews integration** that fetches and displays customer reviews via the Google Business Profile API
- **Custom Hugo theme** (`sargent`) with optimized layouts and partials
- **Performance-optimized** — scores 97/100 mobile, 100/100 desktop on Lighthouse

**Tech Stack:**
- Hugo (static site generator)
- HTML5 / CSS3 / Vanilla JavaScript
- Hugo asset pipeline (CSS minification + fingerprinting, image processing with WebP + srcset)
- Instagram Graph API (official; weekly fetch committed to the repo, then build-time image processing)
- Google Business Profile API v4 (review fetching via OAuth 2.0)
- Cloudflare R2 (hosts Instagram Reels videos, served from `media.sargentupholstery.com`)
- Google Analytics (`gtag.js`) — the one third-party script on the site
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

A **GitHub Actions** workflow (`.github/workflows/hugo.yml`) runs weekly to fetch fresh Google Reviews:
1. Fetches reviews via `scripts/fetch-reviews.go` (requires the four `GOOGLE_*` OAuth secrets)
2. Commits updated `data/reviews.json` back to the repo
3. The commit triggers a Cloudflare Pages rebuild automatically

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
   go run scripts/setup-business-profile.go /path/to/client_secret.json
   ```
4. Add the four printed values as GitHub Actions repository secrets:
   `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `GOOGLE_REFRESH_TOKEN`,
   `GOOGLE_LOCATION_NAME`.

Reviews are filtered to 5-star with 20+ words, saved to `data/reviews.json`, and
displayed on the homepage.

---

## Project Structure

```
sargent-upholstery/
├── assets/
│   ├── css/style.css           # Main stylesheet (processed via Hugo pipes)
│   ├── images/old/             # History page photos (Hugo-processed to WebP)
│   └── instagram/              # Instagram post posters (committed by CI)
├── content/                    # Markdown pages
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
├── data/
│   ├── reviews.json            # Google Reviews (fetched by CI)
│   └── instagram.json          # Instagram posts metadata (fetched by CI)
├── scripts/
│   ├── fetch-reviews.go        # Google Reviews fetch script (Business Profile v4)
│   ├── fetch-instagram.go      # Instagram Graph API fetch script
│   └── setup-business-profile.go  # One-time OAuth setup → prints CI secrets
├── reels/                      # Instagram Reels MP4 staging (gitignored; synced to R2)
├── docs/superpowers/           # Design specs & implementation plans
├── static/
│   ├── favicon.ico
│   ├── fonts/                  # Self-hosted Work Sans woff2
│   ├── _headers                # Cloudflare Pages security + caching headers
│   ├── _redirects              # www → apex domain redirect
│   ├── robots.txt              # Crawl directives
│   └── images/
│       ├── heroes/             # Hero images (WebP with 640w/1024w/1920w variants)
│       ├── logo.svg
│       └── non-oval-logo-white.svg
├── themes/sargent/             # Custom Hugo theme
│   ├── layouts/
│   │   ├── index.html          # Homepage layout
│   │   ├── 404.html
│   │   ├── _default/           # Default templates (single, list, baseof)
│   │   ├── partials/           # Reusable components (nav, footer, head, schema, reviews)
│   │   └── shortcodes/         # Custom shortcodes
│   │       ├── img.html               # Responsive image shortcode (WebP + srcset)
│   │       └── instagram-gallery.html
│   └── static/
│       └── js/main.js          # Lite YouTube facade (gallery lightbox JS is inline in the shortcode)
├── .github/workflows/
│   ├── hugo.yml                # Weekly Google Reviews fetch
│   └── instagram.yml           # Weekly Instagram gallery fetch
├── hugo.toml                   # Hugo configuration
└── README.md
```

---

## Performance

The site is optimized for Core Web Vitals:

| Metric | Mobile | Desktop |
|---|---|---|
| Performance | 97 | 100 |
| FCP | 0.8s | 0.2s |
| LCP | 2.3s | 0.5s |
| TBT | 0ms | 0ms |
| CLS | 0 | 0 |

Key optimizations:
- **Lite YouTube facade** — YouTube iframe loads only on click, eliminating ~500KB of third-party JS
- **Responsive hero images** — WebP format with 640w/1024w/1920w srcset variants, preloaded in `<head>`
- **Hugo image processing** — history page photos auto-resized to WebP with srcset (17 MB → ~1.5 MB)
- **Self-hosted fonts** — Work Sans woff2 served from same origin with `<link rel="preload">`
- **SVGO-optimized SVG logos** — nav logo reduced from 53 KB to 11 KB
- **CSS minification + fingerprinting** — via Hugo asset pipeline
- **HTML minification** — enabled via Hugo config
- **Explicit image dimensions** — width/height on all `<img>` elements to prevent CLS
- **`fetchpriority="high"`** — on hero images for faster LCP discovery
- **Cloudflare Pages caching** — immutable cache headers for static assets, no-cache for HTML

> **Note:** Google Analytics (`gtag.js`) is the single third-party script loaded site-wide. Everything else (fonts, lightbox, the YouTube facade, Reels video) is first-party or loads no JS until interaction.

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
- Automatic image optimization (resize to WebP format)
- Client-side lightbox with keyboard navigation (arrow keys, Escape)
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
- `static/js/` — JavaScript

Site-wide CSS is in `assets/css/style.css` (processed through Hugo's asset pipeline for minification and fingerprinting).

Changes auto-reload in dev server.

---

## Dependencies

- **Hugo Extended** — exact version pinned in [`.hugo-version`](.hugo-version) (for image processing and the asset pipeline)
- No npm, no Node.js, no database required

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
