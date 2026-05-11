/*
 * Helper cascading dropdown Wilayah Indonesia (Provinsi -> Kota -> Kecamatan).
 * Sumber data: https://emsifa.github.io/api-wilayah-indonesia (CORS-enabled).
 *
 * Pemakaian:
 *   await window.setupWilayahCascading('idProv','idKota','idKec', { provinsi:'...', kota:'...', kecamatan:'...' });
 */
(function () {
  "use strict";

  // Pakai proxy backend agar bebas CORS & mixed-content (sumber asli emsifa
  // redirect ke HTTP yang diblokir browser bila app diakses via HTTPS).
  const BASE = "/api/wilayah";

  let provincesCache = null;
  const regenciesCache = {};
  const districtsCache = {};

  async function fetchJSON(url) {
    const res = await fetch(url);
    if (!res.ok) throw new Error("HTTP " + res.status);
    return res.json();
  }

  async function loadProvinces() {
    if (provincesCache) return provincesCache;
    provincesCache = await fetchJSON(BASE + "/provinces");
    return provincesCache;
  }

  async function loadRegencies(provinceId) {
    if (regenciesCache[provinceId]) return regenciesCache[provinceId];
    regenciesCache[provinceId] = await fetchJSON(BASE + "/regencies/" + provinceId);
    return regenciesCache[provinceId];
  }

  async function loadDistricts(regencyId) {
    if (districtsCache[regencyId]) return districtsCache[regencyId];
    districtsCache[regencyId] = await fetchJSON(BASE + "/districts/" + regencyId);
    return districtsCache[regencyId];
  }

  function fillSelect(selectEl, items, placeholder) {
    selectEl.innerHTML = "";
    const opt0 = document.createElement("option");
    opt0.value = "";
    opt0.textContent = placeholder;
    selectEl.appendChild(opt0);
    items.forEach(function (it) {
      const opt = document.createElement("option");
      opt.value = it.name; // simpan NAMA agar mudah dibaca pada laporan
      opt.dataset.id = it.id;
      opt.textContent = it.name;
      selectEl.appendChild(opt);
    });
  }

  function findOptionByLabel(selectEl, label) {
    if (!label) return null;
    const opts = selectEl.querySelectorAll("option");
    for (let i = 0; i < opts.length; i++) {
      if (opts[i].value === label) return opts[i];
    }
    return null;
  }

  async function setupWilayahCascading(provId, kotaId, kecId, initial) {
    const provSel = document.getElementById(provId);
    const kotaSel = document.getElementById(kotaId);
    const kecSel = document.getElementById(kecId);
    if (!provSel || !kotaSel || !kecSel) return;

    try {
      const provs = await loadProvinces();
      fillSelect(provSel, provs, "-- Pilih Provinsi --");
    } catch (err) {
      console.error("Gagal memuat provinsi:", err);
      provSel.innerHTML = '<option value="">Gagal memuat provinsi</option>';
      return;
    }

    provSel.addEventListener("change", async function () {
      kotaSel.innerHTML = '<option value="">Memuat kota...</option>';
      kecSel.innerHTML = '<option value="">-- Pilih Kecamatan --</option>';
      const opt = provSel.options[provSel.selectedIndex];
      const id = opt && opt.dataset ? opt.dataset.id : "";
      if (!id) {
        kotaSel.innerHTML = '<option value="">-- Pilih Kota --</option>';
        return;
      }
      try {
        const regs = await loadRegencies(id);
        fillSelect(kotaSel, regs, "-- Pilih Kota / Kabupaten --");
      } catch (err) {
        kotaSel.innerHTML = '<option value="">Gagal memuat kota</option>';
      }
    });

    kotaSel.addEventListener("change", async function () {
      kecSel.innerHTML = '<option value="">Memuat kecamatan...</option>';
      const opt = kotaSel.options[kotaSel.selectedIndex];
      const id = opt && opt.dataset ? opt.dataset.id : "";
      if (!id) {
        kecSel.innerHTML = '<option value="">-- Pilih Kecamatan --</option>';
        return;
      }
      try {
        const dists = await loadDistricts(id);
        fillSelect(kecSel, dists, "-- Pilih Kecamatan --");
      } catch (err) {
        kecSel.innerHTML = '<option value="">Gagal memuat kecamatan</option>';
      }
    });

    if (initial && initial.provinsi) {
      const provOpt = findOptionByLabel(provSel, initial.provinsi);
      if (provOpt) {
        provSel.value = initial.provinsi;
        provSel.dispatchEvent(new Event("change"));
        await new Promise(function (r) { setTimeout(r, 400); });
        if (initial.kota) {
          const kotaOpt = findOptionByLabel(kotaSel, initial.kota);
          if (kotaOpt) {
            kotaSel.value = initial.kota;
            kotaSel.dispatchEvent(new Event("change"));
            await new Promise(function (r) { setTimeout(r, 400); });
            if (initial.kecamatan) {
              const kecOpt = findOptionByLabel(kecSel, initial.kecamatan);
              if (kecOpt) kecSel.value = initial.kecamatan;
            }
          }
        }
      }
    }
  }

  window.setupWilayahCascading = setupWilayahCascading;
})();
