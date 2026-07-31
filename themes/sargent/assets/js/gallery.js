// Instagram gallery lightbox.
//
// Kept free of template values so a single fingerprinted file serves both
// locales: the two translated strings it needs arrive as data attributes on
// the lightbox element. That is also what lets the CSP drop 'unsafe-inline'
// from script-src -- see static/_headers.
(function () {
  var lightbox = document.getElementById('ig-lightbox');
  if (!lightbox) return;

  var lbImg = lightbox.querySelector('.ig-lightbox-img');
  var lbVideo = lightbox.querySelector('.ig-lightbox-video');
  var lbCaption = lightbox.querySelector('.ig-lightbox-link');
  var lbCaptionWrap = lightbox.querySelector('.ig-lightbox-caption');
  var items = document.querySelectorAll('.ig-gallery-item[data-large]');
  var current = 0;

  var postAlt = lightbox.getAttribute('data-post-alt') || '';
  var viewLabel = lightbox.getAttribute('data-view-label') || '';

  var closeBtn = lightbox.querySelector('.ig-lightbox-close');
  var prevBtn = lightbox.querySelector('.ig-lightbox-prev');
  var nextBtn = lightbox.querySelector('.ig-lightbox-next');
  var focusable = [closeBtn, prevBtn, nextBtn];
  var triggerEl = null;

  function show(index) {
    if (index < 0) index = items.length - 1;
    if (index >= items.length) index = 0;
    current = index;
    var item = items[index];
    var video = item.getAttribute('data-video');
    lbVideo.pause();
    if (video) {
      lbImg.style.display = 'none';
      lbVideo.poster = item.getAttribute('data-large');
      lbVideo.src = video;
      lbVideo.load();
      lbVideo.setAttribute('aria-label', item.getAttribute('data-caption') || postAlt);
      lbVideo.style.display = '';
    } else {
      lbVideo.removeAttribute('src');
      lbVideo.load();
      lbVideo.style.display = 'none';
      lbImg.src = item.getAttribute('data-large');
      lbImg.alt = item.getAttribute('data-caption') || postAlt;
      lbImg.style.display = '';
    }
    var caption = item.getAttribute('data-caption');
    lbCaption.textContent = caption || viewLabel;
    lbCaption.href = item.getAttribute('data-link');
    lbCaptionWrap.style.display = '';
    lightbox.classList.add('active');
    document.body.style.overflow = 'hidden';
    closeBtn.focus();
  }

  function close() {
    lbVideo.pause();
    lbVideo.removeAttribute('src');
    lbVideo.load();
    lbVideo.style.display = 'none';
    lbImg.style.display = '';
    lightbox.classList.remove('active');
    document.body.style.overflow = '';
    if (triggerEl) {
      triggerEl.focus();
      triggerEl = null;
    }
  }

  items.forEach(function (item, i) {
    item.style.cursor = 'pointer';
    item.addEventListener('click', function () { triggerEl = item; show(i); });
  });

  closeBtn.addEventListener('click', close);
  prevBtn.addEventListener('click', function () { show(current - 1); });
  nextBtn.addEventListener('click', function () { show(current + 1); });

  lightbox.addEventListener('click', function (e) {
    if (e.target === lightbox) close();
  });

  document.addEventListener('keydown', function (e) {
    if (!lightbox.classList.contains('active')) return;
    if (e.key === 'Escape') close();
    if (e.key === 'ArrowLeft') show(current - 1);
    if (e.key === 'ArrowRight') show(current + 1);
    if (e.key === 'Tab') {
      if (lbVideo.style.display !== 'none') {
        // Video open: let Tab move through the native video controls; only wrap
        // at the lightbox boundary so focus can't escape.
        var all = Array.prototype.slice.call(lightbox.querySelectorAll(
          'button, [href], video, input, select, textarea, [tabindex]:not([tabindex="-1"])'
        )).filter(function (el) { return !el.disabled; });
        var first = all[0];
        var last = all[all.length - 1];
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault();
          last.focus();
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault();
          first.focus();
        }
      } else {
        var idx = focusable.indexOf(document.activeElement);
        if (e.shiftKey) {
          e.preventDefault();
          focusable[(idx - 1 + focusable.length) % focusable.length].focus();
        } else {
          e.preventDefault();
          focusable[(idx + 1) % focusable.length].focus();
        }
      }
    }
  });
})();
