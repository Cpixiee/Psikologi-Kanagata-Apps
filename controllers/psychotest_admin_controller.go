package controllers

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"psikologi_apps/models"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/xuri/excelize/v2"
)

type PsychotestAdminController struct {
	beego.Controller
}

type PsychotestAdminResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// verifyAdmin ensures only admin can access these APIs
func (c *PsychotestAdminController) verifyAdmin() bool {
	userRole := c.GetSession("user_role")
	roleStr, _ := userRole.(string)
	if roleStr != string(models.RoleAdmin) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Akses ditolak, hanya admin yang boleh mengakses",
		}
		c.ServeJSON()
		return false
	}
	return true
}

// @router /api/admin/test-batches [get]
func (c *PsychotestAdminController) ListBatches() {
	if !c.verifyAdmin() {
		return
	}

	status := c.GetString("status") // active, archived, atau kosong = semua

	o := orm.NewOrm()
	qs := o.QueryTable(new(models.TestBatch)).OrderBy("-CreatedAt")
	if status == models.StatusBatchActive {
		qs = qs.Filter("Status", models.StatusBatchActive)
	} else if status == models.StatusBatchArchived {
		qs = qs.Filter("Status", models.StatusBatchArchived)
	}

	var batches []models.TestBatch
	_, err := qs.All(&batches)
	if err != nil {
		log.Printf("ListBatches error: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Gagal memuat daftar batch tes: " + err.Error(),
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    batches,
	}
	c.ServeJSON()
}

// @router /api/admin/test-batches [post]
func (c *PsychotestAdminController) CreateBatch() {
	if !c.verifyAdmin() {
		return
	}

	var payload struct {
		Name            string `json:"name"`
		Institution     string `json:"institution"`
		EnableIST       bool   `json:"enable_ist"`
		EnableHolland   bool   `json:"enable_holland"`
		PurposeCategory string `json:"purpose_category"`
		PurposeDetail   string `json:"purpose_detail"`
		SendViaEmail    bool   `json:"send_via_email"`
		SendViaBrowser  bool   `json:"send_via_browser"`
	}

	// Parse JSON body (frontend mengirim JSON)
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Data tidak valid",
		}
		c.ServeJSON()
		return
	}

	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	batch := models.TestBatch{
		Name:            payload.Name,
		Institution:     payload.Institution,
		EnableIST:       payload.EnableIST,
		EnableHolland:   payload.EnableHolland,
		PurposeCategory: payload.PurposeCategory,
		PurposeDetail:   payload.PurposeDetail,
		SendViaEmail:    payload.SendViaEmail,
		SendViaBrowser:  payload.SendViaBrowser,
		Status:          models.StatusBatchActive,
		CreatedBy:       userID.(int),
	}

	o := orm.NewOrm()
	if _, err := o.Insert(&batch); err != nil {
		log.Printf("CreateBatch insert error: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Gagal membuat batch tes: " + err.Error(),
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    batch,
	}
	c.ServeJSON()
}

// @router /api/admin/test-batches/:id/invitations [post]
// Buat undangan berdasarkan daftar email (dipisah koma / baris)
func (c *PsychotestAdminController) CreateInvitations() {
	if !c.verifyAdmin() {
		return
	}

	batchID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID batch tidak valid",
		}
		c.ServeJSON()
		return
	}

	var payload struct {
		Emails []string `json:"emails"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil || len(payload.Emails) == 0 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Daftar email tidak boleh kosong",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	// Ambil info batch untuk konfigurasi pengiriman (email / browser)
	batch := models.TestBatch{Id: batchID}
	if err := o.Read(&batch); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Batch tes tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	now := time.Now()
	exp := now.Add(24 * time.Hour)

	var created []models.TestInvitation
	var invalidEmails []string

	for _, rawEmail := range payload.Emails {
		email := strings.TrimSpace(rawEmail)
		if email == "" {
			continue
		}

		// Pastikan email terdaftar sebagai user
		var user models.User
		user.Email = email
		if err := o.Read(&user, "Email"); err != nil {
			invalidEmails = append(invalidEmails, email)
			continue
		}

		batchIDPtr := &batchID
		inv := models.TestInvitation{
			BatchId: batchIDPtr,
			Email:   email,
			UserId:  &user.Id,
			// Token undangan sengaja dibuat lebih pendek (8 karakter)
			// supaya mudah diketik manual oleh peserta.
			Token:     generateToken(8),
			ExpiresAt: exp,
			Status:    models.StatusInvitationPending,
		}
		if _, err := o.Insert(&inv); err != nil {
			log.Printf("CreateInvitations insert error for %s: %v", email, err)
			continue
		}

		// Kirim email undangan jika diaktifkan
		if batch.SendViaEmail {
			go sendInvitationEmail(&batch, &user, &inv)
		}

		// Buat notifikasi browser jika diaktifkan
		if batch.SendViaBrowser {
			go createInvitationNotification(&batch, &user, &inv)
		}

		created = append(created, inv)
	}

	// Jika semua email tidak valid
	if len(created) == 0 && len(invalidEmails) > 0 {
		c.Ctx.Output.SetStatus(422)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Sebagian atau semua email tidak terdaftar di sistem",
			Data: map[string]interface{}{
				"created": nil,
				"invalid": invalidEmails,
			},
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data: map[string]interface{}{
			"created": created,
			"invalid": invalidEmails,
		},
	}
	c.ServeJSON()
}

// @router /api/admin/test-batches/:id/invitations [get]
func (c *PsychotestAdminController) ListInvitations() {
	if !c.verifyAdmin() {
		return
	}

	batchID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID batch tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	var invitations []models.TestInvitation
	_, err = o.QueryTable(new(models.TestInvitation)).
		Filter("BatchId", batchID).
		OrderBy("Email").
		All(&invitations)
	if err != nil {
		log.Printf("ListInvitations error: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Gagal memuat undangan",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    invitations,
	}
	c.ServeJSON()
}

// Helper: kirim email undangan tes psikologi
func sendInvitationEmail(batch *models.TestBatch, user *models.User, inv *models.TestInvitation) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in sendInvitationEmail: %v", r)
		}
	}()

	config := utils.GetEmailConfig()

	appURL := beego.AppConfig.DefaultString("app_url", "http://localhost:112")
	link := fmt.Sprintf("%s/test", strings.TrimRight(appURL, "/"))

	subject := fmt.Sprintf("Undangan Tes Psikologi - %s", batch.Name)

	body := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8" />
		<style>
			body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; margin: 0; padding: 0; }
			.container { max-width: 600px; margin: 0 auto; padding: 20px; }
			.header { background-color: #696cff; color: white; padding: 24px; text-align: center; border-radius: 12px 12px 0 0; }
			.header h2 { margin: 0; font-size: 24px; font-weight: 600; }
			.content { background-color: #f8f9fa; padding: 30px; border-radius: 0 0 12px 12px; }
			.info-list { list-style: none; padding: 0; margin: 20px 0; }
			.info-list li { padding: 8px 0; border-bottom: 1px solid #e0e0e0; }
			.info-list li:last-child { border-bottom: none; }
			.token-container { text-align: center; margin: 30px 0; }
			.token-box { 
				background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%); 
				border-radius: 16px; 
				padding: 35px 20px; 
				box-shadow: 0 10px 30px rgba(102, 126, 234, 0.3);
				margin: 20px 0;
				border: 2px solid rgba(255, 255, 255, 0.2);
			}
			.token-code { 
				font-size: 36px; 
				font-weight: bold; 
				color: #ffffff; 
				letter-spacing: 10px; 
				font-family: 'Courier New', monospace; 
				text-shadow: 0 2px 4px rgba(0, 0, 0, 0.2);
				margin: 0;
			}
			.token-label { 
				color: #666; 
				font-size: 14px; 
				margin-bottom: 15px; 
				font-weight: 500;
			}
			.button { 
				display: inline-block; 
				padding: 14px 28px; 
				background-color: #696cff; 
				color: white; 
				text-decoration: none; 
				border-radius: 8px; 
				margin-top: 20px;
				font-weight: 600;
				box-shadow: 0 4px 12px rgba(105, 108, 255, 0.3);
			}
			.button:hover { 
				background-color: #5a5dff; 
				box-shadow: 0 6px 16px rgba(105, 108, 255, 0.4);
			}
			.small { font-size: 12px; color: #777; margin-top: 20px; line-height: 1.5; }
			.instruction { background-color: #fff3cd; border-left: 4px solid #ffc107; padding: 12px 16px; margin: 20px 0; border-radius: 4px; }
			.instruction p { margin: 0; color: #856404; font-size: 14px; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h2>Undangan Tes Psikologi</h2>
			</div>
			<div class="content">
				<p>Halo <strong>%s</strong>,</p>
				<p>Anda telah diundang untuk mengikuti tes psikologi dengan detail berikut:</p>
				<ul class="info-list">
					<li><strong>Batch</strong>: %s</li>
					<li><strong>Institusi</strong>: %s</li>
					<li><strong>Tipe Tes</strong>: %s</li>
				</ul>
				
				<div class="token-container">
					<div class="token-label">Token Undangan Anda:</div>
					<div class="token-box">
						<div class="token-code">%s</div>
					</div>
				</div>
				
				<div class="instruction">
					<p><strong>Petunjuk:</strong> Klik tombol di bawah untuk membuka halaman tes, lalu masukkan token di atas untuk memulai tes.</p>
				</div>
				
				<div style="text-align: center;">
					<a class="button" href="%s" target="_blank" rel="noopener">Buka Halaman Tes</a>
				</div>
				
				<p class="small">
					<strong>Catatan Penting:</strong><br>
					• Token ini hanya berlaku sampai: <strong>%s</strong><br>
					• Token hanya bisa digunakan dengan akun email ini<br>
					• Jangan bagikan token ini kepada orang lain
				</p>
			</div>
		</div>
	</body>
	</html>
	`, user.NamaLengkap, batch.Name, batch.Institution, invitationTestTypes(batch), inv.Token, link, inv.ExpiresAt.Format("02 Jan 2006 15:04"))

	emailData := utils.EmailData{
		To:      user.Email,
		Subject: subject,
		Body:    body,
	}

	if err := utils.SendEmail(config, emailData); err != nil {
		log.Printf("Gagal mengirim email undangan ke %s: %v", user.Email, err)
	}
}

