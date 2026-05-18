/*
 * Onboarding modal logic.
 *
 * - Mengecek status profil saat halaman dashboard dibuka.
 * - Menampilkan modal pengisian (NISN/NIP, kelas, jurusan, TTL, alamat regional).
 * - Cascading wilayah memakai helper global window.setupWilayahCascading
 *   (di /static/modern/wilayah.js).
 * - Tombol "Simpan & Mulai Test": POST /api/profile/onboarding lalu redirect /test.
 *
 * Markup yang dibutuhkan ada di views/dashboard.html (modal #onboardingModal).
 */
(function () {
  "use strict";

  async function setupCascading(initial) {
    if (typeof window.setupWilayahCascading === "function") {
      await window.setupWilayahCascading("ob_provinsi", "ob_kota", "ob_kecamatan", initial);
    }
  }

  function getValue(id) {
    const el = document.getElementById(id);
    return el ? (el.value || "").trim() : "";
  }

  async function submitOnboarding() {
    const jurusanSelect = document.getElementById("ob_jurusan");
    const jurusanCustom = document.getElementById("ob_jurusan_custom");
    const jurusanValue = (window.AppEnums && window.AppEnums.getJurusanValue)
      ? window.AppEnums.getJurusanValue(jurusanSelect, jurusanCustom)
      : getValue("ob_jurusan");

    const payload = {
      nisn: getValue("ob_nisn"),
      nip: getValue("ob_nip"),
      kelas: getValue("ob_kelas"),
      jurusan: jurusanValue,
      sekolah: getValue("ob_sekolah"),
      tempat_lahir: getValue("ob_tempat_lahir"),
      tanggal_lahir: getValue("ob_tanggal_lahir"),
      alamat: getValue("ob_alamat"),
      kecamatan: getValue("ob_kecamatan"),
      kota: getValue("ob_kota"),
      provinsi: getValue("ob_provinsi"),
    };

    if (!payload.nisn && !payload.nip) {
      Swal.fire({ icon: "warning", title: "NISN atau NIP wajib diisi", confirmButtonText: "OK" });
      return false;
    }
    const required = ["kelas", "jurusan", "sekolah", "tempat_lahir", "tanggal_lahir", "kecamatan", "kota", "provinsi"];
    for (const k of required) {
      if (!payload[k]) {
        Swal.fire({ icon: "warning", title: "Lengkapi semua field", text: "Field " + k.replace(/_/g, " ") + " wajib diisi", confirmButtonText: "OK" });
        return false;
      }
    }

    const res = await fetch("/api/profile/onboarding", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload),
    });
    const data = await res.json();
    if (!res.ok || !data.success) {
      Swal.fire({ icon: "error", title: "Gagal menyimpan", text: data.message || "Terjadi kesalahan" });
      return false;
    }
    return true;
  }

  async function showOnboardingModalIfNeeded() {
    let res, data;
    try {
      res = await fetch("/api/profile/onboarding-status");
      data = await res.json();
    } catch (err) {
      return; // jangan ganggu user kalau API gagal
    }
    if (!data || !data.success || !data.data) return;
    if (data.data.completed) return; // sudah lengkap, tidak perlu modal
    // Admin tidak perlu onboarding peserta — modal disembunyikan.
    if (data.data.role === "admin") return;

    const modalEl = document.getElementById("onboardingModal");
    if (!modalEl || !window.bootstrap) return;

    // Pre-fill bila ada nilai eksisting
    const d = data.data;
    if (d.nisn) document.getElementById("ob_nisn").value = d.nisn;
    if (d.nip) document.getElementById("ob_nip").value = d.nip;
    if (d.sekolah) {
      const sk = document.getElementById("ob_sekolah");
      if (sk) sk.value = d.sekolah;
    }
    if (d.tempat_lahir) document.getElementById("ob_tempat_lahir").value = d.tempat_lahir;
    if (d.tanggal_lahir) document.getElementById("ob_tanggal_lahir").value = d.tanggal_lahir;
    if (d.alamat) document.getElementById("ob_alamat").value = d.alamat;

    // Populate enum kelas & jurusan (pakai helper shared).
    if (window.AppEnums) {
      window.AppEnums.populateKelasSelect(
        document.getElementById("ob_kelas"),
        d.kelas || ""
      );
      window.AppEnums.populateJurusanSelect({
        select: document.getElementById("ob_jurusan"),
        customInput: document.getElementById("ob_jurusan_custom"),
        descriptionEl: document.getElementById("ob_jurusan_desc"),
        currentValue: d.jurusan || "",
      });
    }

    await setupCascading({
      provinsi: d.provinsi || "",
      kota: d.kota || "",
      kecamatan: d.kecamatan || "",
    });

    const modal = new bootstrap.Modal(modalEl, { backdrop: "static", keyboard: false });
    modal.show();

    const btn = document.getElementById("ob_btnSimpanMulai");
    if (btn) {
      btn.addEventListener("click", async function () {
        btn.disabled = true;
        const ok = await submitOnboarding();
        btn.disabled = false;
        if (ok) {
          // Event 1: simpan ✓ (sudah dilakukan). Event 2: redirect ke /test.
          window.location.href = "/test";
        }
      });
    }
  }

  document.addEventListener("DOMContentLoaded", showOnboardingModalIfNeeded);
})();
