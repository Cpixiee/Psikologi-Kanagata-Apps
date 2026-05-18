/* ds_sidebar.js — wiring untuk sidebar dashboard yang dipakai di banyak halaman.
 * - Render icon Lucide.
 * - Logout handler.
 * - Tandai item aktif berdasarkan window.location.pathname jika belum ada
 *   class `active` yang di-set inline.
 */
(function () {
  function initLucide() {
    if (window.lucide && typeof window.lucide.createIcons === 'function') {
      window.lucide.createIcons();
    }
  }

  function bindLogout() {
    function doLogout(e) {
      if (e) e.preventDefault();
      fetch('/api/auth/logout', { method: 'POST' })
        .then(function () { window.location.href = '/login'; })
        .catch(function () { window.location.href = '/login'; });
    }
    var btn = document.getElementById('ds-logout-btn');
    if (btn) btn.addEventListener('click', doLogout);
  }

  function autoActive() {
    var nav = document.querySelector('.ds-sidebar .ds-nav');
    if (!nav) return;
    if (nav.querySelector('a.active')) return; // sudah ditandai inline

    var path = window.location.pathname.replace(/\/+$/, '') || '/';
    var links = nav.querySelectorAll('a[href]');
    var best = null;
    var bestLen = -1;
    links.forEach(function (a) {
      var href = a.getAttribute('href');
      if (!href || href === '#' || href.startsWith('javascript:')) return;
      var clean = href.replace(/\/+$/, '') || '/';
      if (path === clean || (clean !== '/' && path.startsWith(clean + '/'))) {
        if (clean.length > bestLen) { best = a; bestLen = clean.length; }
      }
    });
    if (best) best.classList.add('active');
  }

  function setupMobileDrawer() {
    var aside = document.querySelector('aside.ds-sidebar');
    if (!aside) return;
    if (document.querySelector('.ds-mobile-toggle')) return; // sudah di-inject

    // Tombol hamburger (di-inject sekali)
    var btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'ds-mobile-toggle';
    btn.setAttribute('aria-label', 'Buka menu');
    btn.innerHTML = '<i data-lucide="menu"></i>';
    document.body.appendChild(btn);

    // Backdrop overlay
    var backdrop = document.createElement('div');
    backdrop.className = 'ds-backdrop';
    document.body.appendChild(backdrop);

    function open() { document.body.classList.add('ds-sidebar-open'); btn.setAttribute('aria-label', 'Tutup menu'); btn.innerHTML = '<i data-lucide="x"></i>'; initLucide(); }
    function close() { document.body.classList.remove('ds-sidebar-open'); btn.setAttribute('aria-label', 'Buka menu'); btn.innerHTML = '<i data-lucide="menu"></i>'; initLucide(); }
    function toggle() { document.body.classList.contains('ds-sidebar-open') ? close() : open(); }

    btn.addEventListener('click', toggle);
    backdrop.addEventListener('click', close);

    // Tutup otomatis saat klik link nav (kecuali summary submenu)
    aside.querySelectorAll('a[href]').forEach(function (a) {
      a.addEventListener('click', function () {
        if (window.matchMedia('(max-width: 991.98px)').matches) close();
      });
    });

    // Tutup ketika viewport melebar lagi
    window.addEventListener('resize', function () {
      if (!window.matchMedia('(max-width: 991.98px)').matches) {
        document.body.classList.remove('ds-sidebar-open');
      }
    });

    // ESC untuk menutup
    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') close();
    });
  }

  function fillSideCards() {
    var kelasEl = document.getElementById('sbKelasValue');
    var jurusanEl = document.getElementById('sbJurusanValue');
    if (!kelasEl && !jurusanEl) return; // halaman ini tidak punya side-card
    fetch('/api/profile', { credentials: 'same-origin' })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (res) {
        if (!res) return;
        var u = (res.data && (res.data.user || res.data)) || res || {};
        // Beberapa endpoint membungkus user di res.data, beberapa di root.
        if (res.data && res.data.kelas == null && res.data.user) u = res.data.user;
        var kelas = u.kelas || (res.data && res.data.kelas) || '-';
        var jurusan = u.jurusan || u.asal_instansi ||
          (res.data && (res.data.jurusan || res.data.asal_instansi)) || '-';
        if (kelasEl) kelasEl.textContent = kelas || '-';
        if (jurusanEl) jurusanEl.textContent = jurusan || '-';
      })
      .catch(function () { /* abaikan, biarkan placeholder */ });
  }

  function bootAll() {
    initLucide();
    bindLogout();
    autoActive();
    setupMobileDrawer();
    fillSideCards();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bootAll);
  } else {
    bootAll();
  }
})();