// Helper: buat notifikasi browser
func createInvitationNotification(batch *models.TestBatch, user *models.User, inv *models.TestInvitation) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in createInvitationNotification: %v", r)
		}
	}()

	o := orm.NewOrm()
	notif := models.Notification{
		UserId:  user.Id,
		Type:    "psychotest_invitation",
		Title:   "Undangan Tes Psikologi",
		Message: fmt.Sprintf("Anda diundang untuk mengikuti tes psikologi: %s di %s.", batch.Name, batch.Institution),
	}
	if _, err := o.Insert(&notif); err != nil {
		log.Printf("Gagal membuat notifikasi undangan untuk user %d: %v", user.Id, err)
	}
}

// Helper: format tipe tes yang aktif dalam batch
func invitationTestTypes(batch *models.TestBatch) string {
	var parts []string
	if batch.EnableIST {
		parts = append(parts, "IST (IQ)")
	}
	if batch.EnableHolland {
		parts = append(parts, "Holland (RIASEC)")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// @router /api/admin/test-batches/:id/results [get]
func (c *PsychotestAdminController) ListBatchResults() {
	if !c.verifyAdmin() {
		return
	}

	batchID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID batch tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	// Ambil undangan beserta hasil IST & Holland jika ada
	var invitations []models.TestInvitation
	_, err = o.QueryTable(new(models.TestInvitation)).
		Filter("BatchId", batchID).
		All(&invitations)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Gagal memuat data undangan",
		}
		c.ServeJSON()
		return
	}

	type InvitationSummary struct {
		Invitation models.TestInvitation `json:"invitation"`
		IST        *models.ISTResult     `json:"ist_result,omitempty"`
		Holland    *models.HollandResult `json:"holland_result,omitempty"`
	}

	var result []InvitationSummary

	for _, inv := range invitations {
		summary := InvitationSummary{Invitation: inv}

		var ist models.ISTResult
		// Gunakan raw query untuk memastikan relasi benar
		// Coba dengan ORM dulu, jika gagal coba raw query
		err = o.QueryTable(new(models.ISTResult)).
			Filter("Invitation__Id", inv.Id).
			One(&ist)
		
		// Jika ORM gagal, coba raw query
		if err != nil || ist.Id == 0 {
			err = o.Raw("SELECT * FROM ist_results WHERE invitation_id = $1", inv.Id).QueryRow(&ist)
		}
		
		// Log untuk debugging
		if err != nil {
			logs.Debug("No IST result found for invitation %d: %v", inv.Id, err)
		} else if ist.Id == 0 {
			logs.Debug("IST result found but Id is 0 for invitation %d", inv.Id)
		} else {
			logs.Info("IST result found for invitation %d: Id=%d, IQ=%d, TotalSS=%d, RawSE=%d, RawWA=%d", 
				inv.Id, ist.Id, ist.IQ, ist.TotalStandardScore, ist.RawSE, ist.RawWA)
		}
		
		if err == nil && ist.Id != 0 {
			// Cek apakah ada raw scores (test sudah dikerjakan)
			// Hitung total raw untuk memastikan test sudah selesai
			totalRaw := ist.RawSE + ist.RawWA + ist.RawAN + ist.RawGE + 
			            ist.RawRA + ist.RawZA + ist.RawFA + ist.RawWU + ist.RawME
			hasRawScores := totalRaw > 0
			
			// Jika raw scores masih 0, coba hitung dari ist_answers
			if !hasRawScores {
				logs.Info("Raw scores are 0 for invitation %d, calculating from ist_answers", inv.Id)
				// Hitung raw scores dari ist_answers
				var rawSE, rawWA, rawAN, rawGE, rawRA, rawZA, rawFA, rawWU, rawME int
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code = 'SE'
				`, inv.Id).QueryRow(&rawSE)
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code = 'WA'
				`, inv.Id).QueryRow(&rawWA)
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code = 'AN'
				`, inv.Id).QueryRow(&rawAN)
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code = 'GE'
				`, inv.Id).QueryRow(&rawGE)
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code = 'RA'
				`, inv.Id).QueryRow(&rawRA)
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code IN ('ZA', 'ZR')
				`, inv.Id).QueryRow(&rawZA)
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code = 'FA'
				`, inv.Id).QueryRow(&rawFA)
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code = 'WU'
				`, inv.Id).QueryRow(&rawWU)
				
				o.Raw(`
					SELECT COALESCE(SUM(ia.score), 0) FROM ist_answers ia 
					JOIN ist_questions iq ON ia.question_id = iq.id 
					JOIN ist_subtests ist ON iq.subtest_id = ist.id 
					WHERE ia.invitation_id = $1 AND ia.is_correct = true AND ist.code = 'ME'
				`, inv.Id).QueryRow(&rawME)
				
				// Update raw scores jika ada yang > 0
				if rawSE > 0 || rawWA > 0 || rawAN > 0 || rawGE > 0 || rawRA > 0 || rawZA > 0 || rawFA > 0 || rawWU > 0 || rawME > 0 {
					ist.RawSE = rawSE
					ist.RawWA = rawWA
					ist.RawAN = rawAN
					ist.RawGE = rawGE
					ist.RawRA = rawRA
					ist.RawZA = rawZA
					ist.RawFA = rawFA
					ist.RawWU = rawWU
					ist.RawME = rawME
					
					// Gunakan raw SQL untuk update karena Beego ORM mungkin salah konversi nama field
					_, uerr := o.Raw(`
						UPDATE ist_results 
						SET raw_se = $1, raw_wa = $2, raw_an = $3, raw_ge = $4, 
						    raw_ra = $5, raw_za = $6, raw_fa = $7, raw_wu = $8, raw_me = $9
						WHERE invitation_id = $10
					`, rawSE, rawWA, rawAN, rawGE, rawRA, rawZA, rawFA, rawWU, rawME, inv.Id).Exec()
					if uerr != nil {
						logs.Error("Error updating raw scores for invitation %d: %v", inv.Id, uerr)
					} else {
						logs.Info("Updated raw scores from ist_answers for invitation %d: SE=%d, WA=%d, AN=%d, GE=%d, RA=%d, ZA=%d, FA=%d, WU=%d, ME=%d", 
							inv.Id, rawSE, rawWA, rawAN, rawGE, rawRA, rawZA, rawFA, rawWU, rawME)
						totalRaw = rawSE + rawWA + rawAN + rawGE + rawRA + rawZA + rawFA + rawWU + rawME
						hasRawScores = totalRaw > 0
						// Reload result setelah update raw scores
						o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&ist)
					}
				}
			}
			
			// Recalculate jika: IQ / TotalStandardScore masih 0 (atau data legacy belum dihitung).
			// TotalStandardScore sekarang mengikuti norma TOTAL (SUM RW -> SW TOTAL), jadi jangan pakai rumus rata-rata SW lagi.
			needsRecalc := (ist.IQ == 0 || ist.TotalStandardScore == 0)
			if needsRecalc && hasRawScores {
				logs.Info("Recalculating IST scores for invitation %d (IQ=%d, TotalSS=%d, TotalRaw=%d)", 
					inv.Id, ist.IQ, ist.TotalStandardScore, totalRaw)
				// Ambil user untuk mendapatkan tanggal lahir
				var user models.User
				if inv.UserId != nil {
					user.Id = *inv.UserId
					if o.Read(&user) == nil {
						age := 0
						if user.TanggalLahir != nil {
							// Hitung age pada saat test dikerjakan (CreatedAt dari result)
							testDate := ist.CreatedAt
							if testDate.IsZero() {
								// Fallback ke CreatedAt invitation jika result CreatedAt kosong
								testDate = inv.CreatedAt
							}
							if testDate.IsZero() {
								// Fallback terakhir: gunakan waktu sekarang
								testDate = time.Now()
							}
							age = utils.AgeYears(*user.TanggalLahir, testDate)
							logs.Info("User %d (invitation %d): tanggal_lahir=%v, testDate=%v, calculated age=%d", 
								user.Id, inv.Id, user.TanggalLahir.Format("2006-01-02"), testDate.Format("2006-01-02"), age)
						} else {
							logs.Warning("User %d (invitation %d) has no tanggal_lahir, cannot calculate age", user.Id, inv.Id)
						}
						if age > 0 {
							// Recalculate IQ dengan age yang tepat
							// Setiap subtest dan TotalStandardScore menggunakan age untuk mencari norma
							logs.Info("Calling EnsureISTStandardAndIQScores for invitation %d with age=%d, raw scores: SE=%d, WA=%d, AN=%d, GE=%d, RA=%d, ZA=%d, FA=%d, WU=%d, ME=%d",
								inv.Id, age, ist.RawSE, ist.RawWA, ist.RawAN, ist.RawGE, ist.RawRA, ist.RawZA, ist.RawFA, ist.RawWU, ist.RawME)
							updatedRes, err := utils.EnsureISTStandardAndIQScores(o, &ist, age)
							if err == nil {
								logs.Info("EnsureISTStandardAndIQScores succeeded for invitation %d: TotalSS=%d, IQ=%d, IQCategory=%s",
									inv.Id, updatedRes.TotalStandardScore, updatedRes.IQ, updatedRes.IQCategory)
								ist = *updatedRes
								num, uerr := o.Update(&ist,
									"StdSE", "StdWA", "StdAN", "StdGE", "StdRA", "StdZA", "StdFA", "StdWU", "StdME",
									"TotalStandardScore", "IQ", "IQCategory",
								)
								if uerr != nil {
									logs.Error("Error updating IST result for invitation %d: %v", inv.Id, uerr)
								} else {
									logs.Info("Updated IST result for invitation %d: num=%d rows affected", inv.Id, num)
									if num > 0 {
										// Reload dari DB untuk mendapatkan nilai terbaru
										o.QueryTable(new(models.ISTResult)).
											Filter("Invitation__Id", inv.Id).
											One(&ist)
										logs.Info("Reloaded IST result for invitation %d: TotalSS=%d, IQ=%d", inv.Id, ist.TotalStandardScore, ist.IQ)
									}
								}
							} else {
								logs.Warning("Failed to recalculate IST scores for invitation %d, age %d: %v", inv.Id, age, err)
							}
						} else {
							logs.Warning("Cannot recalculate IST scores for invitation %d: invalid age (%d). User tanggal_lahir: %v", 
								inv.Id, age, user.TanggalLahir)
						}
					} else {
						logs.Warning("Cannot find user for invitation %d (userId: %v)", inv.Id, inv.UserId)
					}
				} else {
					logs.Warning("Invitation %d has no user_id", inv.Id)
				}
			}
			summary.IST = &ist
			logs.Info("Assigned IST result to summary for invitation %d: IQ=%d, TotalSS=%d", inv.Id, ist.IQ, ist.TotalStandardScore)
		} else {
			// Jika tidak ada result, set ke nil (akan di-omit dari JSON karena ada omitempty)
			summary.IST = nil
			logs.Info("No IST result for invitation %d, setting to nil", inv.Id)
		}

		var hol models.HollandResult
		err = o.QueryTable(new(models.HollandResult)).
			Filter("Invitation__Id", inv.Id).
			One(&hol)
		if err == nil && hol.Id != 0 {
			summary.Holland = &hol
		} else {
			summary.Holland = nil
		}

		result = append(result, summary)
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    result,
	}
	c.ServeJSON()
}

