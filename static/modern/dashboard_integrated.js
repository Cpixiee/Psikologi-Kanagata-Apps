/*
 * Integrated Student Dashboard renderer.
 *
 * Fetch paralel:
 *   - GET /api/profile                       (data user)
 *   - GET /api/profile/test-summary          (IST, Holland, LearningStyle, Kraepelin)
 *   - GET /api/profile/rmib                  (history RMIB)
 *   - GET /api/profile/papi-results          (history PAPI)
 *
 * Lalu render setiap kartu pakai data terbaru. Kalau data belum ada,
 * tampilkan placeholder ramah, jangan gagal.
 */
(function () {
  "use strict";

  // ============== Helpers ==============
  const $ = (sel) => document.querySelector(sel);
  const $$ = (sel) => Array.from(document.querySelectorAll(sel));
  function el(tag, attrs, children) {
    const e = document.createElement(tag);
    if (attrs) {
      for (const k in attrs) {
        if (k === "class") e.className = attrs[k];
        else if (k === "style") e.style.cssText = attrs[k];
        else if (k.startsWith("on")) e.addEventListener(k.slice(2), attrs[k]);
        else e.setAttribute(k, attrs[k]);
      }
    }
    if (children) {
      (Array.isArray(children) ? children : [children]).forEach((c) => {
        if (c == null) return;
        e.appendChild(typeof c === "string" ? document.createTextNode(c) : c);
      });
    }
    return e;
  }
  function clamp(n, lo, hi) { return Math.max(lo, Math.min(hi, n)); }
  function pct(n) { return Math.round(clamp(n, 0, 100)); }
  function safeJson(res) { return res && res.ok ? res.json().catch(() => null) : null; }
  async function fetchJson(url) {
    try { const r = await fetch(url); return await safeJson(r); } catch { return null; }
  }
  function setText(sel, val) { const n = $(sel); if (n) n.textContent = val == null ? "-" : String(val); }
  function setRing(node, percent, color) {
    if (!node) return;
    const deg = clamp(percent, 0, 100) * 3.6;
    node.style.setProperty("--pct", deg + "deg");
    if (color) node.style.setProperty("--col", color);
    const num = node.querySelector(".num");
    if (num) num.textContent = pct(percent);
  }
  function descClass(p) { return p >= 75 ? "good" : (p >= 50 ? "fair" : "low"); }
  function descLabel(p) {
    if (p >= 85) return "Excellent";
    if (p >= 70) return "Good";
    if (p >= 50) return "Fair";
    return "Needs Work";
  }

  // ============== Data normalisation ==============
  // RIASEC dimension &#8594; label & deskripsi singkat
  const RIASEC_LABELS = {
    R: "Realistic", I: "Investigative", A: "Artistic",
    S: "Social", E: "Enterprising", C: "Conventional",
  };

  // Mapping IST aspek (urutan StdScores: SE, WA, AN, GE, RA, ZA, FA, WU, ME)
  const IST_KEYS = ["SE", "WA", "AN", "GE", "RA", "ZA", "FA", "WU", "ME"];
  const IST_LABEL = {
    SE: "Common Sense", WA: "Verbal", AN: "Analogi",
    GE: "Konseptual", RA: "Logika Matematika", ZA: "Numerikal",
    FA: "Figural", WU: "Spasial", ME: "Memori",
  };

  const CAREER_BY_LETTER = {
    R: [
      { name: "Supervisor Operasional & Manufaktur", base: 93, icon: "settings" },
      { name: "Teknisi Rekayasa & Otomatisasi", base: 90, icon: "cpu" },
      { name: "Logistik & Supply Chain Specialist", base: 87, icon: "truck" },
      { name: "Field Engineer & Maintenance", base: 84, icon: "wrench" },
    ],
    I: [
      { name: "Data Analyst & Market Researcher", base: 93, icon: "bar-chart-2" },
      { name: "Business Intelligence Analyst", base: 90, icon: "line-chart" },
      { name: "Analyst Sistem & Inovasi", base: 87, icon: "search" },
      { name: "Specialist Riset & Data", base: 84, icon: "database" },
    ],
    A: [
      { name: "Visual & Graphic Designer", base: 94, icon: "palette" },
      { name: "Digital Media & Creative Strategist", base: 91, icon: "pen-tool" },
      { name: "Content Creator & Copywriter", base: 88, icon: "video" },
      { name: "UI/UX & Product Designer", base: 85, icon: "layout" },
    ],
    S: [
      { name: "Public Relations & Communications", base: 93, icon: "users" },
      { name: "Human Resources (HR) & Talent Specialist", base: 90, icon: "contact" },
      { name: "Konselor & Educator", base: 87, icon: "graduation-cap" },
      { name: "Event & Community Coordinator", base: 84, icon: "heart" },
    ],
    E: [
      { name: "Business Development & Sales Manager", base: 95, icon: "briefcase" },
      { name: "Digital Marketer & E-Commerce Specialist", base: 92, icon: "megaphone" },
      { name: "Entrepreneur & Business Owner", base: 89, icon: "rocket" },
      { name: "Retail & Marketing Operations Manager", base: 86, icon: "shopping-bag" },
    ],
    C: [
      { name: "Financial & Tax Analyst", base: 94, icon: "coins" },
      { name: "Accounting & Audit Specialist", base: 91, icon: "calculator" },
      { name: "Operations & Database Administrator", base: 88, icon: "database" },
      { name: "Quality Control & Compliance Officer", base: 85, icon: "file-text" },
    ],
  };

  // ============== Helpers: avatar / role ==============
  function avatarUrl(u) {
    if (u && u.foto_profil) return "/static/uploads/profiles/" + u.foto_profil;
    const name = encodeURIComponent((u && (u.nama_lengkap || u.email)) || "Siswa");
    return "https://ui-avatars.com/api/?name=" + name + "&size=200&background=4f46e5&color=fff&bold=true";
  }
  function roleLabel(role) {
    const map = {
      siswa: "Siswa", mahasiswa: "Mahasiswa", guru: "Guru",
      pekerja: "Pekerja", umum: "Umum", admin: "Administrator",
      sekolah: "Sekolah",
    };
    return map[(role || "").toLowerCase()] || "Siswa";
  }

  function renderSidebarStats(ctx) {
    const u = ctx.user || {};
    const role = (u.role || "").toLowerCase();
    const kelasEl = $("#sbKelasValue");
    const jurusanEl = $("#sbJurusanValue");

    if (role === "sekolah") {
      if (kelasEl) {
        const lbl = kelasEl.previousElementSibling;
        if (lbl) lbl.textContent = "Asal Sekolah";
        kelasEl.textContent = u.sekolah || "-";
      }
      if (jurusanEl) {
        const lbl = jurusanEl.previousElementSibling;
        if (lbl) lbl.textContent = "Bidang";
        jurusanEl.textContent = "BK";
      }
    } else {
      if (kelasEl) {
        const lbl = kelasEl.previousElementSibling;
        if (lbl) lbl.textContent = "Kelas";
        kelasEl.textContent = u.kelas || "-";
      }
      if (jurusanEl) {
        const lbl = jurusanEl.previousElementSibling;
        if (lbl) lbl.textContent = "Jurusan";
        jurusanEl.textContent = u.jurusan || u.asal_instansi || "-";
      }
    }
  }

  // ============== Render: Topbar ==============
  function renderTopbar(ctx) {
    const u = ctx.user || {};
    let displayName = u.nama_lengkap || u.email || "Siswa";
    let roleText = roleLabel(u.role);
    if (u.teacher_name) {
      displayName = u.teacher_name + " (" + displayName + ")";
      roleText = "Guru " + roleText;
    }
    setText("#topUserName", displayName);
    setText("#topUserRole", roleText);
    const ava = $("#topUserAvatar");
    if (ava) ava.src = avatarUrl(u);

    // Sembunyikan judul topbar bawaan siswa jika peran sekolah/guru
    const isSekolah = (u.role || "").toLowerCase() === "sekolah";
    const mainTitle = $("#dsTopbarTitle");
    const mainSubtitle = $("#dsTopbarSubtitle");
    if (isSekolah) {
      if (mainTitle) mainTitle.style.display = "none";
      if (mainSubtitle) mainSubtitle.style.display = "none";
    } else {
      if (mainTitle) mainTitle.style.display = "";
      if (mainSubtitle) mainSubtitle.style.display = "";
    }

    // Tampilkan banner selamat datang untuk akun sekolah
    if ((u.role || "").toLowerCase() === "sekolah") {
      const banner = $("#schoolWelcomeBanner");
      const titleEl = $("#schoolWelcomeTitle");
      if (banner) banner.style.display = "block";
      if (titleEl) {
        let welcomeName = u.sekolah || u.nama_lengkap || "Sekolah";
        if (u.teacher_name) {
          welcomeName = u.teacher_name + " @ " + welcomeName;
        }
        titleEl.textContent = "Selamat datang kembali, " + welcomeName + "!";
      }

      // Ambil jumlah guru terdaftar
      fetchJson("/api/schools/my-teachers").then(function(res) {
        const countEl = $("#schoolTeacherCount");
        if (!countEl) return;
        const list = (res && res.data) ? res.data : (Array.isArray(res) ? res : []);
        countEl.textContent = Array.isArray(list) ? list.length : "-";
      }).catch(function() {});
    }
  }

  // ============== Render: Student Profile ==============
  function renderStudentProfile(ctx) {
    const u = ctx.user || {};
    setText("#spName", u.nama_lengkap || "-");
    setText("#spStudentId", "Student ID: " + (u.nisn || u.nip || "-"));
    const meta = $("#spMeta");
    if (meta) {
      meta.innerHTML = "";
      const addRow = (iconName, txt) => {
        if (!txt) return;
        const row = document.createElement("div");
        row.className = "ds-meta-row";
        row.innerHTML = '<i data-lucide="' + iconName + '" style="width:14px;height:14px;color:var(--ds-primary);flex-shrink:0;"></i><span></span>';
        row.querySelector("span").textContent = txt;
        meta.appendChild(row);
      };
      addRow("clipboard-list", [u.kelas, u.jurusan].filter(Boolean).join(" / ") || null);
      if (u.asal_instansi) addRow("school", u.asal_instansi);
      if (u.tanggal_lahir) {
        const d = new Date(u.tanggal_lahir);
        if (!isNaN(d)) {
          const now = new Date();
          let age = now.getFullYear() - d.getFullYear();
          const m = now.getMonth() - d.getMonth();
          if (m < 0 || (m === 0 && now.getDate() < d.getDate())) age--;
          if (age >= 0) addRow("user", age + " Tahun");
        }
      }
      // Gabungkan tempat lahir & alamat domisili menjadi satu baris ringkas
      const kotaDomisili = [u.kota, u.provinsi].filter(Boolean).join(", ");
      if (u.tempat_lahir && kotaDomisili) {
        addRow("map-pin", u.tempat_lahir + " \u2022 " + kotaDomisili);
      } else if (kotaDomisili) {
        addRow("map-pin", kotaDomisili);
      } else if (u.tempat_lahir) {
        addRow("map-pin", u.tempat_lahir);
      }
    }
    const ava = $("#spAvatar");
    if (ava) ava.src = avatarUrl(u);

    // Stat tiles
    setText("#spKelas", u.kelas || "-");
    setText("#spJurusan", u.jurusan || "-");
    setText("#spTalent", ctx.dominantTalent || "-");
    setText("#spCareerName", ctx.topCareer.name);
    setText("#spCareerMatch", ctx.topCareer.match + "%");
  }

  // ============== Render: Psychological Mapping ==============
  function renderPsychoMap(ctx) {
    setText("#pmPersonality", ctx.personalityType.label);
    setText("#pmPersonalityCode", ctx.personalityType.code);
    setText("#pmLearningStyle", ctx.learningStyleLabel);
    setText("#pmDominant", ctx.dominantIntelligence);
    setText("#pmMotivation", ctx.motivationLevel);
    setText("#pmStress", ctx.stressLevel);
    setText("#pmLeadership", ctx.leadershipPotential);
    setText("#pmNote", ctx.pmNote);

    // Radar
    const canvas = $("#pmRadar");
    if (canvas && window.Chart) {
      if (canvas._chart) canvas._chart.destroy();
      canvas._chart = new Chart(canvas.getContext("2d"), {
        type: "radar",
        data: {
          labels: ["Analytical Thinking", "Creativity", "Social Skill", "Emotional Stability", "Adaptability", "Problem Solving"],
          datasets: [{
            label: "Skor",
            data: ctx.radarValues,
            backgroundColor: "rgba(79, 70, 229, 0.18)",
            borderColor: "rgba(79, 70, 229, 0.9)",
            borderWidth: 2,
            pointBackgroundColor: "rgba(79, 70, 229, 1)",
            pointRadius: 4,
          }],
        },
        options: {
          responsive: true,
          maintainAspectRatio: false,
          layout: { padding: { top: 14, bottom: 14, left: 32, right: 32 } },
          plugins: { legend: { display: false }, tooltip: { enabled: true } },
          scales: {
            r: {
              suggestedMin: 0,
              suggestedMax: 100,
              ticks: { display: false, stepSize: 20 },
              pointLabels: {
                font: { size: 12, weight: "500" },
                color: "#374151",
                padding: 8,
              },
              grid: { color: "#e6eaf2" },
              angleLines: { color: "#e6eaf2" },
            },
          },
        },
      });
      // Pastikan ukuran chart benar setelah layout final.
      requestAnimationFrame(() => {
        try { canvas._chart && canvas._chart.resize(); } catch (_) {}
      });
      setTimeout(() => {
        try { canvas._chart && canvas._chart.resize(); } catch (_) {}
      }, 250);
    }
  }

  // ============== Render: Emotional Analytics ==============
  function renderEmotional(ctx) {
    const g = ctx.emotional;
    const map = [
      ["#gSelfAware", "Self Awareness", g.selfAwareness],
      ["#gSelfReg",   "Self Regulation", g.selfRegulation],
      ["#gMotivation","Motivation",     g.motivation],
      ["#gEmpathy",   "Empathy",        g.empathy],
      ["#gStressMgmt","Stress Management", g.stressManagement],
      ["#gResilience","Mental Resilience", g.resilience],
    ];
    const palette = ["#22c55e", "#3b82f6", "#8b5cf6", "#f59e0b", "#14b8a6", "#ec4899"];
    map.forEach((m, i) => {
      const node = $(m[0]);
      if (node) {
        setRing(node.querySelector(".ring"), m[2], palette[i]);
        const d = node.querySelector(".desc");
        if (d) { d.textContent = descLabel(m[2]); d.className = "desc " + descClass(m[2]); }
      }
    });

    const stable = $("#emoStable");
    if (stable) {
      const avg = (g.selfAwareness + g.selfRegulation + g.resilience) / 3;
      const lab = avg >= 70 ? "Stable" : (avg >= 50 ? "Moderate" : "Unstable");
      const cls = avg >= 70 ? "good" : (avg >= 50 ? "warn" : "bad");
      stable.innerHTML = '<span>Emotional Status</span><span class="v ' + cls + '">● ' + lab + "</span>";
      const modalEmo = $("#modalEmoStatusDesc");
      if (modalEmo) {
        modalEmo.innerHTML = 'Kategori: <strong class="text-' + (cls === 'good' ? 'success' : cls === 'warn' ? 'warning' : 'danger') + '">' + lab + ' (' + Math.round(avg) + '/100)</strong><br/><small class="text-muted mt-1 d-block">Siswa memiliki tingkat kestabilan emosi dan penyesuaian sosial yang ' + (avg >= 70 ? 'sangat memadai' : 'cukup baik, membutuhkan penguatan regulasi diri saat tertekan') + '.</small>';
      }
    }
    const burn = $("#emoBurnout");
    if (burn) {
      const risk = 100 - g.stressManagement;
      const lab = risk >= 60 ? "High" : (risk >= 35 ? "Medium" : "Low");
      const cls = risk >= 60 ? "bad" : (risk >= 35 ? "warn" : "good");
      burn.innerHTML = '<span>Burnout Risk</span><span class="v ' + cls + '">↓ ' + lab + "</span>";
      const modalBurn = $("#modalBurnoutDesc");
      if (modalBurn) {
        modalBurn.innerHTML = 'Tingkat Risiko: <strong class="text-' + (cls === 'good' ? 'success' : cls === 'warn' ? 'warning' : 'danger') + '">' + lab + ' Risk (' + Math.round(risk) + '%)</strong><br/><small class="text-muted mt-1 d-block">' + (risk >= 60 ? 'Perlu perhatian khusus Guru BK untuk pendampingan manajemen stres.' : 'Beban kerja dan stres akademik siswa dalam kisaran wajar.') + '</small>';
      }
    }

    // Populate modal emotional grid
    const mGrid = $("#modalEmotionalGrid");
    if (mGrid) {
      mGrid.innerHTML = "";
      map.forEach((m, i) => {
        const gaugeEl = el("div", { class: "ds-gauge" });
        gaugeEl.innerHTML = '<div class="label">' + m[1] + '</div><div class="ring"><span class="num">' + m[2] + '</span></div><div class="desc ' + descClass(m[2]) + '">' + descLabel(m[2]) + '</div>';
        setRing(gaugeEl.querySelector(".ring"), m[2], palette[i]);
        mGrid.appendChild(gaugeEl);
      });
    }
  }

  // ============== Render: Skill Tracker ==============
  function renderSkills(ctx) {
    const list = $("#skillList");
    const modalList = $("#modalSkillList");
    if (list) list.innerHTML = "";
    if (modalList) modalList.innerHTML = "";

    const colors = ["", "violet", "green", "amber", "teal", "pink"];
    ctx.skills.forEach((s, i) => {
      const row = el("div", { class: "ds-skill-row " + (colors[i % colors.length]) });
      row.appendChild(el("span", { class: "nm" }, s.name));
      row.appendChild(el("span", { class: "vl" }, s.value + "%"));
      const bar = el("div", { class: "bar" });
      bar.appendChild(el("span", { style: "width:" + s.value + "%" }));
      row.appendChild(bar);

      if (list) list.appendChild(row.cloneNode(true));
      if (modalList) modalList.appendChild(row);
    });
    setText("#skillTotal", ctx.skillTotal);
    setText("#skillTotalLabel", ctx.skillTotalLabel);
  }

  // ============== Render: Career Roadmap ==============
  function renderCareer(ctx) {
    const list = $("#careerList");
    const modalList = $("#modalCareerList");
    if (list) list.innerHTML = "";
    if (modalList) modalList.innerHTML = "";

    ctx.careers.forEach((c) => {
      const row = document.createElement("div");
      row.className = "ds-career-row";
      row.innerHTML = '<span class="ic"><i data-lucide="' + (c.icon || "briefcase") + '"></i></span>'
        + '<span class="nm"></span>'
        + '<span class="pct">' + c.match + "%</span>";
      row.querySelector(".nm").textContent = c.name;

      if (list) list.appendChild(row.cloneNode(true));
      if (modalList) modalList.appendChild(row);
    });

    const tl = $("#careerTimeline");
    const modalTl = $("#modalCareerTimeline");
    if (tl) tl.innerHTML = "";
    if (modalTl) modalTl.innerHTML = "";

    ctx.roadmap.forEach((r) => {
      const itm = el("div", { class: "ds-roadmap-item" });
      itm.appendChild(el("h5", null, r.term));
      const ul = el("ul");
      r.items.forEach((i) => ul.appendChild(el("li", null, i)));
      itm.appendChild(ul);

      if (tl) tl.appendChild(itm.cloneNode(true));
      if (modalTl) modalTl.appendChild(itm);
    });

    // Populate preferred subjects & target careers displays in modal
    if (ctx.preferredSubjects) {
      setText("#modalPreferredSubjectsDisplay", Array.isArray(ctx.preferredSubjects) ? ctx.preferredSubjects.join(", ") : ctx.preferredSubjects);
    }
    if (ctx.targetCareers) {
      setText("#modalTargetCareersDisplay", Array.isArray(ctx.targetCareers) ? ctx.targetCareers.join(", ") : ctx.targetCareers);
    }
  }

  // ============== Render: AI Recommendation ==============
  function renderAIRec(ctx) {
    const list = $("#aiRecList");
    const modalList = $("#modalAiRecList");
    if (list) list.innerHTML = "";
    if (modalList) modalList.innerHTML = "";

    ctx.recommendations.forEach((r) => {
      const item = document.createElement("div");
      item.className = "ds-rec-item " + r.color;
      const itemsHtml = r.items.map((t) => {
        const li = document.createElement("li");
        li.textContent = t;
        return li.outerHTML;
      }).join("");
      const titleEl = document.createElement("span");
      titleEl.textContent = r.title;
      item.innerHTML = '<span class="ic"><i data-lucide="' + r.icon + '"></i></span>'
        + '<div style="flex:1;min-width:0;">'
        + '<h5>' + titleEl.innerHTML + '</h5>'
        + '<ul>' + itemsHtml + '</ul>'
        + '</div>'
        + '<i data-lucide="chevron-right" style="width:16px;height:16px;color:var(--ds-muted);align-self:center;"></i>';

      if (list) list.appendChild(item.cloneNode(true));
      if (modalList) modalList.appendChild(item);
    });
  }

  // ============== Render: AI Smart Summary ==============
  function renderSmart(ctx) {
    const fill = (sel, arr) => {
      const ul = $(sel);
      if (!ul) return;
      ul.innerHTML = "";
      arr.forEach((s) => ul.appendChild(el("li", null, s)));
    };
    fill("#smStrengths", ctx.strengths);
    fill("#smDev", ctx.developments);

    setRing($("#smPotentialRing"), ctx.potential, "#8b5cf6");
    setText("#smPotentialVal", ctx.potential + "%");
    setText("#smPotentialDesc", ctx.potentialDesc);

    const ls = $("#smStrategy");
    if (ls) {
      ls.innerHTML = "";
      ctx.strategy.forEach((s) => {
        const wrap = el("div");
        wrap.appendChild(el("div", { class: "row" }, [
          el("span", null, s.name),
          el("span", null, s.value + "%"),
        ]));
        const bar = el("div", { class: "bar" });
        bar.appendChild(el("span", { style: "width:" + s.value + "%" }));
        wrap.appendChild(bar);
        ls.appendChild(wrap);
      });
    }
    setText("#smInsight", ctx.insight);
  }

  // ============== Data assembly ==============
  function buildContext(profile, summary, rmib, papi) {
    const u = profile || {};
    const ist = (summary && summary.last_ist_result) || null;
    const hol = (summary && summary.last_holland_result) || null;
    const vak = (summary && summary.last_learning_style_result) || null;
    const krp = (summary && summary.last_kraepelin_attempt) || null;
    const rmibTop = (rmib && rmib[0]) ? rmib[0].rmib_result : null;
    const papiTop = (papi && papi[0]) ? papi[0].papi_result : null;

    // === Skor IST per aspek ke 0-100 (Std biasanya 80-130; clamp) ===
    const istPct = {};
    if (ist && Array.isArray(ist.std_scores)) {
      IST_KEYS.forEach((k, i) => {
        const v = ist.std_scores[i] || 0;
        // map 80-130 &#8594; 0-100
        istPct[k] = pct(((v - 80) / 50) * 100);
      });
    } else {
      IST_KEYS.forEach((k) => (istPct[k] = 50));
    }
    
    // === Holland percentages (skor R/I/A/S/E/C; max kira-kira 40) ===
    let holPct = { R: 50, I: 50, A: 50, S: 50, E: 50, C: 50 };
    if (hol) {
      const holScores = { R: hol.score_r, I: hol.score_i, A: hol.score_a, S: hol.score_s, E: hol.score_e, C: hol.score_c };
      const holMax = Math.max(40, ...Object.values(holScores));
      Object.keys(holScores).forEach((k) => { holPct[k] = pct((holScores[k] / holMax) * 100); });
    } else {
      // Approximate holPct from RMIB, PAPI, or IST
      if (rmibTop) {
        var parsedRM = {};
        try { parsedRM = JSON.parse(rmibTop.result_json || '{}'); } catch(e) {}
        var getRmPct = function(cat) {
          var item = parsedRM[cat] || {};
          var r = item.rank || 12;
          return Math.round(((13 - r) / 12) * 100);
        };
        holPct.R = Math.round((getRmPct("OUT") + getRmPct("MEC") + getRmPct("PRAC")) / 3);
        holPct.I = Math.round((getRmPct("SCI") + getRmPct("MED")) / 2);
        holPct.A = Math.round((getRmPct("AEST") + getRmPct("MUS") + getRmPct("LIT")) / 3);
        holPct.S = getRmPct("SOC");
        holPct.E = getRmPct("PERS");
        holPct.C = Math.round((getRmPct("COMP") + getRmPct("CLER")) / 2);
      } else if (papiTop) {
        var parsedPapi = {};
        try { parsedPapi = JSON.parse(papiTop.result_json || '{}'); } catch(e) {}
        var getPapiPct = function(codes) {
          var sum = 0, count = 0;
          codes.forEach(function(code) {
            var item = parsedPapi[code] || {};
            sum += (item.score || 0);
            count++;
          });
          return Math.round((sum / (count * 9)) * 100);
        };
        holPct.R = getPapiPct(["G", "T", "K"]);
        holPct.I = getPapiPct(["I", "R", "N"]);
        holPct.A = getPapiPct(["X", "Z"]);
        holPct.S = getPapiPct(["S", "B", "O"]);
        holPct.E = getPapiPct(["L", "V", "A", "P"]);
        holPct.C = getPapiPct(["D", "C", "F", "W"]);
      } else if (ist) {
        holPct.R = Math.round((istPct.FA + istPct.WU) / 2);
        holPct.I = Math.round((istPct.AN + istPct.GE) / 2);
        holPct.A = istPct.GE;
        holPct.S = Math.round((istPct.SE + istPct.WA) / 2);
        holPct.E = istPct.WA;
        holPct.C = Math.round((istPct.RA + istPct.ZA + istPct.ME) / 3);
      }
    }

    // === Personality type from RIASEC ===
    const code = hol ? (hol.code || (hol.top1 + hol.top2 + hol.top3)) : "";
    const personalityLabel = code
      ? code.split("").map((c) => RIASEC_LABELS[c] || c).slice(0,2).join(" - ")
      : "Belum tersedia";

    // === Learning style label ===
    const learningStyleLabel = vak
      ? (vak.dominant_type || "-")
      : "Belum tersedia";

    // === Dominant intelligence dari IST top aspect ===
    let topAspect = IST_KEYS[0]; // default ke aspek pertama supaya tidak undefined
    let topAspectVal = -1;
    IST_KEYS.forEach((k) => {
      const v = istPct[k];
      if (typeof v === "number" && v > topAspectVal) { topAspectVal = v; topAspect = k; }
    });
    const dominantIntelligence = ist
      ? (IST_LABEL[topAspect] || topAspect) + " (" + topAspect + ")"
      : "Belum tersedia";

    // === Motivation / Stress / Leadership (heuristik) ===
    // Motivation: Holland E + RMIB top "Persuasif" sederhana -> pakai E score
    const motivationPct = pct(((holPct.E || 50) + (rmibTop ? 70 : 50)) / 2);
    const motivationLevel = motivationPct >= 70 ? "High" : motivationPct >= 45 ? "Mid" : "Low";

    // Stress: dari Kraepelin error rate (lower=better -> invert)
    let stressPct = 40;
    if (krp) {
      const total = (krp.total_correct || 0) + (krp.total_errors || 0) + (krp.total_skipped || 0);
      if (total > 0) {
        const errPct = ((krp.total_errors + krp.total_skipped) / total) * 100;
        stressPct = pct(errPct * 1.5); // ampliifkasi
      }
    }
    const stressLevel = stressPct >= 60 ? "High" : stressPct >= 35 ? "Medium" : "Low";

    // Leadership: Holland E + S
    const leadershipPct = pct(((holPct.E || 0) + (holPct.S || 0)) / 2);
    const leadershipPotential = leadershipPct >= 65 ? "Strong" : leadershipPct >= 40 ? "Moderate" : "Developing";

    // === Radar values: 6 axis ===
    const radarValues = [
      istPct.AN || 50,                                    // Analytical Thinking
      Math.max(istPct.FA || 0, istPct.GE || 0),           // Creativity
      holPct.S || 50,                                     // Social Skill
      100 - stressPct,                                    // Emotional Stability
      pct(((vak ? 70 : 50) + (istPct.WU || 50)) / 2),     // Adaptability
      istPct.RA || 50,                                    // Problem Solving
    ];

    // === Emotional analytics (heuristik dari data tersedia) ===
    const emotional = {
      selfAwareness:   pct(((istPct.WA || 50) + (vak ? 75 : 50)) / 2),
      selfRegulation:  pct(((istPct.RA || 50) + (100 - stressPct)) / 2),
      motivation:      motivationPct,
      empathy:         pct(((holPct.S || 50) + (istPct.WA || 50)) / 2),
      stressManagement: 100 - stressPct,
      resilience:      pct(((istPct.AN || 50) + (100 - stressPct)) / 2),
    };

    // === Career roadmap ===
    let careerSeed = [];
    if (code) {
      careerSeed = code.split("");
    } else {
      // Dynamic career seed from RMIB, PAPI, or IST
      if (rmibTop) {
        var rmibToRiasec = {
          OUT: "R", MEC: "R", MECH: "R", COMP: "C", SCI: "I",
          PERS: "E", AEST: "A", ART: "A", MUS: "A", LIT: "A",
          SOC: "S", CLER: "C", PRAC: "R", MED: "I"
        };
        var cats = [rmibTop.dominant_category, rmibTop.top1, rmibTop.top2, rmibTop.top3].filter(Boolean);
        cats.forEach(function(c) {
          var letter = rmibToRiasec[c.toUpperCase()];
          if (letter && careerSeed.indexOf(letter) === -1) careerSeed.push(letter);
        });
      }
      if (careerSeed.length < 3 && papiTop) {
        var papiToRiasec = {
          G: "R", L: "E", I: "I", T: "R", V: "E", S: "S", R: "I", D: "C", C: "C", E: "S",
          N: "I", A: "E", P: "E", X: "A", B: "S", O: "S", Z: "A", K: "R", F: "C", W: "C"
        };
        var dom = (papiTop.dominant_category || "").toUpperCase();
        var letter = papiToRiasec[dom];
        if (letter && careerSeed.indexOf(letter) === -1) careerSeed.push(letter);
        if (papiTop.top_categories) {
          var tops = [];
          try { tops = JSON.parse(papiTop.top_categories); } catch(e) {
            tops = String(papiTop.top_categories).split(/[,\[\]\"' →]/).map(x => x.trim()).filter(Boolean);
          }
          if (Array.isArray(tops)) {
            tops.forEach(function(c) {
              var l = papiToRiasec[c.toUpperCase()];
              if (l && careerSeed.indexOf(l) === -1) careerSeed.push(l);
            });
          }
        }
      }
      if (careerSeed.length < 3 && ist) {
        var istScores = [
          { score: istPct.SE || 0, letter: "S" },
          { score: istPct.WA || 0, letter: "S" },
          { score: istPct.AN || 0, letter: "I" },
          { score: istPct.GE || 0, letter: "A" },
          { score: istPct.RA || 0, letter: "C" },
          { score: istPct.ZA || 0, letter: "I" },
          { score: istPct.FA || 0, letter: "R" },
          { score: istPct.WU || 0, letter: "R" },
          { score: istPct.ME || 0, letter: "C" }
        ];
        istScores.sort(function(a, b) { return b.score - a.score; });
        istScores.forEach(function(item) {
          if (careerSeed.indexOf(item.letter) === -1) careerSeed.push(item.letter);
        });
      }

      // Dynamic defaults based on student major / jurusan
      const jurStr = String(u.jurusan || u.kelas || "").toUpperCase();
      let defaultOrder = ["E", "C", "S", "A", "R", "I"]; // Default business & general
      if (jurStr.includes("DKV") || jurStr.includes("DESAIN") || jurStr.includes("SENI")) {
        defaultOrder = ["A", "E", "R", "S", "C", "I"];
      } else if (jurStr.includes("AK") || jurStr.includes("AKUN") || jurStr.includes("KEUANGAN") || jurStr.includes("OTKP") || jurStr.includes("ADMIN")) {
        defaultOrder = ["C", "E", "S", "R", "I", "A"];
      } else if (jurStr.includes("TKJ") || jurStr.includes("RPL") || jurStr.includes("IPA") || jurStr.includes("INFORMATIKA")) {
        defaultOrder = ["I", "C", "R", "E", "A", "S"];
      }

      defaultOrder.forEach(function(l) {
        if (careerSeed.length < 3 && careerSeed.indexOf(l) === -1) careerSeed.push(l);
      });
    }
    careerSeed = careerSeed.slice(0, 3);

    // === Skill Tracker (Dynamic based on primary Holland domain) ===
    let skills = [];
    const topDomain = careerSeed[0] || "E";
    if (topDomain === "E") {
      skills = [
        { name: "Strategi Bisnis & Sales", value: pct(((holPct.E || 85) + (istPct.WA || 80)) / 2) },
        { name: "Negosiasi & Persuasi", value: pct(((holPct.E || 82) + (holPct.S || 78)) / 2) },
        { name: "Komunikasi & Presentasi", value: pct(((istPct.WA || 80) + (holPct.S || 80)) / 2) },
        { name: "Kepemimpinan Tim", value: leadershipPct },
        { name: "Manajemen Proyek", value: istPct.RA || 75 },
        { name: "Kreativitas Pemasaran", value: Math.max(istPct.GE || 70, holPct.A || 70) }
      ];
    } else if (topDomain === "A") {
      skills = [
        { name: "Kreativitas & Konsep", value: Math.max(istPct.GE || 85, holPct.A || 85) },
        { name: "Desain Visual & Estetika", value: pct(((holPct.A || 88) + (istPct.FA || 80)) / 2) },
        { name: "Media Digital & Video", value: pct(((holPct.A || 85) + (istPct.WU || 75)) / 2) },
        { name: "Komunikasi Visual", value: pct(((istPct.WA || 80) + (holPct.A || 80)) / 2) },
        { name: "Inovasi Produk", value: Math.max(istPct.GE || 75, istPct.FA || 75) },
        { name: "Problem Solving", value: istPct.RA || 75 }
      ];
    } else if (topDomain === "S") {
      skills = [
        { name: "Pelayanan & Empati", value: pct(((holPct.S || 88) + (istPct.WA || 80)) / 2) },
        { name: "Komunikasi Interpersonal", value: pct(((istPct.WA || 85) + (holPct.S || 85)) / 2) },
        { name: "Pengembangan SDM & Edukasi", value: pct(((holPct.S || 84) + (istPct.GE || 76)) / 2) },
        { name: "Kerja Sama Tim", value: pct(((holPct.S || 86) + (holPct.E || 76)) / 2) },
        { name: "Resolusi Konflik", value: pct(((istPct.AN || 75) + (holPct.S || 80)) / 2) },
        { name: "Kepemimpinan", value: leadershipPct }
      ];
    } else if (topDomain === "C") {
      skills = [
        { name: "Ketelitian & Akurasi Data", value: pct(((istPct.ME || 85) + (holPct.C || 85)) / 2) },
        { name: "Manajemen Keuangan & Admin", value: pct(((istPct.ZA || 82) + (holPct.C || 84)) / 2) },
        { name: "Administrasi & Dokumentasi", value: pct(((holPct.C || 84) + (istPct.ME || 80)) / 2) },
        { name: "Perencanaan & Organisasi", value: istPct.AN || 80 },
        { name: "Manajemen Risiko", value: istPct.RA || 78 },
        { name: "Critical Thinking", value: istPct.AN || 80 }
      ];
    } else if (topDomain === "R") {
      skills = [
        { name: "Keterampilan Teknik", value: pct(((holPct.R || 85) + (istPct.FA || 80)) / 2) },
        { name: "Troubleshooting & Mekanikal", value: pct(((istPct.WU || 82) + (holPct.R || 82)) / 2) },
        { name: "Manajemen Operasional", value: pct(((holPct.R || 80) + (istPct.RA || 78)) / 2) },
        { name: "Keandalan Kerja Lapangan", value: pct(((holPct.R || 84) + (100 - stressPct)) / 2) },
        { name: "Penggunaan Alat & Teknologi", value: istPct.FA || 80 },
        { name: "Problem Solving", value: istPct.RA || 78 }
      ];
    } else {
      // I (Investigative)
      skills = [
        { name: "Penalaran Logis & Analitis", value: istPct.AN || 85 },
        { name: "Riset & Metodologi Data", value: pct(((istPct.AN || 82) + (holPct.I || 82)) / 2) },
        { name: "Pemecahan Masalah Kompleks", value: istPct.RA || 84 },
        { name: "Berpikir Kritis", value: istPct.AN || 82 },
        { name: "Ketelitian Observasi", value: istPct.ME || 80 },
        { name: "Komunikasi Data", value: istPct.WA || 76 }
      ];
    }
    const skillAvg = pct(skills.reduce((a, b) => a + b.value, 0) / skills.length);
    const skillTotal = skillAvg + " /100";
    const skillTotalLabel = skillAvg >= 75 ? "Good Performance" : skillAvg >= 55 ? "Average" : "Needs Improvement";

    const careerMap = new Map();
    careerSeed.forEach((letter, idx) => {
      const list = CAREER_BY_LETTER[letter] || [];
      list.forEach((c) => {
        const adj = c.base - idx * 2;
        if (!careerMap.has(c.name) || careerMap.get(c.name).match < adj) {
          const ic = idx === 0 ? "bot" : idx === 1 ? "line-chart" : idx === 2 ? "flask-conical" : "briefcase";
          careerMap.set(c.name, { name: c.name, match: clamp(adj, 50, 96), icon: ic });
        }
      });
    });
    const careers = Array.from(careerMap.values()).sort((a, b) => b.match - a.match).slice(0, 5);

    const roadmap = [
      { term: "Short Term (1-2 Tahun)", items: ["Kuasai " + (dominantIntelligence.split(" ")[0] || "skill inti"), "Ikut kompetisi / lomba sekolah"] },
      { term: "Mid Term (3-5 Tahun)", items: ["Magang / internship", (careers[0] ? "Pendalaman jalur " + careers[0].name : "Pendalaman skill")] },
      { term: "Long Term (5+ Tahun)", items: [(careers[0] ? careers[0].name : "Karir spesialis"), "Tech Leader / Founder"] },
    ];

    // === Recommendations ===
    const recommendations = [
      {
        color: "violet", icon: "graduation-cap", title: "Academic Recommendation",
        items: [
          "Fokus pada " + (careerSeed[0] === "I" ? "project STEM & sains data" : "project " + RIASEC_LABELS[careerSeed[0]] || "proyek peminatan"),
          "Tingkatkan kemampuan riset & analisa data",
        ],
      },
      {
        color: "blue", icon: "code-2", title: "Skill Recommendation",
        items: [
          "Deep Learning / Neural Network",
          "Data Analysis & Visualization",
          "Public Speaking & Presentation",
        ],
      },
      {
        color: "pink", icon: "rocket", title: "Activity Recommendation",
        items: [
          "AI & Robotics Club",
          "Innovation Competition",
          "Online Course (Coursera / edX)",
        ],
      },
    ];

    // === Smart summary ===
    const sortedAspects = IST_KEYS.slice().sort((a, b) => (istPct[b] || 0) - (istPct[a] || 0));
    const strengths = [
      "Analytical thinking " + (descLabel(istPct.AN || 50).toLowerCase()),
      "Cepat memahami konsep kompleks",
      "Logis, sistematis dan konsisten",
      "Adaptif terhadap teknologi baru",
    ];
    const developments = [
      "Public speaking perlu ditingkatkan",
      "Ekspresi emosi perlu lebih terbuka",
      "Manajemen waktu jangka panjang",
      "Kolaborasi lintas-disiplin",
    ];
    const potential = clamp(Math.round((skillAvg + (ist ? 90 : 60)) / 2), 60, 96);
    const potentialDesc = "Siswa memiliki potensi tinggi untuk berkarir di bidang " + (RIASEC_LABELS[careerSeed[0]] || "spesialisasi") + " & inovasi.";
    const strategy = [
      { name: "Project Based Learning", value: 95 },
      { name: "Visual Learning",        value: vak ? pct((vak.score_visual || 0) * 10) : 90 },
      { name: "Hands-on Practice",      value: vak ? pct((vak.score_kinesthetic || 0) * 10) : 88 },
      { name: "Collaborative Learning", value: pct(holPct.S || 60) },
    ];
    const insight = (u.nama_lengkap ? u.nama_lengkap.split(" ")[0] : "Siswa")
      + " memiliki potensi besar di bidang " + (RIASEC_LABELS[careerSeed[0]] || "spesialisasi")
      + ". Dengan fokus pengembangan skill yang tepat dan konsisten, ia dapat menjadi pemimpin inovasi di masa depan.";

    // === Student level (heuristik dari IST IQ) ===
    const iq = ist ? (ist.iq || 0) : 0;
    const level = Math.max(1, Math.round((iq || 90) / 5)); // IQ 100 &#8594; Lv 20
    const pctToNext = clamp((iq % 5) * 20, 10, 90);
    const rankBand = iq >= 130 ? 1 : iq >= 120 ? 5 : iq >= 110 ? 10 : iq >= 100 ? 25 : iq >= 90 ? 50 : 75;
    const rankLabel = "Top " + rankBand + "%";
    const rankNote = rankBand <= 10 ? "Excellent Progress" : rankBand <= 25 ? "Good Progress" : "Keep Going";

    const dominantTalent = (function () {
      const top = sortedAspects[0];
      const map = { RA: "Logika", AN: "Analitik", GE: "Konseptual", WA: "Bahasa", FA: "Visual", WU: "Spasial", ME: "Memori", SE: "Pengetahuan", ZA: "Numerikal" };
      return map[top] || "Teknologi";
    })();

    const topCareer = careers[0] || { name: "AI Engineer", match: 90 };

    const pmNote = ist
      ? "Skor analitik kuat, mendukung peran problem solver."
      : "Lakukan tes IST untuk profil kognitif lebih akurat.";

    return {
      user: u,
      studentLevel: { level, pctToNext, rankLabel, rankNote },
      personalityType: { label: personalityLabel, code: code || "-" },
      learningStyleLabel,
      dominantIntelligence,
      motivationLevel, stressLevel, leadershipPotential,
      pmNote,
      radarValues,
      emotional,
      skills, skillTotal, skillTotalLabel,
      careers, roadmap,
      recommendations,
      strengths, developments,
      potential, potentialDesc,
      strategy, insight,
      dominantTalent,
      topCareer,
    };
  }

  // ============== Teacher Batch Dashboard ==============

  var _allBatches = [];

  function getBatchTypeName(b) {
    if (b.enable_ist) return "IST";
    if (b.enable_holland) return "Holland";
    if (b.enable_learning_style) return "Gaya Belajar";
    if (b.enable_kraepelin) return "Kraepelin";
    if (b.enable_rmib) return "RMIB";
    if (b.enable_papi) return "PAPI";
    return "Tes";
  }

  function getBatchTypeKey(b) {
    if (b.enable_ist) return "ist";
    if (b.enable_holland) return "holland";
    if (b.enable_learning_style) return "learning_style";
    if (b.enable_kraepelin) return "kraepelin";
    if (b.enable_rmib) return "rmib";
    if (b.enable_papi) return "papi";
    return "";
  }

  function renderBatchCards(batches) {
    const grid = document.getElementById("teacherBatchGrid");
    if (!grid) return;
    grid.innerHTML = "";

    if (!batches || batches.length === 0) {
      grid.innerHTML = '<div style="grid-column:1/-1;text-align:center;padding:48px 20px;color:var(--ds-muted);">' +
        '<i data-lucide="folder-open" style="width:48px;height:48px;margin:0 auto 12px;display:block;opacity:0.3;"></i>' +
        '<p style="font-size:15px;font-weight:500;margin:0;">Tidak ada batch yang ditemukan</p>' +
        '<p style="font-size:13px;margin-top:4px;">Coba ubah filter pencarian Anda</p></div>';
      if (window.lucide) try { window.lucide.createIcons(); } catch(_) {}
      return;
    }

    batches.forEach(function(b) {
      const typeKey = getBatchTypeKey(b);
      const typeName = getBatchTypeName(b);
      const isArchived = b.status === "archived";
      const status = isArchived ? "archived" : ((b.status || "active") === "active" ? "aktif" : "selesai");
      const statusLabel = isArchived ? "Arsip" : (status === "aktif" ? "Aktif" : "Selesai");
      const total = b.participant_count || 0;
      const done = b.completed_count || 0;
      const pct = total > 0 ? Math.round((done / total) * 100) : 0;
      const kelas = b.kelas || "";
      const jurusan = b.jurusan || "";
      var batchTitle = b.name || b.tahun_ajaran || "-";
      var subList = [];
      if (b.name && b.tahun_ajaran) subList.push(b.tahun_ajaran);
      if (kelas) subList.push("Kelas " + kelas);
      if (jurusan) subList.push(jurusan);
      const subtitle = subList.join(" • ") || b.institution || "-";

      const card = document.createElement("div");
      card.className = "batch-card";
      card.setAttribute("data-batch-id", b.id);
      card.setAttribute("data-kelas", (kelas || "").toUpperCase());
      card.setAttribute("data-jurusan", (jurusan || "").toLowerCase());
      card.setAttribute("data-tipe", typeKey);
      card.setAttribute("data-name", (b.name || "").toLowerCase());
      card.innerHTML =
        '<div class="batch-card-header">' +
          '<span class="batch-type-badge ' + typeKey + '">' + typeName + '</span>' +
          '<span class="batch-status-badge ' + status + '">' + statusLabel + '</span>' +
        '</div>' +
        '<div class="batch-card-body">' +
          '<p class="batch-card-title-text"></p>' +
          '<p class="batch-card-subtitle-text"></p>' +
        '</div>' +
        '<div class="batch-card-footer">' +
          '<div class="batch-meta-item">' +
            '<i data-lucide="users"></i>' +
            '<span>' + total + ' peserta</span>' +
          '</div>' +
          '<div class="batch-meta-item">' +
            '<span class="batch-progress-pct">' + pct + '% selesai</span>' +
          '</div>' +
        '</div>';

      card.querySelector(".batch-card-title-text").textContent = batchTitle;
      card.querySelector(".batch-card-subtitle-text").textContent = subtitle;

      card.addEventListener("click", function() {
        window.location.href = "/dashboard/batch/" + b.id;
      });

      grid.appendChild(card);
    });

    if (window.lucide) try { window.lucide.createIcons(); } catch(_) {}
  }

  function filterAndRenderBatches() {
    const search = (document.getElementById("batchSearchInput") || {}).value || "";
    const kelas = (document.getElementById("batchFilterKelas") || {}).value || "";
    const jurusan = (document.getElementById("batchFilterJurusan") || {}).value || "";
    const tipe = (document.getElementById("batchFilterTipe") || {}).value || "";

    const filtered = _allBatches.filter(function(b) {
      const nameMatch = !search ||
        (b.name || "").toLowerCase().includes(search.toLowerCase()) ||
        (b.tahun_ajaran || "").toLowerCase().includes(search.toLowerCase()) ||
        (b.institution || "").toLowerCase().includes(search.toLowerCase());
      const kelasMatch = !kelas || (b.kelas || "").toUpperCase() === kelas.toUpperCase();
      const jurusanMatch = !jurusan || (b.jurusan || "").toLowerCase().includes(jurusan.toLowerCase());
      const tipeMatch = !tipe || getBatchTypeKey(b) === tipe;
      return nameMatch && kelasMatch && jurusanMatch && tipeMatch;
    });

    renderBatchCards(filtered);
  }

  async function initSchoolBatchDashboard(userData) {
    const section = document.getElementById("schoolDashboardSection");
    const grid = document.getElementById("teacherBatchGrid");
    const studentGrid = document.querySelector(".ds-grid");
    const welcomeBanner = document.getElementById("schoolWelcomeBanner");

    if (section) section.style.display = "block";
    if (studentGrid) studentGrid.style.display = "none";
    if (welcomeBanner) welcomeBanner.style.display = "none";

    const schoolName = userData.sekolah || userData.nama_lengkap || "Sekolah";

    const res = await fetchJson("/api/admin/test-batches");
    const batches = (res && res.data) ? res.data : [];
    _allBatches = batches;

    const activeBatches = batches.filter(function(b) { return b.status === "active"; });
    const titleEl = document.getElementById("teacherDashboardTitle");
    const subtitleEl = document.getElementById("teacherDashboardSubtitle");
    if (titleEl) titleEl.textContent = "Daftar Batch Tes";
    if (subtitleEl) subtitleEl.textContent = schoolName + " \u2014 " + batches.length + " batch aktif";

    renderBatchCards(batches);

    const searchInput = document.getElementById("batchSearchInput");
    const kelasFilter = document.getElementById("batchFilterKelas");
    const jurusanFilter = document.getElementById("batchFilterJurusan");
    const tipeFilter = document.getElementById("batchFilterTipe");

    if (searchInput) searchInput.addEventListener("input", filterAndRenderBatches);
    if (kelasFilter) kelasFilter.addEventListener("change", filterAndRenderBatches);
    if (jurusanFilter) jurusanFilter.addEventListener("change", filterAndRenderBatches);
    if (tipeFilter) tipeFilter.addEventListener("change", filterAndRenderBatches);
  }
 
  // ============== Skeleton Loaders ==============
  function skBar(w) {
    return '<div style="height:8px;background:linear-gradient(90deg,#e2e8f0 25%,#f1f5f9 50%,#e2e8f0 75%);background-size:200% 100%;animation:skShimmer 1.4s infinite;border-radius:99px;width:' + w + '%;margin:3px 0;"></div>';
  }
  (function injectSkStyles() {
    if (document.getElementById('sk-shimmer-style')) return;
    const s = document.createElement('style');
    s.id = 'sk-shimmer-style';
    s.textContent = '@keyframes skShimmer{0%{background-position:200% 0}100%{background-position:-200% 0}}';
    document.head.appendChild(s);
  })();

  function renderSkeletonEmotional() {
    const palette = ["#22c55e", "#3b82f6", "#8b5cf6", "#f59e0b", "#14b8a6", "#ec4899"];
    const ids = ["#gSelfAware","#gSelfReg","#gMotivation","#gEmpathy","#gStressMgmt","#gResilience"];
    ids.forEach((id, i) => {
      const node = $(id);
      if (!node) return;
      const ring = node.querySelector('.ring');
      if (ring) { ring.style.setProperty('--pct', '0deg'); ring.style.setProperty('--col', palette[i]); }
      const num = node.querySelector('.num');
      if (num) num.textContent = '...';
      const d = node.querySelector('.desc');
      if (d) { d.textContent = 'Memuat...'; d.className = 'desc'; }
    });
  }

  function renderSkeletonSkills() {
    const list = $("#skillList");
    if (!list) return;
    list.innerHTML = [
      [70], [55], [80], [60], [45], [65]
    ].map((_, i) => {
      const widths = [70, 55, 80, 60, 45, 65];
      return '<div class="ds-skill-row" style="pointer-events:none">'
        + '<span class="nm">' + skBar(40 + i * 5) + '</span>'
        + '<span class="vl" style="color:transparent">--%</span>'
        + '<div class="bar"><span style="width:' + widths[i] + '%;background:linear-gradient(90deg,#e2e8f0 25%,#f1f5f9 50%,#e2e8f0 75%);background-size:200% 100%;animation:skShimmer 1.4s infinite;"></span></div>'
        + '</div>';
    }).join('');
    const st = $("#skillTotal");
    if (st) st.textContent = '— /100';
    const stl = $("#skillTotalLabel");
    if (stl) stl.textContent = 'Menganalisis...';
  }

  function renderSkeletonCareer() {
    const list = $("#careerList");
    if (list) {
      list.innerHTML = [1,2,3,4,5].map(() =>
        '<div class="ds-career-row" style="pointer-events:none">'
        + '<span class="ic"><i data-lucide="briefcase"></i></span>'
        + '<span class="nm">' + skBar(50) + '</span>'
        + '<span class="pct" style="color:transparent">--%</span>'
        + '</div>'
      ).join('');
      if (window.lucide) try { window.lucide.createIcons(); } catch(_) {}
    }
    const tl = $("#careerTimeline");
    if (tl) {
      tl.innerHTML = ['Short Term','Mid Term','Long Term'].map(term =>
        '<div class="ds-roadmap-item">'
        + '<h5>' + term + '</h5>'
        + '<ul>' + skBar(70) + skBar(55) + '</ul>'
        + '</div>'
      ).join('');
    }
  }

  // ============== Live AI Fetching & Rendering ==============
  async function fetchAndRenderLiveCombinedAI(userData, summaryData, rmibList, papiList, ctx) {
    const list = $("#aiRecList");
    const strengthsUl = $("#smStrengths");
    const devUl = $("#smDev");
    const potentialDesc = $("#smPotentialDesc");
    const insightEl = $("#smInsight");
    const skillList = $("#skillList");
    const careerList = $("#careerList");
    const careerTimeline = $("#careerTimeline");
 
    if (list) {
      list.innerHTML = `<div class="text-center py-4" style="grid-column: 1/-1;"><div class="spinner-border text-primary" role="status" style="width: 24px; height: 24px; margin: 0 auto; display: block;"></div><p class="text-muted text-xs mt-2">Menganalisis rekomendasi karir & kegiatan dengan AI...</p></div>`;
    }
 
    const skLoading = `<li><div style="height:10px; background:#e9ecef; border-radius:4px; margin:6px 0; width:90%; animation: pulse 1.5s infinite;"></div></li>
                       <li><div style="height:10px; background:#e9ecef; border-radius:4px; margin:6px 0; width:80%; animation: pulse 1.5s infinite;"></div></li>`;
    if (strengthsUl) strengthsUl.innerHTML = skLoading;
    if (devUl) devUl.innerHTML = skLoading;
    if (potentialDesc) potentialDesc.textContent = "Menghitung potensi dengan AI...";
    if (insightEl) insightEl.textContent = "Memproses wawasan AI...";

    // Skeleton sudah dirender di init(), tapi pastikan juga jika dipanggil langsung
    if (!ctx) { renderSkeletonSkills(); renderSkeletonCareer(); renderSkeletonEmotional(); }

    // Collect completed tests
    const completedTests = [];
    const resultsPayload = {};
    if (summaryData) {
      if (summaryData.last_ist_result) { completedTests.push("ist"); resultsPayload.ist = summaryData.last_ist_result; }
      if (summaryData.last_holland_result) { completedTests.push("holland"); resultsPayload.holland = summaryData.last_holland_result; }
      if (summaryData.last_learning_style_result) { completedTests.push("learning_style"); resultsPayload.learning_style = summaryData.last_learning_style_result; }
      if (summaryData.last_kraepelin_attempt) { completedTests.push("kraepelin"); resultsPayload.kraepelin = summaryData.last_kraepelin_attempt; }
    }
    if (rmibList && rmibList.length > 0) {
      completedTests.push("rmib");
      resultsPayload.rmib = rmibList[0].rmib_result || rmibList[0];
    }
    if (papiList && papiList.length > 0) {
      completedTests.push("papi");
      resultsPayload.papi = papiList[0].papi_result || papiList[0];
    }
 
    if (completedTests.length === 0) {
      if (list) list.innerHTML = `<p class="text-muted text-xs text-center" style="grid-column: 1/-1;">Siswa belum mengerjakan alat tes apa pun.</p>`;
      if (strengthsUl) strengthsUl.innerHTML = `<li>Belum ada data tes</li>`;
      if (devUl) devUl.innerHTML = `<li>Belum ada data tes</li>`;
      if (potentialDesc) potentialDesc.textContent = "Siswa belum menyelesaikan tes.";
      if (insightEl) insightEl.textContent = "Belum ada tes yang diselesaikan.";
      // Fallback ke heuristik jika tidak ada tes sama sekali
      if (ctx) { renderEmotional(ctx); renderSkills(ctx); renderCareer(ctx); }
      return;
    }
 
    try {
      const studentName = userData.nama_lengkap || userData.email || "Siswa";
      const batchName = userData.sekolah || "Batch Umum";
      const payload = {
        student_name: studentName,
        batch_name: batchName,
        results: resultsPayload
      };
 
      const res = await fetch("/api/ai/student-combined-summary", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload)
      });

      // Deteksi error HTTP (token habis, rate limit, dll)
      if (!res.ok) {
        const status = res.status;
        let errMsg = "Gagal memuat rekomendasi AI.";
        if (status === 402 || status === 429) {
          errMsg = "Token AI tidak mencukupi atau rate limit tercapai. Coba lagi beberapa saat.";
          showAIToast(errMsg, "token");
        } else if (status >= 500) {
          errMsg = "Server AI sedang tidak tersedia. Silakan coba lagi nanti.";
          showAIToast(errMsg, "server");
        } else {
          showAIToast(errMsg, "error");
        }
        throw new Error(errMsg);
      }

      const json = await res.json();
      if (!json || !json.success || !json.data) {
        // Cek apakah error karena token
        const msg = (json && json.message) || "";
        if (/token|quota|rate|insufficient|limit/i.test(msg)) {
          showAIToast("Token AI tidak mencukupi. Rekomendasi menggunakan data lokal.", "token");
        }
        throw new Error("Gagal mengambil data AI");
      }
 
      const aiData = json.data;
 
      // 1. Render Recommendations
      if (list && Array.isArray(aiData.recommendations)) {
        list.innerHTML = "";
        aiData.recommendations.forEach((r) => {
          const item = document.createElement("div");
          item.className = "ds-rec-item " + (r.color || "violet");
          const itemsHtml = (r.items || []).map((t) => {
            const li = document.createElement("li");
            li.textContent = t;
            return li.outerHTML;
          }).join("");
 
          item.innerHTML = '<span class="ic"><i data-lucide="' + (r.icon || "sparkles") + '"></i></span>'
            + '<div style="flex:1;min-width:0;">'
            + '<h5>' + encodeHTML(r.title) + '</h5>'
            + '<ul>' + itemsHtml + '</ul>'
            + '</div>'
            + '<i data-lucide="chevron-right" style="width:16px;height:16px;color:var(--ds-muted);align-self:center;"></i>';
          list.appendChild(item);
        });
      }
 
      // 2. Strengths
      if (strengthsUl && Array.isArray(aiData.strengths)) {
        strengthsUl.innerHTML = "";
        aiData.strengths.forEach(s => {
          const li = document.createElement("li");
          li.textContent = s;
          strengthsUl.appendChild(li);
        });
      }
 
      // 3. Developments
      if (devUl && Array.isArray(aiData.developments)) {
        devUl.innerHTML = "";
        aiData.developments.forEach(d => {
          const li = document.createElement("li");
          li.textContent = d;
          devUl.appendChild(li);
        });
      }
 
      // 4. Potential
      if (aiData.potential != null) {
        setRing($("#smPotentialRing"), aiData.potential, "#8b5cf6");
        setText("#smPotentialVal", aiData.potential + "%");
      }
      if (aiData.potential_desc) {
        setText("#smPotentialDesc", aiData.potential_desc);
      }
 
      // 5. Insight
      if (aiData.insight) {
        setText("#smInsight", aiData.insight);
      }

      // 6. Emotional Analytics dari AI
      if (aiData.emotional_analytics) {
        const ea = aiData.emotional_analytics;
        const emotional = {
          selfAwareness:   typeof ea.selfAwareness   === 'number' ? ea.selfAwareness   : 50,
          selfRegulation:  typeof ea.selfRegulation  === 'number' ? ea.selfRegulation  : 50,
          motivation:      typeof ea.motivation      === 'number' ? ea.motivation      : 50,
          empathy:         typeof ea.empathy         === 'number' ? ea.empathy         : 50,
          stressManagement:typeof ea.stressManagement=== 'number' ? ea.stressManagement: 50,
          resilience:      typeof ea.resilience      === 'number' ? ea.resilience      : 50,
        };
        renderEmotional({ emotional });
      }

      // 7. Skill Tracker dari AI
      if (Array.isArray(aiData.skill_tracker) && aiData.skill_tracker.length > 0) {
        const skills = aiData.skill_tracker.map(s => ({
          name:  s.name  || "Skill",
          value: typeof s.value === 'number' ? Math.min(100, Math.max(0, s.value)) : 50
        }));
        const skillAvg = Math.round(skills.reduce((a, b) => a + b.value, 0) / skills.length);
        const skillTotal = skillAvg + " /100";
        const skillTotalLabel = skillAvg >= 75 ? "Good Performance" : skillAvg >= 55 ? "Average" : "Needs Improvement";
        renderSkills({ skills, skillTotal, skillTotalLabel });
      } else if (ctx) {
        // Tidak ada field dari AI (cache lama), fallback ke heuristik
        renderSkills(ctx);
      }

      // 8. Career Roadmap dari AI
      if (aiData.career_roadmap) {
        const cr = aiData.career_roadmap;
        const careers = Array.isArray(cr.careers) ? cr.careers.map(c => ({
          name:  c.name  || "Karir",
          match: typeof c.match === 'number' ? c.match : 75,
          icon:  c.icon  || "briefcase"
        })) : [];
        const roadmap = Array.isArray(cr.roadmap) ? cr.roadmap : [];

        // Update student profile stat tile (top career)
        if (careers.length > 0) {
          setText("#spCareerName", careers[0].name);
          setText("#spCareerMatch", careers[0].match + "%");
        }

        renderCareer({ careers, roadmap });
      } else if (ctx) {
        // Tidak ada field dari AI (cache lama), fallback ke heuristik
        renderCareer(ctx);
      }
 
      if (window.lucide && typeof window.lucide.createIcons === "function") {
        try { window.lucide.createIcons(); } catch (_) {}
      }
    } catch (err) {
      console.error(err);
      if (list) list.innerHTML = `<p class="text-danger text-xs text-center" style="grid-column: 1/-1;">Gagal memuat rekomendasi AI.</p>`;
      if (strengthsUl) strengthsUl.innerHTML = `<li>Gagal memuat data</li>`;
      if (devUl) devUl.innerHTML = `<li>Gagal memuat data</li>`;
      if (potentialDesc) potentialDesc.textContent = "Gagal memproses potensi.";
      if (insightEl) insightEl.textContent = "Gagal memproses wawasan AI.";
      // Fallback ke heuristik jika AI error
      if (ctx) { renderEmotional(ctx); renderSkills(ctx); renderCareer(ctx); }
      if (window.lucide && typeof window.lucide.createIcons === "function") {
        try { window.lucide.createIcons(); } catch (_) {}
      }
    }
  }
 
  // ============== Toast Alert System ==============
  function showAIToast(message, type) {
    // Pastikan container ada
    let container = document.getElementById("ai-toast-container");
    if (!container) {
      container = document.createElement("div");
      container.id = "ai-toast-container";
      container.style.cssText = [
        "position:fixed",
        "bottom:24px",
        "right:24px",
        "z-index:99999",
        "display:flex",
        "flex-direction:column",
        "gap:10px",
        "pointer-events:none"
      ].join(";");
      document.body.appendChild(container);
    }

    // Warna berdasarkan tipe
    const colors = {
      token:  { bg: "#fef2f2", border: "#fca5a5", icon: "#ef4444", text: "#991b1b" },
      server: { bg: "#fffbeb", border: "#fcd34d", icon: "#f59e0b", text: "#92400e" },
      error:  { bg: "#fef2f2", border: "#fca5a5", icon: "#ef4444", text: "#991b1b" },
      info:   { bg: "#eff6ff", border: "#93c5fd", icon: "#3b82f6", text: "#1e40af" },
    };
    const c = colors[type] || colors.error;

    const icons = {
      token:  "zap-off",
      server: "alert-triangle",
      error:  "alert-circle",
      info:   "info",
    };
    const iconName = icons[type] || "alert-circle";

    const toast = document.createElement("div");
    toast.style.cssText = [
      "pointer-events:auto",
      "display:flex",
      "align-items:flex-start",
      "gap:10px",
      "background:" + c.bg,
      "border:1px solid " + c.border,
      "border-radius:12px",
      "padding:12px 14px",
      "min-width:280px",
      "max-width:360px",
      "box-shadow:0 4px 16px rgba(0,0,0,0.12)",
      "animation:toastSlideIn 0.3s cubic-bezier(0.4,0,0.2,1)",
      "font-family:inherit",
    ].join(";");

    const iconSvg = '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="' + c.icon + '" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;margin-top:1px"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg>';
    const zapSvg  = '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="' + c.icon + '" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;margin-top:1px"><polygon points="13 2 3 14 12 14 11 22 21 10 12 10 13 2"/><line x1="1" y1="1" x2="23" y2="23"/></svg>';
    const warnSvg = '<svg xmlns="http://www.w3.org/2000/svg" width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="' + c.icon + '" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" style="flex-shrink:0;margin-top:1px"><path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg>';
    const usedSvg = type === 'token' ? zapSvg : type === 'server' ? warnSvg : iconSvg;

    const label = type === 'token' ? 'Token AI Habis' : type === 'server' ? 'AI Tidak Tersedia' : 'Error AI';

    toast.innerHTML =
      usedSvg +
      '<div style="flex:1;min-width:0;">' +
        '<div style="font-size:12px;font-weight:700;color:' + c.icon + ';margin-bottom:2px;">' + label + '</div>' +
        '<div style="font-size:12.5px;color:' + c.text + ';line-height:1.5;">' + encodeHTML(message) + '</div>' +
      '</div>' +
      '<button onclick="this.parentElement.remove()" style="background:none;border:none;cursor:pointer;color:' + c.text + ';opacity:0.6;font-size:16px;padding:0;line-height:1;flex-shrink:0;">✕</button>';

    container.appendChild(toast);

    // Auto dismiss setelah 7 detik
    setTimeout(function() {
      toast.style.animation = "toastSlideOut 0.3s ease forwards";
      setTimeout(function() { if (toast.parentElement) toast.remove(); }, 300);
    }, 7000);

    // Inject keyframes jika belum ada
    if (!document.getElementById("ai-toast-styles")) {
      const style = document.createElement("style");
      style.id = "ai-toast-styles";
      style.textContent = [
        "@keyframes toastSlideIn{",
          "from{transform:translateX(120%);opacity:0}",
          "to{transform:translateX(0);opacity:1}",
        "}",
        "@keyframes toastSlideOut{",
          "from{transform:translateX(0);opacity:1}",
          "to{transform:translateX(120%);opacity:0}",
        "}"
      ].join("");
      document.head.appendChild(style);
    }
  }

  // Expose globally for use in other pages
  window.showAIToast = showAIToast;

  function encodeHTML(s) {
    if (s == null) return '';
    return String(s).replace(/[&<>"']/g, function(c) {
      return {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c];
    });
  }
 
  // ============== Bootstrap ==============
  async function init() {
    const [profile, summary, rmibRes, papiRes] = await Promise.all([
      fetchJson("/api/profile"),
      fetchJson("/api/profile/test-summary"),
      fetchJson("/api/profile/rmib"),
      fetchJson("/api/profile/papi-results"),
    ]);
    const userData = profile && profile.data ? profile.data : (profile && profile.success ? profile.data : null);
    const summaryData = summary && summary.data ? summary.data : null;
    const rmibList = rmibRes && rmibRes.data ? rmibRes.data : [];
    const papiList = papiRes && papiRes.data ? papiRes.data : [];
 
    const ctx = buildContext(userData, summaryData, rmibList, papiList);
 
    renderTopbar(ctx);
    renderSidebarStats(ctx);
 
    const role = ((userData || {}).role || "").toLowerCase();
    if (role === "sekolah") {
      await initSchoolBatchDashboard(userData || {});
    } else {
      const isImpersonated = !!(userData && userData.is_impersonated);
      if (!isImpersonated) {
        // Sembunyikan bagian kompleks untuk siswa biasa
        const hideIds = [
          "#cardStudentProfile",
          "#cardPsychoMap",
          "#cardEmotional",
          "#cardSkillTracker",
          "#cardCareerRoadmap",
          "#cardAiRecommendation",
          "#cardAiSmartSummary"
        ];
        hideIds.forEach(id => {
          const el = $(id);
          if (el) el.style.display = "none";
        });
        const stats = $(".ds-profile-stats");
        if (stats) stats.style.display = "none";
 
        // Tampilkan welcome info card
        const welcome = $("#cardStudentWelcome");
        if (welcome) welcome.style.display = "block";
 
        renderStudentProfile(ctx);
      } else {
        // Tampilkan semua bagian kompleks untuk BK / sekolah (impersonasi)
        const welcome = $("#cardStudentWelcome");
        if (welcome) welcome.style.display = "none";
 
        const showIds = [
          "#cardStudentProfile",
          "#cardPsychoMap",
          "#cardEmotional",
          "#cardSkillTracker",
          "#cardCareerRoadmap",
          "#cardAiRecommendation",
          "#cardAiSmartSummary"
        ];
        showIds.forEach(id => {
          const el = $(id);
          if (el) el.style.display = "block";
        });
        const stats = $(".ds-profile-stats");
        if (stats) stats.style.display = "";
 
        renderStudentProfile(ctx);
        renderPsychoMap(ctx);
        // Emotional, Skills, Career: tampilkan skeleton dulu — AI akan isi
        renderSkeletonEmotional();
        renderSkeletonSkills();
        renderSkeletonCareer();
        renderAIRec(ctx); // render default fallback first
        renderSmart(ctx);  // render default fallback first
 
        // Jalankan fetch AI live & dinamis (akan override skeleton di atas)
        fetchAndRenderLiveCombinedAI(userData, summaryData, rmibList, papiList, ctx);
      }
    }
 
    // Render semua Lucide icons
    if (window.lucide && typeof window.lucide.createIcons === "function") {
      try { window.lucide.createIcons(); } catch (_) {}
    }
  }

  document.addEventListener("DOMContentLoaded", init);
})();
