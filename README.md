███████╗ █████╗ ██████╗  ██████╗ ███████╗███╗   ██╗████████╗
██╔════╝██╔══██╗██╔══██╗██╔════╝ ██╔════╝████╗  ██║╚══██╔══╝
███████╗███████║██████╔╝██║  ███╗█████╗  ██╔██╗ ██║   ██║   
╚════██║██╔══██║██╔══██╗██║   ██║██╔══╝  ██║╚██╗██║   ██║   
███████║██║  ██║██║  ██║╚██████╔╝███████╗██║ ╚████║   ██║   
╚══════╝╚═╝  ╚═╝╚═╝  ╚═╝ ╚═════╝ ╚══════╝╚═╝  ╚═══╝   ╚═╝   

# Sargent Upholstery Co. Website

## 📋 Executive Summary

This repository contains a static website for **Sargent Upholstery Co.**, Jacksonville's premier automotive upholstery shop established in 1935. The site is built with **Hugo**, a fast and flexible static site generator, and features:

- **Beautiful responsive design** showcasing upholstery services (automotive, fleet, convertible tops, leather interiors, etc.)
- **Instagram integration shortcode** that dynamically fetches and displays recent Instagram posts with a client-side lightbox gallery
- **Custom Hugo theme** (`sargent`) with optimized layouts and partials
- **Multiple service pages** documenting automotive upholstery expertise
- **Contact, FAQ, gallery, and history pages** for customer engagement

**Tech Stack:**
- Hugo (static site generator)
- HTML5 / CSS3 / Vanilla JavaScript
- Instagram Web API integration (build-time image processing)
- Zero database requirements

---

## 🚀 Quick Start

### Prerequisites
- **Hugo Extended** (0.87.0 or later) — required for image processing
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
# Generate static HTML in the 'public/' directory
hugo