// @router /api/admin/test-batches/:id/export-answers [get]
// Export semua jawaban IST dalam format Excel (tabel format seperti lembar jawaban IST)
func (c *PsychotestAdminController) ExportBatchAnswers() {
	if !c.verifyAdmin() {
		return
	}

	batchID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID batch tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	var batch models.TestBatch
	batch.Id = batchID
	if err := o.Read(&batch); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{Success: false, Message: "Batch tes tidak ditemukan"}
		c.ServeJSON()
		return
	}

	var invitations []models.TestInvitation
	_, err = o.QueryTable(new(models.TestInvitation)).
		Filter("BatchId", batchID).
		All(&invitations)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Gagal memuat undangan",
		}
		c.ServeJSON()
		return
	}

	// Export ALL menjadi 1 ZIP: tiap peserta dapat file sendiri (IST/Holland) dengan nama jelas.
	zipBuf := new(bytes.Buffer)
	zw := zip.NewWriter(zipBuf)

	usedNames := make(map[string]int)
	written := 0
	var errs []string
	for _, inv := range invitations {
		user, _ := getUserForInvitation(o, &inv)
		base := sanitizeFilename(user.NamaLengkap)
		if base == "" {
			base = sanitizeFilename(strings.Split(inv.Email, "@")[0])
		}
		if base == "" {
			base = fmt.Sprintf("Peserta_%d", inv.Id)
		}

		// IST
		if batch.EnableIST {
			content, ferr := buildISTResultXLSX(o, &batch, &inv, user)
			if ferr == nil && len(content) > 0 {
				fname := fmt.Sprintf("%s_Hasil_IST.xlsx", base)
				fname = makeUniqueZipName(usedNames, fname, inv.Id)
				w, _ := zw.Create(fname)
				_, _ = w.Write(content)
				written++
			} else {
				errs = append(errs, fmt.Sprintf("IST inv %d: %v", inv.Id, ferr))
			}
		}

		// Holland (jika ada datanya)
		if batch.EnableHolland {
			content, ferr := buildHollandAnswersCSV(o, &batch, &inv, user)
			if ferr == nil && len(content) > 0 {
				fname := fmt.Sprintf("%s_Hasil_Holland.csv", base)
				fname = makeUniqueZipName(usedNames, fname, inv.Id)
				w, _ := zw.Create(fname)
				_, _ = w.Write(content)
				written++
			}
		}
	}

	// Jangan pernah kirim ZIP kosong: tulis README jika tidak ada file yang berhasil ditulis
	if written == 0 {
		w, _ := zw.Create("README.txt")
		msg := "Tidak ada file yang berhasil di-export.\n"
		if len(errs) > 0 {
			msg += "Error:\n- " + strings.Join(errs, "\n- ") + "\n"
		}
		_, _ = w.Write([]byte(msg))
	}

	_ = zw.Close()

	zipName := sanitizeFilename(batch.Name)
	if zipName == "" {
		zipName = fmt.Sprintf("Batch_%d", batchID)
	}
	filename := fmt.Sprintf("%s_Hasil_%s.zip", zipName, time.Now().Format("20060102"))
	c.Ctx.Output.Header("Content-Type", "application/zip")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(zipBuf.Bytes())
}

