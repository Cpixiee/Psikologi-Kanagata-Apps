// CBT Guard (Anti-Cheat) for IST subtests.
// Catatan: di browser tidak mungkin memblokir Alt+Tab / screenshot 100%.
// Script ini fokus pada deteksi + deterrence + auto-submit saat pelanggaran mencapai limit.
(function () {
  'use strict';

  const CFG = window.__CBT_GUARD_CFG__ || {};
  const enabled = CFG.enabled === true;
  const limit = Number(CFG.violationLimit || 5);
  const violationApi = CFG.violationApi || '/api/test/ist/violation';
  const alarmSrc = CFG.alarmSrc || '/static/allert/Allert.mp3';
  const showToast = typeof window.__istShowToast === 'function' ? window.__istShowToast : null;

  if (!enabled) return;

  let hasStarted = false; // true setelah ada user gesture (click/keydown/touch) => audio bisa autoplay
  let isSubmitting = false;

  // Alarm audio (dipakai hanya saat pelanggaran)
  const alarm = new Audio(alarmSrc);
  alarm.preload = 'auto';
  alarm.loop = false;
  alarm.volume = 1.0;

  // Grace period: abaikan event dalam 3 detik pertama sejak halaman dimuat.
  // Mencegah false-positive di HP saat halaman baru dibuka dan browser trigger visibilitychange.
  const PAGE_LOAD_TIME = Date.now();
  const GRACE_PERIOD_MS = 3000;

  function toast(msg, variant) {
    if (showToast) showToast(msg, variant);
    else alert(msg);
  }

  function ensureFullscreen() {
    const el = document.documentElement;
    if (document.fullscreenElement) return Promise.resolve(true);
    if (!el.requestFullscreen) return Promise.resolve(false);
    return el.requestFullscreen().then(
      () => true,
      () => false
    );
  }

  function playAlarm() {
    if (!hasStarted) return;
    try {
      alarm.currentTime = 0;
      // Jika browser block, abaikan (masih ada toast)
      alarm.play().catch(function () {});
    } catch (e) {
      // ignore
    }
  }

  function forceSubmitNow(reason) {
    if (isSubmitting) return;
    isSubmitting = true;

    // Prefer submit handler milik halaman IST (agar tetap konsisten)
    if (typeof window.__istForceSubmit === 'function') {
      window.__istForceSubmit({ force_submit: true, violation_src: reason || '' });
      return;
    }

    // Fallback (kalau halaman belum expose function)
    toast('Ujian dihentikan otomatis. Mengirim jawaban...', 'warning');
    setTimeout(function () {
      window.location.reload();
    }, 1200);
  }

  let lastViolationTime = 0;
  const VIOLATION_COOLDOWN_MS = 3000; // Cooldown 3 detik — lebih panjang untuk HP agar tidak double-fire

  function reportViolation(type, meta) {
    // Abaikan event dalam grace period setelah halaman dimuat (false-positive di HP)
    if (Date.now() - PAGE_LOAD_TIME < GRACE_PERIOD_MS) {
      return;
    }

    const now = Date.now();
    if (now - lastViolationTime < VIOLATION_COOLDOWN_MS) {
      return; // Cooldown: abaikan event duplikat yang terjadi hampir bersamaan
    }
    lastViolationTime = now;

    playAlarm();

    fetch(violationApi, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: type || 'unknown', meta: meta || '' })
    })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        if (!data || !data.success) return;
        const count = Number(data.count || 0);
        const lim = Number(data.limit || limit);
        if (count >= lim) {
          // Batas tercapai → auto submit (dengan delay 1.5 detik agar toast sempat terbaca)
          toast('⛔ Pelanggaran ke-' + count + '/' + lim + ': Ujian dihentikan & jawaban otomatis terkirim!', 'danger');
          setTimeout(function () { forceSubmitNow(type); }, 1500);
        } else {
          // Tampilkan peringatan ke-N dari total limit
          const remaining = lim - count;
          toast(
            '⚠️ Peringatan ' + count + '/' + lim + ': Jangan pindah tab! ' +
            remaining + ' peringatan lagi sebelum ujian dihentikan otomatis.',
            'warning'
          );
        }
      })
      .catch(function () {
        // Jika API gagal, tampilkan peringatan lokal saja tapi JANGAN auto-submit
        toast('⚠️ Peringatan: Jangan pindah tab/aplikasi saat ujian berlangsung!', 'warning');
      });
  }

  // Mark "started" setelah gesture pertama (supaya audio bisa play)
  window.addEventListener('mousedown', function () { hasStarted = true; }, { capture: true, once: true });
  window.addEventListener('keydown', function () { hasStarted = true; }, { capture: true, once: true });
  window.addEventListener('touchstart', function () { hasStarted = true; }, { capture: true, once: true, passive: true });

  // Deteksi: tab switching / minimize (hanya jika tab benar-benar tersembunyi)
  document.addEventListener('visibilitychange', function () {
    if (document.hidden) {
      reportViolation('hidden', 'visibilitychange');
    }
  });

  // Deteksi: keluar fullscreen
  document.addEventListener('fullscreenchange', function () {
    if (!document.fullscreenElement) {
      reportViolation('exit_fullscreen', 'fullscreenchange');
      // Coba paksa balik fullscreen (best effort)
      ensureFullscreen();
    }
  });

  // Disable copy/paste/cut/context menu
  document.addEventListener('copy', function (e) { e.preventDefault(); reportViolation('copy', ''); });
  document.addEventListener('paste', function (e) { e.preventDefault(); reportViolation('paste', ''); });
  document.addEventListener('cut', function (e) { e.preventDefault(); reportViolation('cut', ''); });
  document.addEventListener('contextmenu', function (e) { e.preventDefault(); reportViolation('contextmenu', ''); });

  // Keyboard blocking (best effort)
  document.addEventListener('keydown', function (e) {
    const k = (e.key || '').toLowerCase();
    const ctrl = e.ctrlKey || e.metaKey;

    // Prevent common escape shortcuts / devtools
    const blocked =
      (ctrl && (k === 'c' || k === 'v' || k === 'x' || k === 'p' || k === 's')) ||
      (ctrl && e.shiftKey && (k === 'i' || k === 'j' || k === 'c')) ||
      (k === 'f12') ||
      (k === 'printscreen');

    if (blocked) {
      e.preventDefault();
      e.stopPropagation();
      reportViolation('keydown', (e.key || '') + (ctrl ? '+ctrl' : '') + (e.shiftKey ? '+shift' : ''));
      return false;
    }
  }, { capture: true });

  // Saat halaman ujian dimuat, coba masuk fullscreen (kalau belum)
  ensureFullscreen();
})();
