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

	// Sudah login: jika session invitation belum ada, coba auto-resolve dari DB
	o := orm.NewOrm()
	userID, _ := sessionUser.(int)
	var user models.User
	user.Id = userID
	_ = o.Read(&user)

	if c.GetSession("current_invitation_id") == nil {
		if inv, ok := ResolveActiveInvitation(o, userID, user.Email); ok {
			c.SetSession("current_invitation_id", inv.Id)
			bID := 0
			if inv.BatchId != nil {
				bID = *inv.BatchId
			}
			c.SetSession("current_batch_id", bID)

			var batch models.TestBatch
			if inv.BatchId != nil {
				batch.Id = *inv.BatchId
				_ = o.Read(&batch)
			}
			nextURL := GetNextTestRedirect(inv.Id, &batch)
			if nextURL != "" {
				c.Redirect(nextURL, 302)
				return
			}
		}
	}

	// Kalau token disediakan via query, auto-fill pada template.
	if tokenQ != "" {
		c.Data["Token"] = tokenQ
	}
	c.TplName = "test_token.html"
}

// ResolveActiveInvitation auto-recovers the latest active pending invitation for a user.
// This ensures checkpoints work even in Incognito mode or after session cookie resets.
func ResolveActiveInvitation(o orm.Ormer, userID int, email string) (*models.TestInvitation, bool) {
	if o == nil {
		o = orm.NewOrm()
	}
	var inv models.TestInvitation
	now := time.Now()

	// 1. Try matching by UserId
	if userID > 0 {
		err := o.QueryTable(new(models.TestInvitation)).
			Filter("UserId", userID).
			Filter("Status", models.StatusInvitationPending).
			Filter("ExpiresAt__gt", now).
			OrderBy("-CreatedAt").
			One(&inv)
		if err == nil && inv.Id > 0 {
			return &inv, true
		}
	}

	// 2. Try matching by Email
	email = strings.TrimSpace(email)
	if email != "" {
		err := o.QueryTable(new(models.TestInvitation)).
			Filter("Email__iexact", email).
			Filter("Status", models.StatusInvitationPending).
			Filter("ExpiresAt__gt", now).
			OrderBy("-CreatedAt").
			One(&inv)
		if err == nil && inv.Id > 0 {
			if userID > 0 && (inv.UserId == nil || *inv.UserId == 0) {
				inv.UserId = &userID
				_, _ = o.Update(&inv, "UserId")
			}
			return &inv, true
		}
	}

	return nil, false
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

	// Jika undangan sudah dipakai (status used), redirect to hasil-tes page.
	if inv.Status == models.StatusInvitationUsed {
		c.Redirect("/hasil-tes", 302)
		return
	}

	// Hanya status pending yang boleh memulai tes baru
	if inv.Status != models.StatusInvitationPending {
		c.Data["Error"] = "Undangan ini sudah tidak bisa digunakan (status: " + inv.Status + "). Jika perlu mengulang, hubungi admin."
		c.TplName = "test_token.html"
		return
	}

	// Simpan informasi undangan di session untuk dipakai alur tes berikutnya
	c.SetSession("current_invitation_id", inv.Id)
	if inv.BatchId != nil {
		c.SetSession("current_batch_id", *inv.BatchId)
	} else {
		c.SetSession("current_batch_id", 0)
	}

	// Setelah token valid, arahkan ke alur test sesuai konfigurasi batch.
	var batch models.TestBatch
	if inv.BatchId != nil {
		batch.Id = *inv.BatchId
		if err := o.Read(&batch); err != nil {
			// jika gagal load batch, fallback IST
			c.Redirect("/test/ist/start", 302)
			return
		}
	}

	nextURL := GetNextTestRedirect(inv.Id, &batch)
	if nextURL != "" {
		c.Redirect(nextURL, 302)
		return
	}

	// If all are completed, mark invitation status as used
	if inv.Status != models.StatusInvitationUsed {
		inv.Status = models.StatusInvitationUsed
		inv.UsedAt = time.Now()
		_, _ = o.Update(&inv, "Status", "UsedAt")
	}
	c.Redirect("/hasil-tes", 302)
}

// GetNextTestRedirect determines which test is incomplete and returns its start URL
func GetNextTestRedirect(invID int, batch *models.TestBatch) string {
	if batch == nil {
		return ""
	}

	var order []string
	if strings.TrimSpace(batch.TestOrder) != "" {
		order = strings.Split(batch.TestOrder, ",")
	} else {
		// Fallback: build default order from active boolean flags
		if batch.EnableIST {
			order = append(order, "ist")
		}
		if batch.EnableHolland {
			order = append(order, "holland")
		}
		if batch.EnableLearningStyle {
			order = append(order, "learning_style")
		}
		if batch.EnableKraepelin {
			order = append(order, "kraepelin")
		}
		if batch.EnableRMIB {
			order = append(order, "rmib")
		}
		if batch.EnablePAPI {
			order = append(order, "papi")
		}
	}

	o := orm.NewOrm()
	for _, test := range order {
		test = strings.TrimSpace(strings.ToLower(test))
		switch test {
		case "ist":
			var progressList []models.ISTProgress
			_, _ = o.QueryTable(new(models.ISTProgress)).
				Filter("Invitation__Id", invID).
				Filter("Status", "completed").
				All(&progressList)
			allSubtests := []string{"SE", "WA", "AN", "GE", "RA", "ZR", "FA", "WU", "ME"}
			completedSet := make(map[string]bool)
			for _, p := range progressList {
				code := strings.ToUpper(strings.TrimSpace(p.SubtestCode))
				completedSet[code] = true
				if code == "ZA" {
					completedSet["ZR"] = true
				}
			}
			istComplete := true
			for _, sub := range allSubtests {
				if !completedSet[sub] {
					istComplete = false
					break
				}
			}
			if !istComplete {
				return "/test/ist/start"
			}
		case "holland":
			var r models.HollandResult
			if err := o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", invID).One(&r); err != nil || r.Id == 0 {
				return "/test/holland/start"
			}
		case "learning_style", "learningstyle", "learning-style", "vak":
			var r models.LearningStyleResult
			if err := o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", invID).One(&r); err != nil || r.Id == 0 {
				return "/test/learning-style/start"
			}
		case "kraepelin":
			var r models.KraepelinAttempt
			if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", invID).Filter("Status", "finished").One(&r); err != nil || r.Id == 0 {
				return "/test/kraepelin/start"
			}
		case "rmib":
			var r models.RMIBResult
			if err := o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", invID).One(&r); err != nil || r.Id == 0 {
				return "/test/rmib/start"
			}
		case "papi":
			var r models.PAPIResult
			if err := o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", invID).One(&r); err != nil || r.Id == 0 {
				return "/test/papi/start"
			}
		}
	}

	return ""
}