// @router /api/admin/test-batches/:batchId/invitations/:invId/export [get]
// Export jawaban IST & Holland untuk satu anak (satu invitation)
func (c *PsychotestAdminController) ExportInvitationAnswers() {
	if !c.verifyAdmin() {
		return
	}

	batchID, err := strconv.Atoi(c.Ctx.Input.Param(":batchId"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID batch tidak valid",
		}
		c.ServeJSON()
		return
	}

	invID, err := strconv.Atoi(c.Ctx.Input.Param(":invId"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID undangan tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	inv := models.TestInvitation{Id: invID}
	if err := o.Read(&inv); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Undangan tidak ditemukan",
		}
		c.ServeJSON()
		return
	}
	// Cek apakah batch_id sesuai (handle NULL)
	if inv.BatchId == nil || *inv.BatchId != batchID {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Undangan tidak ditemukan untuk batch ini",
		}
		c.ServeJSON()
		return
	}

	// Build ZIP untuk 1 peserta: berisi IST/Holland file terpisah (nama jelas).
	var batch models.TestBatch
	batch.Id = batchID
	_ = o.Read(&batch)

	user, _ := getUserForInvitation(o, &inv)
	base := sanitizeFilename(user.NamaLengkap)
	if base == "" {
		base = sanitizeFilename(strings.Split(inv.Email, "@")[0])
	}
	if base == "" {
		base = fmt.Sprintf("Peserta_%d", inv.Id)
	}

	zipBuf := new(bytes.Buffer)
	zw := zip.NewWriter(zipBuf)
	usedNames := make(map[string]int)
	written := 0
	var errs []string

	if batch.EnableIST {
		content, ferr := buildISTResultXLSX(o, &batch, &inv, user)
		if ferr == nil && len(content) > 0 {
			fname := makeUniqueZipName(usedNames, fmt.Sprintf("%s_Hasil_IST.xlsx", base), inv.Id)
			w, _ := zw.Create(fname)
			_, _ = w.Write(content)
			written++
		} else {
			errs = append(errs, fmt.Sprintf("IST: %v", ferr))
		}
	}
	if batch.EnableHolland {
		content, ferr := buildHollandAnswersCSV(o, &batch, &inv, user)
		if ferr == nil && len(content) > 0 {
			fname := makeUniqueZipName(usedNames, fmt.Sprintf("%s_Hasil_Holland.csv", base), inv.Id)
			w, _ := zw.Create(fname)
			_, _ = w.Write(content)
			written++
		}
	}

	if written == 0 {
		w, _ := zw.Create("README.txt")
		msg := "Tidak ada file yang berhasil di-export.\n"
		if len(errs) > 0 {
			msg += "Error:\n- " + strings.Join(errs, "\n- ") + "\n"
		}
		_, _ = w.Write([]byte(msg))
	}

	_ = zw.Close()

	filename := fmt.Sprintf("%s_Hasil_Batch_%d.zip", base, batchID)
	c.Ctx.Output.Header("Content-Type", "application/zip")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(zipBuf.Bytes())
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Replace karakter yang tidak aman untuk nama file.
	var b strings.Builder
	b.Grow(len(s))
	lastUnderscore := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		switch r {
		case ' ', '-', '_':
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		default:
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if len(out) > 80 {
		out = out[:80]
	}
	return out
}

