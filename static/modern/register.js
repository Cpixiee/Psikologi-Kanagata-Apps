let registerCaptchaId = "";

function registerLoadCaptcha() {
  fetch("/api/auth/captcha")
    .then((response) => response.json())
    .then((data) => {
      if (data.success) {
        registerCaptchaId = data.data.captcha_id;
        const el = document.getElementById("captchaImage");
        if (el) {
          el.innerHTML =
            '<img src="/api/auth/captcha/' +
            registerCaptchaId +
            '" alt="CAPTCHA">';
        }
      }
    })
    .catch(() => {
      psShowAlert("Gagal memuat CAPTCHA", "error");
    });
}

window.addEventListener("DOMContentLoaded", function () {
  registerLoadCaptcha();
  psTogglePassword("password", "togglePassword");

  // Auto-fill email dari ?email=... (mis. dari link undangan tes psikologi).
  // Saat email berasal dari undangan, kunci field-nya supaya peserta tidak
  // mengubah email dan kehilangan tautan ke invitation.
  try {
    const params = new URLSearchParams(window.location.search);
    const prefillEmail = params.get("email");
    if (prefillEmail) {
      const emailInput = document.getElementById("email");
      if (emailInput) {
        emailInput.value = prefillEmail;
        emailInput.readOnly = true;
        emailInput.classList.add("bg-slate-100");
      }
    }
  } catch (_) {}

  const captchaImage = document.getElementById("captchaImage");
  if (captchaImage) {
    captchaImage.addEventListener("click", function () {
      registerLoadCaptcha();
      const input = document.getElementById("captchaValue");
      if (input) input.value = "";
    });
  }

  const form = document.getElementById("registerForm");
  if (!form) return;

  form.addEventListener("submit", function (e) {
    e.preventDefault();

    const formData = {
      nama_lengkap: document.getElementById("namaLengkap").value,
      alamat: document.getElementById("alamat").value,
      jenis_kelamin: document.getElementById("jenisKelamin").value,
      email: document.getElementById("email").value,
      no_handphone: document.getElementById("noHandphone").value,
      password: document.getElementById("password").value,
      captcha_id: registerCaptchaId,
      captcha_value: document.getElementById("captchaValue").value,
      role: document.getElementById("role")
        ? document.getElementById("role").value
        : "siswa",
    };

    if (!registerCaptchaId || !formData.captcha_value) {
      psShowAlert("Harap isi CAPTCHA", "error");
      return;
    }

    const loading = document.getElementById("loading");
    const btn = document.getElementById("registerBtn");
    if (loading) loading.style.display = "flex";
    if (btn) btn.disabled = true;

    fetch("/api/auth/register", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(formData),
    })
      .then((response) => response.json())
      .then((data) => {
        if (loading) loading.style.display = "none";
        if (btn) btn.disabled = false;

        if (data.success) {
          // Jika peserta datang dari link undangan (?next=...), arahkan ke
          // /login dengan parameter next agar setelah login otomatis
          // diteruskan ke halaman tes (/test?token=...).
          let redirectTo = "/";
          try {
            const params = new URLSearchParams(window.location.search);
            const next = params.get("next");
            if (next) {
              redirectTo = "/login?next=" + encodeURIComponent(next);
            }
          } catch (_) {}
          psShowAlert("Registrasi berhasil! Silakan login.", "success");
          setTimeout(() => {
            window.location.href = redirectTo;
          }, 1500);
        } else {
          psShowAlert(data.message || "Registrasi gagal", "error");
          registerLoadCaptcha();
        }
      })
      .catch(() => {
        if (loading) loading.style.display = "none";
        if (btn) btn.disabled = false;
        psShowAlert("Terjadi kesalahan. Silakan coba lagi.", "error");
        registerLoadCaptcha();
      });
  });
});