# The 'public/' folder now contains your website, ready to deploy
```

---

## 🌐 Free Hosting with GitHub Pages

### Setup (5 minutes)

1. **Create a new repository** (if not already done):
   - Visit https://github.com/new
   - Name: `sargent-upholstery`
   - Select **Public** or **Private** (Private requires GitHub Pro for Pages)
   - Don't initialize with README

2. **Add remote and push code** (one-time):
   ```bash
   cd /home/possum/Projects/sargent-upholstery
   git branch -M main
   git remote add origin https://github.com/seanctodd/sargent-upholstery.git
   git push -u origin main
   ```

3. **Enable GitHub Pages**:
   - Go to **Settings** → **Pages**
   - Source: Branch `main` / Folder `/ (root)`
   - **Save**

4. **Add GitHub Actions workflow** for automatic deployment:
   - Create `.github/workflows/deploy.yml`:

   ```yaml
   name: Deploy with Hugo

   on:
     push:
       branches:
         - main

   jobs:
     deploy:
       runs-on: ubuntu-latest
       steps:
         - uses: actions/checkout@v3

         - name: Setup Hugo
           uses: peaceiris/actions-hugo@v4
           with:
             hugo-version: 'latest'
             extended: true

         - name: Build
           run: hugo --minify

         - name: Upload to GitHub Pages
           uses: peaceiris/actions-gh-pages@v3
           with:
             github_token: ${{ secrets.GITHUB_TOKEN }}
             publish_dir: ./public
   ```

5. **Your site will be live at:**
   ```
   https://seanctodd.github.io/sargent-upholstery/
   ```

6. **Update `baseURL` in `hugo.toml`** to reflect your GitHub Pages URL:
   ```toml
   baseURL = 'https://seanctodd.github.io/sargent-upholstery/'
   ```

---

## ☁️ Free Hosting with Cloudflare Pages

### Setup (5-10 minutes)

1. **Create Cloudflare Account**:
   - Visit https://dash.cloudflare.com/sign-up
   - Sign up with email or GitHub

2. **Connect GitHub Repository**:
   - Go to **Pages** (left sidebar) → **Create a project**
   - Select **Connect to Git** → authorize GitHub
   - Choose `sargent-upholstery` repository
   - Click **Begin setup**

3. **Configure Build Settings**:
   - **Project name**: `sargent-upholstery` (or custom domain name)
   - **Production branch**: `main`
   - **Framework preset**: Select `Hugo`
   - **Build command**: `hugo --minify`
   - **Build output directory**: `public`
   - **Environment variables** (optional): 
     - `HUGO_VERSION` = `0.121.0` (or latest)
   - Click **Save and deploy**

4. **Cloudflare will automatically**:
   - Clone your repo
   - Build with Hugo
   - Deploy to global CDN
   - Provide HTTPS and SSL

5. **Your site will be live at:**
   ```
   https://sargent-upholstery.pages.dev/
   ```

6. **Update `baseURL` in `hugo.toml`**:
   ```toml
   baseURL = 'https://sargent-upholstery.pages.dev/'
   ```

### (Optional) Add Custom Domain to Cloudflare Pages

1. **Register or transfer domain** to Cloudflare (or point nameservers):
   - In Cloudflare dashboard: **Websites** → add your domain

2. **Connect to Pages project**:
   - Go to **Pages** → your project → **Custom domain**
   - Enter your domain (e.g., `sargentupholstery.com`)
   - Cloudflare auto-configures DNS

3. **Update `baseURL` in `hugo.toml`**:
   ```toml
   baseURL = 'https://sargentupholstery.com/'
   ```

---

## 📊 Hosting Comparison

| Feature | GitHub Pages | Cloudflare Pages |
|---------|--------------|-----------------|
| **Cost** | Free | Free |
| **Bandwidth** | Unlimited | Unlimited |
| **Build time** | ~1 min | ~1 min |
| **CDN** | GitHub's CDN | Global Cloudflare CDN |
| **SSL/HTTPS** | Yes | Yes |
| **Custom domain** | Free | Free |
| **Subdomain** | `username.github.io` | `repo-name.pages.dev` |
| **Setup complexity** | Easy (manual or Actions) | Very easy (auto CI/CD) |
| **Recommended** | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |

---

## 🖼️ Project Structure

```
sargent-upholstery/
├── archetypes/              # Hugo content templates
├── content/                 # Markdown pages (Gallery, Contact, Services, etc.)
│   ├── _index.md           # Homepage
│   ├── gallery.md          # Gallery with Instagram integration
│   ├── contact.md
│   ├── estimate.md
│   ├── faq.md
│   ├── our-history.md
│   └── service/            # Service detail pages
├── themes/sargent/         # Custom Hugo theme
│   ├── layouts/
│   │   ├── index.html      # Homepage layout
│   │   ├── partials/       # Reusable components (nav, footer, head)
│   │   ├── _default/       # Default templates (single pages, lists)
│   │   └── shortcodes/     # Custom shortcodes
│   │       └── instagram-gallery.html  # Instagram gallery shortcode
│   └── static/             # CSS, JavaScript, fonts
├── static/                 # Static assets (images, etc.)
├── hugo.toml               # Hugo configuration
└── README.md               # This file
```

---

## 🔧 Instagram Gallery Shortcode

The `instagram-gallery` shortcode automatically fetches Instagram posts from `@sargentupholstery` and displays them in an interactive lightbox gallery.

**Usage in Markdown:**
```markdown
{{< instagram-gallery count="20" username="sargentupholstery" >}}
```

**Parameters:**
- `username` (default: `sargentupholstery`) — Instagram username to fetch posts from
- `count` (default: `20`) — Number of posts to display

**Features:**
- Builds at **compile time** (fast static site, no runtime API calls)
- Automatic image optimization (resize, format conversion)
- Client-side lightbox with keyboard navigation (arrow keys, Escape)
- Lazy loading for performance
- Responsive grid layout

**Note:** Uses Instagram's internal web API. Instagram may change or block access; consider alternatives if fetch fails frequently.

---

## 🛠️ Development

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
- `static/css/` — Stylesheets
- `static/js/` — JavaScript

Changes auto-reload in dev server.

---

## 📦 Dependencies

- **Hugo Extended** 0.87.0+ (for image processing)
- No npm, no Node.js, no database required

---

## 📝 License

Repository contents © Sargent Upholstery Co. All rights reserved.

---

## 🤝 Contributing

For questions or updates, contact the repository owner or submit a pull request.

---

## 📞 Contact

**Sargent Upholstery Co.**
- Address: 44 E 1st St, Jacksonville, FL 32206
- Phone: (904) 355-2529
- Email: sales@sargentupholstery.com
- Instagram: [@sargentupholstery](https://instagram.com/sargentupholstery)
- Facebook: [Sargent Upholstery](https://facebook.com/sargentupholstery)