func makeUniqueZipName(used map[string]int, name string, invID int) string {
	name = filepath.Base(name)
	name = strings.ReplaceAll(name, "\\", "_")
	name = strings.ReplaceAll(name, "/", "_")
	if name == "" {
		name = fmt.Sprintf("Peserta_%d.csv", invID)
	}
	if used[name] == 0 {
		used[name] = 1
		return name
	}
	used[name]++
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	return fmt.Sprintf("%s_%d%s", base, invID, ext)
}

func getUserForInvitation(o orm.Ormer, inv *models.TestInvitation) (*models.User, error) {
	if inv == nil {
		return &models.User{}, fmt.Errorf("nil invitation")
	}
	var user models.User
	if inv.UserId != nil && *inv.UserId != 0 {
		user.Id = *inv.UserId
		if err := o.Read(&user); err == nil {
			return &user, nil
		}
	}
	if inv.Email != "" {
		user.Email = inv.Email
		_ = o.Read(&user, "Email")
	}
	return &user, nil
}

func normalizeISTSubtestCodeForExport(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "ZA" {
		return "ZR"
	}
	return code
}

func buildISTResultCSV(o orm.Ormer, batch *models.TestBatch, inv *models.TestInvitation, user *models.User) ([]byte, string, error) {
	if inv == nil {
		return nil, "", fmt.Errorf("nil invitation")
	}
	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)
	defer w.Flush()

	// Siapkan struktur penampung jawaban per subtest & raw score
	answersBySubtest := make(map[string]map[int]string) // code -> nomor soal global (1-176) -> jawaban (A-E)
	rawBySubtest := make(map[string]int)                // code -> raw score

	// Load / build result
	var res models.ISTResult
	_ = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
	// Raw scores from answers (lebih akurat untuk legacy data)
	res.RawSE = rawBySubtest["SE"]
	res.RawWA = rawBySubtest["WA"]
	res.RawAN = rawBySubtest["AN"]
	res.RawGE = rawBySubtest["GE"]
	res.RawRA = rawBySubtest["RA"]
	res.RawZA = rawBySubtest["ZR"] // ZR -> field ZA
	res.RawFA = rawBySubtest["FA"]
	res.RawWU = rawBySubtest["WU"]
	res.RawME = rawBySubtest["ME"]

	age := 0
	if user != nil && user.TanggalLahir != nil {
		age = utils.AgeYears(*user.TanggalLahir, time.Now())
	}
	_, _ = utils.EnsureISTStandardAndIQScores(o, &res, age)

	// Header: bentuk mengikuti contoh lembar jawaban IST
	_ = w.Write([]string{"LEMBAR JAWABAN I.S.T."})

	nama := ""
	email := ""
	dob := ""
	gender := ""
	if user != nil {
		nama = user.NamaLengkap
		email = user.Email
		if user.TanggalLahir != nil {
			dob = user.TanggalLahir.Format("2006-01-02")
		}
		if string(user.JenisKelamin) != "" {
			gender = string(user.JenisKelamin)
		}
	}
	if email == "" {
		email = inv.Email
	}

	// Baris identitas (disusun supaya mirip template Excel kamu)
	_ = w.Write([]string{"Nomor", ":", strconv.Itoa(inv.Id), "", "L / P", ":", gender})
	_ = w.Write([]string{"Nama", ":", nama})
	_ = w.Write([]string{"Tempat Tanggal Lahir", ":", dob, "", "Usia", ":", strconv.Itoa(age), "Thn"})
	_ = w.Write([]string{"Pendidikan Terakhir", ":", ""})
	batchName := ""
	if batch != nil {
		batchName = batch.Name
	}
	_ = w.Write([]string{"Tujuan Pemeriksaan", ":", batchName})
	_ = w.Write([]string{"Tanggal Pemeriksaan", ":", inv.CreatedAt.Format("2006-01-02")})
	_ = w.Write([]string{}) // baris kosong pemisah

	// Header subtest (SUBTES 1-9)
	subtestOrder := []string{"SE", "WA", "AN", "GE", "RA", "ZR", "FA", "WU", "ME"}
	_ = w.Write([]string{"No.", "SUBTES I", "", "SUBTES II", "", "SUBTES III", "", "SUBTES IV", "", "SUBTES V", "", "SUBTES VI", "", "SUBTES VII", "", "SUBTES VIII", "", "SUBTES IX", ""})
	_ = w.Write([]string{"", "Jawaban", "", "Jawaban", "", "Jawaban", "", "Jawaban", "", "Jawaban", "", "Jawaban", "", "Jawaban", "", "Jawaban", "", "Jawaban", ""})

	questionRanges := []struct {
		start, end int
		subtest    string
	}{
		{1, 20, "SE"},
		{21, 40, "WA"},
		{41, 60, "AN"},
		{61, 76, "GE"},
		{77, 96, "RA"},
		{97, 116, "ZR"},
		{117, 136, "FA"},
		{137, 156, "WU"},
		{157, 176, "ME"},
	}

	// Isi jawaban & raw score per subtest langsung dari DB,
	// supaya robust meskipun ada perubahan relasi.
	for _, subtestCode := range subtestOrder {
		// Cari master subtest
		sub, err := findISTSubtestByCode(o, subtestCode)
		if err != nil || sub == nil {
			continue
		}

		// Ambil soal untuk subtest ini (pakai range global 176 & filter dummy)
		start, end := istQuestionRangeByCode(subtestCode)
		var qs []models.ISTQuestion
		q := o.QueryTable(new(models.ISTQuestion)).Filter("Subtest__Id", sub.Id)
		if start > 0 && end > 0 {
			q = q.Filter("Number__gte", start).Filter("Number__lte", end)
		}
		_, _ = q.OrderBy("Number").All(&qs)
		qs = filterISTDummyQuestions(qs)
		if len(qs) == 0 {
			continue
		}

		codeNorm := normalizeISTSubtestCodeForExport(subtestCode)
		if answersBySubtest[codeNorm] == nil {
			answersBySubtest[codeNorm] = make(map[int]string)
		}

		// Untuk tiap soal, ambil jawaban peserta (jika ada)
		for _, qn := range qs {
			var ans models.ISTAnswer
			err := o.QueryTable(new(models.ISTAnswer)).
				Filter("Invitation__Id", inv.Id).
				Filter("Question__Id", qn.Id).
				One(&ans)
			if err != nil || ans.Id == 0 {
				continue
			}
			answersBySubtest[codeNorm][qn.Number] = strings.ToUpper(strings.TrimSpace(ans.Answer))
			if ans.IsCorrect {
				rawBySubtest[codeNorm]++
			}
		}
	}

	for qNum := 1; qNum <= 176; qNum++ {
		row := make([]string, 19)
		row[0] = strconv.Itoa(qNum)
		for i, subtestCode := range subtestOrder {
			colIdx := 1 + i*2
			inRange := false
			localNum := 0
			for _, rng := range questionRanges {
				if rng.subtest == subtestCode && qNum >= rng.start && qNum <= rng.end {
					inRange = true
					// Konversi nomor global (1-176) ke nomor lokal per subtes (1-20, dst.)
					localNum = qNum - rng.start + 1
					break
				}
			}
			if inRange {
				if ans, ok := answersBySubtest[subtestCode][localNum]; ok {
					row[colIdx] = ans
				} else {
					row[colIdx] = ""
				}
			} else {
				row[colIdx] = ""
			}
			row[colIdx+1] = ""
		}
		_ = w.Write(row)
	}

	// RW row
	rwRow := make([]string, 19)
	rwRow[0] = "RW"
	for i, subtestCode := range subtestOrder {
		colIdx := 1 + i*2
		raw := rawBySubtest[subtestCode]
		rwRow[colIdx] = strconv.Itoa(raw)
		rwRow[colIdx+1] = ""
	}
	_ = w.Write(rwRow)

	// SS row
	ssRow := make([]string, 19)
	ssRow[0] = "SS"
	stdMap := map[string]int{
		"SE": res.StdSE,
		"WA": res.StdWA,
		"AN": res.StdAN,
		"GE": res.StdGE,
		"RA": res.StdRA,
		"ZR": res.StdZA, // ZR -> field ZA
		"FA": res.StdFA,
		"WU": res.StdWU,
		"ME": res.StdME,
	}
	for i, subtestCode := range subtestOrder {
		colIdx := 1 + i*2
		ssRow[colIdx] = strconv.Itoa(stdMap[subtestCode])
		ssRow[colIdx+1] = ""
	}
	_ = w.Write(ssRow)

	_ = w.Write([]string{"Total SS", strconv.Itoa(res.TotalStandardScore)})
	_ = w.Write([]string{"IQ", strconv.Itoa(res.IQ)})
	_ = w.Write([]string{"Kategori IQ", res.IQCategory})

	return buf.Bytes(), "", nil
}

