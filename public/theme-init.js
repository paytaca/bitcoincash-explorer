(function () {
  var KEY = 'bchexplorer.theme';
  function getDark() {
    try {
      var t = localStorage.getItem(KEY);
      if (t === 'dark') return true;
      if (t === 'light') return false;
      return window.matchMedia('(prefers-color-scheme: dark)').matches;
    } catch (e) { return false; }
  }
  function applyDark(dark) {
    document.documentElement.classList.toggle('dark', !!dark);
  }
  function setStored(mode) {
    try { localStorage.setItem(KEY, mode); } catch (e) {}
  }
  try { applyDark(getDark()); } catch (e) {}
  document.addEventListener('DOMContentLoaded', function () {
    function syncButton(btn) {
      if (btn) btn.setAttribute('data-dark', getDark() ? 'true' : 'false');
    }
    document.querySelectorAll('[data-theme-toggle]').forEach(syncButton);
  });
})();
