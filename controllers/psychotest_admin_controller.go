package controllers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
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
		Name           string `json:"name"`
		Institution    string `json:"institution"`
		EnableIST      bool   `json:"enable_ist"`
		EnableHolland  bool   `json:"enable_holland"`
		PurposeCategory string `json:"purpose_category"`
		PurposeDetail  string `json:"purpose_detail"`
		SendViaEmail   bool   `json:"send_via_email"`
		SendViaBrowser bool   `json:"send_via_browser"`
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
		Name:           payload.Name,
		Institution:    payload.Institution,
		EnableIST:      payload.EnableIST,
		EnableHolland:  payload.EnableHolland,
		PurposeCategory: payload.PurposeCategory,
		PurposeDetail:  payload.PurposeDetail,
		SendViaEmail:   payload.SendViaEmail,
		SendViaBrowser: payload.SendViaBrowser,
		Status:         models.StatusBatchActive,
		CreatedBy:      userID.(int),
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

		inv := models.TestInvitation{
			BatchId:   batchID,
			Email:     email,
			UserId:    &user.Id,
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

	appURL := beego.AppConfig.DefaultString("app_url", "http://localhost:8081")
	link := fmt.Sprintf("%s/test", strings.TrimRight(appURL, "/"))

	subject := fmt.Sprintf("Undangan Tes Psikologi - %s", batch.Name)

	body := fmt.Sprintf(`
	<!DOCTYPE html>
	<html>
	<head>
		<meta charset="UTF-8" />
		<style>
			body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
			.container { max-width: 600px; margin: 0 auto; padding: 20px; }
			.header { background-color: #696cff; color: white; padding: 16px; text-align: center; border-radius: 8px 8px 0 0; }
			.content { background-color: #f8f9fa; padding: 20px; border-radius: 0 0 8px 8px; }
			.button { display: inline-block; padding: 10px 18px; background-color: #696cff; color: white; text-decoration: none; border-radius: 4px; margin-top: 12px; }
			.small { font-size: 12px; color: #777; margin-top: 16px; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h2>Undangan Tes Psikologi</h2>
			</div>
			<div class="content">
				<p>Halo %s,</p>
				<p>Anda telah diundang untuk mengikuti tes psikologi:</p>
				<ul>
					<li><strong>Batch</strong>: %s</li>
					<li><strong>Institusi</strong>: %s</li>
					<li><strong>Tipe Tes</strong>: %s</li>
				</ul>
				<p>Klik tombol di bawah ini untuk membuka halaman tes, lalu masukkan token berikut secara manual:</p>
				<p><strong>Token:</strong> <code>%s</code></p>
				<p><a class="button" href="%s" target="_blank" rel="noopener">Buka Halaman Tes</a></p>
				<p class="small">Token ini hanya berlaku sampai: %s dan hanya bisa digunakan dengan akun email ini.</p>
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
		IST       *models.ISTResult      `json:"ist_result,omitempty"`
		Holland   *models.HollandResult  `json:"holland_result,omitempty"`
	}

	var result []InvitationSummary

	for _, inv := range invitations {
		summary := InvitationSummary{Invitation: inv}

		var ist models.ISTResult
		err = o.QueryTable(new(models.ISTResult)).
			Filter("Invitation__Id", inv.Id).
			One(&ist)
		if err == nil && ist.Id != 0 {
			summary.IST = &ist
		}

		var hol models.HollandResult
		err = o.QueryTable(new(models.HollandResult)).
			Filter("Invitation__Id", inv.Id).
			One(&hol)
		if err == nil && hol.Id != 0 {
			summary.Holland = &hol
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
// Export semua jawaban IST & Holland dalam format CSV supaya bisa dibuka di Excel
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

	// Setup CSV response
	filename := fmt.Sprintf("psychotest_batch_%d_%s.csv", batchID, time.Now().Format("20060102"))
	c.Ctx.Output.Header("Content-Type", "text/csv; charset=utf-8")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")

	writer := csv.NewWriter(c.Ctx.ResponseWriter)
	defer writer.Flush()

	// Header kolom
	_ = writer.Write([]string{
		"batch_id",
		"invitation_id",
		"email",
		"user_id",
		"subtest_code",
		"question_number",
		"answer_option",
		"is_correct",
		"test_type",
	})

	// Export IST answers
	for _, inv := range invitations {
		var answers []models.ISTAnswer
		_, err := o.QueryTable(new(models.ISTAnswer)).
			Filter("Invitation__Id", inv.Id).
			RelatedSel().
			All(&answers)
		if err != nil {
			continue
		}

		for _, ans := range answers {
			_ = writer.Write([]string{
				strconv.Itoa(batchID),
				strconv.Itoa(inv.Id),
				inv.Email,
				intPtrToString(ans.User),
				ans.Subtest.Code,
				strconv.Itoa(ans.Question.Number),
				ans.Answer,
				boolToString(ans.IsCorrect),
				"IST",
			})
		}
	}

	// Export Holland answers
	for _, inv := range invitations {
		var answers []models.HollandAnswer
		_, err := o.QueryTable(new(models.HollandAnswer)).
			Filter("Invitation__Id", inv.Id).
			RelatedSel().
			All(&answers)
		if err != nil {
			continue
		}

		for _, ans := range answers {
			_ = writer.Write([]string{
				strconv.Itoa(batchID),
				strconv.Itoa(inv.Id),
				inv.Email,
				intPtrToString(ans.User),
				ans.Question.Code,
				strconv.Itoa(ans.Question.Number),
				strconv.Itoa(ans.Value),
				"", // is_correct tidak relevan untuk Holland
				"HOLLAND",
			})
		}
	}
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
	if err := o.Read(&inv); err != nil || inv.BatchId != batchID {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Undangan tidak ditemukan untuk batch ini",
		}
		c.ServeJSON()
		return
	}

	// Setup CSV response untuk satu anak
	filename := fmt.Sprintf("batch_%d_inv_%d_%s.csv", batchID, invID, time.Now().Format("20060102"))
	c.Ctx.Output.Header("Content-Type", "text/csv; charset=utf-8")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")

	writer := csv.NewWriter(c.Ctx.ResponseWriter)
	defer writer.Flush()

	_ = writer.Write([]string{
		"batch_id",
		"invitation_id",
		"email",
		"user_id",
		"subtest_code",
		"question_number",
		"answer_option",
		"is_correct",
		"test_type",
	})

	// IST answers untuk undangan ini
	var istAnswers []models.ISTAnswer
	_, err = o.QueryTable(new(models.ISTAnswer)).
		Filter("Invitation__Id", invID).
		RelatedSel().
		All(&istAnswers)
	if err == nil {
		for _, ans := range istAnswers {
			_ = writer.Write([]string{
				strconv.Itoa(batchID),
				strconv.Itoa(invID),
				inv.Email,
				intPtrToString(ans.User),
				ans.Subtest.Code,
				strconv.Itoa(ans.Question.Number),
				ans.Answer,
				boolToString(ans.IsCorrect),
				"IST",
			})
		}
	}

	// Holland answers untuk undangan ini
	var holAnswers []models.HollandAnswer
	_, err = o.QueryTable(new(models.HollandAnswer)).
		Filter("Invitation__Id", invID).
		RelatedSel().
		All(&holAnswers)
	if err == nil {
		for _, ans := range holAnswers {
			_ = writer.Write([]string{
				strconv.Itoa(batchID),
				strconv.Itoa(invID),
				inv.Email,
				intPtrToString(ans.User),
				ans.Question.Code,
				strconv.Itoa(ans.Question.Number),
				strconv.Itoa(ans.Value),
				"", // is_correct tidak relevan untuk Holland
				"HOLLAND",
			})
		}
	}
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
		Name         string `json:"name"`
		Institution  string `json:"institution"`
		Status       string `json:"status"` // active, archived
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

// generateToken membuat string acak panjang n untuk token undangan
func generateToken(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

