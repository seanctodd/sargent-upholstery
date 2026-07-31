// Google Analytics loader.
//
// Replaces Hugo's _internal/google_analytics.html, which emits this inline and
// so forced 'unsafe-inline' into the CSP's script-src. Behaviour is the same,
// including honouring Do Not Track. Rendered through ExecuteAsTemplate so the
// measurement ID comes from hugo.toml rather than being duplicated here.
(function () {
  var dnt = navigator.doNotTrack || window.doNotTrack || navigator.msDoNotTrack;
  if (dnt === '1' || dnt === 'yes') return;

  window.dataLayer = window.dataLayer || [];
  function gtag() { window.dataLayer.push(arguments); }
  gtag('js', new Date());
  gtag('config', '{{ . }}');
})();
