// Shared helpers for login & register pages

function psShowAlert(message, type) {
  // Popup modern dengan SweetAlert2 jika tersedia
  if (typeof Swal !== "undefined" && Swal.fire) {
    const icon =
      type === "success" ? "success" :
      type === "error" ? "error" : "info";

    Swal.fire({
      icon: icon,
      title: icon === "success" ? "Berhasil" : "Informasi",
      text: message,
      confirmButtonText: "OK",
      confirmButtonColor: "#4f46e5"
    });
  } else {
    // fallback ke alert biasa
    try {
      window.alert(message);
    } catch (e) {}
  }

  // Tetap isi badge kecil di halaman jika ada
  const alert = document.getElementById("alert");
  if (!alert) return;

  const base =
    "text-sm rounded-xl px-3.5 py-2.5 mb-2 border transition";
  const variants = {
    success:
      "bg-emerald-50 text-emerald-700 border-emerald-200",
    error:
      "bg-rose-50 text-rose-700 border-rose-200",
    info:
      "bg-sky-50 text-sky-700 border-sky-200",
  };

  const variant = variants[type] || variants.info;
  alert.className = base + " " + variant;
  alert.textContent = message;
  alert.style.display = "block";

  setTimeout(() => {
    alert.style.display = "none";
  }, 5000);
}

function psTogglePassword(id, toggleId) {
  const input = document.getElementById(id);
  const toggle = document.getElementById(toggleId);
  if (!input || !toggle) return;

  toggle.addEventListener("click", function () {
    const isPassword = input.type === "password";
    input.type = isPassword ? "text" : "password";
    toggle.textContent = isPassword ? "Sembunyikan" : "Tampilkan";
  });
}

