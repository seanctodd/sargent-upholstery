// Behavioural tests for the Instagram gallery lightbox in
// themes/sargent/assets/js/gallery.js.
//
// Run:  npm install --no-save jsdom && node tests/gallery.test.js
// Pass a path to test a different build of the file (e.g. the minified output).

const fs = require('fs');
const path = require('path');
const { JSDOM, VirtualConsole } = require('jsdom');

const DEFAULT_SRC = path.join(__dirname, '..', 'themes', 'sargent', 'assets', 'js', 'gallery.js');
const code = fs.readFileSync(process.argv[2] || DEFAULT_SRC, 'utf8');

// Two tiles: one photo, one video. The translated strings arrive as data
// attributes so a single file can serve both locales.
const HTML = `<!doctype html><html><body>
<div class="ig-gallery">
  <button class="ig-gallery-item" data-large="/big-a.webp" data-caption="A caption" data-link="https://instagram.com/p/a"><img src="/a.webp"></button>
  <button class="ig-gallery-item is-video" data-large="/big-b.webp" data-caption="" data-link="https://instagram.com/p/b" data-video="https://media.example/b.mp4"><img src="/b.webp"></button>
</div>
<div class="ig-lightbox" id="ig-lightbox" data-post-alt="Publicación de Instagram" data-view-label="Ver publicación">
  <button class="ig-lightbox-close"></button>
  <button class="ig-lightbox-prev"></button>
  <button class="ig-lightbox-next"></button>
  <div class="ig-lightbox-content">
    <img class="ig-lightbox-img" src="" alt="">
    <video class="ig-lightbox-video" style="display:none"></video>
    <div class="ig-lightbox-caption"><a class="ig-lightbox-link" href=""></a></div>
  </div>
</div>
</body></html>`;

const listenerErrors = [];
const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', (e) => listenerErrors.push(e));

const dom = new JSDOM(HTML, { runScripts: 'outside-only', virtualConsole });
const { window } = dom;
const doc = window.document;

// jsdom implements neither play/pause nor load on <video>; stub them so the
// script under test can call them as it would in a browser.
window.HTMLMediaElement.prototype.play = function () {};
window.HTMLMediaElement.prototype.pause = function () {};
window.HTMLMediaElement.prototype.load = function () {};

window.eval(code);

const results = [];
const ok = (name, cond) => results.push({ name, pass: !!cond });

const tiles = doc.querySelectorAll('.ig-gallery-item');
const lb = doc.getElementById('ig-lightbox');
const lbImg = lb.querySelector('.ig-lightbox-img');
const lbVideo = lb.querySelector('.ig-lightbox-video');
const lbLink = lb.querySelector('.ig-lightbox-link');
const click = (el) => el.dispatchEvent(new window.MouseEvent('click', { bubbles: true }));
const key = (k) => doc.dispatchEvent(new window.KeyboardEvent('keydown', { key: k, bubbles: true }));

// --- photo tile ---
click(tiles[0]);
ok('clicking a tile opens the lightbox', lb.classList.contains('active'));
ok('photo tile shows the large image', lbImg.getAttribute('src') === '/big-a.webp');
ok('photo tile hides the video element', lbVideo.style.display === 'none');
ok('caption text is used as the link label', lbLink.textContent === 'A caption');
ok('link points at the permalink', lbLink.getAttribute('href') === 'https://instagram.com/p/a');
ok('body scroll is locked while open', doc.body.style.overflow === 'hidden');

// --- Escape closes and restores scrolling ---
key('Escape');
ok('Escape closes the lightbox', !lb.classList.contains('active'));
ok('body scroll is restored on close', doc.body.style.overflow === '');

// --- video tile, and the localized fallbacks from data attributes ---
click(tiles[1]);
ok('video tile shows the video element', lbVideo.style.display !== 'none');
ok('video tile hides the image', lbImg.style.display === 'none');
ok('video src comes from data-video', lbVideo.getAttribute('src') === 'https://media.example/b.mp4');
ok('poster falls back to the large image', lbVideo.getAttribute('poster') === '/big-b.webp');
ok('empty caption uses the localized view label', lbLink.textContent === 'Ver publicación');
ok('empty caption uses the localized alt on the video',
  lbVideo.getAttribute('aria-label') === 'Publicación de Instagram');

// --- arrow navigation wraps ---
key('ArrowRight');
ok('ArrowRight wraps back to the first tile', lbImg.getAttribute('src') === '/big-a.webp');
key('ArrowLeft');
ok('ArrowLeft wraps to the last tile', lbVideo.getAttribute('src') === 'https://media.example/b.mp4');

// --- nothing threw along the way ---
ok('no listener errors were raised', listenerErrors.length === 0);

// --- the script must not throw on pages with no gallery ---
const bare = new JSDOM('<!doctype html><html><body></body></html>', { runScripts: 'outside-only' });
let threw = false;
try { bare.window.eval(code); } catch (_) { threw = true; }
ok('no-op on a page with no lightbox', !threw);

for (const r of results) console.log(`${r.pass ? 'PASS' : 'FAIL'}  ${r.name}`);
const failed = results.filter((r) => !r.pass).length;
console.log(`\n${results.length - failed}/${results.length} passed`);
process.exit(failed ? 1 : 0);
