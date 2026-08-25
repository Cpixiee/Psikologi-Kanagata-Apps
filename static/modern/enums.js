/*
 * Daftar enum kelas & jurusan yang dipakai di modal onboarding (dashboard)
 * maupun form profil (settings). Disentralkan di sini supaya pilihan dan
 * deskripsi selalu konsisten di seluruh aplikasi.
 *
 * Penggunaan:
 *   window.AppEnums.kelasList       -> ["X", "XI", "XII"]
 *   window.AppEnums.jurusanList     -> [{code, label, description}]
 *   window.AppEnums.populateKelasSelect(selectEl, currentValue)
 *   window.AppEnums.populateJurusanSelect({
 *       select, customInput, descriptionEl, currentValue
 *   })
 *
 * Untuk jurusan, bila user pilih "Lainnya" maka customInput ditampilkan
 * dan value efektif diambil dari input tersebut (lihat getJurusanValue()).
 */
(function () {
  "use strict";

  const kelasList = ["VII", "VIII", "IX", "X", "XI", "XII"];

  // Catatan: code dipakai sebagai value yang disimpan ke DB supaya ringkas
  // dan seragam. Label = teks yang ditampilkan di option. description =
  // penjelasan singkat yang muncul sebagai hint ketika option dipilih.
  const jurusanList = [
    { code: "IPA", label: "IPA", description: "Ilmu Pengetahuan Alam" },
    { code: "IPS", label: "IPS", description: "Ilmu Pengetahuan Sosial" },
    { code: "Bahasa", label: "Bahasa", description: "Peminatan Bahasa & Budaya" },
    { code: "Umum", label: "Umum", description: "Peminatan umum / SMA tanpa penjurusan" },
    { code: "TKJ", label: "TKJ", description: "Teknik Komputer dan Jaringan" },
    { code: "RPL", label: "RPL", description: "Rekayasa Perangkat Lunak" },
    { code: "MM", label: "MM", description: "Multimedia" },
    { code: "DKV", label: "DKV", description: "Desain Komunikasi Visual" },
    { code: "BR", label: "BR", description: "Bisnis Ritel" },
    { code: "BD", label: "BD", description: "Bisnis Digital" },
    { code: "AKL", label: "AKL", description: "Akuntansi dan Keuangan Lembaga" },
    { code: "BDP", label: "BDP", description: "Bisnis Daring dan Pemasaran" },
    { code: "OTKP", label: "OTKP", description: "Otomatisasi dan Tata Kelola Perkantoran" },
    { code: "TBSM", label: "TBSM", description: "Teknik dan Bisnis Sepeda Motor" },
    { code: "TKRO", label: "TKRO", description: "Teknik Kendaraan Ringan Otomotif" },
    { code: "TEI", label: "TEI", description: "Teknik Elektronika Industri" },
    { code: "TAV", label: "TAV", description: "Teknik Audio Video" },
    { code: "TP", label: "TP", description: "Teknik Pemesinan" },
    { code: "Kuliner", label: "Kuliner", description: "Tata Boga / Kuliner" },
    { code: "Perhotelan", label: "Perhotelan", description: "Akomodasi Perhotelan" },
    { code: "Farmasi", label: "Farmasi", description: "Farmasi Klinis & Komunitas" },
    { code: "Keperawatan", label: "Keperawatan", description: "Asisten Keperawatan" },
  ];

  const LAINNYA = "__LAINNYA__";

  function populateKelasSelect(select, currentValue) {
    if (!select) return;
    select.innerHTML = "";
    const placeholder = document.createElement("option");
    placeholder.value = "";
    placeholder.textContent = "-- Pilih Kelas --";
    select.appendChild(placeholder);
    kelasList.forEach(function (k) {
      const o = document.createElement("option");
      o.value = k;
      o.textContent = "Kelas " + k;
      select.appendChild(o);
    });
    if (currentValue && kelasList.indexOf(currentValue) !== -1) {
      select.value = currentValue;
    }
  }

  function populateJurusanSelect(opts) {
    const select = opts.select;
    const customInput = opts.customInput;
    const descriptionEl = opts.descriptionEl;
    const currentValue = opts.currentValue || "";
    if (!select) return;

    select.innerHTML = "";
    const placeholder = document.createElement("option");
    placeholder.value = "";
    placeholder.textContent = "-- Pilih Jurusan --";
    select.appendChild(placeholder);
    jurusanList.forEach(function (j) {
      const o = document.createElement("option");
      o.value = j.code;
      o.textContent = j.label + " - " + j.description;
      o.title = j.description;
      o.dataset.description = j.description;
      select.appendChild(o);
    });
    const lainnyaOpt = document.createElement("option");
    lainnyaOpt.value = LAINNYA;
    lainnyaOpt.textContent = "Lainnya (ketik manual)...";
    select.appendChild(lainnyaOpt);

    function applyDescription() {
      if (!descriptionEl) return;
      const selOpt = select.options[select.selectedIndex];
      const desc = selOpt ? (selOpt.dataset.description || "") : "";
      if (select.value === LAINNYA) {
        descriptionEl.textContent = "Masukkan nama jurusan Anda secara manual.";
      } else if (desc) {
        descriptionEl.textContent = desc;
      } else {
        descriptionEl.textContent = "";
      }
    }

    function applyCustomVisibility() {
      if (!customInput) return;
      if (select.value === LAINNYA) {
        customInput.classList.remove("d-none");
        customInput.required = true;
      } else {
        customInput.classList.add("d-none");
        customInput.required = false;
        customInput.value = "";
      }
    }

    select.addEventListener("change", function () {
      applyDescription();
      applyCustomVisibility();
    });

    // Prefill: kalau currentValue ada di list &#8594; pilih. Kalau tidak ada tapi
    // bukan kosong &#8594; anggap sebagai "Lainnya" dan isi customInput.
    if (currentValue) {
      const found = jurusanList.some(function (j) { return j.code === currentValue; });
      if (found) {
        select.value = currentValue;
      } else {
        select.value = LAINNYA;
        if (customInput) customInput.value = currentValue;
      }
    }
    applyDescription();
    applyCustomVisibility();
  }

  function getJurusanValue(select, customInput) {
    if (!select) return "";
    if (select.value === LAINNYA) {
      return customInput ? (customInput.value || "").trim() : "";
    }
    return (select.value || "").trim();
  }

  window.AppEnums = {
    kelasList: kelasList,
    jurusanList: jurusanList,
    LAINNYA: LAINNYA,
    populateKelasSelect: populateKelasSelect,
    populateJurusanSelect: populateJurusanSelect,
    getJurusanValue: getJurusanValue,
  };
})();
