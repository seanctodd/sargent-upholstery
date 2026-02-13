// Mobile nav toggle
(function () {
  var toggle = document.querySelector('.nav-toggle');
  var nav = document.getElementById('site-nav');
  if (!toggle || !nav) return;

  toggle.addEventListener('click', function () {
    var expanded = toggle.getAttribute('aria-expanded') === 'true';
    toggle.setAttribute('aria-expanded', String(!expanded));
    nav.classList.toggle('open');
  });

  // Touch-friendly dropdown toggle for mobile
  var dropdownParents = document.querySelectorAll('.has-dropdown > a');
  dropdownParents.forEach(function (link) {
    link.addEventListener('click', function (e) {
      if (window.innerWidth <= 768) {
        var parent = link.parentElement;
        var isOpen = parent.classList.contains('dropdown-open');
        // Close any other open dropdowns
        document.querySelectorAll('.has-dropdown.dropdown-open').forEach(function (el) {
          el.classList.remove('dropdown-open');
          el.querySelector('a').setAttribute('aria-expanded', 'false');
        });
        if (!isOpen) {
          e.preventDefault();
          parent.classList.add('dropdown-open');
          link.setAttribute('aria-expanded', 'true');
        }
      }
    });
  });
})();
