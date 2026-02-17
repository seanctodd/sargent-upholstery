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
- **Performance-optimized** — scores 99/100/100/100 on Lighthouse (Performance/Accessibility/Best Practices/SEO)

**Tech Stack:**
- Hugo (static site generator)
- HTML5 / CSS3 / Vanilla JavaScript
- Hugo asset pipeline (CSS minification + fingerprinting)
- Instagram Web API integration (build-time image processing)
- Google Places API (build-time review fetching)
- GitHub Actions CI/CD with GitHub Pages hosting
- Zero database requirements

---

## Quick Start

### Prerequisites
- **Hugo Extended** (0.123.7 or later) — required for image processing
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

The site is deployed automatically via **GitHub Actions** to **GitHub Pages** on every push to `main`.

The workflow (`.github/workflows/hugo.yml`):
1. Installs Hugo Extended 0.123.7
2. Fetches Google Reviews via `scripts/fetch-reviews.sh` (requires `GOOGLE_API_KEY` secret)
3. Builds with `hugo --minify`
4. Deploys to GitHub Pages

**Live site:** https://seanctodd.github.io/sargent-upholstery/

### Google Reviews Setup

The build fetches reviews from the Google Places API. To configure:
1. Create a Google API key with Places API access
2. Add it as a repository secret named `GOOGLE_API_KEY`
3. Reviews are saved to `data/reviews.json` and displayed on the homepage

---

## Project Structure

```
sargent-upholstery/
├── assets/
│   └── css/style.css           # Main stylesheet (processed via Hugo pipes)
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
│   └── fetch-reviews.sh        # Google Reviews fetch script
├── static/
│   ├── favicon.ico
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

The site is optimized for Core Web Vitals and scores **99/100/100/100** on Google Lighthouse (mobile):

| Metric | Score |
|---|---|
| Performance | 99 |
| Accessibility | 100 |
| Best Practices | 100 |
| SEO | 100 |

Key optimizations:
- **Lite YouTube facade** — YouTube iframe loads only on click, eliminating ~500KB of third-party JS
- **Responsive hero images** — WebP format with 640w/1024w/1920w srcset variants
- **Non-render-blocking fonts** — Google Fonts loaded via preload/onload pattern
- **CSS minification + fingerprinting** — via Hugo asset pipeline
- **HTML minification** — enabled via Hugo config
- **Explicit image dimensions** — width/height on all `<img>` elements to prevent CLS
- **`fetchpriority="high"`** — on hero images for faster LCP discovery

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

- **Hugo Extended** 0.123.7+ (for image processing and asset pipeline)
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