type istAnswerExportRow struct {
	Number     int    `orm:"column(number)"`
	Answer     string `orm:"column(answer_option)"`
	IsCorrect  bool   `orm:"column(is_correct)"`
	SubtestRaw string `orm:"column(subtest_code)"`
}

func buildISTResultXLSX(o orm.Ormer, batch *models.TestBatch, inv *models.TestInvitation, user *models.User) ([]byte, error) {
	if inv == nil {
		return nil, fmt.Errorf("nil invitation")
	}

	// Load answers via raw SQL join (lebih robust daripada RelatedSel)
	var rows []istAnswerExportRow
	_, err := o.Raw(`
		SELECT q.number, a.answer_option, a.is_correct, s.code AS subtest_code
		FROM ist_answers a
		JOIN ist_questions q ON q.id = a.question_id
		JOIN ist_subtests s ON s.id = a.subtest_id
		WHERE a.invitation_id = ?
		ORDER BY q.number
	`, inv.Id).QueryRows(&rows)
	if err != nil {
		return nil, fmt.Errorf("failed to load IST answers: %v", err)
	}

	answersByNumber := make(map[int]string)   // global number 1..176 -> A-E
	rawBySubtest := make(map[string]int)      // SE/WA/... -> raw score
	for _, r := range rows {
		answersByNumber[r.Number] = strings.ToUpper(strings.TrimSpace(r.Answer))
		code := normalizeISTSubtestCodeForExport(r.SubtestRaw)
		if r.IsCorrect {
			rawBySubtest[code]++
		}
	}

	age := 0
	if user != nil && user.TanggalLahir != nil {
		age = utils.AgeYears(*user.TanggalLahir, time.Now())
	}

	nama := ""
	email := ""
	dob := ""
	gender := ""
	if user != nil {
		nama = user.NamaLengkap
		email = user.Email
		if user.TanggalLahir != nil {
			dob = user.TanggalLahir.Format("2006-01-02")
		}
		if string(user.JenisKelamin) != "" {
			gender = string(user.JenisKelamin)
		}
	}
	if email == "" {
		email = inv.Email
	}

	// Workbook
	f := excelize.NewFile()
	sheet := "IST"
	f.SetSheetName(f.GetSheetName(0), sheet)

	// Styles
	border := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
	styleTitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styleHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	styleCell, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	styleLabel, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})

	// Title
	_ = f.SetCellValue(sheet, "A1", "LEMBAR JAWABAN I.S.T.")
	_ = f.MergeCell(sheet, "A1", "N1")
	_ = f.SetRowHeight(sheet, 1, 22)
	_ = f.SetCellStyle(sheet, "A1", "A1", styleTitle)

	// Identity block (simple)
	_ = f.SetCellValue(sheet, "A3", "Nomor")
	_ = f.SetCellStyle(sheet, "A3", "A3", styleLabel)
	_ = f.SetCellValue(sheet, "B3", ":")
	_ = f.SetCellValue(sheet, "C3", inv.Id)
	_ = f.SetCellValue(sheet, "E3", "L / P")
	_ = f.SetCellStyle(sheet, "E3", "E3", styleLabel)
	_ = f.SetCellValue(sheet, "F3", ":")
	_ = f.SetCellValue(sheet, "G3", gender)

	_ = f.SetCellValue(sheet, "A4", "Nama")
	_ = f.SetCellStyle(sheet, "A4", "A4", styleLabel)
	_ = f.SetCellValue(sheet, "B4", ":")
	_ = f.SetCellValue(sheet, "C4", nama)

	_ = f.SetCellValue(sheet, "A5", "Tempat Tanggal Lahir")
	_ = f.SetCellStyle(sheet, "A5", "A5", styleLabel)
	_ = f.SetCellValue(sheet, "B5", ":")
	_ = f.SetCellValue(sheet, "C5", dob)
	_ = f.SetCellValue(sheet, "E5", "Usia")
	_ = f.SetCellStyle(sheet, "E5", "E5", styleLabel)
	_ = f.SetCellValue(sheet, "F5", ":")
	_ = f.SetCellValue(sheet, "G5", fmt.Sprintf("%d Thn", age))

	_ = f.SetCellValue(sheet, "A6", "Pendidikan Terakhir")
	_ = f.SetCellStyle(sheet, "A6", "A6", styleLabel)
	_ = f.SetCellValue(sheet, "B6", ":")

	purpose := ""
	if batch != nil {
		purpose = batch.PurposeDetail
	}
	_ = f.SetCellValue(sheet, "A7", "Tujuan Pemeriksaan")
	_ = f.SetCellStyle(sheet, "A7", "A7", styleLabel)
	_ = f.SetCellValue(sheet, "B7", ":")
	_ = f.SetCellValue(sheet, "C7", purpose)

	_ = f.SetCellValue(sheet, "A8", "Tanggal Pemeriksaan")
	_ = f.SetCellStyle(sheet, "A8", "A8", styleLabel)
	_ = f.SetCellValue(sheet, "B8", ":")
	_ = f.SetCellValue(sheet, "C8", inv.CreatedAt.Format("2006-01-02"))

	// Column widths (approx)
	_ = f.SetColWidth(sheet, "A", "N", 11)
	_ = f.SetColWidth(sheet, "A", "A", 6)  // No
	_ = f.SetColWidth(sheet, "B", "B", 3)  // :
	_ = f.SetColWidth(sheet, "C", "D", 22) // value

	// Helper for blocks
	writeBlock := func(colNo, colAns int, topRow int, title string, startNum, endNum int, subtestCode string) {
		colNoName, _ := excelize.ColumnNumberToName(colNo)
		colAnsName, _ := excelize.ColumnNumberToName(colAns)
		topLeft := fmt.Sprintf("%s%d", colNoName, topRow)
		topRight := fmt.Sprintf("%s%d", colAnsName, topRow)
		_ = f.MergeCell(sheet, topLeft, topRight)
		_ = f.SetCellValue(sheet, topLeft, title)
		_ = f.SetCellStyle(sheet, topLeft, topRight, styleHeader)

		// Header row
		hRow := topRow + 1
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", colNoName, hRow), "No.")
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", colAnsName, hRow), "Jawaban")
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", colNoName, hRow), fmt.Sprintf("%s%d", colAnsName, hRow), styleHeader)

		// Data rows
		r := hRow + 1
		for n := startNum; n <= endNum; n++ {
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", colNoName, r), n)
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", colAnsName, r), answersByNumber[n])
			_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", colNoName, r), fmt.Sprintf("%s%d", colAnsName, r), styleCell)
			r++
		}

		// RW row
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", colNoName, r), "RW")
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", colAnsName, r), rawBySubtest[subtestCode])
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", colNoName, r), fmt.Sprintf("%s%d", colAnsName, r), styleCell)
	}

	// Top blocks (SUBTES 1-4)
	writeBlock(1, 2, 10, "SUBTES 1", 1, 20, "SE")
	writeBlock(4, 5, 10, "SUBTES 2", 21, 40, "WA")
	writeBlock(7, 8, 10, "SUBTES 3", 41, 60, "AN")
	writeBlock(10, 11, 10, "SUBTES 4", 61, 76, "GE")

	// Bottom blocks (SUBTES 5-9)
	writeBlock(1, 2, 35, "SUBTES 5", 77, 96, "RA")
	writeBlock(4, 5, 35, "SUBTES 6", 97, 116, "ZR")
	writeBlock(7, 8, 35, "SUBTES 7", 117, 136, "FA")
	writeBlock(10, 11, 35, "SUBTES 8", 137, 156, "WU")
	writeBlock(13, 14, 35, "SUBTES 9", 157, 176, "ME")

	// If no answers at all, still return a valid xlsx (user can see empty)
	_ = f.SetCellValue(sheet, "A4", "Nama")
	_ = f.SetCellValue(sheet, "C4", nama)
	_ = f.SetCellValue(sheet, "A9", "Email")
	_ = f.SetCellStyle(sheet, "A9", "A9", styleLabel)
	_ = f.SetCellValue(sheet, "B9", ":")
	_ = f.SetCellValue(sheet, "C9", email)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("failed to write xlsx: %v", err)
	}
	return buf.Bytes(), nil
}

