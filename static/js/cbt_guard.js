// CBT Guard (Anti-Cheat) for IST subtests.
// Catatan: di browser tidak mungkin memblokir Alt+Tab / screenshot 100%.
// Script ini fokus pada deteksi + deterrence + auto-submit saat pelanggaran mencapai limit.
(function () {
  'use strict';

  const CFG = window.__CBT_GUARD_CFG__ || {};
  const enabled = CFG.enabled === true;
  const limit = Number(CFG.violationLimit || 3);
  const violationApi = CFG.violationApi || '/api/test/ist/violation';
  const alarmSrc = CFG.alarmSrc || '/static/allert/Allert.mp3';
  const showToast = typeof window.__istShowToast === 'function' ? window.__istShowToast : null;

  if (!enabled) return;

  let hasStarted = false; // true setelah ada user gesture (click/keydown) => audio bisa autoplay
  let isSubmitting = false;

  // Alarm audio (dipakai hanya saat pelanggaran)
  const alarm = new Audio(alarmSrc);
  alarm.preload = 'auto';
  alarm.loop = false;
  alarm.volume = 1.0;

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

  function reportViolation(type, meta) {
    playAlarm();
    toast('Peringatan: aktivitas terdeteksi (' + type + '). Pelanggaran akan dicatat.', 'warning');

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
        if (count >= lim && data.force_submit) {
          toast('Batas pelanggaran tercapai (' + count + '/' + lim + '). Ujian dihentikan & auto submit.', 'danger');
          forceSubmitNow(type);
        } else {
          toast('Pelanggaran: ' + count + '/' + lim, 'warning');
        }
      })
      .catch(function () {
        // Jika API gagal, tetap tahan secara UI (tanpa auto-submit)
      });
  }

  // Mark "started" setelah gesture pertama (supaya audio bisa play)
  window.addEventListener('mousedown', function () { hasStarted = true; }, { capture: true, once: true });
  window.addEventListener('keydown', function () { hasStarted = true; }, { capture: true, once: true });

  // Deteksi: tab switching / minimize / pindah aplikasi
  document.addEventListener('visibilitychange', function () {
    if (document.hidden) reportViolation('hidden', 'visibilitychange');
  });
  window.addEventListener('blur', function () {
    reportViolation('blur', 'window blur');
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
      (k === 'printscreen') ||
      (k === 'escape');

    if (blocked) {
      e.preventDefault();
      e.stopPropagation();
      reportViolation('keydown', (e.key || '') + (ctrl ? '+ctrl' : '') + (e.shiftKey ? '+shift' : ''));
      return false;
    }
  }, { capture: true });

  // Saat halaman ujian dimuat, coba masuk fullscreen (kalau belum)
  // Request fullscreen biasanya butuh user gesture, tapi beberapa browser akan allow jika navigasi dari klik.
  ensureFullscreen();
})();

