// Behavioural tests for the lite-YouTube facade in
// themes/sargent/static/js/main.js -- the only hand-written JS that ships
// site-wide (the gallery lightbox script is inline in its shortcode).
//
// Run:  npm install --no-save jsdom && node tests/lite-youtube.test.js
// Pass a path to test a different revision of the file:
//       node tests/lite-youtube.test.js /tmp/old-main.js
//
// jsdom is deliberately NOT a committed dependency -- the project ships with
// no package.json so that a plain `hugo` build stays the only requirement.

const fs = require('fs');
const path = require('path');
const { JSDOM, VirtualConsole } = require('jsdom');

const DEFAULT_SRC = path.join(__dirname, '..', 'themes', 'sargent', 'static', 'js', 'main.js');
const SRC = process.argv[2] || DEFAULT_SRC;
const code = fs.readFileSync(SRC, 'utf8');

// Three facades: a well-formed one, one missing aria-label entirely, and one
// whose label has no "prefix:" to strip.
const HTML = `<!doctype html><html><head><title>Sargent Upholstery Co.</title></head><body>
<div class="lite-youtube" data-videoid="VID1" role="button" tabindex="0" aria-label="Play: Sargent Upholstery Co."><img src="x.jpg" alt="thumb"></div>
<div class="lite-youtube" data-videoid="VID2" role="button" tabindex="0"><img src="x.jpg" alt="thumb"></div>
<div class="lite-youtube" data-videoid="VID3" role="button" tabindex="0" aria-label="NoColonLabel"><img src="x.jpg" alt="thumb"></div>
</body></html>`;

// jsdom does NOT rethrow exceptions raised inside event listeners -- it reports
// them to the virtual console. A try/catch around dispatchEvent therefore passes
// even when a listener throws, so capture listener errors explicitly instead.
const listenerErrors = [];
const virtualConsole = new VirtualConsole();
virtualConsole.on('jsdomError', (e) => listenerErrors.push(e));

const dom = new JSDOM(HTML, { runScripts: 'outside-only', virtualConsole });
const { window } = dom;
const doc = window.document;

window.eval(code);

const results = [];
const ok = (name, cond) => results.push({ name, pass: !!cond });

const els = doc.querySelectorAll('.lite-youtube');
function key(el, k) {
  const e = new window.KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true });
  el.dispatchEvent(e);
  return e;
}

// role="button" must respond to Space, and must not let it scroll the page.
const spaceEvent = key(els[0], ' ');
ok('Space activates the facade', els[0].querySelector('iframe'));
ok('Space calls preventDefault (page must not scroll)', spaceEvent.defaultPrevented === true);

// The iframe is built correctly from the label and video id.
const f0 = els[0].querySelector('iframe');
ok('iframe title strips the "Play: " prefix', f0 && f0.title === 'Sargent Upholstery Co.');
ok('iframe src embeds the videoid', f0 && f0.src.includes('/embed/VID1'));
ok('iframe src targets youtube.com/embed', f0 && f0.src.startsWith('https://www.youtube.com/embed/'));

// Once the real player is in place the wrapper is no longer a button.
ok('role removed after activation', !els[0].hasAttribute('role'));
ok('tabindex removed after activation', !els[0].hasAttribute('tabindex'));

// Re-activating must not rebuild the iframe (which would restart playback).
const before = els[0].querySelector('iframe');
key(els[0], 'Enter');
els[0].dispatchEvent(new window.MouseEvent('click', { bubbles: true }));
ok('re-activation does not rebuild the iframe', els[0].querySelector('iframe') === before);
ok('exactly one iframe after re-activation', els[0].querySelectorAll('iframe').length === 1);

// A missing aria-label must not throw, and must still activate.
const errsBefore = listenerErrors.length;
key(els[1], 'Enter');
ok('missing aria-label does not throw', listenerErrors.length === errsBefore);
ok('missing aria-label still activates', els[1].querySelector('iframe'));
const f1 = els[1].querySelector('iframe');
ok('missing aria-label falls back to document.title', f1 && f1.title === doc.title);

// A label with no "prefix:" is preserved verbatim, and Enter still works.
key(els[2], 'Enter');
const f2 = els[2].querySelector('iframe');
ok('prefix-less label preserved verbatim', f2 && f2.title === 'NoColonLabel');
ok('Enter still activates (no regression)', els[2].querySelector('iframe'));

for (const r of results) console.log(`${r.pass ? 'PASS' : 'FAIL'}  ${r.name}`);
const failed = results.filter((r) => !r.pass).length;
console.log(`\n${results.length - failed}/${results.length} passed`);
process.exit(failed ? 1 : 0);
