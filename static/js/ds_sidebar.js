/* ds_sidebar.js — wiring untuk sidebar dashboard yang dipakai di banyak halaman.
 * - Render icon Lucide.
 * - Logout handler (swapped to exit-impersonate if impersonating).
 * - Inject "Daftar Siswa" for school/admin roles.
 * - Impersonation ribbon for active impersonation sessions.
 * - Tandai item aktif berdasarkan window.location.pathname jika belum ada
 *   class `active` yang di-set inline.
 */
(function () {
  function initLucide() {
    if (window.lucide && typeof window.lucide.createIcons === 'function') {
      window.lucide.createIcons();
    }
  }

  function handleExitImpersonate(e) {
    if (e) e.preventDefault();
    fetch('/api/auth/exit-impersonate', { method: 'POST' })
      .then(function (r) { return r.json(); })
      .then(function (res) {
        if (res && res.success) {
          window.location.href = '/dashboard/students';
        } else {
          window.location.href = '/dashboard';
        }
      })
      .catch(function () {
        window.location.href = '/dashboard';
      });
  }

  function interceptLogoutForImpersonation() {
    var logoutBtns = document.querySelectorAll('#ds-logout-btn, #ds-logout-btn-2, a[href*="logout"]');
    logoutBtns.forEach(function (btn) {
      btn.innerHTML = '<i data-lucide="arrow-left-circle" class="ds-icon"></i> Kembali ke Akun Sekolah';
      
      // Clone to discard old event listeners
      var newBtn = btn.cloneNode(true);
      if (btn.parentNode) {
        btn.parentNode.replaceChild(newBtn, btn);
        newBtn.addEventListener('click', handleExitImpersonate);
      }
    });
    initLucide();
  }

  function showImpersonationRibbon(studentName) {
    if (document.getElementById('impersonation-warning-ribbon')) return;

    var ribbon = document.createElement('div');
    ribbon.id = 'impersonation-warning-ribbon';
    ribbon.style.cssText = 'position: fixed; top: 0; left: 0; right: 0; height: 44px; background: linear-gradient(135deg, #ea580c 0%, #c2410c 100%); color: #fff; display: flex; align-items: center; justify-content: space-between; padding: 0 24px; font-family: system-ui, -apple-system, sans-serif; font-size: 14px; font-weight: 500; box-shadow: 0 4px 12px rgba(0,0,0,0.15); z-index: 99999; border-bottom: 1px solid rgba(255,255,255,0.1);';
    
    var infoText = document.createElement('span');
    infoText.style.display = 'flex';
    infoText.style.alignItems = 'center';
    infoText.style.gap = '8px';
    infoText.innerHTML = '<i data-lucide="eye" style="width: 18px; height: 18px;"></i> Anda sedang mengakses akun siswa: <strong>' + studentName + '</strong>';
    
    var exitBtn = document.createElement('button');
    exitBtn.type = 'button';
    exitBtn.style.cssText = 'background: rgba(255, 255, 255, 0.2); border: 1px solid rgba(255, 255, 255, 0.4); color: #fff; padding: 6px 16px; border-radius: 6px; cursor: pointer; font-size: 13px; font-weight: 600; display: inline-flex; align-items: center; gap: 6px; transition: all 0.2s ease;';
    exitBtn.innerHTML = '<i data-lucide="arrow-left-circle" style="width: 14px; height: 14px;"></i> Keluar Akses Siswa';
    
    exitBtn.addEventListener('mouseenter', function() {
      exitBtn.style.background = 'rgba(255, 255, 255, 0.3)';
    });
    exitBtn.addEventListener('mouseleave', function() {
      exitBtn.style.background = 'rgba(255, 255, 255, 0.2)';
    });
    
    exitBtn.addEventListener('click', handleExitImpersonate);

    ribbon.appendChild(infoText);
    ribbon.appendChild(exitBtn);
    document.body.prepend(ribbon);
    
    var shell = document.querySelector('.ds-shell');
    if (shell) {
      shell.style.marginTop = '44px';
    } else {
      document.body.style.marginTop = '44px';
    }

    var sidebar = document.querySelector('.ds-sidebar');
    if (sidebar) {
      sidebar.style.top = '44px';
      sidebar.style.height = 'calc(100vh - 44px)';
    }
    
    initLucide();
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

    // Remove existing active classes first to recompute cleanly after link injection
    nav.querySelectorAll('a.active').forEach(function (a) {
      a.classList.remove('active');
    });

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

    window.addEventListener('resize', function () {
      if (!window.matchMedia('(max-width: 991.98px)').matches) {
        document.body.classList.remove('ds-sidebar-open');
      }
    });

    document.addEventListener('keydown', function (e) {
      if (e.key === 'Escape') close();
    });
  }

  function loadProfileAndConfigureSidebar() {
    var kelasEl = document.getElementById('sbKelasValue');
    var jurusanEl = document.getElementById('sbJurusanValue');

    fetch('/api/profile', { credentials: 'same-origin' })
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (res) {
        if (!res) return;
        var u = (res.data && (res.data.user || res.data)) || res || {};
        if (res.data && res.data.kelas == null && res.data.user) u = res.data.user;
        
        var role = (u.role || "").toLowerCase();
        var isImpersonated = res.data && res.data.is_impersonated;

        // Populate side cards if elements exist
        if (kelasEl || jurusanEl) {
          var labelKelas = kelasEl ? kelasEl.previousElementSibling : null;
          var labelJurusan = jurusanEl ? jurusanEl.previousElementSibling : null;

          if (role === "sekolah") {
            if (labelKelas) labelKelas.textContent = "Asal Sekolah";
            if (kelasEl) kelasEl.textContent = u.sekolah || "-";
            if (labelJurusan) labelJurusan.textContent = "Bidang";
            if (jurusanEl) {
              jurusanEl.textContent = "BK";
              jurusanEl.style.fontSize = "14px";
            }
          } else {
            var kelas = u.kelas || (res.data && res.data.kelas) || '-';
            var jurusan = u.jurusan || u.asal_instansi ||
              (res.data && (res.data.jurusan || res.data.asal_instansi)) || '-';
            if (labelKelas) labelKelas.textContent = "Kelas";
            if (kelasEl) kelasEl.textContent = kelas || '-';
            if (labelJurusan) labelJurusan.textContent = "Jurusan";
            if (jurusanEl) {
              jurusanEl.textContent = jurusan || '-';
              jurusanEl.style.fontSize = "14px";
            }
          }
        }

        // Dynamic menu visibility based on role
        var nav = document.querySelector('.ds-sidebar .ds-nav');
        if (nav) {
          var isStudent = role === "siswa";
          var isStaff = role === "sekolah" || role === "admin";

          nav.querySelectorAll('.sekolah-or-admin-only').forEach(function (el) {
            if (isStaff) {
              el.hidden = false;
              el.removeAttribute("hidden");
              el.style.setProperty("display", "", "important");
            } else {
              el.hidden = true;
              el.setAttribute("hidden", "");
              el.style.setProperty("display", "none", "important");
            }
          });

          nav.querySelectorAll('.student-only').forEach(function (el) {
            if (isStudent) {
              el.hidden = false;
              el.removeAttribute("hidden");
              el.style.setProperty("display", "", "important");
            } else {
              el.hidden = true;
              el.setAttribute("hidden", "");
              el.style.setProperty("display", "none", "important");
            }
          });

          // Handle AI Asisten link removal if any remaining
          nav.querySelectorAll('a[href="/ai"]').forEach(function (el) {
            el.remove();
          });

          autoActive();
          initLucide();
        }

        // Impersonation ribbon & logout modification
        if (isImpersonated) {
          showImpersonationRibbon(u.nama_lengkap || u.email);
          interceptLogoutForImpersonation();
        }
      })
      .catch(function () { /* ignore */ });
  }

  function bootAll() {
    initLucide();
    bindLogout();
    autoActive();
    setupMobileDrawer();
    loadProfileAndConfigureSidebar();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', bootAll);
  } else {
    bootAll();
  }
})();
