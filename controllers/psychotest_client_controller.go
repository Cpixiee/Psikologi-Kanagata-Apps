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
// Mendukung deep link `?token=...` dari email/WA undangan.
// Jika user belum login:
//   - token+email punya akun  -> redirect /login?next=/test?token=...
//   - token+email belum punya akun -> redirect /register?email=...&next=/test?token=...
//   - tanpa token              -> redirect /login?next=/test seperti biasa.
func (c *PsychotestClientController) TokenPage() {
	tokenQ := strings.TrimSpace(c.GetString("token"))
	sessionUser := c.GetSession("user_id")

	if sessionUser == nil {
		if tokenQ != "" {
			// Coba lookup invitation untuk arahkan ke register / login otomatis.
			o := orm.NewOrm()
			var inv models.TestInvitation
			if err := o.QueryTable(new(models.TestInvitation)).Filter("Token__iexact", tokenQ).One(&inv); err == nil && inv.Id != 0 {
				next := "/test?token=" + tokenQ
				if inv.UserId != nil && *inv.UserId != 0 {
					c.Redirect("/login?next="+next, 302)
					return
				}
				// Email belum terdaftar -> register dengan email pre-filled.
				c.Redirect("/register?email="+inv.Email+"&next="+next, 302)
				return
			}
		}
		c.Redirect("/login?next=/test", 302)
		return
	}

	// Sudah login: kalau token disediakan via query, auto-fill pada template.
	if tokenQ != "" {
		c.Data["Token"] = tokenQ
	}
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

	// Pastikan token memang milik user yang login (proteksi jika token dibocorkan).
	// Pencocokan dilakukan via EMAIL (case-insensitive) karena invitation untuk
	// email yang belum terdaftar boleh memiliki UserId == nil; setelah user
	// registrasi, kita auto-link UserId-nya.
	if !strings.EqualFold(strings.TrimSpace(inv.Email), strings.TrimSpace(user.Email)) {
		c.Data["Error"] = "Token ini tidak terhubung dengan akun yang sedang login. Silakan login dengan email yang diundang."
		c.TplName = "test_token.html"
		return
	}
	if inv.UserId == nil || *inv.UserId == 0 {
		inv.UserId = &user.Id
		_, _ = o.Update(&inv, "UserId")
	} else if *inv.UserId != user.Id {
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

		// Learning style fallback: jika hasil VAK ada, arahkan ke finish.
		var vakRes models.LearningStyleResult
		if err := o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&vakRes); err == nil && vakRes.Id != 0 {
			c.SetSession("current_invitation_id", inv.Id)
			c.SetSession("current_batch_id", inv.BatchId)
			c.Redirect("/profile/learning-style", 302)
			return
		}

		// Holland fallback: jika hasil Holland ada, arahkan ke profile.
		var holRes models.HollandResult
		if err := o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&holRes); err == nil && holRes.Id != 0 {
			c.SetSession("current_invitation_id", inv.Id)
			c.SetSession("current_batch_id", inv.BatchId)
			c.Redirect("/profile/holland", 302)
			return
		}

		// PAPI fallback: jika hasil PAPI ada, arahkan ke profile.
		var papiRes models.PAPIResult
		if err := o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&papiRes); err == nil && papiRes.Id != 0 {
			c.SetSession("current_invitation_id", inv.Id)
			c.SetSession("current_batch_id", inv.BatchId)
			c.Redirect("/profile/papi", 302)
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

	// Setelah token valid, arahkan ke alur test sesuai konfigurasi batch.
	// Batch harus memilih SATU jenis tes (tidak ada prioritas/default).
	var batch models.TestBatch
	if inv.BatchId != nil {
		batch.Id = *inv.BatchId
		if err := o.Read(&batch); err != nil {
			// jika gagal load batch, fallback IST
			c.Redirect("/test/ist/start", 302)
			return
		}
	}
	if batch.EnableIST {
		c.Redirect("/test/ist/start", 302)
		return
	}
	if batch.EnableHolland {
		c.Redirect("/test/holland/start", 302)
		return
	}
	if batch.EnableLearningStyle {
		c.Redirect("/test/learning-style/start", 302)
		return
	}
	if batch.EnableKraepelin {
		c.Redirect("/test/kraepelin/start", 302)
		return
	}
	if batch.EnableRMIB {
		c.Redirect("/test/rmib/start", 302)
		return
	}
	if batch.EnablePAPI {
		c.Redirect("/test/papi/start", 302)
		return
	}
	c.Data["Error"] = "Batch tes belum dikonfigurasi (pilih jenis tes). Silakan hubungi admin."
	c.TplName = "test_token.html"
}

