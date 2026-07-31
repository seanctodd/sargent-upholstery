(function () {
  document.querySelectorAll('.lite-youtube').forEach(function (el) {
    var activated = false;

    function activate() {
      // Guard re-entry: the keydown handler stays bound after the facade is
      // replaced, and re-running would rebuild the iframe and restart playback.
      if (activated) return;
      activated = true;

      // aria-label is "Play: <video title>"; strip the action prefix to leave
      // the title. Tolerate a missing/prefix-less label rather than throwing.
      var label = el.getAttribute('aria-label') || '';
      var iframe = document.createElement('iframe');
      iframe.src = 'https://www.youtube.com/embed/' + el.dataset.videoid + '?autoplay=1';
      iframe.title = label.replace(/^[^:]*:\s*/, '') || document.title;
      iframe.allow = 'accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture';
      iframe.setAttribute('allowfullscreen', '');
      iframe.style.cssText = 'position:absolute;top:0;left:0;width:100%;height:100%;border:none';
      el.textContent = '';
      el.appendChild(iframe);

      // The facade is gone, so stop advertising the wrapper as a button.
      el.removeAttribute('role');
      el.removeAttribute('tabindex');
      el.removeAttribute('aria-label');
    }

    el.addEventListener('click', activate);
    el.addEventListener('keydown', function (e) {
      // A role="button" must respond to Space as well as Enter, and Space
      // must not fall through to scrolling the page.
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        activate();
      }
    });
  });
})();
