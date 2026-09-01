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
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"psikologi_apps/models"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/jung-kurt/gofpdf"
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

// verifyAdminOrSchool mengizinkan admin maupun akun sekolah (read-only).
// Mengembalikan true jika valid, plus role & sekolah session (sekolah string
// kosong jika user adalah admin).
func (c *PsychotestAdminController) verifyAdminOrSchool() (bool, string, string) {
	userRole := c.GetSession("user_role")
	roleStr, _ := userRole.(string)
	if roleStr != string(models.RoleAdmin) && roleStr != string(models.RoleSekolah) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Akses ditolak",
		}
		c.ServeJSON()
		return false, "", ""
	}
	sekolah := ""
	if roleStr == string(models.RoleSekolah) {
		uidAny := c.GetSession("user_id")
		if uid, ok := uidAny.(int); ok && uid > 0 {
			u := models.User{Id: uid}
			if err := orm.NewOrm().Read(&u); err == nil {
				sekolah = u.Sekolah
			}
		}
	}
	return true, roleStr, sekolah
}

// filterInvitationsBySchool memfilter slice TestInvitation hanya menyisakan
// yang user-nya (berdasarkan UserId atau email) berasal dari sekolah yang
// diberikan. Jika sekolah kosong (admin), slice dikembalikan apa adanya.
// Logika: INCLUDE jika: (1) tidak bisa resolve user, (2) sekolah user kosong,
// atau (3) sekolah user cocok. EXCLUDE hanya jika sekolah user eksplisit
// berbeda dari sekolah filter — ini mencegah cross-school data leak.
func filterInvitationsBySchool(invs []models.TestInvitation, sekolah string) []models.TestInvitation {
	if sekolah == "" {
		return invs
	}
	if len(invs) == 0 {
		return invs
	}
	o := orm.NewOrm()
	out := make([]models.TestInvitation, 0, len(invs))
	for _, inv := range invs {
		var u models.User
		var err error
		if inv.UserId != nil && *inv.UserId > 0 {
			u = models.User{Id: *inv.UserId}
			err = o.Read(&u)
		} else if strings.TrimSpace(inv.Email) != "" {
			u = models.User{Email: strings.TrimSpace(inv.Email)}
			err = o.Read(&u, "Email")
		} else {
			// Tidak ada user info — tetap include (batch sudah terfilter)
			out = append(out, inv)
			continue
		}
		if err != nil {
			// User tidak ditemukan — tetap include (batch sudah terfilter)
			out = append(out, inv)
			continue
		}
		// User ditemukan: include jika sekolah user kosong (belum diisi) atau cocok
		if u.Sekolah == "" || strings.EqualFold(u.Sekolah, sekolah) {
			out = append(out, inv)
		}
		// Jika sekolah user eksplisit berbeda → skip (cross-school protection)
	}
	return out
}


