let loginCaptchaId = "";

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

  const captchaImage = document.getElementById("captchaImage");
  if (captchaImage) {
    captchaImage.addEventListener("click", function () {
      loginLoadCaptcha();
      const input = document.getElementById("captchaValue");
      if (input) input.value = "";
    });
  }

  const form = document.getElementById("loginForm");
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
            window.location.href = "/dashboard";
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

