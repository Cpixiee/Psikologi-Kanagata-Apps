package controllers

import (
	"strings"
	"time"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

// PsychotestClientController menangani alur peserta ketika membuka link undangan tes
type PsychotestClientController struct {
	beego.Controller
}

// @router /test [get]
// Halaman input token undangan tes psikologi.
// User harus login, lalu memasukkan token yang dikirim via email.
func (c *PsychotestClientController) TokenPage() {
	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login?next=/test", 302)
		return
	}

	// Pesan error (jika ada) bisa dikirim via flash / query, tapi untuk sederhana sekarang langsung render form.
	c.TplName = "test_token.html"
}

// @router /test/start [post]
// Halaman entry point tes psikologi berbasis token undangan.
// Syarat:
// - User sudah login
// - Token valid, belum kedaluwarsa, dan masih berstatus pending
// - Token memang milik user yang sedang login (berdasarkan Email/UserId)
func (c *PsychotestClientController) StartTest() {
	// Pastikan user sudah login (filter global seharusnya sudah mengecek, tapi kita jaga-jaga lagi)
	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login?next=/test", 302)
		return
	}

	userID, ok := sessionUser.(int)
	if !ok || userID == 0 {
		c.Redirect("/login?next=/test", 302)
		return
	}

	// Ambil token dari form POST
	token := strings.TrimSpace(c.GetString("token"))
	if token == "" {
		c.Data["Error"] = "Token wajib diisi."
		c.TplName = "test_token.html"
		return
	}

	o := orm.NewOrm()

	// Ambil user yang sedang login
	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Data["Error"] = "Akun Anda tidak ditemukan. Silakan login ulang."
		c.TplName = "test_token.html"
		return
	}

	// Cari undangan berdasarkan token (tidak sensitif huruf besar/kecil)
	// Gunakan iexact agar peserta tidak bermasalah jika salah menggunakan Caps Lock / lowercase.
	var inv models.TestInvitation
	if err := o.QueryTable(new(models.TestInvitation)).Filter("Token__iexact", token).One(&inv); err != nil || inv.Id == 0 {
		c.Data["Error"] = "Token undangan tidak dikenal atau sudah dicabut. Pastikan Anda mengetik token dengan benar."
		c.TplName = "test_token.html"
		return
	}

	// Pastikan token memang milik user yang login (proteksi jika token dibocorkan)
	if inv.UserId == nil || *inv.UserId != user.Id || inv.Email != user.Email {
		c.Data["Error"] = "Token ini tidak terhubung dengan akun yang sedang login. Silakan login dengan email yang diundang."
		c.TplName = "test_token.html"
		return
	}

	now := time.Now()

	// Cek kedaluwarsa
	if now.After(inv.ExpiresAt) {
		// Update status menjadi expired jika belum
		if inv.Status != models.StatusInvitationExpired {
			inv.Status = models.StatusInvitationExpired
			_, _ = o.Update(&inv, "Status")
		}

		c.Data["Error"] = "Masa berlaku undangan sudah habis (lebih dari 1 hari). Silakan hubungi admin untuk mengirim undangan baru."
		c.TplName = "test_token.html"
		return
	}

	// Jika undangan sudah dipakai (status used) dan hasil IST sudah ada,
	// gunakan token sebagai "kartu akses" untuk melihat kembali hasil.
	if inv.Status == models.StatusInvitationUsed {
		var istRes models.ISTResult
		if err := o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&istRes); err == nil && istRes.Id != 0 {
			c.SetSession("current_invitation_id", inv.Id)
			c.SetSession("current_batch_id", inv.BatchId)
			c.Redirect("/test/ist/result", 302)
			return
		}
	}

	// Hanya status pending yang boleh memulai tes baru
	if inv.Status != models.StatusInvitationPending {
		c.Data["Error"] = "Undangan ini sudah tidak bisa digunakan (status: " + inv.Status + "). Jika perlu mengulang, hubungi admin."
		c.TplName = "test_token.html"
		return
	}

	// Simpan informasi undangan di session untuk dipakai alur tes berikutnya
	c.SetSession("current_invitation_id", inv.Id)
	c.SetSession("current_batch_id", inv.BatchId)

	// Setelah token valid, langsung arahkan ke alur test IST (start page).
	c.Redirect("/test/ist/start", 302)
}

