let loginCaptchaId = "";

function getNextPathFromQuery() {
  try {
    const qs = new URLSearchParams(window.location.search);
    const next = (qs.get("next") || "").trim();
    if (next && next.startsWith("/")) return next;
  } catch (e) {}
  return "/dashboard";
}

function getGoogleErrorMessage(code) {
  const map = {
    config: "Google login belum dikonfigurasi oleh admin.",
    state: "Sesi login Google tidak valid. Silakan coba lagi.",
    token: "Gagal verifikasi token Google.",
    userinfo: "Gagal mengambil data akun Google.",
    userdata: "Data akun Google tidak valid.",
    internal: "Terjadi kesalahan internal saat login Google.",
    device: "Perangkat ini diblokir. Hubungi administrator.",
  };
  return map[code] || "";
}

function loginLoadCaptcha() {
  fetch("/api/auth/captcha")
    .then((response) => response.json())
    .then((data) => {
      if (data.success) {
        loginCaptchaId = data.data.captcha_id;
        const el = document.getElementById("captchaImage");
        if (el) {
          el.innerHTML =
            '<img src="/api/auth/captcha/' +
            loginCaptchaId +
            '" alt="CAPTCHA">';
        }
      }
    })
    .catch(() => {
      psShowAlert("Gagal memuat CAPTCHA", "error");
    });
}

window.addEventListener("DOMContentLoaded", function () {
  loginLoadCaptcha();
  psTogglePassword("password", "togglePassword");
  const nextPath = getNextPathFromQuery();
  try {
    const qs = new URLSearchParams(window.location.search);
    const gErr = (qs.get("google_error") || "").trim();
    const gMsg = getGoogleErrorMessage(gErr);
    if (gMsg) psShowAlert(gMsg, "error");
  } catch (e) {}

  const captchaImage = document.getElementById("captchaImage");
  if (captchaImage) {
    captchaImage.addEventListener("click", function () {
      loginLoadCaptcha();
      const input = document.getElementById("captchaValue");
      if (input) input.value = "";
    });
  }

  const form = document.getElementById("loginForm");
  const googleBtn = document.getElementById("googleLoginBtn");
  if (googleBtn) {
    googleBtn.addEventListener("click", function () {
      const googleURL =
        "/api/auth/google/login?next=" + encodeURIComponent(nextPath);
      window.location.href = googleURL;
    });
  }
  if (!form) return;

  form.addEventListener("submit", function (e) {
    e.preventDefault();

    const email = document.getElementById("email").value;
    const password = document.getElementById("password").value;
    const captchaValue = document.getElementById("captchaValue").value;

    if (!loginCaptchaId || !captchaValue) {
      psShowAlert("Harap isi CAPTCHA", "error");
      return;
    }

    const loading = document.getElementById("loading");
    const btn = document.getElementById("loginBtn");
    if (loading) loading.style.display = "flex";
    if (btn) btn.disabled = true;

    fetch("/api/auth/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify({
        email: email,
        password: password,
        captcha_id: loginCaptchaId,
        captcha_value: captchaValue,
      }),
    })
      .then((response) => response.json())
      .then((data) => {
        if (loading) loading.style.display = "none";
        if (btn) btn.disabled = false;

        if (data.success) {
          psShowAlert("Login berhasil!", "success");
          setTimeout(() => {
            window.location.href = nextPath;
          }, 900);
        } else {
          psShowAlert(data.message || "Login gagal", "error");
          loginLoadCaptcha();
        }
      })
      .catch(() => {
        if (loading) loading.style.display = "none";
        if (btn) btn.disabled = false;
        psShowAlert("Terjadi kesalahan. Silakan coba lagi.", "error");
        loginLoadCaptcha();
      });
  });
});

