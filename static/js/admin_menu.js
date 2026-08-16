// Menampilkan menu admin & sekolah di sidebar berdasarkan role user.
// Berlaku untuk semua halaman yang memuat skrip ini.
(function () {
  function applyRole(role) {
    var canSeeAdmin = role === "admin" || role === "sekolah";
    var isStrictAdmin = role === "admin";
    var isStudent = role === "siswa";

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
    document.querySelectorAll(".student-only").forEach(function (el) {
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
  }

  function init() {
    try {
      var cached = sessionStorage.getItem("user_role");
      if (cached) applyRole(cached);
    } catch (e) {}

    fetch("/api/profile")
      .then(function (r) { return r.ok ? r.json() : null; })
      .then(function (d) {
        if (d && d.success && d.data) {
          var u = d.data.user || d.data;
          var role = typeof d.data.role === "string" ? d.data.role : (u.role || "");
          if (role) {
            try { sessionStorage.setItem("user_role", role); } catch (e) {}
            applyRole(role);
          }
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
