// Menampilkan menu admin di sidebar (elemen ber-class .admin-only) untuk
// role 'admin' dan 'sekolah'. Untuk role 'sekolah', elemen tertentu yang
// hanya boleh diakses admin (ber-class .strict-admin-only, mis. link
// "Tambah Batch", link "Akun Sekolah", tombol create batch) tetap disembunyikan.
// Berlaku untuk semua halaman yang memuat skrip ini.
(function () {
  function applyRole(role) {
    var canSeeAdmin = role === "admin" || role === "sekolah";
    var isStrictAdmin = role === "admin";
    document.querySelectorAll(".admin-only").forEach(function (el) {
      if (canSeeAdmin) {
        el.hidden = false;
        el.removeAttribute("hidden");
      } else {
        el.hidden = true;
        el.setAttribute("hidden", "");
      }
    });
    document.querySelectorAll(".sekolah-or-admin-only").forEach(function (el) {
      if (canSeeAdmin) {
        el.hidden = false;
        el.removeAttribute("hidden");
        el.style.setProperty("display", "", "important");
      } else {
        el.hidden = true;
        el.setAttribute("hidden", "");
        el.style.setProperty("display", "none", "important");
      }
    });
    document.querySelectorAll(".strict-admin-only").forEach(function (el) {
      if (isStrictAdmin) {
        el.hidden = false;
        el.removeAttribute("hidden");
        el.classList.remove("d-none");
      } else {
        el.hidden = true;
        el.setAttribute("hidden", "");
        el.classList.add("d-none");
      }
    });
  }

  function init() {
    // Terapkan role yang dicache agar tidak ada flicker.
    try {
      var cached = sessionStorage.getItem("user_role");
      if (cached) applyRole(cached);
    } catch (e) {}

    // Refresh dari API supaya tetap akurat.
    fetch("/api/profile")
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (d) {
        if (d && d.success && d.data && typeof d.data.role === "string") {
          try { sessionStorage.setItem("user_role", d.data.role); } catch (e) {}
          applyRole(d.data.role);
        }
      })
      .catch(function () {});
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