// @router /api/admin/test-batches [get]
func (c *PsychotestAdminController) ListBatches() {
	ok, roleStr, sekolahStr := c.verifyAdminOrSchool()
	if !ok {
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

	if roleStr == string(models.RoleSekolah) {
		if sekolahStr != "" {
			cond := orm.NewCondition().
				Or("Sekolah", sekolahStr).
				Or("Institution", sekolahStr)
			qs = qs.SetCond(cond)
		}
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

	// Fetch all school teachers to map teacher_id to teacher_name
	var teachers []models.SchoolTeacher
	_, _ = o.QueryTable(new(models.SchoolTeacher)).All(&teachers)
	teacherMap := make(map[int]string)
	for _, t := range teachers {
		teacherMap[t.Id] = t.Nama
	}

	type BatchResponseItem struct {
		models.TestBatch
		TeacherName      string `json:"teacher_name,omitempty"`
		ParticipantCount int    `json:"participant_count"`
		CompletedCount   int    `json:"completed_count"`
	}

	resData := make([]BatchResponseItem, len(batches))
	for i, b := range batches {
		var teacherName string
		if b.TeacherId != nil {
			teacherName = teacherMap[*b.TeacherId]
		}
		
		var partCount, compCount int
		// PostgreSQL uses $1, but SQLite/MySQL uses ?. We can use Beego Raw or just direct query.
		// Wait, let's see which database driver is active. Beego ORM uses standard placeholder.
		// Since other Raw queries in this codebase use $1 (PostgreSQL placeholder), let's use $1.
		// Wait! Let's check how other o.Raw queries are written in this file:
		// Line 967: SELECT * FROM ist_results WHERE invitation_id = $1
		// Yes, the driver is Postgres, so it uses $1 placeholder!
		o.Raw("SELECT COUNT(*) FROM test_invitations WHERE batch_id = $1", b.Id).QueryRow(&partCount)
		
		if partCount > 0 {
			o.Raw("SELECT COUNT(*) FROM test_invitations WHERE batch_id = $1 AND status = 'used'", b.Id).QueryRow(&compCount)
		}

		resData[i] = BatchResponseItem{
			TestBatch:        b,
			TeacherName:      teacherName,
			ParticipantCount: partCount,
			CompletedCount:   compCount,
		}
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    resData,
	}
	c.ServeJSON()
}

// @router /api/admin/test-batches/:id [get]
func (c *PsychotestAdminController) GetBatchDetail() {
	ok, _, sekolah := c.verifyAdminOrSchool()
	if !ok {
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
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Batch tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	isDemo := strings.Contains(strings.ToLower(batch.Name), "demo") || batch.Sekolah == ""
	if sekolah != "" && !strings.EqualFold(batch.Sekolah, sekolah) && !isDemo {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Akses ditolak: Anda tidak memiliki akses ke batch ini",
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

// @router /api/admin/test-batches [post]
func (c *PsychotestAdminController) CreateBatch() {
	if !c.verifyAdmin() {
		return
	}

	var payload struct {
		Name            string `json:"name"`
		Institution     string `json:"institution"`
		TahunAjaran     string `json:"tahun_ajaran"`
		Sekolah         string `json:"sekolah"`
		JenjangSekolah  string `json:"jenjang_sekolah"`
		Kelas           string `json:"kelas"`
		Jurusan         string `json:"jurusan"`
		EnableIST       bool   `json:"enable_ist"`
		EnableHolland   bool   `json:"enable_holland"`
		EnableLearningStyle bool `json:"enable_learning_style"`
		EnableKraepelin bool `json:"enable_kraepelin"`
		EnableRMIB      bool   `json:"enable_rmib"`
		EnablePAPI      bool   `json:"enable_papi"`
		TestOrder       string `json:"test_order"`
		PurposeCategory string `json:"purpose_category"`
		PurposeDetail   string `json:"purpose_detail"`
		SendViaEmail    bool   `json:"send_via_email"`
		SendViaBrowser  bool   `json:"send_via_browser"`
		SendViaWhatsApp bool   `json:"send_via_whatsapp"`
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

	enabledCount := 0
	if payload.EnableIST {
		enabledCount++
	}
	if payload.EnableHolland {
		enabledCount++
	}
	if payload.EnableLearningStyle {
		enabledCount++
	}
	if payload.EnableKraepelin {
		enabledCount++
	}
	if payload.EnableRMIB {
		enabledCount++
	}
	if payload.EnablePAPI {
		enabledCount++
	}
	if enabledCount < 1 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Pilih minimal satu jenis tes untuk batch ini.",
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

	var teacherId *int
	if tID := c.GetSession("teacher_id"); tID != nil {
		if idVal, ok := tID.(int); ok {
			teacherId = &idVal
		}
	}

	batch := models.TestBatch{
		Name:            payload.Name,
		Institution:     payload.Institution,
		TahunAjaran:     payload.TahunAjaran,
		Sekolah:         payload.Sekolah,
		JenjangSekolah:  payload.JenjangSekolah,
		Kelas:           payload.Kelas,
		Jurusan:         payload.Jurusan,
		EnableIST:       payload.EnableIST,
		EnableHolland:   payload.EnableHolland,
		EnableLearningStyle: payload.EnableLearningStyle,
		EnableKraepelin: payload.EnableKraepelin,
		EnableRMIB:      payload.EnableRMIB,
		EnablePAPI:      payload.EnablePAPI,
		TestOrder:       payload.TestOrder,
		PurposeCategory: payload.PurposeCategory,
		PurposeDetail:   payload.PurposeDetail,
		SendViaEmail:    payload.SendViaEmail,
		SendViaBrowser:  payload.SendViaBrowser,
		SendViaWhatsApp: payload.SendViaWhatsApp,
		Status:          models.StatusBatchActive,
		CreatedBy:       userID.(int),
		TeacherId:       teacherId,
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
// Buat undangan dari daftar peserta. Setiap entry boleh berformat:
//   - "email"
//   - "email,phone"   (phone opsional, untuk WA)
//   - {email, phone}  (object, jika frontend mengirim daftar terstruktur)
//
// Email TIDAK harus sudah terdaftar di tabel users; cukup validasi FORMAT.
// Saat undangan dibuat, sistem hanya mengirim PENGUMUMAN (tanpa token).
// Token sesungguhnya baru dikirim saat operator klik "Kirim Code".
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

	// Terima dua bentuk: array string (legacy) atau array object {email, phone}.
	var payload struct {
		Emails    []string `json:"emails"`
		Recipients []struct {
			Email string `json:"email"`
			Phone string `json:"phone"`
		} `json:"recipients"`
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

	type recipient struct {
		Email string
		Phone string
	}
	var recipients []recipient
	for _, r := range payload.Recipients {
		recipients = append(recipients, recipient{Email: strings.TrimSpace(r.Email), Phone: strings.TrimSpace(r.Phone)})
	}
	for _, raw := range payload.Emails {
		// Setiap baris dari Excel / textarea (pisahkan via tab, koma, titik koma, atau pipe)
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		var email, phone string
		seps := []string{"\t", ",", ";", "|"}
		var cells []string
		for _, sep := range seps {
			if strings.Contains(line, sep) {
				for _, part := range strings.Split(line, sep) {
					trimmed := strings.TrimSpace(part)
					if trimmed != "" {
						cells = append(cells, trimmed)
					}
				}
				break
			}
		}
		if len(cells) == 0 {
			cells = []string{line}
		}

		// Smart scan sel untuk mencari email (@) & nomor HP (digit)
		for _, cell := range cells {
			if email == "" && isValidEmailFormat(cell) {
				email = cell
			} else if phone == "" && isPossiblePhoneNumber(cell) {
				phone = cell
			}
		}
		// Fallback: kalau belum terdeteksi via scan tapi sel pertama valid
		if email == "" && len(cells) > 0 && isValidEmailFormat(cells[0]) {
			email = cells[0]
			if phone == "" && len(cells) > 1 && isPossiblePhoneNumber(cells[1]) {
				phone = cells[1]
			}
		}

		if email != "" {
			recipients = append(recipients, recipient{Email: email, Phone: phone})
		}
	}

	if len(recipients) == 0 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Daftar email tidak boleh kosong",
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
			Message: "Batch tes tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	now := time.Now()
	exp := now.Add(7 * 24 * time.Hour) // perpanjang ke 7 hari sejak token bersifat pengumuman dulu

	var created []models.TestInvitation
	var invalidEmails []string
	seen := map[string]bool{}

	for _, r := range recipients {
		email := strings.ToLower(strings.TrimSpace(r.Email))
		if email == "" {
			continue
		}
		if seen[email] {
			continue
		}
		seen[email] = true
		if !isValidEmailFormat(email) {
			invalidEmails = append(invalidEmails, r.Email)
			continue
		}

		// Auto-link ke user jika sudah terdaftar (opsional, tidak wajib).
		var user models.User
		user.Email = email
		var userIDPtr *int
		var displayName string
		var phoneFromUser string
		if err := o.Read(&user, "Email"); err == nil {
			userIDPtr = &user.Id
			displayName = user.NamaLengkap
			phoneFromUser = user.NoHandphone
		}

		// Pilih nomor HP: dari input (kalau diisi) > dari user.NoHandphone.
		phone := strings.TrimSpace(r.Phone)
		if phone == "" {
			phone = phoneFromUser
		}

		var teacherId *int
		if tID := c.GetSession("teacher_id"); tID != nil {
			if idVal, ok := tID.(int); ok {
				teacherId = &idVal
			}
		}

		batchIDPtr := &batchID
		inv := models.TestInvitation{
			BatchId:   batchIDPtr,
			Email:     email,
			Phone:     phone,
			UserId:    userIDPtr,
			Token:     generateToken(8),
			ExpiresAt: exp,
			Status:    models.StatusInvitationPending,
			TeacherId: teacherId,
		}
		if _, err := o.Insert(&inv); err != nil {
			log.Printf("CreateInvitations insert error for %s: %v", email, err)
			continue
		}

		// Kirim PENGUMUMAN (tanpa token) sesuai channel batch.
		if batch.SendViaEmail {
			go sendInvitationAnnouncementEmail(&batch, displayName, email, &inv)
		}
		if batch.SendViaWhatsApp && phone != "" {
			go sendInvitationAnnouncementWA(&batch, displayName, phone, &inv)
		}
		if batch.SendViaBrowser && userIDPtr != nil {
			go createInvitationNotification(&batch, &user, &inv)
		}

		created = append(created, inv)
	}

	if len(created) == 0 && len(invalidEmails) > 0 {
		c.Ctx.Output.SetStatus(422)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Format email tidak valid",
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

// isValidEmailFormat validasi format email sederhana (tidak query DNS / Google).
func isValidEmailFormat(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 254 {
		return false
	}
	at := strings.IndexByte(s, '@')
	if at <= 0 || at == len(s)-1 {
		return false
	}
	local := s[:at]
	domain := s[at+1:]
	if len(local) == 0 || len(local) > 64 {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	for _, ch := range s {
		if ch <= ' ' || ch == ',' || ch == ';' {
			return false
		}
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	return true
}

func isPossiblePhoneNumber(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	cleaned := strings.ReplaceAll(s, "-", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "(", "")
	cleaned = strings.ReplaceAll(cleaned, ")", "")
	if strings.HasPrefix(cleaned, "+") {
		cleaned = cleaned[1:]
	}
	if len(cleaned) < 8 || len(cleaned) > 15 {
		return false
	}
	for _, ch := range cleaned {
		if ch < '0' || ch > '9' {
			return false
		}
	}
	return true
}

// @router /api/admin/test-batches/:id/invitations [get]
func (c *PsychotestAdminController) ListInvitations() {
	ok, _, sekolah := c.verifyAdminOrSchool()
	if !ok {
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
			Message: "Batch tes tidak ditemukan",
		}
		c.ServeJSON()
		return
	}
	isDemo := strings.Contains(strings.ToLower(batch.Name), "demo") || batch.Sekolah == ""
	if sekolah != "" && !strings.EqualFold(batch.Sekolah, sekolah) && !isDemo {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Akses ditolak: Anda tidak memiliki akses ke batch ini",
		}
		c.ServeJSON()
		return
	}

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

	invitations = filterInvitationsBySchool(invitations, sekolah)

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    invitations,
	}
	c.ServeJSON()
}

// resolveDisplayName memilih nama untuk salam email/WA.
func resolveDisplayName(name, email string) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return email
}

// Helper: kirim email PENGUMUMAN undangan (tanpa token).
// Email ini hanya memberitahu peserta bahwa mereka diundang ikut tes batch tertentu.
func sendInvitationAnnouncementEmail(batch *models.TestBatch, displayName, email string, inv *models.TestInvitation) {
	sendInvitationCodeEmail(batch, displayName, email, inv)
}

// Helper: kirim email berisi KODE / TOKEN tes.
func sendInvitationCodeEmail(batch *models.TestBatch, displayName, email string, inv *models.TestInvitation) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in sendInvitationCodeEmail: %v", r)
		}
	}()

	config := utils.GetEmailConfig()

	appURL := utils.GetAppBaseURL()
	link := fmt.Sprintf("%s/test?token=%s", appURL, inv.Token)

	subject := fmt.Sprintf("Kode Tes Psikologi - %s", batch.Name)

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
	`, resolveDisplayName(displayName, email), batch.Name, batch.Institution, invitationTestTypes(batch), inv.Token, link, inv.ExpiresAt.Format("02 Jan 2006 15:04"))

	emailData := utils.EmailData{
		To:      email,
		Subject: subject,
		Body:    body,
	}

	if err := utils.SendEmail(config, emailData); err != nil {
		log.Printf("Gagal mengirim email kode tes ke %s: %v", email, err)
	}
}

// Helper: kirim PENGUMUMAN undangan via WhatsApp.
func sendInvitationAnnouncementWA(batch *models.TestBatch, displayName, phone string, inv *models.TestInvitation) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in sendInvitationAnnouncementWA: %v", r)
		}
	}()

	cfg := utils.GetWhatsAppConfig()
	appURL := utils.GetAppBaseURL()
	link := fmt.Sprintf("%s/test", appURL)

	msg := fmt.Sprintf(
		"Halo %s,\n\nAnda diundang mengikuti tes psikologi:\n• Batch: %s\n• Institusi: %s\n• Tipe Tes: %s\n\n"+
			"Kode (token) akses akan dikirim terpisah oleh admin. Anda bisa membuka halaman tes terlebih dahulu di:\n%s\n\n"+
			"Undangan berlaku sampai: %s.\n— Psychee Wellness",
		resolveDisplayName(displayName, ""), batch.Name, batch.Institution, invitationTestTypes(batch),
		link, inv.ExpiresAt.Format("02 Jan 2006 15:04"))

	if err := utils.SendWhatsApp(cfg, phone, msg); err != nil {
		log.Printf("Gagal mengirim WA pengumuman ke %s: %v", phone, err)
	}
}

// Helper: kirim KODE tes via WhatsApp.
func sendInvitationCodeWA(batch *models.TestBatch, displayName, phone string, inv *models.TestInvitation) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic in sendInvitationCodeWA: %v", r)
		}
	}()

	cfg := utils.GetWhatsAppConfig()
	appURL := utils.GetAppBaseURL()
	link := fmt.Sprintf("%s/test?token=%s", appURL, inv.Token)

	msg := fmt.Sprintf(
		"Halo %s,\n\nBerikut kode akses Tes Psikologi - %s:\n\n*KODE: %s*\n\n"+
			"Buka halaman tes lalu masukkan kode di atas, atau klik link berikut:\n%s\n\n"+
			"Kode berlaku sampai: %s. Jangan bagikan kode ini kepada orang lain.\n— Psychee Wellness",
		resolveDisplayName(displayName, ""), batch.Name, inv.Token, link,
		inv.ExpiresAt.Format("02 Jan 2006 15:04"))

	if err := utils.SendWhatsApp(cfg, phone, msg); err != nil {
		log.Printf("Gagal mengirim WA kode ke %s: %v", phone, err)
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
	if batch.EnableLearningStyle {
		parts = append(parts, "Gaya Belajar (VAK)")
	}
	if batch.EnableKraepelin {
		parts = append(parts, "Kraepelin")
	}
	if batch.EnableRMIB {
		parts = append(parts, "RMIB")
	}
	if batch.EnablePAPI {
		parts = append(parts, "PAPI")
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, ", ")
}

// @router /api/admin/test-batches/:id/results [get]
func (c *PsychotestAdminController) ListBatchResults() {
	ok, _, sekolahFilter := c.verifyAdminOrSchool()
	if !ok {
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
			Message: "Batch tes tidak ditemukan",
		}
		c.ServeJSON()
		return
	}
	isDemo := strings.Contains(strings.ToLower(batch.Name), "demo") || batch.Sekolah == ""
	if sekolahFilter != "" && !strings.EqualFold(batch.Sekolah, sekolahFilter) && !isDemo {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Akses ditolak: Anda tidak memiliki akses ke batch ini",
		}
		c.ServeJSON()
		return
	}

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

	if !isDemo {
		invitations = filterInvitationsBySchool(invitations, sekolahFilter)
	}

	// Fetch all school teachers to map teacher_id to teacher_name
	var teachers []models.SchoolTeacher
	_, _ = o.QueryTable(new(models.SchoolTeacher)).All(&teachers)
	teacherMap := make(map[int]string)
	for _, t := range teachers {
		teacherMap[t.Id] = t.Nama
	}

	type InvitationSummary struct {
		Invitation        models.TestInvitation       `json:"invitation"`
		TeacherName       string                      `json:"teacher_name,omitempty"`
		StudentName       string                      `json:"student_name,omitempty"`
		StudentEmail      string                      `json:"student_email,omitempty"`
		StudentAvatar     string                      `json:"student_avatar,omitempty"`
		IST               *models.ISTResult           `json:"ist_result,omitempty"`
		ISTCompletedCount int                         `json:"ist_completed_count"`
		Holland           *models.HollandResult       `json:"holland_result,omitempty"`
		RMIB              *models.RMIBResult          `json:"rmib_result,omitempty"`
		LearningStyle     *models.LearningStyleResult `json:"learning_style_result,omitempty"`
		Kraepelin         *models.KraepelinAttempt    `json:"kraepelin_attempt,omitempty"`
		PAPI              *models.PAPIResult          `json:"papi_result,omitempty"`
	}

	var result []InvitationSummary

	for _, inv := range invitations {
		var teacherName string
		if inv.TeacherId != nil {
			teacherName = teacherMap[*inv.TeacherId]
		}
		var istProgressCount int64
		_, _ = o.QueryTable(new(models.ISTProgress)).Filter("Invitation__Id", inv.Id).Count()

		summary := InvitationSummary{
			Invitation:        inv,
			TeacherName:       teacherName,
			StudentEmail:      inv.Email,
			ISTCompletedCount: int(istProgressCount),
		}

		// Resolve student real name from User table
		if inv.UserId != nil {
			var u models.User
			u.Id = *inv.UserId
			if o.Read(&u) == nil {
				if u.NamaLengkap != "" {
					summary.StudentName = u.NamaLengkap
				}
				if u.FotoProfil != "" {
					summary.StudentAvatar = u.FotoProfil
				}
			}
		}
		if summary.StudentName == "" {
			// Fallback: derive name from email prefix
			summary.StudentName = strings.Split(inv.Email, "@")[0]
		}

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
		if err != nil || hol.Id == 0 {
			err = o.Raw("SELECT * FROM holland_results WHERE invitation_id = $1", inv.Id).QueryRow(&hol)
		}
		if err == nil && hol.Id != 0 {
			summary.Holland = &hol
		} else {
			summary.Holland = nil
		}

		var rmib models.RMIBResult
		err = o.QueryTable(new(models.RMIBResult)).
			Filter("Invitation__Id", inv.Id).
			One(&rmib)
		if err != nil || rmib.Id == 0 {
			err = o.Raw("SELECT * FROM rmib_results WHERE invitation_id = $1", inv.Id).QueryRow(&rmib)
		}
		if err == nil && rmib.Id != 0 {
			summary.RMIB = &rmib
		} else {
			summary.RMIB = nil
		}

		var vak models.LearningStyleResult
		err = o.QueryTable(new(models.LearningStyleResult)).
			Filter("Invitation__Id", inv.Id).
			One(&vak)
		if err != nil || vak.Id == 0 {
			err = o.Raw("SELECT * FROM learning_style_results WHERE invitation_id = $1", inv.Id).QueryRow(&vak)
		}
		if err == nil && vak.Id != 0 {
			summary.LearningStyle = &vak
		} else {
			summary.LearningStyle = nil
		}

		var krp models.KraepelinAttempt
		err = o.QueryTable(new(models.KraepelinAttempt)).
			Filter("Invitation__Id", inv.Id).
			One(&krp)
		if err != nil || krp.Id == 0 {
			err = o.Raw("SELECT * FROM kraepelin_attempts WHERE invitation_id = $1", inv.Id).QueryRow(&krp)
		}
		if err == nil && krp.Id != 0 {
			summary.Kraepelin = &krp
		} else {
			summary.Kraepelin = nil
		}

		var papi models.PAPIResult
		err = o.QueryTable(new(models.PAPIResult)).
			Filter("Invitation__Id", inv.Id).
			One(&papi)
		if err != nil || papi.Id == 0 {
			err = o.Raw("SELECT * FROM papi_results WHERE invitation_id = $1", inv.Id).QueryRow(&papi)
		}
		if err == nil && papi.Id != 0 {
			summary.PAPI = &papi
		} else {
			summary.PAPI = nil
		}

		result = append(result, summary)
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    result,
	}
	c.ServeJSON()
}

// GetInvitationResult mengambil detail hasil seluruh alat tes untuk satu undangan (peserta).
func (c *PsychotestAdminController) GetInvitationResult() {
	ok, _, sekolahFilter := c.verifyAdminOrSchool()
	if !ok {
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
	var inv models.TestInvitation
	inv.Id = invID
	if err := o.Read(&inv); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Undangan tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	// Filter berdasarkan sekolah jika login sebagai sekolah
	if inv.BatchId != nil {
		var batch models.TestBatch
		batch.Id = *inv.BatchId
		if o.Read(&batch) == nil {
			if sekolahFilter != "" && !strings.EqualFold(batch.Sekolah, sekolahFilter) {
				c.Ctx.Output.SetStatus(403)
				c.Data["json"] = PsychotestAdminResponse{
					Success: false,
					Message: "Akses ditolak: Anda tidak memiliki akses ke data undangan ini",
				}
				c.ServeJSON()
				return
			}
		}
	}

	type InvitationSummary struct {
		Invitation    models.TestInvitation       `json:"invitation"`
		StudentName   string                      `json:"student_name"`
		StudentEmail  string                      `json:"student_email"`
		StudentAvatar string                      `json:"student_avatar"`
		StudentGender string                      `json:"student_gender"`
		StudentDob    string                      `json:"student_dob"`
		StudentPob    string                      `json:"student_pob"`
		StudentSchool string                      `json:"student_school"`
		StudentClass  string                      `json:"student_class"`
		StudentMajor  string                      `json:"student_major"`
		Batch         *models.TestBatch           `json:"batch,omitempty"`
		IST           *models.ISTResult           `json:"ist_result,omitempty"`
		Holland       *models.HollandResult       `json:"holland_result,omitempty"`
		RMIB          *models.RMIBResult          `json:"rmib_result,omitempty"`
		LearningStyle *models.LearningStyleResult `json:"learning_style_result,omitempty"`
		Kraepelin     *models.KraepelinAttempt    `json:"kraepelin_attempt,omitempty"`
		PAPI          *models.PAPIResult          `json:"papi_result,omitempty"`
	}

	var batch models.TestBatch
	if inv.BatchId != nil {
		batch.Id = *inv.BatchId
		_ = o.Read(&batch)
	}

	summary := InvitationSummary{
		Invitation:   inv,
		StudentEmail: inv.Email,
	}
	if inv.BatchId != nil && batch.Id != 0 {
		summary.Batch = &batch
	}

	// Resolve student real name and avatar from User table
	if inv.UserId != nil {
		var u models.User
		u.Id = *inv.UserId
		if o.Read(&u) == nil {
			if u.NamaLengkap != "" {
				summary.StudentName = u.NamaLengkap
			}
			if u.FotoProfil != "" {
				summary.StudentAvatar = u.FotoProfil
			}
			if u.JenisKelamin == models.GenderLakiLaki {
				summary.StudentGender = "Laki-laki"
			} else if u.JenisKelamin == models.GenderPerempuan {
				summary.StudentGender = "Perempuan"
			}
			if u.TanggalLahir != nil {
				summary.StudentDob = u.TanggalLahir.Format("02 January 2006")
			}
			summary.StudentPob = u.TempatLahir
			summary.StudentSchool = u.Sekolah
			summary.StudentClass = u.Kelas
			summary.StudentMajor = u.Jurusan
		}
	}
	if summary.StudentName == "" {
		summary.StudentName = strings.Split(inv.Email, "@")[0]
	}
	if summary.StudentSchool == "" {
		if batch.Sekolah != "" {
			summary.StudentSchool = batch.Sekolah
		} else {
			summary.StudentSchool = batch.Institution
		}
	}
	if summary.StudentClass == "" {
		summary.StudentClass = batch.Kelas
	}
	if summary.StudentMajor == "" {
		summary.StudentMajor = batch.Jurusan
	}

	// Get IST
	var ist models.ISTResult
	err = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&ist)
	if err != nil || ist.Id == 0 {
		_ = o.Raw("SELECT * FROM ist_results WHERE invitation_id = $1", inv.Id).QueryRow(&ist)
	}
	if ist.Id != 0 {
		summary.IST = &ist
	}

	// Get Holland
	var hol models.HollandResult
	err = o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&hol)
	if err != nil || hol.Id == 0 {
		_ = o.Raw("SELECT * FROM holland_results WHERE invitation_id = $1", inv.Id).QueryRow(&hol)
	}
	if hol.Id != 0 {
		summary.Holland = &hol
	}

	// Get RMIB
	var rmib models.RMIBResult
	err = o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).One(&rmib)
	if err != nil || rmib.Id == 0 {
		_ = o.Raw("SELECT * FROM rmib_results WHERE invitation_id = $1", inv.Id).QueryRow(&rmib)
	}
	if rmib.Id != 0 {
		summary.RMIB = &rmib
	}

	// Get Learning Style
	var vak models.LearningStyleResult
	err = o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&vak)
	if err != nil || vak.Id == 0 {
		_ = o.Raw("SELECT * FROM learning_style_results WHERE invitation_id = $1", inv.Id).QueryRow(&vak)
	}
	if vak.Id != 0 {
		summary.LearningStyle = &vak
	}

	// Get Kraepelin
	var krp models.KraepelinAttempt
	err = o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&krp)
	if err != nil || krp.Id == 0 {
		_ = o.Raw("SELECT * FROM kraepelin_attempts WHERE invitation_id = $1", inv.Id).QueryRow(&krp)
	}
	if krp.Id != 0 {
		summary.Kraepelin = &krp
	}

	// Get PAPI
	var papi models.PAPIResult
	err = o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&papi)
	if err != nil || papi.Id == 0 {
		_ = o.Raw("SELECT * FROM papi_results WHERE invitation_id = $1", inv.Id).QueryRow(&papi)
	}
	if papi.Id != 0 {
		summary.PAPI = &papi
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Data:    summary,
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

		// Learning Style (VAK)
		if batch.EnableLearningStyle {
			content, ferr := buildLearningStyleResultXLSX(o, &batch, &inv, user)
			if ferr == nil && len(content) > 0 {
				fname := fmt.Sprintf("%s_Hasil_Gaya_Belajar.xlsx", base)
				fname = makeUniqueZipName(usedNames, fname, inv.Id)
				w, _ := zw.Create(fname)
				_, _ = w.Write(content)
				written++
			}
		}

		// RMIB
		if batch.EnableRMIB {
			content, ferr := buildRMIBResultXLSX(o, &batch, &inv, user)
			if ferr == nil && len(content) > 0 {
				fname := fmt.Sprintf("%s_Hasil_RMIB.xlsx", base)
				fname = makeUniqueZipName(usedNames, fname, inv.Id)
				w, _ := zw.Create(fname)
				_, _ = w.Write(content)
				written++
			}
		}

		// PAPI
		if batch.EnablePAPI {
			content, ferr := buildPAPIResultXLSX(o, &batch, &inv, user)
			if ferr == nil && len(content) > 0 {
				fname := fmt.Sprintf("%s_Hasil_PAPI.xlsx", base)
				fname = makeUniqueZipName(usedNames, fname, inv.Id)
				w, _ := zw.Create(fname)
				_, _ = w.Write(content)
				written++
			}
		}

		// Kraepelin
		if batch.EnableKraepelin {
			content, ferr := buildKraepelinResultXLSX(o, &batch, &inv, user)
			if ferr == nil && len(content) > 0 {
				fname := fmt.Sprintf("%s_Hasil_Kraepelin.xlsx", base)
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
	if batch.EnableLearningStyle {
		content, ferr := buildLearningStyleResultXLSX(o, &batch, &inv, user)
		if ferr == nil && len(content) > 0 {
			fname := makeUniqueZipName(usedNames, fmt.Sprintf("%s_Hasil_Gaya_Belajar.xlsx", base), inv.Id)
			w, _ := zw.Create(fname)
			_, _ = w.Write(content)
			written++
		}
	}
	if batch.EnableRMIB {
		content, ferr := buildRMIBResultXLSX(o, &batch, &inv, user)
		if ferr == nil && len(content) > 0 {
			fname := makeUniqueZipName(usedNames, fmt.Sprintf("%s_Hasil_RMIB.xlsx", base), inv.Id)
			w, _ := zw.Create(fname)
			_, _ = w.Write(content)
			written++
		}
	}
	if batch.EnablePAPI {
		content, ferr := buildPAPIResultXLSX(o, &batch, &inv, user)
		if ferr == nil && len(content) > 0 {
			fname := makeUniqueZipName(usedNames, fmt.Sprintf("%s_Hasil_PAPI.xlsx", base), inv.Id)
			w, _ := zw.Create(fname)
			_, _ = w.Write(content)
			written++
		}
	}
	if batch.EnableKraepelin {
		content, ferr := buildKraepelinResultXLSX(o, &batch, &inv, user)
		if ferr == nil && len(content) > 0 {
			fname := makeUniqueZipName(usedNames, fmt.Sprintf("%s_Hasil_Kraepelin.xlsx", base), inv.Id)
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

	// 1. Fetch IST Result record
	var res models.ISTResult
	err := o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)

	// Ensure IQ & Standard Scores if user TanggalLahir is known
	age := 0
	if user != nil && user.TanggalLahir != nil {
		age = utils.AgeYears(*user.TanggalLahir, time.Now())
	}
	if err == nil && res.Id > 0 && age > 0 {
		if updatedRes, uErr := utils.EnsureISTStandardAndIQScores(o, &res, age); uErr == nil && updatedRes != nil {
			res = *updatedRes
		}
	}

	// Load raw answers for sheet 2 / backup calculation
	var rows []istAnswerExportRow
	_, _ = o.Raw(`
		SELECT q.number, a.answer_option, a.is_correct, s.code AS subtest_code
		FROM ist_answers a
		JOIN ist_questions q ON q.id = a.question_id
		JOIN ist_subtests s ON s.id = a.subtest_id
		WHERE a.invitation_id = ?
		ORDER BY q.number
	`, inv.Id).QueryRows(&rows)

	answersByNumber := make(map[int]string)
	rawBySubtest := make(map[string]int)
	for _, r := range rows {
		answersByNumber[r.Number] = strings.ToUpper(strings.TrimSpace(r.Answer))
		code := normalizeISTSubtestCodeForExport(r.SubtestRaw)
		if r.IsCorrect {
			rawBySubtest[code]++
		}
	}

	// Extract participant identity
	nama := ""
	email := inv.Email
	dob := "-"
	tempatLahir := "-"
	gender := "-"
	nisnOrNip := "-"
	sekolah := "-"
	kelas := "-"
	jurusan := "-"

	if user != nil {
		if user.NamaLengkap != "" {
			nama = user.NamaLengkap
		}
		if user.Email != "" {
			email = user.Email
		}
		if user.TempatLahir != "" {
			tempatLahir = user.TempatLahir
		}
		if user.TanggalLahir != nil {
			dob = user.TanggalLahir.Format("2006-01-02")
		}
		if string(user.JenisKelamin) != "" {
			gender = string(user.JenisKelamin)
		}
		if strings.TrimSpace(user.NISN) != "" {
			nisnOrNip = user.NISN
		} else if strings.TrimSpace(user.NIP) != "" {
			nisnOrNip = user.NIP
		}
		if user.Sekolah != "" {
			sekolah = user.Sekolah
		}
		if user.Kelas != "" {
			kelas = user.Kelas
		}
		if user.Jurusan != "" {
			jurusan = user.Jurusan
		}
	}

	ttl := tempatLahir
	if dob != "-" {
		ttl = fmt.Sprintf("%s, %s", tempatLahir, dob)
	}

	// Create Excel File
	f := excelize.NewFile()
	sheet1 := "Psikogram IST"
	f.SetSheetName(f.GetSheetName(0), sheet1)

	// Define styles & colors
	borderThin := []excelize.Border{
		{Type: "left", Color: "D0D0D0", Style: 1},
		{Type: "right", Color: "D0D0D0", Style: 1},
		{Type: "top", Color: "D0D0D0", Style: 1},
		{Type: "bottom", Color: "D0D0D0", Style: 1},
	}
	borderHeader := []excelize.Border{
		{Type: "left", Color: "4682B4", Style: 1},
		{Type: "right", Color: "4682B4", Style: 1},
		{Type: "top", Color: "4682B4", Style: 1},
		{Type: "bottom", Color: "4682B4", Style: 1},
	}

	styleTitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "1E293B"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styleSectionHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 11, Color: "1E3A8A"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"EFF6FF"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
		Border:    borderHeader,
	})
	styleTableHeaderGroup, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "1E293B"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F1F5F9"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderThin,
	})
	styleTableHeaderSub, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 9, Color: "475569"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"F8FAFC"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderThin,
	})
	styleCellLeft, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 9, Color: "334155"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border:    borderThin,
	})
	styleCellCenter, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "0F172A"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderThin,
	})
	styleCheckmark, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "2563EB"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderThin,
	})
	styleLabelBold, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "334155"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})
	styleValue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10, Color: "0F172A"},
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center"},
	})

	// Set Column Widths
	_ = f.SetColWidth(sheet1, "A", "A", 5)   // NO
	_ = f.SetColWidth(sheet1, "B", "B", 28)  // ASPEK PSIKOLOGIS
	_ = f.SetColWidth(sheet1, "C", "C", 55)  // URAIAN
	_ = f.SetColWidth(sheet1, "D", "D", 14)  // KURANG SEKALI
	_ = f.SetColWidth(sheet1, "E", "E", 14)  // KURANG
	_ = f.SetColWidth(sheet1, "F", "F", 14)  // CUKUP
	_ = f.SetColWidth(sheet1, "G", "G", 14)  // CUKUP BAIK
	_ = f.SetColWidth(sheet1, "H", "H", 14)  // BAIK
	_ = f.SetColWidth(sheet1, "I", "I", 14)  // BAIK SEKALI

	// 1. Judul
	_ = f.SetCellValue(sheet1, "A1", "LAPORAN PSIKOGRAM HASIL TES IST (INTELLIGENZ STRUKTUR TEST)")
	_ = f.MergeCell(sheet1, "A1", "I1")
	_ = f.SetRowHeight(sheet1, 1, 25)
	_ = f.SetCellStyle(sheet1, "A1", "I1", styleTitle)

	// 2. Data Identitas Peserta
	_ = f.SetCellValue(sheet1, "A3", "DATA PESERTA TES")
	_ = f.MergeCell(sheet1, "A3", "I3")
	_ = f.SetCellStyle(sheet1, "A3", "I3", styleSectionHeader)

	identities := []struct {
		r int
		l1, v1, l2, v2 string
	}{
		{4, "Nama Lengkap", nama, "NISN / NIP", nisnOrNip},
		{5, "Tempat, Tgl Lahir", ttl, "Usia", fmt.Sprintf("%d Tahun", age)},
		{6, "Jenis Kelamin", gender, "Kelas / Jurusan", fmt.Sprintf("%s / %s", kelas, jurusan)},
		{7, "Sekolah / Institusi", sekolah, "Tanggal Tes", inv.CreatedAt.Format("2006-01-02")},
		{8, "Email / Kontak", email, "-", "-"},
	}
	for _, iden := range identities {
		_ = f.SetCellValue(sheet1, fmt.Sprintf("A%d", iden.r), iden.l1)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("B%d", iden.r), ":")
		_ = f.SetCellValue(sheet1, fmt.Sprintf("C%d", iden.r), iden.v1)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("F%d", iden.r), iden.l2)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("G%d", iden.r), ":")
		_ = f.SetCellValue(sheet1, fmt.Sprintf("H%d", iden.r), iden.v2)

		_ = f.SetCellStyle(sheet1, fmt.Sprintf("A%d", iden.r), fmt.Sprintf("A%d", iden.r), styleLabelBold)
		_ = f.SetCellStyle(sheet1, fmt.Sprintf("C%d", iden.r), fmt.Sprintf("C%d", iden.r), styleValue)
		_ = f.SetCellStyle(sheet1, fmt.Sprintf("F%d", iden.r), fmt.Sprintf("F%d", iden.r), styleLabelBold)
		_ = f.SetCellStyle(sheet1, fmt.Sprintf("H%d", iden.r), fmt.Sprintf("H%d", iden.r), styleValue)
	}

	// 3. Ringkasan Kapasitas Intelektual (IQ)
	_ = f.SetCellValue(sheet1, "A9", "I. RINGKASAN KAPASITAS INTELEKTUAL (IQ)")
	_ = f.MergeCell(sheet1, "A9", "I9")
	_ = f.SetCellStyle(sheet1, "A9", "I9", styleSectionHeader)

	totalRW := res.RawSE + res.RawWA + res.RawAN + res.RawGE + res.RawRA + res.RawZA + res.RawFA + res.RawWU + res.RawME
	iqCat := res.IQCategory
	if iqCat == "" {
		iqCat = "-"
	}

	_ = f.SetCellValue(sheet1, "A10", "IQ (Skala IST)")
	_ = f.SetCellValue(sheet1, "B10", ":")
	_ = f.SetCellValue(sheet1, "C10", fmt.Sprintf("%d (%s)", res.IQ, iqCat))
	_ = f.SetCellValue(sheet1, "F10", "Total Skor Standar (SW)")
	_ = f.SetCellValue(sheet1, "G10", ":")
	_ = f.SetCellValue(sheet1, "H10", res.TotalStandardScore)

	_ = f.SetCellValue(sheet1, "A11", "Total Raw Score (RW)")
	_ = f.SetCellValue(sheet1, "B11", ":")
	_ = f.SetCellValue(sheet1, "C11", totalRW)
	_ = f.SetCellValue(sheet1, "F11", "Kategori IQ")
	_ = f.SetCellValue(sheet1, "G11", ":")
	_ = f.SetCellValue(sheet1, "H11", iqCat)

	_ = f.SetCellStyle(sheet1, "A10", "A11", styleLabelBold)
	_ = f.SetCellStyle(sheet1, "C10", "C11", styleValue)
	_ = f.SetCellStyle(sheet1, "F10", "F11", styleLabelBold)
	_ = f.SetCellStyle(sheet1, "H10", "H11", styleValue)

	// 4. II. PSIKOGRAM
	_ = f.SetCellValue(sheet1, "A13", "II. PSIKOGRAM")
	_ = f.MergeCell(sheet1, "A13", "I13")
	_ = f.SetCellStyle(sheet1, "A13", "I13", styleSectionHeader)

	// Header Psikogram (Row 14-17)
	_ = f.SetCellValue(sheet1, "A14", "NO")
	_ = f.MergeCell(sheet1, "A14", "A17")
	_ = f.SetCellValue(sheet1, "B14", "ASPEK PSIKOLOGIS")
	_ = f.MergeCell(sheet1, "B14", "B17")
	_ = f.SetCellValue(sheet1, "C14", "URAIAN")
	_ = f.MergeCell(sheet1, "C14", "C17")

	_ = f.SetCellValue(sheet1, "D14", "KATEGORI")
	_ = f.MergeCell(sheet1, "D14", "I14")

	_ = f.SetCellValue(sheet1, "D15", "KURANG")
	_ = f.MergeCell(sheet1, "D15", "E15")
	_ = f.SetCellValue(sheet1, "F15", "CUKUP")
	_ = f.MergeCell(sheet1, "F15", "G15")
	_ = f.SetCellValue(sheet1, "H15", "BAIK")
	_ = f.MergeCell(sheet1, "H15", "I15")

	_ = f.SetCellValue(sheet1, "D16", "KURANG SEKALI")
	_ = f.SetCellValue(sheet1, "E16", "KURANG")
	_ = f.SetCellValue(sheet1, "F16", "CUKUP")
	_ = f.SetCellValue(sheet1, "G16", "CUKUP BAIK")
	_ = f.SetCellValue(sheet1, "H16", "BAIK")
	_ = f.SetCellValue(sheet1, "I16", "BAIK SEKALI")

	_ = f.SetCellValue(sheet1, "D17", "1")
	_ = f.SetCellValue(sheet1, "E17", "2")
	_ = f.SetCellValue(sheet1, "F17", "3")
	_ = f.SetCellValue(sheet1, "G17", "4")
	_ = f.SetCellValue(sheet1, "H17", "5")
	_ = f.SetCellValue(sheet1, "I17", "6")

	// Apply header styles
	_ = f.SetCellStyle(sheet1, "A14", "I14", styleTableHeaderGroup)
	_ = f.SetCellStyle(sheet1, "A15", "I15", styleTableHeaderGroup)
	_ = f.SetCellStyle(sheet1, "A16", "I16", styleTableHeaderSub)
	_ = f.SetCellStyle(sheet1, "A17", "I17", styleTableHeaderSub)

	// Section A Row
	_ = f.SetCellValue(sheet1, "A18", "A. KECERDASAN UMUM (Skala IST)")
	_ = f.MergeCell(sheet1, "A18", "I18")
	_ = f.SetCellStyle(sheet1, "A18", "I18", styleSectionHeader)

	_ = f.SetCellValue(sheet1, "A19", "")
	_ = f.SetCellValue(sheet1, "B19", fmt.Sprintf("IQ = %d -- %s", res.IQ, iqCat))
	_ = f.MergeCell(sheet1, "B19", "C19")
	_ = f.SetCellStyle(sheet1, "A19", "I19", styleCellLeft)

	// Section B Row
	_ = f.SetCellValue(sheet1, "A20", "B. KEMAMPUAN KHUSUS")
	_ = f.MergeCell(sheet1, "A20", "I20")
	_ = f.SetCellStyle(sheet1, "A20", "I20", styleSectionHeader)

	// Helper average
	avg := func(vals ...int) int {
		sum := 0
		n := 0
		for _, v := range vals {
			if v > 0 {
				sum += v
				n++
			}
		}
		if n == 0 {
			return 0
		}
		return sum / n
	}

	type aspectDef struct {
		no      int
		nama    string
		uraian  string
		scoreSW int
	}

	aspects := []aspectDef{
		{1, "Penalaran Konkret", "Kemampuan berpikir praktis, sesuai kenyataan dan mengambil keputusan secara mandiri berdasarkan data maupun situasi serta kondisi yang ada.", avg(res.StdSE, res.StdGE)},
		{2, "Penalaran Verbal", "Kemampuan berpikir logis dalam penggunaan bahasa terkait informasi yang pernah diterima.", avg(res.StdSE, res.StdWA, res.StdGE)},
		{3, "Fleksibilitas Berpikir", "Kemampuan mengalihkan perhatian, beradaptasi, dan mencari sudut pandang baru dalam pemecahan masalah.", res.StdAN},
		{4, "Kemampuan Berpikir Abstrak", "Kemampuan membentuk konsep, abstraksi verbal, dan menemukan prinsip dasar dari suatu masalah.", res.StdGE},
		{5, "Penalaran Praktis & Berhitung", "Kemampuan bernalar logis secara praktis menggunakan angka dan hitungan matematis.", res.StdRA},
		{6, "Penalaran Numerik", "Kemampuan berpikir logis menggunakan hubungan antar angka dan pola kuantitatif.", res.StdZA},
		{7, "Visualisasi Ruang 2D", "Kemampuan membayangkan, mengolah, dan memvisualisasikan bentuk bidang dua dimensi.", res.StdFA},
		{8, "Orientasi Ruang 3D", "Kemampuan memahami struktur, volume, dan rotasi bentuk tiga dimensi.", res.StdWU},
		{9, "Daya Ingat & Konsentrasi", "Kemampuan mengingat kata/informasi jangka pendek serta mempertahankan konsentrasi.", res.StdME},
	}

	colNames := []string{"D", "E", "F", "G", "H", "I"}
	startRow := 21

	for _, asp := range aspects {
		r := startRow
		_ = f.SetRowHeight(sheet1, r, 28)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("A%d", r), asp.no)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("B%d", r), asp.nama)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("C%d", r), asp.uraian)

		_ = f.SetCellStyle(sheet1, fmt.Sprintf("A%d", r), fmt.Sprintf("A%d", r), styleCellCenter)
		_ = f.SetCellStyle(sheet1, fmt.Sprintf("B%d", r), fmt.Sprintf("B%d", r), styleLabelBold)
		_ = f.SetCellStyle(sheet1, fmt.Sprintf("C%d", r), fmt.Sprintf("C%d", r), styleCellLeft)

		catIdx := psychogramCatIdxFromSW(asp.scoreSW)
		for idx, col := range colNames {
			cellRef := fmt.Sprintf("%s%d", col, r)
			if idx == catIdx {
				_ = f.SetCellValue(sheet1, cellRef, "✓")
				_ = f.SetCellStyle(sheet1, cellRef, cellRef, styleCheckmark)
			} else {
				_ = f.SetCellValue(sheet1, cellRef, "")
				_ = f.SetCellStyle(sheet1, cellRef, cellRef, styleCellCenter)
			}
		}
		startRow++
	}

	// 5. Smart Summary IST (Row 31)
	sumRow := startRow + 1
	_ = f.SetCellValue(sheet1, fmt.Sprintf("A%d", sumRow), "SMART SUMMARY IST")
	_ = f.MergeCell(sheet1, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("I%d", sumRow))
	_ = f.SetCellStyle(sheet1, fmt.Sprintf("A%d", sumRow), fmt.Sprintf("I%d", sumRow), styleSectionHeader)

	type subPair struct {
		name  string
		score int
	}
	subPairs := []subPair{
		{"SE (Berpikir logis & Praktis)", res.StdSE},
		{"WA (Memahami arti kata & bahasa)", res.StdWA},
		{"AN (Fleksibilitas berpikir)", res.StdAN},
		{"GE (Kemampuan membentuk konsep & abstraksi verbal)", res.StdGE},
		{"RA (Penalaran & berhitung praktis)", res.StdRA},
		{"ZR (Berpikir logis menggunakan angka)", res.StdZA},
		{"FA (Imajinasi ruang & visualisasi 2D)", res.StdFA},
		{"WU (Memahami struktur ruang & 3D)", res.StdWU},
		{"ME (Daya ingat & konsentrasi)", res.StdME},
	}
	sort.Slice(subPairs, func(i, j int) bool {
		return subPairs[i].score > subPairs[j].score
	})
	top2Text := fmt.Sprintf("%s & %s", subPairs[0].name, subPairs[1].name)

	smartSummaryText := fmt.Sprintf("Berdasarkan Hasil Test IST, Kamu adalah seseorang yang memiliki IQ sebesar %d (%s) dengan dominan Intelligent %s.", res.IQ, iqCat, top2Text)

	sumTextRow := sumRow + 1
	_ = f.SetRowHeight(sheet1, sumTextRow, 30)
	_ = f.SetCellValue(sheet1, fmt.Sprintf("A%d", sumTextRow), smartSummaryText)
	_ = f.MergeCell(sheet1, fmt.Sprintf("A%d", sumTextRow), fmt.Sprintf("I%d", sumTextRow))
	_ = f.SetCellStyle(sheet1, fmt.Sprintf("A%d", sumTextRow), fmt.Sprintf("I%d", sumTextRow), styleCellLeft)

	// 6. Detail Skor Subtes (Row 35)
	tblRow := sumTextRow + 2
	_ = f.SetCellValue(sheet1, fmt.Sprintf("A%d", tblRow), "III. RINGKASAN SKOR PER SUBTES IST")
	_ = f.MergeCell(sheet1, fmt.Sprintf("A%d", tblRow), fmt.Sprintf("I%d", tblRow))
	_ = f.SetCellStyle(sheet1, fmt.Sprintf("A%d", tblRow), fmt.Sprintf("I%d", tblRow), styleSectionHeader)

	hRow := tblRow + 1
	_ = f.SetCellValue(sheet1, fmt.Sprintf("A%d", hRow), "No")
	_ = f.SetCellValue(sheet1, fmt.Sprintf("B%d", hRow), "Kode Subtes")
	_ = f.SetCellValue(sheet1, fmt.Sprintf("C%d", hRow), "Nama Subtes")
	_ = f.SetCellValue(sheet1, fmt.Sprintf("D%d", hRow), "Raw Score (RW)")
	_ = f.SetCellValue(sheet1, fmt.Sprintf("E%d", hRow), "Skor Standar (SW)")
	_ = f.SetCellStyle(sheet1, fmt.Sprintf("A%d", hRow), fmt.Sprintf("E%d", hRow), styleTableHeaderGroup)

	subDefs := []struct {
		no   int
		code string
		nama string
		rw   int
		sw   int
	}{
		{1, "SE", "Berpikir logis & Praktis", res.RawSE, res.StdSE},
		{2, "WA", "Memahami arti kata dan bahasa", res.RawWA, res.StdWA},
		{3, "AN", "Fleksibilitas berpikir", res.RawAN, res.StdAN},
		{4, "GE", "Kemampuan membentuk konsep dan abstraksi", res.RawGE, res.StdGE},
		{5, "RA", "Penalaran dan kemampuan berhitung", res.RawRA, res.StdRA},
		{6, "ZR", "Berpikir logis menggunakan angka", res.RawZA, res.StdZA},
		{7, "FA", "Imajinasi ruang dan visualisasi 2D", res.RawFA, res.StdFA},
		{8, "WU", "Memahami struktur ruang dan bentuk 3D", res.RawWU, res.StdWU},
		{9, "ME", "Daya ingat dan konsentrasi", res.RawME, res.StdME},
	}

	curR := hRow + 1
	for _, sd := range subDefs {
		_ = f.SetCellValue(sheet1, fmt.Sprintf("A%d", curR), sd.no)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("B%d", curR), sd.code)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("C%d", curR), sd.nama)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("D%d", curR), sd.rw)
		_ = f.SetCellValue(sheet1, fmt.Sprintf("E%d", curR), sd.sw)

		_ = f.SetCellStyle(sheet1, fmt.Sprintf("A%d", curR), fmt.Sprintf("B%d", curR), styleCellCenter)
		_ = f.SetCellStyle(sheet1, fmt.Sprintf("C%d", curR), fmt.Sprintf("C%d", curR), styleCellLeft)
		_ = f.SetCellStyle(sheet1, fmt.Sprintf("D%d", curR), fmt.Sprintf("E%d", curR), styleCellCenter)
		curR++
	}

	// Total Row
	_ = f.SetCellValue(sheet1, fmt.Sprintf("A%d", curR), "")
	_ = f.SetCellValue(sheet1, fmt.Sprintf("B%d", curR), "TOTAL")
	_ = f.SetCellValue(sheet1, fmt.Sprintf("C%d", curR), "Total Skor")
	_ = f.SetCellValue(sheet1, fmt.Sprintf("D%d", curR), totalRW)
	_ = f.SetCellValue(sheet1, fmt.Sprintf("E%d", curR), res.TotalStandardScore)
	_ = f.SetCellStyle(sheet1, fmt.Sprintf("A%d", curR), fmt.Sprintf("E%d", curR), styleTableHeaderGroup)

	// 7. Sheet 2 (Jawaban Detail)
	if len(answersByNumber) > 0 {
		sheet2 := "Jawaban Subtes"
		_, _ = f.NewSheet(sheet2)
		_ = f.SetCellValue(sheet2, "A1", "LEMBAR JAWABAN SUBTES IST")
		_ = f.SetCellStyle(sheet2, "A1", "A1", styleTitle)

		writeBlock := func(colNo, colAns int, topRow int, title string, startNum, endNum int, subtestCode string) {
			colNoName, _ := excelize.ColumnNumberToName(colNo)
			colAnsName, _ := excelize.ColumnNumberToName(colAns)
			topLeft := fmt.Sprintf("%s%d", colNoName, topRow)
			topRight := fmt.Sprintf("%s%d", colAnsName, topRow)
			_ = f.MergeCell(sheet2, topLeft, topRight)
			_ = f.SetCellValue(sheet2, topLeft, title)
			_ = f.SetCellStyle(sheet2, topLeft, topRight, styleTableHeaderGroup)

			hR := topRow + 1
			_ = f.SetCellValue(sheet2, fmt.Sprintf("%s%d", colNoName, hR), "No.")
			_ = f.SetCellValue(sheet2, fmt.Sprintf("%s%d", colAnsName, hR), "Jawaban")
			_ = f.SetCellStyle(sheet2, fmt.Sprintf("%s%d", colNoName, hR), fmt.Sprintf("%s%d", colAnsName, hR), styleTableHeaderSub)

			r := hR + 1
			for n := startNum; n <= endNum; n++ {
				_ = f.SetCellValue(sheet2, fmt.Sprintf("%s%d", colNoName, r), n)
				_ = f.SetCellValue(sheet2, fmt.Sprintf("%s%d", colAnsName, r), answersByNumber[n])
				_ = f.SetCellStyle(sheet2, fmt.Sprintf("%s%d", colNoName, r), fmt.Sprintf("%s%d", colAnsName, r), styleCellCenter)
				r++
			}

			_ = f.SetCellValue(sheet2, fmt.Sprintf("%s%d", colNoName, r), "RW")
			_ = f.SetCellValue(sheet2, fmt.Sprintf("%s%d", colAnsName, r), rawBySubtest[subtestCode])
			_ = f.SetCellStyle(sheet2, fmt.Sprintf("%s%d", colNoName, r), fmt.Sprintf("%s%d", colAnsName, r), styleTableHeaderSub)
		}

		writeBlock(1, 2, 3, "SUBTES 1 (SE)", 1, 20, "SE")
		writeBlock(4, 5, 3, "SUBTES 2 (WA)", 21, 40, "WA")
		writeBlock(7, 8, 3, "SUBTES 3 (AN)", 41, 60, "AN")
		writeBlock(10, 11, 3, "SUBTES 4 (GE)", 61, 76, "GE")

		writeBlock(1, 2, 28, "SUBTES 5 (RA)", 77, 96, "RA")
		writeBlock(4, 5, 28, "SUBTES 6 (ZR)", 97, 116, "ZR")
		writeBlock(7, 8, 28, "SUBTES 7 (FA)", 117, 136, "FA")
		writeBlock(10, 11, 28, "SUBTES 8 (WU)", 137, 156, "WU")
		writeBlock(13, 14, 28, "SUBTES 9 (ME)", 157, 176, "ME")
	}

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
	nisnNip := ""
	idLabel := "NISN/NIP"
	kelas := ""
	jurusan := ""
	if user != nil {
		nama = user.NamaLengkap
		if user.Email != "" {
			email = user.Email
		}
		nisnNip = strings.TrimSpace(user.NISN)
		idLabel = "NISN"
		if nisnNip == "" && strings.TrimSpace(user.NIP) != "" {
			nisnNip = user.NIP
			idLabel = "NIP"
		}
		kelas = user.Kelas
		jurusan = user.Jurusan
	}

	_ = w.Write([]string{"HASIL HOLLAND (RIASEC)"})
	_ = w.Write([]string{"Nama", nama})
	_ = w.Write([]string{idLabel, nisnNip})
	_ = w.Write([]string{"Kelas", kelas})
	_ = w.Write([]string{"Jurusan", jurusan})
	_ = w.Write([]string{"Email", email})
	if batch != nil {
		_ = w.Write([]string{"Batch", batch.Name})
		_ = w.Write([]string{"Institusi", batch.Institution})
	}
	_ = w.Write([]string{})
	_ = w.Write([]string{"Code", "Number", "Value"})
	for _, a := range holAnswers {
		_ = w.Write([]string{a.Question.Code, strconv.Itoa(a.Question.Number), strconv.Itoa(a.Value)})
	}
	return buf.Bytes(), nil
}

func buildLearningStyleResultXLSX(o orm.Ormer, batch *models.TestBatch, inv *models.TestInvitation, user *models.User) ([]byte, error) {
	if inv == nil {
		return nil, fmt.Errorf("nil invitation")
	}
	var res models.LearningStyleResult
	err := o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		return nil, fmt.Errorf("no learning style result")
	}

	f := excelize.NewFile()
	sheet := "Resume"
	f.SetSheetName(f.GetSheetName(0), sheet)
	showGridLines := false
	_ = f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGridLines})

	borderAll := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
	styleHeaderGreen, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF", Size: 16},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#00A65A"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
	})
	styleTableHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#00A65A"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    borderAll,
	})
	styleBody, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Border:    borderAll,
	})
	styleCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    borderAll,
	})
	styleTypeCell, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", TextRotation: 90, WrapText: true},
		Border:    borderAll,
	})
	styleScoreBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#8DB4E2"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll,
	})

	_ = f.SetColWidth(sheet, "A", "A", 18)
	_ = f.SetColWidth(sheet, "B", "B", 70)
	_ = f.SetColWidth(sheet, "C", "C", 14)

	_ = f.MergeCell(sheet, "A1", "C1")
	_ = f.SetRowHeight(sheet, 1, 32)
	_ = f.SetCellValue(sheet, "A1", "RESUME\nTES GAYA BELAJAR (VAK)")
	_ = f.SetCellStyle(sheet, "A1", "C1", styleHeaderGreen)

	// Identitas peserta — pakai data User (lebih authoritative dari res.TestName)
	nama := res.TestName
	nisnNip := ""
	idLabel := "NISN/NIP"
	kelas := ""
	jurusan := ""
	if user != nil {
		if strings.TrimSpace(user.NamaLengkap) != "" {
			nama = user.NamaLengkap
		}
		nisnNip = strings.TrimSpace(user.NISN)
		idLabel = "NISN"
		if nisnNip == "" && strings.TrimSpace(user.NIP) != "" {
			nisnNip = user.NIP
			idLabel = "NIP"
		}
		kelas = user.Kelas
		jurusan = user.Jurusan
	}

	_ = f.SetCellValue(sheet, "A3", "Nama")
	_ = f.SetCellValue(sheet, "B3", nama)
	_ = f.SetCellValue(sheet, "A4", idLabel)
	_ = f.SetCellValue(sheet, "B4", nisnNip)
	_ = f.SetCellValue(sheet, "A5", "Kelas")
	_ = f.SetCellValue(sheet, "B5", kelas)
	_ = f.SetCellValue(sheet, "A6", "Jurusan")
	_ = f.SetCellValue(sheet, "B6", jurusan)
	_ = f.SetCellValue(sheet, "A7", "Usia")
	_ = f.SetCellValue(sheet, "B7", res.TestAge)
	_ = f.SetCellValue(sheet, "A8", "Pendidikan")
	_ = f.SetCellValue(sheet, "B8", res.TestInstitution)
	_ = f.SetCellValue(sheet, "A9", "Jenis kelamin")
	_ = f.SetCellValue(sheet, "B9", res.TestGender)
	_ = f.SetCellValue(sheet, "A10", "Tanggal")
	_ = f.SetCellValue(sheet, "B10", res.TestDate.Format("02-01-2006"))
	_ = f.SetCellStyle(sheet, "A3", "A10", styleCenter)
	_ = f.SetCellStyle(sheet, "B3", "B10", styleBody)

	startRow := 12
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", startRow), "TIPE GAYA\nBELAJAR")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", startRow), "INTERPRETASI")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", startRow), "NILAI SKOR")
	_ = f.SetRowHeight(sheet, startRow, 28)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", startRow), fmt.Sprintf("C%d", startRow), styleTableHeader)

	writeRow := func(row int, tipe string, interp string, skor int) int {
		_ = f.SetRowHeight(sheet, row, 120)
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), strings.ToUpper(tipe))
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), strings.TrimSpace(interp))
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), skor)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), styleTypeCell)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), styleBody)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("C%d", row), fmt.Sprintf("C%d", row), styleScoreBlue)
		return row + 1
	}

	r := startRow + 1
	r = writeRow(r, "Visual", res.InterpretationVisual, res.ScoreVisual)
	r = writeRow(r, "Auditori", res.InterpretationAuditory, res.ScoreAuditory)
	_ = writeRow(r, "Kinestetik", res.InterpretationKinesthetic, res.ScoreKinesthetic)

	buf, werr := f.WriteToBuffer()
	if werr != nil {
		return nil, fmt.Errorf("failed to write xlsx: %v", werr)
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
// @router /api/admin/test-invitations/:id/delete [post]
func (c *PsychotestAdminController) DeleteInvitation() {
	ok, roleStr, sekolahFilter := c.verifyAdminOrSchool()
	if !ok {
		return
	}

	invID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil || invID <= 0 {
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

	// Dynamic check for school account ownership
	if roleStr == string(models.RoleSekolah) && sekolahFilter != "" {
		if inv.BatchId != nil {
			var batch models.TestBatch
			batch.Id = *inv.BatchId
			if err := o.Read(&batch); err == nil {
				if batch.Sekolah != "" && !strings.EqualFold(batch.Sekolah, sekolahFilter) {
					c.Ctx.Output.SetStatus(403)
					c.Data["json"] = PsychotestAdminResponse{
						Success: false,
						Message: "Akses ditolak: Undangan bukan milik sekolah Anda",
					}
					c.ServeJSON()
					return
				}
			}
		}
	}

	// Delete related attempts & sessions for clean deletion
	_, _ = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).Delete()

	_, _ = o.Raw("DELETE FROM papi_answers WHERE session_id IN (SELECT id FROM papi_sessions WHERE invitation_id = ?)", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM papi_sessions WHERE invitation_id = ?", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM rmib_answers WHERE session_id IN (SELECT id FROM rmib_sessions WHERE invitation_id = ?)", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM rmib_sessions WHERE invitation_id = ?", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM holland_answers WHERE invitation_id = ?", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM learning_style_answers WHERE invitation_id = ?", inv.Id).Exec()

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
		Message: "Peserta berhasil dihapus dari batch",
	}
	c.ServeJSON()
}

// @router /api/admin/test-invitations/:id/reset [post]
func (c *PsychotestAdminController) ResetInvitationProgress() {
	ok, roleStr, sekolahFilter := c.verifyAdminOrSchool()
	if !ok {
		return
	}

	invID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil || invID <= 0 {
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

	// Verify school ownership if school role
	if roleStr == string(models.RoleSekolah) && sekolahFilter != "" {
		if inv.BatchId != nil {
			var batch models.TestBatch
			batch.Id = *inv.BatchId
			if err := o.Read(&batch); err == nil {
				if batch.Sekolah != "" && !strings.EqualFold(batch.Sekolah, sekolahFilter) {
					c.Ctx.Output.SetStatus(403)
					c.Data["json"] = PsychotestAdminResponse{
						Success: false,
						Message: "Akses ditolak: Undangan bukan milik sekolah Anda",
					}
					c.ServeJSON()
					return
				}
			}
		}
	}

	// Delete all test results & sessions for this invitation so student can restart cleanly
	_, _ = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.ISTAnswer)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).Delete()
	_, _ = o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).Delete()

	_, _ = o.Raw("DELETE FROM papi_answers WHERE session_id IN (SELECT id FROM papi_sessions WHERE invitation_id = ?)", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM papi_sessions WHERE invitation_id = ?", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM rmib_answers WHERE session_id IN (SELECT id FROM rmib_sessions WHERE invitation_id = ?)", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM rmib_sessions WHERE invitation_id = ?", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM holland_answers WHERE invitation_id = ?", inv.Id).Exec()
	_, _ = o.Raw("DELETE FROM learning_style_answers WHERE invitation_id = ?", inv.Id).Exec()

	// Reset invitation status to pending
	_, err = o.Raw("UPDATE test_invitations SET status = 'pending', used_at = NULL WHERE id = ?", inv.Id).Exec()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{
			Success: false,
			Message: "Gagal mereset status undangan",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: true,
		Message: "Berhasil mereset tes peserta. Peserta dapat memulai tes kembali dari awal.",
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
	case "send_code":
		// Kirim KODE (token) ke semua undangan terpilih melalui channel sesuai konfigurasi batch.
		var invs []models.TestInvitation
		_, _ = o.QueryTable(new(models.TestInvitation)).Filter("Id__in", payload.IDs).All(&invs)
		sent := 0
		var errMsgs []string
		for i := range invs {
			if i > 0 {
				time.Sleep(350 * time.Millisecond) // Jeda 350ms antarpengiriman agar Gmail SMTP tidak terkena 454 rate limit
			}
			if err := dispatchSendCode(&invs[i]); err == nil {
				sent++
			} else {
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", invs[i].Email, err.Error()))
			}
		}
		msg := fmt.Sprintf("Kode (token) berhasil dikirim untuk %d dari %d undangan terpilih.", sent, len(invs))
		if sent == 0 && len(errMsgs) > 0 {
			msg = fmt.Sprintf("Gagal mengirim kode: %s", strings.Join(errMsgs, "; "))
		}
		c.Data["json"] = PsychotestAdminResponse{
			Success: sent > 0,
			Message: msg,
			Data:    map[string]interface{}{"sent": sent, "total": len(invs)},
		}
		c.ServeJSON()
		return
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

// dispatchSendCode mengirim kode (token) tes ke 1 invitation lewat channel
// yang aktif di batch (email / WA). Browser notif tidak ikut karena fungsinya
// untuk memberitahu pengumuman, bukan kirim kode.
//
// Untuk channel WhatsApp, nomor diambil berurutan dari:
//  1. inv.Phone (yang dimasukkan operator saat CreateInvitations), kalau ada
//  2. users.no_handphone (jika invitation sudah ter-link ke user)
//  3. cari user by email (jika belum ter-link), lalu auto-link & ambil HP-nya
func dispatchSendCode(inv *models.TestInvitation) error {
	if inv == nil || inv.Id == 0 {
		return fmt.Errorf("invitation kosong")
	}
	if inv.BatchId == nil {
		return fmt.Errorf("invitation tidak terkait batch")
	}
	o := orm.NewOrm()

	batch := models.TestBatch{Id: *inv.BatchId}
	if err := o.Read(&batch); err != nil {
		return fmt.Errorf("batch tidak ditemukan: %w", err)
	}

	displayName := ""
	phoneFromUser := ""

	// Resolve user: kalau sudah ter-link gunakan UserId; kalau belum, coba lookup via email
	// supaya undangan lama (yang dibuat sebelum user registrasi) tetap bisa diisi nomor HP-nya.
	if inv.UserId != nil && *inv.UserId != 0 {
		var u models.User
		u.Id = *inv.UserId
		if err := o.Read(&u); err == nil {
			displayName = u.NamaLengkap
			phoneFromUser = u.NoHandphone
		}
	} else if inv.Email != "" {
		var u models.User
		u.Email = inv.Email
		if err := o.Read(&u, "Email"); err == nil {
			displayName = u.NamaLengkap
			phoneFromUser = u.NoHandphone
			// auto-link untuk konsistensi data
			inv.UserId = &u.Id
			_, _ = o.Update(inv, "UserId")
		}
	}

	// Fallback: kalau inv.Phone kosong tapi user punya nomor, pakai nomor user
	// dan simpan ke invitation supaya konsisten dengan history.
	if strings.TrimSpace(inv.Phone) == "" && strings.TrimSpace(phoneFromUser) != "" {
		inv.Phone = strings.TrimSpace(phoneFromUser)
		_, _ = o.Update(inv, "Phone")
	}

	phoneToUse := strings.TrimSpace(inv.Phone)
	if phoneToUse == "" {
		phoneToUse = strings.TrimSpace(phoneFromUser)
	}

	logs.Info("dispatchSendCode invID=%d email=%q phone=%q via_email=%v via_wa=%v",
		inv.Id, inv.Email, phoneToUse, batch.SendViaEmail, batch.SendViaWhatsApp)

	sendEmail := strings.TrimSpace(inv.Email) != ""
	// Kirim via WA jika WA di-enable di batch ATAU jika nomor HP tersedia saat kirim kode
	sendWA := (batch.SendViaWhatsApp || phoneToUse != "") && phoneToUse != ""

	if !sendEmail && !sendWA {
		return fmt.Errorf("tidak ada channel pengiriman (email/WA) aktif atau nomor HP tidak tersedia untuk %s", inv.Email)
	}

	if sendEmail {
		go sendInvitationCodeEmail(&batch, displayName, inv.Email, inv)
	}
	if sendWA {
		go sendInvitationCodeWA(&batch, displayName, phoneToUse, inv)
	}
	// Buat notifikasi in-app saat kode/token dikirim (jika invitation sudah
	// ter-link ke akun user).
	if inv.UserId != nil {
		batchName := strings.TrimSpace(batch.Name)
		if batchName == "" {
			batchName = fmt.Sprintf("#%d", batch.Id)
		}
		go utils.SendInvitationCodeSentNotification(inv.UserId, batchName)
	}
	return nil
}

// @router /api/admin/test-invitations/:id/send-code [post]
// Kirim TOKEN ke 1 peserta lewat channel sesuai pengaturan batch.
func (c *PsychotestAdminController) SendCode() {
	if !c.verifyAdmin() {
		return
	}
	invID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = PsychotestAdminResponse{Success: false, Message: "ID undangan tidak valid"}
		c.ServeJSON()
		return
	}
	o := orm.NewOrm()
	var inv models.TestInvitation
	if err := o.QueryTable(new(models.TestInvitation)).Filter("Id", invID).One(&inv); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = PsychotestAdminResponse{Success: false, Message: "Undangan tidak ditemukan"}
		c.ServeJSON()
		return
	}
	if err := dispatchSendCode(&inv); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = PsychotestAdminResponse{Success: false, Message: err.Error()}
		c.ServeJSON()
		return
	}
	c.Data["json"] = PsychotestAdminResponse{Success: true, Message: "Kode tes sedang dikirim"}
	c.ServeJSON()
}

// @router /api/admin/test-invitations/push-all-pending [post]
// Mengirimkan kode token ke SELURUH undangan bertipe 'pending' yang belum menerima token.
func (c *PsychotestAdminController) PushAllPendingCodes() {
	if !c.verifyAdmin() {
		return
	}
	o := orm.NewOrm()
	var invs []models.TestInvitation
	_, err := o.QueryTable(new(models.TestInvitation)).Filter("Status", models.StatusInvitationPending).All(&invs)
	if err != nil || len(invs) == 0 {
		c.Data["json"] = PsychotestAdminResponse{Success: true, Message: "Tidak ada undangan pending yang ditemukan"}
		c.ServeJSON()
		return
	}

	sent := 0
	var errMsgs []string
	for i := range invs {
		if i > 0 {
			time.Sleep(350 * time.Millisecond) // Jeda 350ms antarpengiriman agar Gmail SMTP tidak rate limit 454
		}
		if err := dispatchSendCode(&invs[i]); err == nil {
			sent++
		} else {
			errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", invs[i].Email, err.Error()))
		}
	}

	msg := fmt.Sprintf("Kode token berhasil di-push ke %d dari %d undangan pending.", sent, len(invs))
	if sent == 0 && len(errMsgs) > 0 {
		msg = fmt.Sprintf("Gagal mengirim kode: %s", strings.Join(errMsgs, "; "))
	}

	c.Data["json"] = PsychotestAdminResponse{
		Success: sent > 0,
		Message: msg,
		Data:    map[string]interface{}{"sent": sent, "total": len(invs)},
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
		Name                string `json:"name"`
		Institution         string `json:"institution"`
		TahunAjaran         string `json:"tahun_ajaran"`
		Sekolah             string `json:"sekolah"`
		Status              string `json:"status"` // active, archived
		PurposeDetail       string `json:"purpose_detail"`
		EnableIST           *bool  `json:"enable_ist"`
		EnableHolland       *bool  `json:"enable_holland"`
		EnableLearningStyle *bool  `json:"enable_learning_style"`
		EnableKraepelin     *bool  `json:"enable_kraepelin"`
		EnableRMIB          *bool  `json:"enable_rmib"`
		EnablePAPI          *bool  `json:"enable_papi"`
		TestOrder           *string `json:"test_order"`
		SendViaEmail        *bool  `json:"send_via_email"`
		SendViaBrowser      *bool  `json:"send_via_browser"`
		SendViaWhatsApp     *bool  `json:"send_via_whatsapp"`
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
	if payload.TahunAjaran != "" && payload.TahunAjaran != batch.TahunAjaran {
		batch.TahunAjaran = payload.TahunAjaran
		fields = append(fields, "TahunAjaran")
	}
	if payload.Sekolah != "" && payload.Sekolah != batch.Sekolah {
		batch.Sekolah = payload.Sekolah
		fields = append(fields, "Sekolah")
	}
	if payload.PurposeDetail != "" && payload.PurposeDetail != batch.PurposeDetail {
		batch.PurposeDetail = payload.PurposeDetail
		fields = append(fields, "PurposeDetail")
	}
	if payload.EnableIST != nil && batch.EnableIST != *payload.EnableIST {
		batch.EnableIST = *payload.EnableIST
		fields = append(fields, "EnableIST")
	}
	if payload.EnableHolland != nil && batch.EnableHolland != *payload.EnableHolland {
		batch.EnableHolland = *payload.EnableHolland
		fields = append(fields, "EnableHolland")
	}
	if payload.EnableLearningStyle != nil && batch.EnableLearningStyle != *payload.EnableLearningStyle {
		batch.EnableLearningStyle = *payload.EnableLearningStyle
		fields = append(fields, "EnableLearningStyle")
	}
	if payload.EnableKraepelin != nil && batch.EnableKraepelin != *payload.EnableKraepelin {
		batch.EnableKraepelin = *payload.EnableKraepelin
		fields = append(fields, "EnableKraepelin")
	}
	if payload.EnableRMIB != nil && batch.EnableRMIB != *payload.EnableRMIB {
		batch.EnableRMIB = *payload.EnableRMIB
		fields = append(fields, "EnableRMIB")
	}
	if payload.EnablePAPI != nil && batch.EnablePAPI != *payload.EnablePAPI {
		batch.EnablePAPI = *payload.EnablePAPI
		fields = append(fields, "EnablePAPI")
	}
	if payload.TestOrder != nil && batch.TestOrder != *payload.TestOrder {
		batch.TestOrder = *payload.TestOrder
		fields = append(fields, "TestOrder")
	}
	if payload.SendViaEmail != nil && batch.SendViaEmail != *payload.SendViaEmail {
		batch.SendViaEmail = *payload.SendViaEmail
		fields = append(fields, "SendViaEmail")
	}
	if payload.SendViaBrowser != nil && batch.SendViaBrowser != *payload.SendViaBrowser {
		batch.SendViaBrowser = *payload.SendViaBrowser
		fields = append(fields, "SendViaBrowser")
	}
	if payload.SendViaWhatsApp != nil && batch.SendViaWhatsApp != *payload.SendViaWhatsApp {
		batch.SendViaWhatsApp = *payload.SendViaWhatsApp
		fields = append(fields, "SendViaWhatsApp")
	}

	// Enforce at least one test enabled when toggles are provided.
	if payload.EnableIST != nil || payload.EnableHolland != nil || payload.EnableLearningStyle != nil || payload.EnableKraepelin != nil || payload.EnableRMIB != nil || payload.EnablePAPI != nil {
		enabledCount := 0
		if batch.EnableIST {
			enabledCount++
		}
		if batch.EnableHolland {
			enabledCount++
		}
		if batch.EnableLearningStyle {
			enabledCount++
		}
		if batch.EnableKraepelin {
			enabledCount++
		}
		if batch.EnableRMIB {
			enabledCount++
		}
		if batch.EnablePAPI {
			enabledCount++
		}
		if enabledCount < 1 {
			c.Ctx.Output.SetStatus(400)
			c.Data["json"] = PsychotestAdminResponse{
				Success: false,
				Message: "Batch harus mengaktifkan minimal satu jenis tes.",
			}
			c.ServeJSON()
			return
		}
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

// ExportSingleResultZIP exports a single result (PDF Page 1 and Excel Page 2) into a single ZIP.
// @router /api/results/export-zip [get]
func (c *PsychotestAdminController) ExportSingleResultZIP() {
	o := orm.NewOrm()

	writeErr := func(status int, msg string) {
		c.Ctx.Output.SetStatus(status)
		c.Ctx.Output.Header("Content-Type", "text/plain; charset=utf-8")
		_, _ = c.Ctx.ResponseWriter.Write([]byte(msg))
	}

	// 1. Authenticate user and check invitation access
	invIDStr := strings.TrimSpace(c.GetString("invId"))
	if invIDStr == "" {
		writeErr(400, "Parameter invId wajib diisi")
		return
	}
	invID, err := strconv.Atoi(invIDStr)
	if err != nil || invID <= 0 {
		writeErr(400, "Parameter invId tidak valid")
		return
	}

	var inv models.TestInvitation
	inv.Id = invID
	if err := o.Read(&inv); err != nil {
		writeErr(404, "Undangan tidak ditemukan")
		return
	}

	// Check access permissions
	allowed, err := c.checkResultAccessInternal(o, &inv)
	if !allowed || err != nil {
		msg := "Akses ditolak"
		if err != nil {
			msg = err.Error()
		}
		writeErr(403, msg)
		return
	}

	// Resolve target test type
	testType := strings.ToLower(strings.TrimSpace(c.GetString("test")))

	// 2. Fetch User & Batch data
	var user models.User
	if inv.UserId != nil {
		user.Id = *inv.UserId
		_ = o.Read(&user)
	}
	var batch models.TestBatch
	if inv.BatchId != nil {
		batch.Id = *inv.BatchId
		_ = o.Read(&batch)
	}

	// Determine which test types we should export
	var testTypesToExport []string
	isKeseluruhan := testType == "keseluruhan" || testType == ""

	if isKeseluruhan {
		if batch.EnableIST {
			var res models.ISTResult
			if o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res) == nil && res.Id > 0 {
				testTypesToExport = append(testTypesToExport, "ist")
			}
		}
		if batch.EnableHolland {
			var res models.HollandResult
			if o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&res) == nil && res.Id > 0 {
				testTypesToExport = append(testTypesToExport, "holland")
			}
		}
		if batch.EnableLearningStyle {
			var res models.LearningStyleResult
			if o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res) == nil && res.Id > 0 {
				testTypesToExport = append(testTypesToExport, "learning_style")
			}
		}
		if batch.EnableRMIB {
			var res models.RMIBResult
			if o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).One(&res) == nil && res.Id > 0 {
				testTypesToExport = append(testTypesToExport, "rmib")
			}
		}
		if batch.EnablePAPI {
			var res models.PAPIResult
			if o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&res) == nil && res.Id > 0 {
				testTypesToExport = append(testTypesToExport, "papi")
			}
		}
		if batch.EnableKraepelin {
			var res models.KraepelinAttempt
			if o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&res) == nil && res.Id > 0 {
				testTypesToExport = append(testTypesToExport, "kraepelin")
			}
		}

		// Fallback to first enabled if none completed yet
		if len(testTypesToExport) == 0 {
			if batch.EnableIST {
				testTypesToExport = append(testTypesToExport, "ist")
			} else if batch.EnableHolland {
				testTypesToExport = append(testTypesToExport, "holland")
			} else if batch.EnableLearningStyle {
				testTypesToExport = append(testTypesToExport, "learning_style")
			} else if batch.EnableRMIB {
				testTypesToExport = append(testTypesToExport, "rmib")
			} else if batch.EnablePAPI {
				testTypesToExport = append(testTypesToExport, "papi")
			} else if batch.EnableKraepelin {
				testTypesToExport = append(testTypesToExport, "kraepelin")
			} else {
				testTypesToExport = append(testTypesToExport, "ist")
			}
		}
	} else {
		testTypesToExport = []string{testType}
	}

	// 6. Build ZIP archive
	zipBuf := new(bytes.Buffer)
	zw := zip.NewWriter(zipBuf)

	studentRealName := user.NamaLengkap
	if studentRealName == "" {
		studentRealName = strings.Split(inv.Email, "@")[0]
	}
	studentSafe := sanitizeFilename(studentRealName)
	if studentSafe == "" {
		studentSafe = fmt.Sprintf("Siswa_%d", inv.Id)
	}

	var firstFriendlyName string

	for _, tType := range testTypesToExport {
		var excelBytes []byte
		var resultData interface{}
		var friendlyTestName string
		var fetchErr error

		switch tType {
		case "ist":
			friendlyTestName = "IST"
			var res models.ISTResult
			fetchErr = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
			if fetchErr == nil && res.Id > 0 {
				age := 0
				if user.TanggalLahir != nil {
					age = utils.AgeYears(*user.TanggalLahir, time.Now())
				}
				if age > 0 {
					if updatedRes, err := utils.EnsureISTStandardAndIQScores(o, &res, age); err == nil && updatedRes != nil {
						res = *updatedRes
					}
				}
				resultData = res
				excelBytes, fetchErr = buildISTResultXLSX(o, &batch, &inv, &user)
			}
		case "holland":
			friendlyTestName = "Holland"
			var res models.HollandResult
			fetchErr = o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&res)
			if fetchErr == nil && res.Id > 0 {
				resultData = res
				excelBytes, fetchErr = buildHollandResultXLSX(o, &batch, &inv, &user)
			}
		case "learning_style", "vak":
			friendlyTestName = "Gaya_Belajar"
			var res models.LearningStyleResult
			fetchErr = o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res)
			if fetchErr == nil && res.Id > 0 {
				resultData = res
				excelBytes, fetchErr = buildLearningStyleResultXLSX(o, &batch, &inv, &user)
			}
		case "kraepelin":
			friendlyTestName = "Kraepelin"
			var res models.KraepelinAttempt
			fetchErr = o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&res)
			if fetchErr == nil && res.Id > 0 {
				resultData = res
				excelBytes, fetchErr = buildKraepelinResultXLSX(o, &batch, &inv, &user)
			}
		case "rmib":
			friendlyTestName = "RMIB"
			var res models.RMIBResult
			fetchErr = o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).One(&res)
			if fetchErr == nil && res.Id > 0 {
				resultData = res
				excelBytes, fetchErr = buildRMIBResultXLSX(o, &batch, &inv, &user)
			}
		case "papi":
			friendlyTestName = "PAPI"
			var res models.PAPIResult
			fetchErr = o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&res)
			if fetchErr == nil && res.Id > 0 {
				resultData = res
				excelBytes, fetchErr = buildPAPIResultXLSX(o, &batch, &inv, &user)
			}
		default:
			if !isKeseluruhan {
				writeErr(400, "Tipe alat tes tidak dikenali")
				return
			}
			continue
		}

		if firstFriendlyName == "" {
			firstFriendlyName = friendlyTestName
		}

		if fetchErr != nil {
			if !isKeseluruhan {
				writeErr(404, fmt.Sprintf("Hasil %s belum tersedia", friendlyTestName))
				return
			}
			continue
		}

		if len(excelBytes) == 0 {
			if !isKeseluruhan {
				writeErr(500, fmt.Sprintf("Gagal menyusun laporan excel untuk %s", friendlyTestName))
				return
			}
			continue
		}

		// Retrieve or generate AI Summary
		summaryData, sumErr := GetOrGenerateTestSummaryInternal(o, friendlyTestName, resultData, studentRealName)
		if sumErr != nil {
			logs.Error("Single export: failed to fetch AI summary for %s: %v", friendlyTestName, sumErr)
			summaryData = map[string]interface{}{
				"summary":           "Laporan hasil evaluasi psikologis.",
				"kekuatan":          []interface{}{"Mandiri", "Disiplin"},
				"area_pengembangan": []interface{}{"Perlu mengoptimalkan komunikasi interpersonal"},
				"rekomendasi_siswa": []interface{}{"Pertahankan motivasi belajar"},
				"rekomendasi_ortu":  []interface{}{"Dukung minat karir anak"},
				"rekomendasi_bk":    []interface{}{"Bimbing pilihan studi karir anak"},
			}
		}

		// Generate Professional PDF
		pdfBytes, pdfErr := c.generateProfessionalPDFReport(o, &inv, &user, &batch, friendlyTestName, summaryData, resultData)
		if pdfErr != nil {
			if !isKeseluruhan {
				writeErr(500, fmt.Sprintf("Gagal menyusun laporan PDF untuk %s: %v", friendlyTestName, pdfErr))
				return
			}
			continue
		}

		// Add PDF file inside ZIP
		pdfName := fmt.Sprintf("Laporan_Hasil_%s_%s.pdf", friendlyTestName, studentSafe)
		wPdf, err := zw.Create(pdfName)
		if err == nil {
			_, _ = wPdf.Write(pdfBytes)
		}

		// Add Excel file inside ZIP
		xlsxName := fmt.Sprintf("Psikogram_Hasil_%s_%s.xlsx", friendlyTestName, studentSafe)
		wXlsx, err := zw.Create(xlsxName)
		if err == nil {
			_, _ = wXlsx.Write(excelBytes)
		}
	}

	// 5b. Generate Comprehensive PDF
	comprehensivePdfBytes, err := c.generateComprehensivePDFReport(o, &inv, &user, &batch)
	if err != nil {
		logs.Error("Single export: failed to generate comprehensive PDF: %v", err)
	}
	if len(comprehensivePdfBytes) > 0 {
		compPdfName := fmt.Sprintf("Laporan_Keseluruhan_Hasil_Asesmen_%s.pdf", studentSafe)
		wCompPdf, err := zw.Create(compPdfName)
		if err == nil {
			_, _ = wCompPdf.Write(comprehensivePdfBytes)
		}
	}

	_ = zw.Close()

	// 7. Write ZIP response
	var zipName string
	if isKeseluruhan {
		zipName = fmt.Sprintf("%s_Hasil_Lengkap.zip", studentSafe)
	} else {
		zipName = fmt.Sprintf("%s_%s_Lengkap.zip", studentSafe, firstFriendlyName)
	}
	c.Ctx.Output.Header("Content-Type", "application/zip")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+zipName+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(zipBuf.Bytes())
}

// checkResultAccessInternal determines if the session user has right to download
func (c *PsychotestAdminController) checkResultAccessInternal(o orm.Ormer, inv *models.TestInvitation) (bool, error) {
	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		return false, fmt.Errorf("Silakan login terlebih dahulu")
	}
	userID := sessionUser.(int)

	userRole := c.GetSession("user_role")
	roleStr, _ := userRole.(string)

	// Admin is always allowed
	if roleStr == string(models.RoleAdmin) {
		return true, nil
	}

	// Counselor is allowed if batch school matches counselor's school
	if roleStr == string(models.RoleSekolah) {
		var sekolah string
		u := models.User{Id: userID}
		if err := o.Read(&u); err == nil {
			sekolah = u.Sekolah
		}
		if inv.BatchId != nil {
			var batch models.TestBatch
			batch.Id = *inv.BatchId
			if err := o.Read(&batch); err == nil {
				if strings.EqualFold(batch.Sekolah, sekolah) {
					return true, nil
				}
			}
		}
		return false, fmt.Errorf("Akses ditolak: Anda tidak memiliki akses ke data undangan ini")
	}

	// Student is allowed if they own the invitation
	if inv.UserId != nil && *inv.UserId == userID {
		return true, nil
	}
	var loggedInUser models.User
	loggedInUser.Id = userID
	if err := o.Read(&loggedInUser); err == nil {
		if strings.TrimSpace(inv.Email) != "" && strings.EqualFold(strings.TrimSpace(inv.Email), strings.TrimSpace(loggedInUser.Email)) {
			return true, nil
		}
	}

	return false, fmt.Errorf("Anda tidak memiliki akses ke hasil ini")
}

// Score-to-category classification helpers for Go PDF reports
func getCategoryFromSW(sw int) string {
	if sw >= 120 {
		return "Sangat Baik"
	}
	if sw >= 110 {
		return "Baik"
	}
	if sw >= 100 {
		return "Cukup Baik"
	}
	if sw >= 90 {
		return "Cukup"
	}
	if sw >= 80 {
		return "Kurang"
	}
	return "Kurang Sekali"
}

func getHollandCategory(score int) string {
	if score >= 25 {
		return "Sangat Baik"
	}
	if score >= 18 {
		return "Baik"
	}
	if score >= 12 {
		return "Cukup Baik"
	}
	if score >= 6 {
		return "Cukup"
	}
	return "Kurang"
}

func getVAKCategory(score int) string {
	if score >= 15 {
		return "Sangat Baik"
	}
	if score >= 12 {
		return "Baik"
	}
	if score >= 8 {
		return "Cukup Baik"
	}
	if score >= 5 {
		return "Cukup"
	}
	return "Kurang"
}

func getRMIBCategory(rank int) string {
	if rank <= 3 {
		return "Sangat Baik"
	}
	if rank <= 6 {
		return "Baik"
	}
	if rank <= 9 {
		return "Cukup Baik"
	}
	return "Kurang"
}

func getPAPICategory(score int) string {
	if score >= 8 {
		return "Sangat Baik"
	}
	if score >= 6 {
		return "Baik"
	}
	if score >= 4 {
		return "Cukup Baik"
	}
	if score == 3 {
		return "Cukup"
	}
	return "Kurang"
}

func getKraepelinCategory(score float64, aspectType string) string {
	if aspectType == "kecepatan" || aspectType == "ketahanan" {
		if score >= 180 {
			return "Sangat Baik"
		}
		if score >= 130 {
			return "Baik"
		}
		if score >= 90 {
			return "Cukup Baik"
		}
		if score >= 60 {
			return "Cukup"
		}
		return "Kurang"
	} else if aspectType == "ketelitian" {
		if score <= 5 {
			return "Sangat Baik"
		}
		if score <= 15 {
			return "Baik"
		}
		if score <= 25 {
			return "Cukup Baik"
		}
		if score <= 40 {
			return "Cukup"
		}
		return "Kurang"
	} else if aspectType == "konsentrasi" {
		if score >= 95 {
			return "Sangat Baik"
		}
		if score >= 85 {
			return "Baik"
		}
		if score >= 75 {
			return "Cukup Baik"
		}
		if score >= 60 {
			return "Cukup"
		}
		return "Kurang"
	}
	return "Kurang"
}

// generateProfessionalPDFReport renders the 1st page narrative report professionally using gofpdf
func (c *PsychotestAdminController) generateProfessionalPDFReport(o orm.Ormer, inv *models.TestInvitation, user *models.User, batch *models.TestBatch, testType string, summaryData map[string]interface{}, resultData interface{}) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Laporan Hasil Tes "+testType, false)
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Font Setup
	pdf.SetFont("Arial", "B", 15)
	pdf.CellFormat(0, 8, "LAPORAN HASIL EVALUASI PSIKOLOGIS", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 5, "ALAT TES: "+strings.ToUpper(testType), "", 1, "C", false, 0, "")
	pdf.Ln(6)

	// --- 1. IDENTITAS PESERTA ---
	pdf.SetFont("Arial", "B", 10.5)
	pdf.CellFormat(0, 5, "IDENTITAS PESERTA", "", 1, "L", false, 0, "")
	pdf.SetLineWidth(0.3)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(2)

	pdf.SetFont("Arial", "", 9.5)
	nama := user.NamaLengkap
	if nama == "" {
		nama = inv.Email
	}
	gender := "-"
	if user.JenisKelamin == models.GenderLakiLaki {
		gender = "Laki-laki"
	} else if user.JenisKelamin == models.GenderPerempuan {
		gender = "Perempuan"
	}
	
	dob := "-"
	if user.TanggalLahir != nil {
		dob = user.TanggalLahir.Format("02 January 2006")
	}
	pob := user.TempatLahir
	if pob == "" {
		pob = "-"
	}
	pobDob := pob + ", " + dob
	if pob == "-" && dob == "-" {
		pobDob = "-"
	}

	sekolah := batch.Sekolah
	if sekolah == "" {
		sekolah = user.Sekolah
	}
	if sekolah == "" {
		sekolah = batch.Institution
	}
	if sekolah == "" {
		sekolah = "-"
	}

	kelasJurusan := "-"
	if user.Kelas != "" || user.Jurusan != "" {
		kelasJurusan = user.Kelas + " / " + user.Jurusan
	} else if batch.Kelas != "" || batch.Jurusan != "" {
		kelasJurusan = batch.Kelas + " / " + batch.Jurusan
	}

	tglPeriksa := inv.UsedAt.Format("02 January 2006")
	if inv.UsedAt.IsZero() {
		tglPeriksa = inv.CreatedAt.Format("02 January 2006")
	}

	drawRow := func(label, val string) {
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(45, 5, label, "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(5, 5, ":", "", 0, "L", false, 0, "")
		pdf.CellFormat(0, 5, val, "", 1, "L", false, 0, "")
	}

	drawRow("Nama", nama)
	drawRow("Jenis Kelamin", gender)
	drawRow("Tempat/Tanggal Lahir", pobDob)
	drawRow("Sekolah", sekolah)
	drawRow("Kelas / Jurusan", kelasJurusan)
	drawRow("Tanggal Pemeriksaan", tglPeriksa)
	pdf.Ln(4)

	// --- 2. TUJUAN PEMERIKSAAN ---
	pdf.SetFont("Arial", "B", 10.5)
	pdf.CellFormat(0, 5, "TUJUAN PEMERIKSAAN", "", 1, "L", false, 0, "")
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(2)

	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(0, 4.5, "Pemeriksaan psikologis untuk memperoleh gambaran mengenai potensi intelektual, minat, serta karakteristik kepribadian peserta didik sebagai bahan pertimbangan dalam pengembangan diri dan perencanaan karier.", "", "L", false)
	pdf.Ln(4)

	// --- 3. HASIL PEMERIKSAAN ---
	pdf.SetFillColor(0, 0, 0)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Arial", "B", 10.5)
	pdf.CellFormat(0, 7, "  HASIL PEMERIKSAAN", "0", 1, "L", true, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(3)

	// 1. KEMAMPUAN INTELEKTUAL
	pdf.SetFont("Arial", "B", 9.5)
	pdf.CellFormat(0, 4.5, "1. KEMAMPUAN INTELEKTUAL", "", 1, "L", false, 0, "")
	pdf.Ln(1)

	pdf.SetFont("Arial", "", 9)
	if testType == "IST" {
		if ist, ok := resultData.(models.ISTResult); ok {
			pdf.SetFont("Arial", "B", 9)
			pdf.CellFormat(30, 4.5, "Skor IQ", "", 0, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 4.5, fmt.Sprintf("%d", ist.IQ), "", 1, "L", false, 0, "")

			pdf.SetFont("Arial", "B", 9)
			pdf.CellFormat(30, 4.5, "Kategori", "", 0, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 4.5, ist.IQCategory, "", 1, "L", false, 0, "")
			pdf.Ln(2.5)

			pdf.SetFont("Arial", "I", 9)
			pdf.CellFormat(0, 4.5, "Interpretasi:", "", 1, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			summaryText, _ := summaryData["summary"].(string)
			if summaryText == "" {
				summaryText = fmt.Sprintf("Peserta didik memiliki potensi kognitif di kategori %s. Potensi ini menggambarkan kapasitas umum peserta didik dalam memahami masalah, melakukan penalaran terstruktur, serta memproses informasi akademis secara memadai.", ist.IQCategory)
			}
			pdf.MultiCell(0, 4.5, summaryText, "", "L", false)
		}
	} else if testType == "Kraepelin" {
		if krp, ok := resultData.(models.KraepelinAttempt); ok {
			tot := krp.TotalCorrect + krp.TotalErrors + krp.TotalSkipped
			acc := 0.0
			if tot > 0 {
				acc = float64(krp.TotalCorrect) / float64(tot) * 100.0
			}

			pdf.SetFont("Arial", "B", 9)
			pdf.CellFormat(45, 4.5, "Total Jawaban Benar", "", 0, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 4.5, fmt.Sprintf("%d", krp.TotalCorrect), "", 1, "L", false, 0, "")

			pdf.SetFont("Arial", "B", 9)
			pdf.CellFormat(45, 4.5, "Total Jawaban Salah", "", 0, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 4.5, fmt.Sprintf("%d", krp.TotalErrors), "", 1, "L", false, 0, "")

			pdf.SetFont("Arial", "B", 9)
			pdf.CellFormat(45, 4.5, "Total Baris Dilewati", "", 0, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 4.5, fmt.Sprintf("%d", krp.TotalSkipped), "", 1, "L", false, 0, "")

			pdf.SetFont("Arial", "B", 9)
			pdf.CellFormat(45, 4.5, "Tingkat Akurasi Kerja", "", 0, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
			pdf.CellFormat(0, 4.5, fmt.Sprintf("%.1f%%", acc), "", 1, "L", false, 0, "")
			pdf.Ln(2.5)

			pdf.SetFont("Arial", "I", 9)
			pdf.CellFormat(0, 4.5, "Interpretasi:", "", 1, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			summaryText, _ := summaryData["summary"].(string)
			pdf.MultiCell(0, 4.5, summaryText, "", "L", false)
		}
	} else {
		pdf.CellFormat(0, 4.5, "Potensi kecerdasan umum tidak diukur secara langsung dalam alat tes ini.", "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)

	// 2. SPESIFIK HASIL TES (minat/kepribadian/gaya belajar/intelegensi/kraepelin)
	if testType == "Holland" {
		if hol, ok := resultData.(models.HollandResult); ok {
			pdf.SetFont("Arial", "B", 9.5)
			pdf.CellFormat(0, 4.5, "2. MINAT KARIER DOMINAN (RIASEC)", "", 1, "L", false, 0, "")
			pdf.Ln(1)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(0, 4.5, fmt.Sprintf("Kode RIASEC Dominan (Top 3): %s", hol.Code), "", 1, "L", false, 0, "")
			pdf.Ln(1.5)

			// Table Header
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(15, 5.5, "No", "1", 0, "C", true, 0, "")
			pdf.CellFormat(115, 5.5, "Aspek Minat Holland", "1", 0, "L", true, 0, "")
			pdf.CellFormat(50, 5.5, "Kategori", "1", 1, "C", true, 0, "")

			aspects := []struct {
				No   int
				Name string
				Cat  string
			}{
				{1, "Realistic (R)", getHollandCategory(hol.ScoreR)},
				{2, "Investigative (I)", getHollandCategory(hol.ScoreI)},
				{3, "Artistic (A)", getHollandCategory(hol.ScoreA)},
				{4, "Social (S)", getHollandCategory(hol.ScoreS)},
				{5, "Enterprising (E)", getHollandCategory(hol.ScoreE)},
				{6, "Conventional (C)", getHollandCategory(hol.ScoreC)},
			}

			pdf.SetFont("Arial", "", 8.5)
			for _, a := range aspects {
				pdf.CellFormat(15, 5, fmt.Sprintf("%d", a.No), "1", 0, "C", false, 0, "")
				pdf.CellFormat(115, 5, a.Name, "1", 0, "L", false, 0, "")
				pdf.CellFormat(50, 5, a.Cat, "1", 1, "C", false, 0, "")
			}
			pdf.Ln(2.5)

			detailText, _ := summaryData["interpretasi_detail"].(string)
			if detailText == "" {
				detailText, _ = summaryData["summary"].(string)
			}
			if detailText != "" {
				pdf.SetFont("Arial", "I", 9)
				pdf.CellFormat(0, 4.5, "Interpretasi Minat Karir:", "", 1, "L", false, 0, "")
				pdf.SetFont("Arial", "", 9)
				pdf.MultiCell(0, 4.5, detailText, "", "L", false)
			}
		}
	} else if testType == "RMIB" {
		if rmib, ok := resultData.(models.RMIBResult); ok {
			pdf.SetFont("Arial", "B", 9.5)
			pdf.CellFormat(0, 4.5, "2. MINAT KARIER DOMINAN (RMIB)", "", 1, "L", false, 0, "")
			pdf.Ln(1)
			pdf.SetFont("Arial", "", 9)

			type entry struct {
				Label string `json:"label"`
				Score int    `json:"score"`
				Rank  int    `json:"rank"`
			}
			parsed := map[string]entry{}
			_ = json.Unmarshal([]byte(rmib.ResultJSON), &parsed)

			codes := []string{"OUT", "MEC", "COMP", "SCI", "PERS", "AEST", "MUS", "LIT", "SOC", "CLER", "PRAC", "MED"}
			labels := map[string]string{
				"OUT": "Outdoor", "MEC": "Mechanical", "COMP": "Computational", "SCI": "Scientific",
				"PERS": "Personal Contact", "AEST": "Aesthetic", "MUS": "Musical", "LIT": "Literary",
				"SOC": "Social Service", "CLER": "Clerical", "PRAC": "Practical", "MED": "Medical",
			}

			type aspectRow struct {
				Name string
				Rank int
				Cat  string
			}
			var aspects []aspectRow
			for _, c := range codes {
				item := parsed[c]
				rank := item.Rank
				if rank == 0 {
					rank = 12
				}
				aspects = append(aspects, aspectRow{
					Name: labels[c] + " (" + c + ")",
					Rank: rank,
					Cat:  getRMIBCategory(rank),
				})
			}
			sort.Slice(aspects, func(i, j int) bool {
				return aspects[i].Rank < aspects[j].Rank
			})

			var top3 []string
			for idx, s := range aspects {
				if idx >= 3 {
					break
				}
				top3 = append(top3, fmt.Sprintf("%s (Peringkat %d)", s.Name, s.Rank))
			}
			pdf.CellFormat(0, 4.5, "Minat Teratas (Top 3): "+strings.Join(top3, ", "), "", 1, "L", false, 0, "")
			pdf.Ln(1.5)

			// Table Header
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(15, 5.5, "No", "1", 0, "C", true, 0, "")
			pdf.CellFormat(115, 5.5, "Aspek Minat RMIB", "1", 0, "L", true, 0, "")
			pdf.CellFormat(50, 5.5, "Kategori", "1", 1, "C", true, 0, "")

			pdf.SetFont("Arial", "", 8.5)
			for i, a := range aspects {
				pdf.CellFormat(15, 5, fmt.Sprintf("%d", i+1), "1", 0, "C", false, 0, "")
				pdf.CellFormat(115, 5, a.Name, "1", 0, "L", false, 0, "")
				pdf.CellFormat(50, 5, a.Cat, "1", 1, "C", false, 0, "")
			}
			pdf.Ln(2.5)

			detailText, _ := summaryData["interpretasi_detail"].(string)
			if detailText == "" {
				detailText, _ = summaryData["summary"].(string)
			}
			if detailText != "" {
				pdf.SetFont("Arial", "I", 9)
				pdf.CellFormat(0, 4.5, "Interpretasi RMIB:", "", 1, "L", false, 0, "")
				pdf.SetFont("Arial", "", 9)
				pdf.MultiCell(0, 4.5, detailText, "", "L", false)
			}
		}
	} else if testType == "Gaya_Belajar" {
		if vak, ok := resultData.(models.LearningStyleResult); ok {
			pdf.SetFont("Arial", "B", 9.5)
			pdf.CellFormat(0, 4.5, "2. PROFIL GAYA BELAJAR (VAK)", "", 1, "L", false, 0, "")
			pdf.Ln(1)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(0, 4.5, fmt.Sprintf("Gaya Belajar Dominan: %s", vak.DominantType), "", 1, "L", false, 0, "")
			pdf.Ln(1.5)

			// Table Header
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(15, 5.5, "No", "1", 0, "C", true, 0, "")
			pdf.CellFormat(115, 5.5, "Aspek Gaya Belajar", "1", 0, "L", true, 0, "")
			pdf.CellFormat(50, 5.5, "Kategori", "1", 1, "C", true, 0, "")

			aspects := []struct {
				No   int
				Name string
				Score int
			}{
				{1, "Visual", vak.ScoreVisual},
				{2, "Auditori", vak.ScoreAuditory},
				{3, "Kinestetik", vak.ScoreKinesthetic},
			}

			pdf.SetFont("Arial", "", 8.5)
			for _, a := range aspects {
				pdf.CellFormat(15, 5, fmt.Sprintf("%d", a.No), "1", 0, "C", false, 0, "")
				pdf.CellFormat(115, 5, a.Name, "1", 0, "L", false, 0, "")
				pdf.CellFormat(50, 5, getVAKCategory(a.Score), "1", 1, "C", false, 0, "")
			}
			pdf.Ln(2.5)

			detailText, _ := summaryData["interpretasi_detail"].(string)
			if detailText == "" {
				detailText, _ = summaryData["summary"].(string)
			}
			if detailText != "" {
				pdf.SetFont("Arial", "I", 9)
				pdf.CellFormat(0, 4.5, "Interpretasi Gaya Belajar:", "", 1, "L", false, 0, "")
				pdf.SetFont("Arial", "", 9)
				pdf.MultiCell(0, 4.5, detailText, "", "L", false)
			}
		}
	} else if testType == "PAPI" {
		if papi, ok := resultData.(models.PAPIResult); ok {
			pdf.SetFont("Arial", "B", 9.5)
			pdf.CellFormat(0, 4.5, "2. ASPEK KEPRIBADIAN (PAPI-Kostick)", "", 1, "L", false, 0, "")
			pdf.Ln(1)
			pdf.SetFont("Arial", "", 9)
			pdf.CellFormat(0, 4.5, fmt.Sprintf("Kategori Perilaku Dominan: %s", papi.DominantCategory), "", 1, "L", false, 0, "")
			pdf.Ln(1.5)

			type entry struct {
				Label string `json:"label"`
				Score int    `json:"score"`
				Rank  int    `json:"rank"`
			}
			parsed := map[string]entry{}
			_ = json.Unmarshal([]byte(papi.ResultJSON), &parsed)

			codes := []string{"G", "L", "I", "T", "V", "S", "R", "D", "C", "E", "N", "A", "P", "X", "B", "O", "Z", "K", "F", "W"}
			labels := map[string]string{
				"G": "Pekerja keras", "L": "Kepemimpinan", "I": "Mudah membuat keputusan",
				"T": "Tipe orang yang sibuk", "V": "Tipe orang yang bersemangat",
				"S": "Hubungan sosial luas", "R": "Tipe teoritis",
				"D": "Tipe orang teratur", "C": "Mengatur/mengorganisir",
				"E": "Pengendalian emosi", "N": "Penyelesaian mandiri",
				"A": "Kebutuhan berprestasi", "P": "Mengatur orang lain",
				"X": "Untuk mendapat perhatian", "B": "Diterima kelompok",
				"O": "Hubungan akrab", "Z": "Hasrat berubah",
				"K": "Agresi", "F": "Mendukung atasan", "W": "Mengikuti aturan",
			}

			// Draw 2-column table header
			pdf.SetFont("Arial", "B", 7.5)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(60, 4.5, "Aspek Kepribadian", "1", 0, "L", true, 0, "")
			pdf.CellFormat(25, 4.5, "Kategori", "1", 0, "C", true, 0, "")
			pdf.CellFormat(10, 4.5, "", "", 0, "C", false, 0, "") // spacer
			pdf.CellFormat(60, 4.5, "Aspek Kepribadian", "1", 0, "L", true, 0, "")
			pdf.CellFormat(25, 4.5, "Kategori", "1", 1, "C", true, 0, "")

			pdf.SetFont("Arial", "", 7.5)
			for i := 0; i < 10; i++ {
				cLeft := codes[i]
				itemLeft := parsed[cLeft]
				catLeft := getPAPICategory(itemLeft.Score)

				cRight := codes[i+10]
				itemRight := parsed[cRight]
				catRight := getPAPICategory(itemRight.Score)

				pdf.CellFormat(60, 3.8, fmt.Sprintf("%s - %s", cLeft, labels[cLeft]), "1", 0, "L", false, 0, "")
				pdf.CellFormat(25, 3.8, catLeft, "1", 0, "C", false, 0, "")
				pdf.CellFormat(10, 3.8, "", "", 0, "C", false, 0, "") // spacer
				pdf.CellFormat(60, 3.8, fmt.Sprintf("%s - %s", cRight, labels[cRight]), "1", 0, "L", false, 0, "")
				pdf.CellFormat(25, 3.8, catRight, "1", 1, "C", false, 0, "")
			}
			pdf.Ln(2.5)

			pdf.SetFont("Arial", "I", 9)
			pdf.CellFormat(0, 4.5, "Interpretasi Kepribadian Kerja:", "", 1, "L", false, 0, "")
			pdf.SetFont("Arial", "", 9)
			summaryText, _ := summaryData["summary"].(string)
			if summaryText != "" {
				pdf.MultiCell(0, 4.5, summaryText, "", "L", false)
				pdf.Ln(2)
			}

			// Render dimensions if available
			if dimMap, ok := summaryData["papi_dimensions"].(map[string]interface{}); ok && len(dimMap) > 0 {
				order := []string{"work_direction", "leadership", "activity", "social_nature", "work_style", "temperament", "follower_authority"}
				for _, k := range order {
					if dVal, exists := dimMap[k]; exists && dVal != nil {
						if dObj, okObj := dVal.(map[string]interface{}); okObj {
							title, _ := dObj["title"].(string)
							if title != "" {
								if pdf.GetY() > 260 {
									pdf.AddPage()
								}
								pdf.SetFont("Arial", "B", 8.5)
								pdf.CellFormat(0, 4.5, "•  "+title, "", 1, "L", false, 0, "")
								pdf.SetFont("Arial", "", 8.5)
								if items, okItems := dObj["items"].([]map[string]string); okItems {
									for _, itm := range items {
										pdf.SetFont("Arial", "I", 8)
										pdf.CellFormat(0, 4, "   "+itm["aspect"]+": "+itm["score"], "", 1, "L", false, 0, "")
										pdf.SetFont("Arial", "", 8)
										pdf.SetX(pdf.GetX() + 3)
										pdf.MultiCell(0, 4, itm["desc"], "", "L", false)
										pdf.Ln(1)
									}
								} else if itemsGeneric, okGen := dObj["items"].([]interface{}); okGen {
									for _, itmGen := range itemsGeneric {
										if itm, okM := itmGen.(map[string]interface{}); okM {
											aspect, _ := itm["aspect"].(string)
											score, _ := itm["score"].(string)
											desc, _ := itm["desc"].(string)
											pdf.SetFont("Arial", "I", 8)
											pdf.CellFormat(0, 4, "   "+aspect+": "+score, "", 1, "L", false, 0, "")
											pdf.SetFont("Arial", "", 8)
											pdf.SetX(pdf.GetX() + 3)
											pdf.MultiCell(0, 4, desc, "", "L", false)
											pdf.Ln(1)
										}
									}
								}
								pdf.Ln(1.5)
							}
						}
					}
				}
			}
		}
	} else if testType == "IST" {
		if ist, ok := resultData.(models.ISTResult); ok {
			pdf.SetFont("Arial", "B", 9.5)
			pdf.CellFormat(0, 4.5, "2. ASPEK INTELEKTUAL SPESIFIK (IST)", "", 1, "L", false, 0, "")
			pdf.Ln(1)

			// Table Header
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(15, 5.5, "No", "1", 0, "C", true, 0, "")
			pdf.CellFormat(115, 5.5, "Aspek Intelektual", "1", 0, "L", true, 0, "")
			pdf.CellFormat(50, 5.5, "Kategori", "1", 1, "C", true, 0, "")

			avgSW := func(vals ...int) int {
				sum := 0
				n := 0
				for _, v := range vals {
					if v > 0 {
						sum += v
						n++
					}
				}
				if n == 0 {
					return 0
				}
				return sum / n
			}

			se, wa, an, ge, ra, za, fa, wu, me := ist.StdSE, ist.StdWA, ist.StdAN, ist.StdGE, ist.StdRA, ist.StdZA, ist.StdFA, ist.StdWU, ist.StdME
			aspects := []struct {
				No   int
				Name string
				Cat  string
			}{
				{1, "Penalaran Konkret", getCategoryFromSW(avgSW(se, ge))},
				{2, "Penalaran Verbal", getCategoryFromSW(avgSW(se, wa, ge))},
				{3, "Daya Analisis", getCategoryFromSW(an)},
				{4, "Penalaran Abstrak", getCategoryFromSW(za)},
				{5, "Daya Ingat", getCategoryFromSW(me)},
				{6, "Kemampuan Berhitung", getCategoryFromSW(ra)},
				{7, "Analogi Angka", getCategoryFromSW(za)},
				{8, "Daya Bayang Konstruksional", getCategoryFromSW(fa)},
				{9, "Daya Bayang Ruang", getCategoryFromSW(wu)},
			}

			pdf.SetFont("Arial", "", 8.5)
			for _, a := range aspects {
				pdf.CellFormat(15, 5, fmt.Sprintf("%d", a.No), "1", 0, "C", false, 0, "")
				pdf.CellFormat(115, 5, a.Name, "1", 0, "L", false, 0, "")
				pdf.CellFormat(50, 5, a.Cat, "1", 1, "C", false, 0, "")
			}
			pdf.Ln(2.5)

			detailText, _ := summaryData["interpretasi_detail"].(string)
			if detailText == "" {
				detailText, _ = summaryData["summary"].(string)
			}
			if detailText != "" {
				pdf.SetFont("Arial", "I", 9)
				pdf.CellFormat(0, 4.5, "Interpretasi Tes IST:", "", 1, "L", false, 0, "")
				pdf.SetFont("Arial", "", 9)
				pdf.MultiCell(0, 4.5, detailText, "", "L", false)
			}
		}
	} else if testType == "Kraepelin" {
		if krp, ok := resultData.(models.KraepelinAttempt); ok {
			pdf.SetFont("Arial", "B", 9.5)
			pdf.CellFormat(0, 4.5, "2. HASIL PENGUKURAN SPESIFIK (Kraepelin)", "", 1, "L", false, 0, "")
			pdf.Ln(1)

			tot := krp.TotalCorrect + krp.TotalErrors + krp.TotalSkipped
			acc := 0.0
			if tot > 0 {
				acc = float64(krp.TotalCorrect) / float64(tot) * 100.0
			}

			// Table Header
			pdf.SetFont("Arial", "B", 8.5)
			pdf.SetFillColor(240, 240, 240)
			pdf.CellFormat(15, 5.5, "No", "1", 0, "C", true, 0, "")
			pdf.CellFormat(115, 5.5, "Aspek Performansi Kerja", "1", 0, "L", true, 0, "")
			pdf.CellFormat(50, 5.5, "Kategori", "1", 1, "C", true, 0, "")

			aspects := []struct {
				No   int
				Name string
				Cat  string
			}{
				{1, "Kecepatan Kerja", getKraepelinCategory(float64(krp.TotalCorrect), "kecepatan")},
				{2, "Ketelitian Kerja", getKraepelinCategory(float64(krp.TotalErrors), "ketelitian")},
				{3, "Konsentrasi Kerja", getKraepelinCategory(acc, "konsentrasi")},
				{4, "Ketahanan Kerja", getKraepelinCategory(float64(krp.TotalCorrect), "ketahanan")},
			}

			pdf.SetFont("Arial", "", 8.5)
			for _, a := range aspects {
				pdf.CellFormat(15, 5, fmt.Sprintf("%d", a.No), "1", 0, "C", false, 0, "")
				pdf.CellFormat(115, 5, a.Name, "1", 0, "L", false, 0, "")
				pdf.CellFormat(50, 5, a.Cat, "1", 1, "C", false, 0, "")
			}
			pdf.Ln(2.5)

			detailText, _ := summaryData["interpretasi_detail"].(string)
			if detailText == "" {
				detailText, _ = summaryData["summary"].(string)
			}
			if detailText != "" {
				pdf.SetFont("Arial", "I", 9)
				pdf.CellFormat(0, 4.5, "Interpretasi Kraepelin:", "", 1, "L", false, 0, "")
				pdf.SetFont("Arial", "", 9)
				pdf.MultiCell(0, 4.5, detailText, "", "L", false)
			}
		}
	}

	buf := new(bytes.Buffer)
	err := pdf.Output(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseArrayOrString(val interface{}) []string {
	if val == nil {
		return nil
	}
	switch v := val.(type) {
	case []interface{}:
		var out []string
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	case string:
	}
	return nil
}

func getSubtestConclusionFromCombined(combinedSummary map[string]interface{}, key string) string {
	if combinedSummary == nil {
		return ""
	}
	details, ok := combinedSummary["kesimpulan_detail"].(map[string]interface{})
	if !ok || details == nil {
		return ""
	}
	concl, _ := details[key].(string)
	return concl
}

// generateComprehensivePDFReport renders a multi-page, comprehensive report of all test tools completed in a batch.
func (c *PsychotestAdminController) generateComprehensivePDFReport(o orm.Ormer, inv *models.TestInvitation, user *models.User, batch *models.TestBatch) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Laporan Hasil Asesmen Keseluruhan", false)
	pdf.SetMargins(15, 15, 15)

	// Fetch all results
	var istRes models.ISTResult
	_ = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&istRes)

	var hollandRes models.HollandResult
	_ = o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&hollandRes)

	var learningRes models.LearningStyleResult
	_ = o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&learningRes)

	var kraepelinRes models.KraepelinAttempt
	_ = o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&kraepelinRes)

	var rmibRes models.RMIBResult
	_ = o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).One(&rmibRes)

	var papiRes models.PAPIResult
	_ = o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&papiRes)

	// Build results map
	resultsMap := map[string]interface{}{}
	var completedTestNames []string
	if istRes.Id > 0 {
		resultsMap["ist"] = istRes
		completedTestNames = append(completedTestNames, "IST")
	}
	if hollandRes.Id > 0 {
		resultsMap["holland"] = hollandRes
		completedTestNames = append(completedTestNames, "HOLLAND")
	}
	if learningRes.Id > 0 {
		resultsMap["learning_style"] = learningRes
		completedTestNames = append(completedTestNames, "VAK")
	}
	if kraepelinRes.Id > 0 {
		resultsMap["kraepelin"] = kraepelinRes
		completedTestNames = append(completedTestNames, "KRAEPELIN")
	}
	if rmibRes.Id > 0 {
		resultsMap["rmib"] = rmibRes
		completedTestNames = append(completedTestNames, "RMIB")
	}
	if papiRes.Id > 0 {
		resultsMap["papi"] = papiRes
		completedTestNames = append(completedTestNames, "PAPI")
	}

	nama := user.NamaLengkap
	if nama == "" {
		nama = inv.Email
	}

	combinedSummary, err := GetOrGenerateCombinedSummaryInternal(nama, batch.Name, resultsMap)
	if err != nil {
		logs.Error("Combined report: failed to generate combined summary: %v", err)
	}

	// Set Footer callback (applies automatically to AddPage)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-22)
		pdf.SetLineWidth(0.2)
		pdf.SetDrawColor(200, 200, 200)
		pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
		pdf.Ln(2)
		
		pdf.SetFont("Arial", "", 7)
		pdf.SetTextColor(100, 100, 100)
		pdf.CellFormat(0, 3, "KANAGATA INSTITUTE", "", 1, "R", false, 0, "")
		pdf.CellFormat(0, 3, "RUMAH SADANARA, Jl. Maatum No.2, RT.10/RW.5, Utan Kayu Utara, Kec. Matraman, Kota Jakarta Timur, Daerah Khusus Ibukota Jakarta 13120", "", 1, "R", false, 0, "")
		pdf.CellFormat(0, 3, "Whatsapp : +62 81110811188 | www.kanagata.co.id | kanagata@sadanara.co.id", "", 1, "R", false, 0, "")
	})

	addNewPage := func() {
		pdf.AddPage()
		// Watermark background
		pdf.SetAlpha(0.03, "Normal")
		pdf.Image("static/icons/icon_psikologi_kanagata.png", 40, 85, 130, 0, false, "", 0, "")
		pdf.SetAlpha(1.0, "Normal")
		
		// Page Header (on Page 2+)
		if pdf.PageNo() > 1 {
			pdf.Image("static/icons/icon_psikologi_kanagata.png", 97, 8, 16, 0, false, "", 0, "")
			pdf.SetY(25)
			pdf.SetTextColor(0, 0, 0)
			pdf.SetFont("Arial", "B", 9)
			pdf.CellFormat(0, 4, "KANAGATA INSTITUTE", "", 1, "C", false, 0, "")
			pdf.Ln(3)
		}
	}

	// PAGE 1
	addNewPage()

	// Title block on Page 1
	pdf.Image("static/icons/icon_psikologi_kanagata.png", 91, 10, 28, 0, false, "", 0, "")
	pdf.SetY(39)
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 5, "KANAGATA INSTITUTE", "", 1, "C", false, 0, "")
	pdf.Ln(2)
	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(0, 5, "SMART PSIKOTEST STUDENT REPORT", "", 1, "C", false, 0, "")
	pdf.Ln(8)

	// Participant Info
	gender := "-"
	if user.JenisKelamin == models.GenderLakiLaki {
		gender = "LAKI-LAKI"
	} else if user.JenisKelamin == models.GenderPerempuan {
		gender = "PEREMPUAN"
	}
	age := 0
	if user.TanggalLahir != nil {
		age = utils.AgeYears(*user.TanggalLahir, time.Now())
	}
	usiaStr := "-"
	if age > 0 {
		usiaStr = fmt.Sprintf("%d TAHUN", age)
	}
	sekolah := batch.Sekolah
	if sekolah == "" {
		sekolah = user.Sekolah
	}
	if sekolah == "" {
		sekolah = batch.Institution
	}
	if sekolah == "" {
		sekolah = "-"
	}
	kelas := batch.Kelas
	if kelas == "" {
		kelas = user.Kelas
	}
	if kelas == "" {
		kelas = "-"
	}
	tglPeriksa := inv.UsedAt.Format("02 January 2006")
	if inv.UsedAt.IsZero() {
		tglPeriksa = inv.CreatedAt.Format("02 January 2006")
	}
	alatTestUsed := strings.Join(completedTestNames, ", ")

	drawRow := func(label, val string) {
		pdf.SetFont("Arial", "", 9.5)
		pdf.CellFormat(65, 5.5, label, "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 5.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9.5)
		pdf.CellFormat(0, 5.5, val, "", 1, "L", false, 0, "")
	}

	drawRow("NAMA", strings.ToUpper(nama))
	drawRow("USIA", strings.ToUpper(usiaStr))
	drawRow("JENIS KELAMIN", strings.ToUpper(gender))
	drawRow("KELAS", strings.ToUpper(kelas))
	drawRow("ASAL SEKOLAH", strings.ToUpper(sekolah))
	drawRow("TANGGAL ASESMEN", strings.ToUpper(tglPeriksa))
	drawRow("ALAT TEST YANG DIGUNAKAN", strings.ToUpper(alatTestUsed))
	pdf.Ln(6)

	// Section 1: Narrative Summary
	pdf.SetFont("Arial", "B", 10.5)
	pdf.SetTextColor(220, 38, 38) // red
	pdf.CellFormat(0, 6, "(PROFIL SISWA DARI SUMMARY SEMUA ALAT TEST)", "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(3)

	pdf.SetFont("Arial", "", 9)
	kesimpulanText := ""
	if combinedSummary != nil {
		if k, ok := combinedSummary["kesimpulan_gabungan"].(string); ok {
			kesimpulanText = k
		}
	}
	if kesimpulanText == "" {
		kesimpulanText = fmt.Sprintf("Berdasarkan hasil asesmen yang dilaksanakan, %s menunjukkan profil psikologis yang bervariasi sesuai dengan alat tes yang digunakan. Analisis lengkap per aspek terlampir pada halaman berikutnya.", nama)
	}

	paragraphs := strings.Split(kesimpulanText, "\n")
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		pdf.MultiCell(0, 4.5, p, "", "L", false)
		pdf.Ln(3)
	}

	// Helper to draw check box bullets
	drawBulletItem := func(text string) {
		x := pdf.GetX()
		y := pdf.GetY()
		
		// Check Y page overflow space
		if y + 6 > 270 {
			addNewPage()
			y = pdf.GetY()
			x = pdf.GetX()
		}
		
		pdf.SetLineWidth(0.2)
		pdf.SetDrawColor(0, 0, 0)
		pdf.Rect(x, y+0.8, 3.2, 3.2, "D")
		pdf.Line(x+0.7, y+2.2, x+1.4, y+3.0)
		pdf.Line(x+1.4, y+3.0, x+2.6, y+1.3)
		
		pdf.SetXY(x + 5.5, y)
		pdf.SetFont("Arial", "", 8.5)
		pdf.MultiCell(0, 4.5, text, "", "L", false)
		pdf.Ln(1)
	}

	checkPageSpace := func(h float64) {
		if pdf.GetY() + h > 270 {
			addNewPage()
		}
	}

	// PAGE 2+: STUDENT POTENTIAL
	addNewPage()

	pdf.SetFont("Arial", "B", 10.5)
	pdf.SetTextColor(220, 38, 38) // red
	pdf.CellFormat(0, 6, "STUDENT POTENTIAL (KESIMPULAN DARI HASIL MASING-MASING ALAT TEST)", "", 1, "L", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
	pdf.Ln(4)

	// Loop completed tests
	var testIndex = 0

	// 1. IST
	if istRes.Id > 0 {
		testIndex++
		checkPageSpace(60)
		
		pdf.SetFont("Arial", "B", 9.5)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. KEMAMPUAN KOGNITIF (IST)", testIndex), "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(40, 4.5, "Intelligence Quotient (IQ)", "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 4.5, fmt.Sprintf("%d", istRes.IQ), "", 1, "L", false, 0, "")
		
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(40, 4.5, "Kategori", "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 4.5, istRes.IQCategory, "", 1, "L", false, 0, "")
		pdf.Ln(3)

		// Table
		pdf.SetFillColor(240, 240, 240)
		pdf.SetFont("Arial", "B", 8)
		pdf.CellFormat(120, 5.5, "ASPEK", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 5.5, "PENILAIAN", "1", 1, "C", true, 0, "")
		
		avgSW := func(vals ...int) int {
			sum, n := 0, 0
			for _, v := range vals {
				if v > 0 {
					sum += v
					n++
				}
			}
			if n == 0 {
				return 0
			}
			return sum / n
		}
		se, wa, an, ge, ra, za, fa, wu, me := istRes.StdSE, istRes.StdWA, istRes.StdAN, istRes.StdGE, istRes.StdRA, istRes.StdZA, istRes.StdFA, istRes.StdWU, istRes.StdME
		aspects := []struct {
			Name string
			Cat  string
		}{
			{"Kemampuan Verbal", getCategoryFromSW(avgSW(se, wa, ge))},
			{"Kemampuan Numerik", getCategoryFromSW(avgSW(ra, za))},
			{"Kemampuan Analitis dan Logika", getCategoryFromSW(an)},
			{"Kemampuan Visual dan Spasial", getCategoryFromSW(avgSW(fa, wu))},
			{"Kemampuan Memori dan Konsentrasi", getCategoryFromSW(me)},
		}
		
		pdf.SetFont("Arial", "", 8)
		for _, a := range aspects {
			pdf.CellFormat(120, 5, a.Name, "1", 0, "L", false, 0, "")
			pdf.CellFormat(60, 5, a.Cat, "1", 1, "C", false, 0, "")
		}
		pdf.Ln(3)

		istSummary, _ := GetOrGenerateTestSummaryInternal(o, "IST", istRes, nama)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Interpretasi Hasil", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 8.5)
		istConcl := getSubtestConclusionFromCombined(combinedSummary, "ist")
		if istConcl == "" {
			istConcl, _ = istSummary["summary"].(string)
		}
		pdf.MultiCell(0, 4, istConcl, "", "L", false)
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Kekuatan Utama", "", 1, "L", false, 0, "")
		for _, k := range parseArrayOrString(istSummary["kekuatan"]) {
			drawBulletItem(k)
		}
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Rekomendasi Pengembangan", "", 1, "L", false, 0, "")
		for _, r := range parseArrayOrString(istSummary["rekomendasi_siswa"]) {
			drawBulletItem(r)
		}
		pdf.Ln(4)
	}

	// 2. HOLLAND
	if hollandRes.Id > 0 {
		testIndex++
		checkPageSpace(60)
		
		pdf.SetFont("Arial", "B", 9.5)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. MINAT DAN BAKAT (HOLLAND)", testIndex), "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(40, 4.5, "Kode RIASEC", "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 4.5, hollandRes.Code, "", 1, "L", false, 0, "")
		
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(40, 4.5, "Minat Teratas", "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 4.5, strings.Join([]string{hollandRes.Top1, hollandRes.Top2, hollandRes.Top3}, " -> "), "", 1, "L", false, 0, "")
		pdf.Ln(3)

		// Table
		pdf.SetFillColor(240, 240, 240)
		pdf.SetFont("Arial", "B", 8)
		pdf.CellFormat(120, 5.5, "ASPEK MINAT HOLLAND", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 5.5, "PENILAIAN", "1", 1, "C", true, 0, "")

		aspects := []struct {
			Name string
			Cat  string
		}{
			{"Realistic (R)", getHollandCategory(hollandRes.ScoreR)},
			{"Investigative (I)", getHollandCategory(hollandRes.ScoreI)},
			{"Artistic (A)", getHollandCategory(hollandRes.ScoreA)},
			{"Social (S)", getHollandCategory(hollandRes.ScoreS)},
			{"Enterprising (E)", getHollandCategory(hollandRes.ScoreE)},
			{"Conventional (C)", getHollandCategory(hollandRes.ScoreC)},
		}
		
		pdf.SetFont("Arial", "", 8)
		for _, a := range aspects {
			pdf.CellFormat(120, 5, a.Name, "1", 0, "L", false, 0, "")
			pdf.CellFormat(60, 5, a.Cat, "1", 1, "C", false, 0, "")
		}
		pdf.Ln(3)

		hollandSummary, _ := GetOrGenerateTestSummaryInternal(o, "Holland", hollandRes, nama)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Interpretasi Hasil", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 8.5)
		hollandConcl := getSubtestConclusionFromCombined(combinedSummary, "holland")
		if hollandConcl == "" {
			hollandConcl, _ = hollandSummary["summary"].(string)
		}
		pdf.MultiCell(0, 4, hollandConcl, "", "L", false)
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Kekuatan Utama", "", 1, "L", false, 0, "")
		for _, k := range parseArrayOrString(hollandSummary["kekuatan"]) {
			drawBulletItem(k)
		}
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Rekomendasi Pengembangan", "", 1, "L", false, 0, "")
		for _, r := range parseArrayOrString(hollandSummary["rekomendasi_siswa"]) {
			drawBulletItem(r)
		}
		pdf.Ln(4)
	}

	// 3. VAK (Learning Style)
	if learningRes.Id > 0 {
		testIndex++
		checkPageSpace(60)
		
		pdf.SetFont("Arial", "B", 9.5)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. GAYA BELAJAR (VAK)", testIndex), "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(40, 4.5, "Gaya Belajar Dominan", "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 4.5, learningRes.DominantType, "", 1, "L", false, 0, "")
		pdf.Ln(3)

		// Table
		pdf.SetFillColor(240, 240, 240)
		pdf.SetFont("Arial", "B", 8)
		pdf.CellFormat(120, 5.5, "ASPEK GAYA BELAJAR", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 5.5, "PENILAIAN", "1", 1, "C", true, 0, "")

		aspects := []struct {
			Name string
			Cat  string
		}{
			{"Visual", getVAKCategory(learningRes.ScoreVisual)},
			{"Auditori", getVAKCategory(learningRes.ScoreAuditory)},
			{"Kinestetik", getVAKCategory(learningRes.ScoreKinesthetic)},
		}
		
		pdf.SetFont("Arial", "", 8)
		for _, a := range aspects {
			pdf.CellFormat(120, 5, a.Name, "1", 0, "L", false, 0, "")
			pdf.CellFormat(60, 5, a.Cat, "1", 1, "C", false, 0, "")
		}
		pdf.Ln(3)

		vakSummary, _ := GetOrGenerateTestSummaryInternal(o, "Gaya_Belajar", learningRes, nama)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Interpretasi Hasil", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 8.5)
		vakConcl := getSubtestConclusionFromCombined(combinedSummary, "learning_style")
		if vakConcl == "" {
			vakConcl, _ = vakSummary["summary"].(string)
		}
		pdf.MultiCell(0, 4, vakConcl, "", "L", false)
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Kekuatan Utama", "", 1, "L", false, 0, "")
		for _, k := range parseArrayOrString(vakSummary["kekuatan"]) {
			drawBulletItem(k)
		}
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Rekomendasi Pengembangan", "", 1, "L", false, 0, "")
		for _, r := range parseArrayOrString(vakSummary["rekomendasi_siswa"]) {
			drawBulletItem(r)
		}
		pdf.Ln(4)
	}

	// 4. RMIB
	if rmibRes.Id > 0 {
		testIndex++
		checkPageSpace(60)
		
		pdf.SetFont("Arial", "B", 9.5)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. MINAT PEKERJAAN (RMIB)", testIndex), "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(40, 4.5, "Minat Dominan", "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 4.5, rmibRes.DominantCategory, "", 1, "L", false, 0, "")
		
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(40, 4.5, "Urutan Minat (Top 3)", "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 4.5, strings.Join([]string{rmibRes.Top1, rmibRes.Top2, rmibRes.Top3}, " -> "), "", 1, "L", false, 0, "")
		pdf.Ln(3)

		// Table
		pdf.SetFillColor(240, 240, 240)
		pdf.SetFont("Arial", "B", 8)
		pdf.CellFormat(120, 5.5, "ASPEK MINAT RMIB", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 5.5, "PENILAIAN", "1", 1, "C", true, 0, "")

		type entry struct {
			Label string `json:"label"`
			Score int    `json:"score"`
			Rank  int    `json:"rank"`
		}
		parsed := map[string]entry{}
		_ = json.Unmarshal([]byte(rmibRes.ResultJSON), &parsed)
		
		codes := []string{"OUT", "MEC", "COMP", "SCI", "PERS", "AEST", "MUS", "LIT", "SOC", "CLER", "PRAC", "MED"}
		labels := map[string]string{
			"OUT": "Outdoor", "MEC": "Mechanical", "COMP": "Computational", "SCI": "Scientific",
			"PERS": "Personal Contact", "AEST": "Aesthetic", "MUS": "Musical", "LIT": "Literary",
			"SOC": "Social Service", "CLER": "Clerical", "PRAC": "Practical", "MED": "Medical",
		}
		
		type aspectRow struct {
			Name string
			Rank int
			Cat  string
		}
		var aspects []aspectRow
		for _, c := range codes {
			item := parsed[c]
			rank := item.Rank
			if rank == 0 {
				rank = 12
			}
			aspects = append(aspects, aspectRow{
				Name: labels[c] + " (" + c + ")",
				Rank: rank,
				Cat:  getRMIBCategory(rank),
			})
		}
		sort.Slice(aspects, func(i, j int) bool {
			return aspects[i].Rank < aspects[j].Rank
		})

		pdf.SetFont("Arial", "", 8)
		// Limit to top 5 for comprehensive report to save space
		for i, a := range aspects {
			if i >= 5 {
				break
			}
			pdf.CellFormat(120, 5, fmt.Sprintf("%d. %s", i+1, a.Name), "1", 0, "L", false, 0, "")
			pdf.CellFormat(60, 5, a.Cat, "1", 1, "C", false, 0, "")
		}
		pdf.Ln(3)

		rmibSummary, _ := GetOrGenerateTestSummaryInternal(o, "RMIB", rmibRes, nama)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Interpretasi Hasil", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 8.5)
		rmibConcl := getSubtestConclusionFromCombined(combinedSummary, "rmib")
		if rmibConcl == "" {
			rmibConcl, _ = rmibSummary["summary"].(string)
		}
		pdf.MultiCell(0, 4, rmibConcl, "", "L", false)
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Kekuatan Utama", "", 1, "L", false, 0, "")
		for _, k := range parseArrayOrString(rmibSummary["kekuatan"]) {
			drawBulletItem(k)
		}
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Rekomendasi Pengembangan", "", 1, "L", false, 0, "")
		for _, r := range parseArrayOrString(rmibSummary["rekomendasi_siswa"]) {
			drawBulletItem(r)
		}
		pdf.Ln(4)
	}

	// 5. PAPI
	if papiRes.Id > 0 {
		testIndex++
		checkPageSpace(70)
		
		pdf.SetFont("Arial", "B", 9.5)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. KEPRIBADIAN KERJA (PAPI)", testIndex), "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 9)
		pdf.CellFormat(40, 4.5, "Kategori Dominan", "", 0, "L", false, 0, "")
		pdf.CellFormat(5, 4.5, ":", "", 0, "L", false, 0, "")
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 4.5, papiRes.DominantCategory, "", 1, "L", false, 0, "")
		pdf.Ln(3)

		// 2-Column Table of Aspect Categories
		pdf.SetFont("Arial", "B", 7.5)
		pdf.SetFillColor(240, 240, 240)
		pdf.CellFormat(60, 4.5, "Aspek Kepribadian", "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 4.5, "Kategori", "1", 0, "C", true, 0, "")
		pdf.CellFormat(10, 4.5, "", "", 0, "C", false, 0, "") // spacer
		pdf.CellFormat(60, 4.5, "Aspek Kepribadian", "1", 0, "L", true, 0, "")
		pdf.CellFormat(25, 4.5, "Kategori", "1", 1, "C", true, 0, "")

		type entry struct {
			Label string `json:"label"`
			Score int    `json:"score"`
			Rank  int    `json:"rank"`
		}
		parsed := map[string]entry{}
		_ = json.Unmarshal([]byte(papiRes.ResultJSON), &parsed)

		codes := []string{"G", "L", "I", "T", "V", "S", "R", "D", "C", "E", "N", "A", "P", "X", "B", "O", "Z", "K", "F", "W"}
		labels := map[string]string{
			"G": "Pekerja keras", "L": "Kepemimpinan", "I": "Mudah mengambil keputusan",
			"T": "Tipe orang sibuk", "V": "Tipe orang bersemangat",
			"S": "Hubungan sosial luas", "R": "Tipe teoritis",
			"D": "Tipe orang teratur", "C": "Mengorganisir",
			"E": "Pengendalian emosi", "N": "Penyelesaian mandiri",
			"A": "Kebutuhan berprestasi", "P": "Mengatur orang lain",
			"X": "Untuk mendapat perhatian", "B": "Diterima kelompok",
			"O": "Hubungan akrab", "Z": "Hasrat berubah",
			"K": "Agresi", "F": "Mendukung atasan", "W": "Mengikuti aturan",
		}

		pdf.SetFont("Arial", "", 7.5)
		// Render top 5 rows (10 aspects total) to save space
		for i := 0; i < 5; i++ {
			cLeft := codes[i]
			itemLeft := parsed[cLeft]
			catLeft := getPAPICategory(itemLeft.Score)

			cRight := codes[i+5]
			itemRight := parsed[cRight]
			catRight := getPAPICategory(itemRight.Score)

			pdf.CellFormat(60, 3.8, fmt.Sprintf("%s - %s", cLeft, labels[cLeft]), "1", 0, "L", false, 0, "")
			pdf.CellFormat(25, 3.8, catLeft, "1", 0, "C", false, 0, "")
			pdf.CellFormat(10, 3.8, "", "", 0, "C", false, 0, "") // spacer
			pdf.CellFormat(60, 3.8, fmt.Sprintf("%s - %s", cRight, labels[cRight]), "1", 0, "L", false, 0, "")
			pdf.CellFormat(25, 3.8, catRight, "1", 1, "C", false, 0, "")
		}
		pdf.Ln(3)

		papiSummary, _ := GetOrGenerateTestSummaryInternal(o, "PAPI", papiRes, nama)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Interpretasi Hasil", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 8.5)
		papiConcl := getSubtestConclusionFromCombined(combinedSummary, "papi")
		if papiConcl == "" {
			papiConcl, _ = papiSummary["summary"].(string)
		}
		pdf.MultiCell(0, 4, papiConcl, "", "L", false)
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Kekuatan Utama", "", 1, "L", false, 0, "")
		for _, k := range parseArrayOrString(papiSummary["kekuatan"]) {
			drawBulletItem(k)
		}
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Rekomendasi Pengembangan", "", 1, "L", false, 0, "")
		for _, r := range parseArrayOrString(papiSummary["rekomendasi_siswa"]) {
			drawBulletItem(r)
		}
		pdf.Ln(4)
	}

	// 6. KRAEPELIN
	if kraepelinRes.Id > 0 {
		testIndex++
		checkPageSpace(60)
		
		pdf.SetFont("Arial", "B", 9.5)
		pdf.CellFormat(0, 5, fmt.Sprintf("%d. PERFORMANSI KERJA (KRAEPELIN)", testIndex), "", 1, "L", false, 0, "")
		pdf.Ln(2)

		tot := kraepelinRes.TotalCorrect + kraepelinRes.TotalErrors + kraepelinRes.TotalSkipped
		acc := 0.0
		if tot > 0 {
			acc = float64(kraepelinRes.TotalCorrect) / float64(tot) * 100.0
		}

		// Table
		pdf.SetFillColor(240, 240, 240)
		pdf.SetFont("Arial", "B", 8)
		pdf.CellFormat(120, 5.5, "ASPEK PERFORMANSI KERJA", "1", 0, "L", true, 0, "")
		pdf.CellFormat(60, 5.5, "PENILAIAN", "1", 1, "C", true, 0, "")

		aspects := []struct {
			Name string
			Cat  string
		}{
			{"Kecepatan Kerja", getKraepelinCategory(float64(kraepelinRes.TotalCorrect), "kecepatan")},
			{"Ketelitian Kerja", getKraepelinCategory(float64(kraepelinRes.TotalErrors), "ketelitian")},
			{"Konsentrasi Kerja", getKraepelinCategory(acc, "konsentrasi")},
			{"Ketahanan Kerja", getKraepelinCategory(float64(kraepelinRes.TotalCorrect), "ketahanan")},
		}
		
		pdf.SetFont("Arial", "", 8)
		for _, a := range aspects {
			pdf.CellFormat(120, 5, a.Name, "1", 0, "L", false, 0, "")
			pdf.CellFormat(60, 5, a.Cat, "1", 1, "C", false, 0, "")
		}
		pdf.Ln(3)

		kraepelinSummary, _ := GetOrGenerateTestSummaryInternal(o, "Kraepelin", kraepelinRes, nama)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Interpretasi Hasil", "", 1, "L", false, 0, "")
		pdf.SetFont("Arial", "", 8.5)
		kraepelinConcl := getSubtestConclusionFromCombined(combinedSummary, "kraepelin")
		if kraepelinConcl == "" {
			kraepelinConcl, _ = kraepelinSummary["summary"].(string)
		}
		pdf.MultiCell(0, 4, kraepelinConcl, "", "L", false)
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Kekuatan Utama", "", 1, "L", false, 0, "")
		for _, k := range parseArrayOrString(kraepelinSummary["kekuatan"]) {
			drawBulletItem(k)
		}
		pdf.Ln(2)

		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(0, 5, "Rekomendasi Pengembangan", "", 1, "L", false, 0, "")
		for _, r := range parseArrayOrString(kraepelinSummary["rekomendasi_siswa"]) {
			drawBulletItem(r)
		}
		pdf.Ln(4)
	}

	buf := new(bytes.Buffer)
	err = pdf.Output(buf)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

