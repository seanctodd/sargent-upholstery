███████╗ █████╗ ██████╗  ██████╗ ███████╗███╗   ██╗████████╗
██╔════╝██╔══██╗██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝
███████╗███████║██████╔╝██║  ███╗█████╗  ██╔██╗ ██║   ██║
╚════██║██╔══██║██╔══██╗██║   ██║██╔══╝  ██║╚██╗██║   ██║
███████║██║  ██║██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║
╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝

# Sargent Upholstery Co. Website

## Executive Summary

This repository contains a static website for **Sargent Upholstery Co.**, Jacksonville's premier automotive upholstery shop established in 1935. The site is built with **Hugo**, a fast and flexible static site generator, and features:

- **Responsive design** showcasing upholstery services (automotive, fleet, marine, convertible tops, leather interiors, etc.)
- **Instagram integration shortcode** that fetches and displays recent Instagram posts with a client-side lightbox gallery
- **Google Reviews integration** that fetches and displays customer reviews from Google Maps
- **Custom Hugo theme** (`sargent`) with optimized layouts and partials
- **Performance-optimized** — scores 97/100 mobile, 100/100 desktop on Lighthouse

**Tech Stack:**
- Hugo (static site generator)
- HTML5 / CSS3 / Vanilla JavaScript
- Hugo asset pipeline (CSS minification + fingerprinting, image processing with WebP + srcset)
- Instagram Web API integration (build-time image processing)
- Google Places API (build-time review fetching)
- Cloudflare Pages hosting with GitHub Actions for scheduled review fetching
- Zero database requirements

---

## Quick Start

### Prerequisites
- **Hugo Extended** (0.141.0 or later) — required for image processing and `try` keyword
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
- **Environment variable:** `HUGO_VERSION` = `0.147.8`

A **GitHub Actions** workflow (`.github/workflows/hugo.yml`) runs weekly to fetch fresh Google Reviews:
1. Fetches reviews via `scripts/fetch-reviews.go` (requires `GOOGLE_API_KEY` secret)
2. Commits updated `data/reviews.json` back to the repo
3. The commit triggers a Cloudflare Pages rebuild automatically

**Live site:** https://sargentupholstery.com/

### Google Reviews Setup

The workflow fetches reviews from the Google Places API. To configure:
1. Create a Google API key with Places API access
2. Add it as a repository secret named `GOOGLE_API_KEY`
3. Reviews are saved to `data/reviews.json` and displayed on the homepage

---

## Project Structure

```
sargent-upholstery/
├── assets/
│   ├── css/style.css           # Main stylesheet (processed via Hugo pipes)
│   └── images/old/             # History page photos (Hugo-processed to WebP)
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
│   └── reviews.json            # Google Reviews (fetched at build time)
├── scripts/
│   └── fetch-reviews.go        # Google Reviews fetch script
├── static/
│   ├── favicon.ico
│   ├── fonts/                  # Self-hosted Work Sans woff2
│   ├── _headers                # Cloudflare Pages security + caching headers
│   ├── _redirects              # www → apex domain redirect
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
│   │       ├── instagram-gallery.html
│   │       └── google-reviews.html
│   └── static/
│       └── js/main.js          # Lite YouTube facade + lightbox JS
├── .github/workflows/
│   └── hugo.yml                # CI/CD pipeline
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

---

## Instagram Gallery Shortcode

The `instagram-gallery` shortcode automatically fetches Instagram posts and displays them in an interactive lightbox gallery.

**Usage in Markdown:**
```markdown
{{< instagram-gallery count="20" username="sargentupholsteryco" >}}
```

**Parameters:**
- `username` (default: `sargentupholsteryco`) — Instagram username to fetch posts from
- `count` (default: `20`) — Number of posts to display

**Features:**
- Builds at **compile time** (fast static site, no runtime API calls)
- Automatic image optimization (resize to WebP format)
- Client-side lightbox with keyboard navigation (arrow keys, Escape)
- Lazy loading for performance
- Responsive grid layout

**Note:** Uses Instagram's internal web API. Instagram may change or block access; consider alternatives if fetch fails frequently.

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

- **Hugo Extended** 0.141.0+ (for image processing, asset pipeline, and `try` keyword)
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
