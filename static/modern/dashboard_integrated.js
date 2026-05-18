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
  // RIASEC dimension → label & deskripsi singkat
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

  // Career rekomendasi berdasarkan kombinasi top RIASEC
  const CAREER_BY_LETTER = {
    R: [
      { name: "Mechanical Engineer", base: 88 },
      { name: "Robotics Engineer", base: 84 },
      { name: "Field Technician", base: 80 },
    ],
    I: [
      { name: "Data Scientist", base: 90 },
      { name: "AI Engineer", base: 92 },
      { name: "Research Scientist", base: 88 },
    ],
    A: [
      { name: "UI/UX Designer", base: 87 },
      { name: "Creative Director", base: 84 },
      { name: "Content Creator", base: 82 },
    ],
    S: [
      { name: "Guru / Educator", base: 86 },
      { name: "Konselor", base: 84 },
      { name: "HR Specialist", base: 82 },
    ],
    E: [
      { name: "Entrepreneur", base: 88 },
      { name: "Tech Lead / Founder", base: 86 },
      { name: "Marketing Manager", base: 82 },
    ],
    C: [
      { name: "Software Developer", base: 89 },
      { name: "Cyber Security Analyst", base: 86 },
      { name: "Financial Analyst", base: 84 },
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
    };
    return map[(role || "").toLowerCase()] || "Siswa";
  }

  // ============== Render: Sidebar (kelas & jurusan) ==============
  function renderSidebarStats(ctx) {
    const u = ctx.user || {};
    setText("#sbKelasValue", u.kelas || "-");
    setText("#sbJurusanValue", u.jurusan || u.asal_instansi || "-");
  }

  // ============== Render: Topbar ==============
  function renderTopbar(ctx) {
    const u = ctx.user || {};
    setText("#topUserName", u.nama_lengkap || u.email || "Siswa");
    setText("#topUserRole", roleLabel(u.role));
    const ava = $("#topUserAvatar");
    if (ava) ava.src = avatarUrl(u);
  }

  // ============== Render: Student Profile ==============
  function renderStudentProfile(ctx) {
    const u = ctx.user || {};
    setText("#spName", u.nama_lengkap || "-");
    setText("#spStudentId", "Student ID: " + (u.nisn || u.nip || "—"));
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
    setText("#spTalent", ctx.dominantTalent || "—");
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
      if (!node) return;
      setRing(node.querySelector(".ring"), m[2], palette[i]);
      const d = node.querySelector(".desc");
      if (d) { d.textContent = descLabel(m[2]); d.className = "desc " + descClass(m[2]); }
    });

    const stable = $("#emoStable");
    if (stable) {
      const avg = (g.selfAwareness + g.selfRegulation + g.resilience) / 3;
      const lab = avg >= 70 ? "Stable" : (avg >= 50 ? "Moderate" : "Unstable");
      const cls = avg >= 70 ? "good" : (avg >= 50 ? "warn" : "bad");
      stable.innerHTML = '<span>Emotional Status</span><span class="v ' + cls + '">● ' + lab + "</span>";
    }
    const burn = $("#emoBurnout");
    if (burn) {
      const risk = 100 - g.stressManagement;
      const lab = risk >= 60 ? "High" : (risk >= 35 ? "Medium" : "Low");
      const cls = risk >= 60 ? "bad" : (risk >= 35 ? "warn" : "good");
      burn.innerHTML = '<span>Burnout Risk</span><span class="v ' + cls + '">↓ ' + lab + "</span>";
    }
  }

  // ============== Render: Skill Tracker ==============
  function renderSkills(ctx) {
    const list = $("#skillList");
    if (!list) return;
    list.innerHTML = "";
    const colors = ["", "violet", "green", "amber", "teal", "pink"];
    ctx.skills.forEach((s, i) => {
      const row = el("div", { class: "ds-skill-row " + (colors[i % colors.length]) });
      row.appendChild(el("span", { class: "nm" }, s.name));
      row.appendChild(el("span", { class: "vl" }, s.value + "%"));
      const bar = el("div", { class: "bar" });
      bar.appendChild(el("span", { style: "width:" + s.value + "%" }));
      row.appendChild(bar);
      list.appendChild(row);
    });
    setText("#skillTotal", ctx.skillTotal);
    setText("#skillTotalLabel", ctx.skillTotalLabel);
  }

  // ============== Render: Career Roadmap ==============
  function renderCareer(ctx) {
    const list = $("#careerList");
    if (!list) return;
    list.innerHTML = "";
    ctx.careers.forEach((c) => {
      const row = document.createElement("div");
      row.className = "ds-career-row";
      row.innerHTML = '<span class="ic"><i data-lucide="' + (c.icon || "briefcase") + '"></i></span>'
        + '<span class="nm"></span>'
        + '<span class="pct">' + c.match + "%</span>";
      row.querySelector(".nm").textContent = c.name;
      list.appendChild(row);
    });

    const tl = $("#careerTimeline");
    if (tl) {
      tl.innerHTML = "";
      ctx.roadmap.forEach((r) => {
        const itm = el("div", { class: "ds-roadmap-item" });
        itm.appendChild(el("h5", null, r.term));
        const ul = el("ul");
        r.items.forEach((i) => ul.appendChild(el("li", null, i)));
        itm.appendChild(ul);
        tl.appendChild(itm);
      });
    }
  }

  // ============== Render: AI Recommendation ==============
  function renderAIRec(ctx) {
    const list = $("#aiRecList");
    if (!list) return;
    list.innerHTML = "";
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
      list.appendChild(item);
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
        // map 80–130 → 0–100
        istPct[k] = pct(((v - 80) / 50) * 100);
      });
    } else {
      IST_KEYS.forEach((k) => (istPct[k] = 50));
    }

    // === Holland percentages (skor R/I/A/S/E/C; max kira-kira 40) ===
    const holScores = hol ? { R: hol.score_r, I: hol.score_i, A: hol.score_a, S: hol.score_s, E: hol.score_e, C: hol.score_c } : { R:0,I:0,A:0,S:0,E:0,C:0 };
    const holMax = Math.max(40, ...Object.values(holScores));
    const holPct = {};
    Object.keys(holScores).forEach((k) => { holPct[k] = pct((holScores[k] / holMax) * 100); });

    // === Personality type from RIASEC ===
    const code = hol ? (hol.code || (hol.top1 + hol.top2 + hol.top3)) : "";
    const personalityLabel = code
      ? code.split("").map((c) => RIASEC_LABELS[c] || c).slice(0,2).join(" - ")
      : "Belum tersedia";

    // === Learning style label ===
    const learningStyleLabel = vak
      ? (vak.dominant_type || "—")
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
    // Motivation: Holland E + RMIB top "Persuasif" sederhana → pakai E score
    const motivationPct = pct(((holPct.E || 50) + (rmibTop ? 70 : 50)) / 2);
    const motivationLevel = motivationPct >= 70 ? "High" : motivationPct >= 45 ? "Mid" : "Low";

    // Stress: dari Kraepelin error rate (lower=better → invert)
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

    // === Skill Tracker (mapping IST + Holland) ===
    const skills = [
      { name: "Coding & Programming",  value: pct(((istPct.RA || 0) + (holPct.I || 0)) / 2) },
      { name: "Problem Solving",       value: istPct.RA || 50 },
      { name: "Critical Thinking",     value: istPct.AN || 50 },
      { name: "Communication",         value: pct(((istPct.WA || 0) + (holPct.S || 0)) / 2) },
      { name: "Leadership",            value: leadershipPct },
      { name: "Creativity",            value: Math.max(istPct.GE || 0, istPct.FA || 0, holPct.A || 0) },
    ];
    const skillAvg = pct(skills.reduce((a, b) => a + b.value, 0) / skills.length);
    const skillTotal = skillAvg + " /100";
    const skillTotalLabel = skillAvg >= 75 ? "Good Performance" : skillAvg >= 55 ? "Average" : "Needs Improvement";

    // === Career roadmap ===
    const careerSeed = (code || "ISE").split("");
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
    const level = Math.max(1, Math.round((iq || 90) / 5)); // IQ 100 → Lv 20
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
      personalityType: { label: personalityLabel, code: code || "—" },
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
    renderStudentProfile(ctx);
    renderPsychoMap(ctx);
    renderEmotional(ctx);
    renderSkills(ctx);
    renderCareer(ctx);
    renderAIRec(ctx);
    renderSmart(ctx);

    // Render semua Lucide icons (termasuk yang baru dimasukkan dinamis).
    if (window.lucide && typeof window.lucide.createIcons === "function") {
      try { window.lucide.createIcons(); } catch (_) {}
    }
  }

  document.addEventListener("DOMContentLoaded", init);
})();