func buildHollandAnswersCSV(o orm.Ormer, batch *models.TestBatch, inv *models.TestInvitation, user *models.User) ([]byte, error) {
	if inv == nil {
		return nil, fmt.Errorf("nil invitation")
	}
	var holAnswers []models.HollandAnswer
	_, err := o.QueryTable(new(models.HollandAnswer)).
		Filter("Invitation__Id", inv.Id).
		RelatedSel("Question").
		OrderBy("Question__Code", "Question__Number").
		All(&holAnswers)
	if err != nil || len(holAnswers) == 0 {
		return nil, fmt.Errorf("no holland answers")
	}

	buf := new(bytes.Buffer)
	w := csv.NewWriter(buf)
	defer w.Flush()

	nama := ""
	email := inv.Email
	if user != nil {
		nama = user.NamaLengkap
		if user.Email != "" {
			email = user.Email
		}
	}

	_ = w.Write([]string{"HASIL HOLLAND (RIASEC)"})
	_ = w.Write([]string{"Nama", nama})
	_ = w.Write([]string{"Email", email})
	if batch != nil {
		_ = w.Write([]string{"Batch", batch.Name})
	}
	_ = w.Write([]string{})
	_ = w.Write([]string{"Code", "Number", "Value"})
	for _, a := range holAnswers {
		_ = w.Write([]string{a.Question.Code, strconv.Itoa(a.Question.Number), strconv.Itoa(a.Value)})
	}
	return buf.Bytes(), nil
}

