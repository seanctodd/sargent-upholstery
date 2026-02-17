(function () {
  document.querySelectorAll('.lite-youtube').forEach(function (el) {
    function activate() {
      var iframe = document.createElement('iframe');
      iframe.src = 'https://www.youtube.com/embed/' + el.dataset.videoid + '?autoplay=1';
      iframe.title = el.getAttribute('aria-label').replace('Play: ', '');
      iframe.allow = 'accelerometer; autoplay; encrypted-media; gyroscope; picture-in-picture';
      iframe.allowFullscreen = true;
      iframe.style.cssText = 'position:absolute;top:0;left:0;width:100%;height:100%;border:none';
      el.textContent = '';
      el.appendChild(iframe);
      el.removeEventListener('click', activate);
    }
    el.addEventListener('click', activate);
    el.addEventListener('keydown', function (e) {
      if (e.key === 'Enter') activate();
    });
  });
})();