// @router /api/admin/test-invitations/:id [put]
func (c *PsychotestAdminController) UpdateInvitation() {
	if !c.verifyAdmin() {
		return
	}

	invID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID undangan tidak valid",
		}
		c.ServeJSON()
		return
	}

	var payload struct {
		Email  string `json:"email"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Data tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	inv := models.TestInvitation{Id: invID}
	if err := o.Read(&inv); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Undangan tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	fields := []string{}

	// Update email jika diisi
	if payload.Email != "" && payload.Email != inv.Email {
		email := strings.TrimSpace(payload.Email)
		var user models.User
		user.Email = email
		if err := o.Read(&user, "Email"); err != nil {
			c.Ctx.Output.SetStatus(422)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: fmt.Sprintf("Email %s tidak terdaftar di sistem", email),
			}
			c.ServeJSON()
			return
		}
		inv.Email = email
		inv.UserId = &user.Id
		fields = append(fields, "Email", "UserId")
	}

	// Update status jika diisi
	if payload.Status != "" && payload.Status != inv.Status {
		switch payload.Status {
		case models.StatusInvitationPending,
			models.StatusInvitationUsed,
			models.StatusInvitationExpired,
			models.StatusInvitationCanceled,
			models.StatusInvitationArchived:
			inv.Status = payload.Status
			fields = append(fields, "Status")
		default:
			c.Ctx.Output.SetStatus(400)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Status undangan tidak valid",
			}
			c.ServeJSON()
			return
		}
	}

	if len(fields) > 0 {
		if _, err := o.Update(&inv, fields...); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Gagal memperbarui undangan",
			}
			c.ServeJSON()
			return
		}
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    inv,
	}
	c.ServeJSON()
}

// @router /api/admin/test-invitations/:id [delete]
func (c *PsychotestAdminController) DeleteInvitation() {
	if !c.verifyAdmin() {
		return
	}

	invID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID undangan tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	inv := models.TestInvitation{Id: invID}
	if err := o.Read(&inv); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Undangan tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	if _, err := o.Delete(&inv); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Gagal menghapus undangan",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Message: "Undangan berhasil dihapus",
	}
	c.ServeJSON()
}

// @router /api/admin/test-invitations/bulk [post]
func (c *PsychotestAdminController) BulkInvitations() {
	if !c.verifyAdmin() {
		return
	}

	var payload struct {
		Action string `json:"action"`
		IDs    []int  `json:"ids"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil || len(payload.IDs) == 0 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Data tidak valid atau tidak ada undangan yang dipilih",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	switch payload.Action {
	case "delete":
		if _, err := o.QueryTable(new(models.TestInvitation)).Filter("Id__in", payload.IDs).Delete(); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Gagal menghapus undangan",
			}
			c.ServeJSON()
			return
		}
	case "archive", "cancel":
		newStatus := models.StatusInvitationArchived
		if payload.Action == "cancel" {
			newStatus = models.StatusInvitationCanceled
		}
		if _, err := o.QueryTable(new(models.TestInvitation)).
			Filter("Id__in", payload.IDs).
			Update(orm.Params{"Status": newStatus}); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Gagal memperbarui status undangan",
			}
			c.ServeJSON()
			return
		}
	default:
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Aksi tidak dikenal",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Message: "Aksi massal berhasil dijalankan",
	}
	c.ServeJSON()
}

// @router /api/admin/test-batches/:id [put]
func (c *PsychotestAdminController) UpdateBatch() {
	if !c.verifyAdmin() {
		return
	}

	batchID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID batch tidak valid",
		}
		c.ServeJSON()
		return
	}

	var payload struct {
		Name          string `json:"name"`
		Institution   string `json:"institution"`
		Status        string `json:"status"` // active, archived
		PurposeDetail string `json:"purpose_detail"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Data tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	batch := models.TestBatch{Id: batchID}
	if err := o.Read(&batch); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Batch tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	fields := []string{}
	if payload.Name != "" && payload.Name != batch.Name {
		batch.Name = payload.Name
		fields = append(fields, "Name")
	}
	if payload.Institution != "" && payload.Institution != batch.Institution {
		batch.Institution = payload.Institution
		fields = append(fields, "Institution")
	}
	if payload.PurposeDetail != "" && payload.PurposeDetail != batch.PurposeDetail {
		batch.PurposeDetail = payload.PurposeDetail
		fields = append(fields, "PurposeDetail")
	}
	if payload.Status != "" && payload.Status != batch.Status {
		switch payload.Status {
		case models.StatusBatchActive, models.StatusBatchArchived:
			batch.Status = payload.Status
			fields = append(fields, "Status")
		default:
			c.Ctx.Output.SetStatus(400)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Status batch tidak valid",
			}
			c.ServeJSON()
			return
		}
	}

	if len(fields) > 0 {
		if _, err := o.Update(&batch, fields...); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Gagal memperbarui batch",
			}
			c.ServeJSON()
			return
		}
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    batch,
	}
	c.ServeJSON()
}

// @router /api/admin/test-batches/:id [delete]
func (c *PsychotestAdminController) DeleteBatch() {
	if !c.verifyAdmin() {
		return
	}

	batchID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "ID batch tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	batch := models.TestBatch{Id: batchID}
	if err := o.Read(&batch); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Batch tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	if _, err := o.Delete(&batch); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Gagal menghapus batch",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Message: "Batch berhasil dihapus",
	}
	c.ServeJSON()
}

// @router /api/admin/test-batches/bulk [post]
func (c *PsychotestAdminController) BulkBatches() {
	if !c.verifyAdmin() {
		return
	}

	var payload struct {
		Action string `json:"action"` // archive, delete
		IDs    []int  `json:"ids"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil || len(payload.IDs) == 0 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Data tidak valid atau tidak ada batch yang dipilih",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	switch payload.Action {
	case "delete":
		if _, err := o.QueryTable(new(models.TestBatch)).Filter("Id__in", payload.IDs).Delete(); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Gagal menghapus batch",
			}
			c.ServeJSON()
			return
		}
	case "archive":
		if _, err := o.QueryTable(new(models.TestBatch)).
			Filter("Id__in", payload.IDs).
			Update(orm.Params{"Status": models.StatusBatchArchived}); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Gagal mengarsipkan batch",
			}
			c.ServeJSON()
			return
		}
	case "restore":
		if _, err := o.QueryTable(new(models.TestBatch)).
			Filter("Id__in", payload.IDs).
			Update(orm.Params{"Status": models.StatusBatchActive}); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Gagal mengembalikan batch ke aktif",
			}
			c.ServeJSON()
			return
		}
	default:
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Aksi batch tidak dikenal",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Message: "Aksi batch berhasil dijalankan",
	}
	c.ServeJSON()
}

func intPtrToString(u *models.User) string {
	if u == nil {
		return ""
	}
	return strconv.Itoa(u.Id)
}

func boolToString(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

// generateToken membuat string acak panjang n untuk token undangan.
// Token dibatasi huruf besar + angka supaya mudah dibaca dan diketik ulang.
func generateToken(n int) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}
