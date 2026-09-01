package controllers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
)

// KraepelinTestController menangani alur pengerjaan tes Kraepelin.
// Catatan: versi pertama ini fokus pada flow lengkap + penyimpanan data + export Excel,
// lalu scoring/faktor bisa disempurnakan bertahap agar sesuai template.
type KraepelinTestController struct {
	beego.Controller
}

func (c *KraepelinTestController) mustGetSessionInvitation() (*models.TestInvitation, *models.User, bool) {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		return nil, nil, false
	}
	userID, ok := userIDAny.(int)
	if !ok || userID == 0 {
		return nil, nil, false
	}

	o := orm.NewOrm()
	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		return nil, nil, false
	}

	invIDAny := c.GetSession("current_invitation_id")
	var inv models.TestInvitation
	if invIDAny == nil {
		activeInv, ok := ResolveActiveInvitation(o, user.Id, user.Email)
		if !ok {
			return nil, nil, false
		}
		inv = *activeInv
		bID := 0
		if inv.BatchId != nil {
			bID = *inv.BatchId
		}
		c.SetSession("current_invitation_id", inv.Id)
		c.SetSession("current_batch_id", bID)
	} else {
		invID, _ := invIDAny.(int)
		inv.Id = invID
		if err := o.Read(&inv); err != nil {
			activeInv, ok := ResolveActiveInvitation(o, user.Id, user.Email)
			if !ok {
				return nil, nil, false
			}
			inv = *activeInv
			bID := 0
			if inv.BatchId != nil {
				bID = *inv.BatchId
			}
			c.SetSession("current_invitation_id", inv.Id)
			c.SetSession("current_batch_id", bID)
		}
	}

	// Ownership guard
	if inv.UserId == nil || *inv.UserId != userID {
		if strings.TrimSpace(inv.Email) == "" || user.Email == "" || !strings.EqualFold(inv.Email, user.Email) {
			return nil, nil, false
		}
	}
	return &inv, &user, true
}

func (c *KraepelinTestController) ensureBatchAllowsKraepelin(inv *models.TestInvitation) bool {
	if inv == nil || inv.BatchId == nil {
		return false
	}
	o := orm.NewOrm()
	var batch models.TestBatch
	batch.Id = *inv.BatchId
	if err := o.Read(&batch); err != nil {
		return false
	}
	return batch.EnableKraepelin
}

func getKraepelinResumeColumn(att *models.KraepelinAttempt) int {
	if att == nil || strings.TrimSpace(att.AnswersJSON) == "" {
		return 1
	}
	var allAnswers [][]*int
	if err := json.Unmarshal([]byte(att.AnswersJSON), &allAnswers); err != nil || len(allAnswers) == 0 {
		return 1
	}
	for colIdx, colAns := range allAnswers {
		if len(colAns) == 0 {
			return colIdx + 1
		}
		allNil := true
		for _, v := range colAns {
			if v != nil {
				allNil = false
				break
			}
		}
		if allNil {
			return colIdx + 1
		}
	}
	return 40
}

// @router /test/kraepelin/start [get]
func (c *KraepelinTestController) StartPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsKraepelin(inv) {
		c.Redirect("/test", 302)
		return
	}

	// If already finished, check next test in batch. If in_progress, auto-resume.
	o := orm.NewOrm()
	var att models.KraepelinAttempt
	if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&att); err == nil && att.Id != 0 {
		if att.Status == "finished" {
			var batch models.TestBatch
			if inv.BatchId != nil {
				batch.Id = *inv.BatchId
				_ = o.Read(&batch)
			}
			nextURL := GetNextTestRedirect(inv.Id, &batch)
			if nextURL != "" {
				c.Redirect(nextURL, 302)
			} else {
				c.Redirect("/hasil-tes", 302)
			}
			return
		} else if att.Status == "in_progress" {
			c.Redirect("/test/kraepelin/questions", 302)
			return
		}
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_kraepelin_start.html"
}

// @router /test/kraepelin/start [post]
func (c *KraepelinTestController) SubmitStart() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsKraepelin(inv) {
		c.Redirect("/test", 302)
		return
	}

	// Biodata fields
	name := strings.TrimSpace(c.GetString("name"))
	gender := strings.TrimSpace(c.GetString("gender"))
	birthPlace := strings.TrimSpace(c.GetString("birth_place"))
	birthDateStr := strings.TrimSpace(c.GetString("birth_date")) // yyyy-mm-dd
	ageStr := strings.TrimSpace(c.GetString("age"))
	address := strings.TrimSpace(c.GetString("address"))
	education := strings.TrimSpace(c.GetString("education"))
	major := strings.TrimSpace(c.GetString("major"))
	job := strings.TrimSpace(c.GetString("job"))
	tester := strings.TrimSpace(c.GetString("tester"))

	age, _ := strconv.Atoi(ageStr)
	if name == "" || (gender != "laki-laki" && gender != "perempuan") || birthPlace == "" || age <= 0 || address == "" || education == "" || major == "" || tester == "" {
		c.Data["Error"] = "Mohon lengkapi biodata dengan benar."
		c.Data["User"] = user
		c.Data["Invitation"] = inv
		c.TplName = "test_kraepelin_start.html"
		return
	}

	var birthDateISO *string
	if birthDateStr != "" {
		if _, err := time.Parse("2006-01-02", birthDateStr); err == nil {
			s := birthDateStr
			birthDateISO = &s
		}
	}

	// Generate digits: 50 columns x 27 digits (1..9)
	columnCount := 40
	digitsPerCol := 27
	secondsPerCol := 30 // default 30 detik per kolom (bisa diubah nanti via konfigurasi batch).

	digits := kraepelinFixedDigits()
	// Pastikan payload soal selalu 40 kolom x 27 digit sesuai format tes.
	if len(digits) > columnCount {
		digits = digits[:columnCount]
	}
	digitsJSONBytes, _ := json.Marshal(digits)

	o := orm.NewOrm()
	var att models.KraepelinAttempt
	err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&att)
	if err != nil || att.Id == 0 {
		att = models.KraepelinAttempt{
			Invitation: inv,
			User:       user,
			TestDate:   time.Now(),
			TestName:   name,
			TestGender: gender,
			TestBirthPlace: birthPlace,
			TestAge:    age,
			TestAddress: address,
			TestEducation: education,
			TestMajor:  major,
			Tester:     tester,
			Status:     "in_progress",
			ColumnCount: columnCount,
			DigitsPerColumn: digitsPerCol,
			SecondsPerColumn: secondsPerCol,
			DigitsJSON: string(digitsJSONBytes),
		}
		if birthDateISO != nil {
			att.TestBirthDate = birthDateISO
		}
		if job != "" {
			att.TestJob = job
		}
		if _, ierr := o.Insert(&att); ierr != nil {
			// Log detail error ke file agar mudah dilacak, tapi tetap tampilkan pesan umum ke user.
			logs.Error("Failed to insert KraepelinAttempt for invitation %d (user %d): %v", inv.Id, user.Id, ierr)
			c.Data["Error"] = "Gagal menyimpan biodata. Silakan coba lagi."
			c.Data["User"] = user
			c.Data["Invitation"] = inv
			c.TplName = "test_kraepelin_start.html"
			return
		}
	} else {
		att.TestName = name
		att.TestGender = gender
		att.TestBirthPlace = birthPlace
		if birthDateISO != nil {
			att.TestBirthDate = birthDateISO
		} else {
			att.TestBirthDate = nil
		}
		att.TestAge = age
		att.TestAddress = address
		att.TestEducation = education
		att.TestMajor = major
		att.TestJob = job
		att.Tester = tester
		if att.TestDate.IsZero() {
			att.TestDate = time.Now()
		}
		att.ColumnCount = columnCount
		att.DigitsPerColumn = digitsPerCol
		att.SecondsPerColumn = secondsPerCol
		att.DigitsJSON = string(digitsJSONBytes)
		att.Status = "in_progress"
		_, _ = o.Update(&att,
			"TestName", "TestGender", "TestBirthPlace", "TestBirthDate", "TestAge",
			"TestAddress", "TestEducation", "TestMajor", "TestJob", "Tester", "TestDate",
			"ColumnCount", "DigitsPerColumn", "SecondsPerColumn", "DigitsJSON", "Status",
		)
	}

	c.Redirect("/test/kraepelin/instruction", 302)
}

// @router /test/kraepelin/instruction [get]
func (c *KraepelinTestController) InstructionPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsKraepelin(inv) {
		c.Redirect("/test", 302)
		return
	}
	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_kraepelin_instruction.html"
}

// @router /test/kraepelin/questions [get]
func (c *KraepelinTestController) QuestionsPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsKraepelin(inv) {
		c.Redirect("/test", 302)
		return
	}

	o := orm.NewOrm()
	var att models.KraepelinAttempt
	if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&att); err != nil || att.Id == 0 {
		c.Redirect("/test/kraepelin/start", 302)
		return
	}
	if att.Status == "finished" {
		var batch models.TestBatch
		if inv.BatchId != nil {
			batch.Id = *inv.BatchId
			_ = o.Read(&batch)
		}
		nextURL := GetNextTestRedirect(inv.Id, &batch)
		if nextURL != "" {
			c.Redirect(nextURL, 302)
		} else {
			c.Redirect("/test/kraepelin/finish", 302)
		}
		return
	}

	// Hardening: pastikan digits selalu valid 40x27.
	// Jika data lama rusak/kosong, regenerate dari matrix fixed agar UI tidak menampilkan "-".
	needRegenerateDigits := false
	var parsedDigits [][]int
	if strings.TrimSpace(att.DigitsJSON) == "" {
		needRegenerateDigits = true
	} else if err := json.Unmarshal([]byte(att.DigitsJSON), &parsedDigits); err != nil {
		needRegenerateDigits = true
	} else if len(parsedDigits) != 40 {
		needRegenerateDigits = true
	} else {
		for _, col := range parsedDigits {
			if len(col) != 27 {
				needRegenerateDigits = true
				break
			}
		}
	}
	if needRegenerateDigits {
		fixed := kraepelinFixedDigits()
		if len(fixed) > 40 {
			fixed = fixed[:40]
		}
		if b, err := json.Marshal(fixed); err == nil {
			att.DigitsJSON = string(b)
			att.ColumnCount = 40
			att.DigitsPerColumn = 27
			_, _ = o.Update(&att, "DigitsJSON", "ColumnCount", "DigitsPerColumn")
		}
	}

	rawIdx, _ := strconv.Atoi(strings.TrimSpace(c.GetString("raw")))
	if rawIdx <= 0 {
		rawIdx = getKraepelinResumeColumn(&att)
	}
	if rawIdx > 40 {
		rawIdx = 40
	}

	// Send digits to frontend (JSON string) so client can render 40 columns with lock timer.
	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["Attempt"] = att
	c.Data["CurrentRaw"] = rawIdx
	c.Data["TotalRaw"] = 40
	c.Data["IsDev"] = strings.EqualFold(beego.BConfig.RunMode, "dev")
	c.TplName = "test_kraepelin_questions.html"
}

type kraepelinSubmitPayload struct {
	ColumnIndex int       `json:"column_index"` // 0..39
	Answers     []*int    `json:"answers"`      // len 26, nil = kosong/skip
	ClientAt    time.Time `json:"client_at"`
	ForceFinish bool      `json:"force_finish"`
}

// @router /api/test/kraepelin/submit [post]
func (c *KraepelinTestController) SubmitAnswersAPI() {
	inv, _, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}
	if !c.ensureBatchAllowsKraepelin(inv) {
		c.Ctx.Output.SetStatus(409)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Batch tidak mengaktifkan Tes Kraepelin"}
		c.ServeJSON()
		return
	}

	var p kraepelinSubmitPayload
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &p); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}
	if p.ColumnIndex < 0 || p.ColumnIndex >= 40 {
		c.Ctx.Output.SetStatus(422)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Kolom tidak valid"}
		c.ServeJSON()
		return
	}
	if len(p.Answers) != 26 {
		c.Ctx.Output.SetStatus(422)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Jawaban harus 26 item (antar angka)"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	var att models.KraepelinAttempt
	if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&att); err != nil || att.Id == 0 {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Attempt tidak ditemukan"}
		c.ServeJSON()
		return
	}
	if att.Status == "finished" {
		c.Data["json"] = map[string]interface{}{"success": true, "message": "Sudah selesai"}
		c.ServeJSON()
		return
	}

	// Load digits
	var digits [][]int
	if err := json.Unmarshal([]byte(att.DigitsJSON), &digits); err != nil || len(digits) != 40 {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal rusak / tidak valid"}
		c.ServeJSON()
		return
	}
	colDigits := digits[p.ColumnIndex]
	if len(colDigits) != 27 {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal kolom tidak valid"}
		c.ServeJSON()
		return
	}

	// Compute correct/errors/skipped for that column
	correct := 0
	errors := 0
	skipped := 0
	for i := 0; i < 26; i++ {
		expected := (colDigits[i] + colDigits[i+1]) % 10
		if p.Answers[i] == nil {
			skipped++
			continue
		}
		ans := *p.Answers[i]
		if ans < 0 || ans > 9 {
			errors++
			continue
		}
		if ans == expected {
			correct++
		} else {
			errors++
		}
	}

	// Merge answers into AnswersJSON ([][]*int), build if empty.
	var allAnswers [][]*int
	if strings.TrimSpace(att.AnswersJSON) != "" {
		_ = json.Unmarshal([]byte(att.AnswersJSON), &allAnswers)
	}
	if len(allAnswers) != 40 {
		allAnswers = make([][]*int, 40)
		for i := 0; i < 40; i++ {
			allAnswers[i] = make([]*int, 26)
		}
	}
	allAnswers[p.ColumnIndex] = p.Answers
	allAnswersJSON, _ := json.Marshal(allAnswers)

	// CorrectCountsJSON []int
	var counts []int
	if strings.TrimSpace(att.CorrectCountsJSON) != "" {
		_ = json.Unmarshal([]byte(att.CorrectCountsJSON), &counts)
	}
	if len(counts) != 40 {
		counts = make([]int, 40)
	}
	counts[p.ColumnIndex] = correct
	countsJSON, _ := json.Marshal(counts)

	// Recompute totals
	totalCorrect := 0
	for _, v := range counts {
		totalCorrect += v
	}
	att.AnswersJSON = string(allAnswersJSON)
	att.CorrectCountsJSON = string(countsJSON)
	att.TotalCorrect = totalCorrect
	// Recompute totals from scratch (idempotent per resubmit).
	// Penting: jangan hitung kolom yang belum sempat dikerjakan, agar total skipped tidak
	// membengkak karena kolom yang belum dibuka.
	totalErrors := 0
	totalSkipped := 0
	consideredCols := p.ColumnIndex + 1
	if consideredCols < 0 {
		consideredCols = 0
	}
	if consideredCols > 40 {
		consideredCols = 40
	}
	for col := 0; col < consideredCols; col++ {
		colDigits := digits[col]
		if len(colDigits) != 27 {
			continue
		}
		for i := 0; i < 26; i++ {
			expected := (colDigits[i] + colDigits[i+1]) % 10
			a := allAnswers[col][i]
			if a == nil {
				totalSkipped++
				continue
			}
			ans := *a
			if ans < 0 || ans > 9 || ans != expected {
				totalErrors++
			}
		}
	}
	att.TotalErrors = totalErrors
	att.TotalSkipped = totalSkipped

	// Finish only if explicitly requested (e.g. time ran out on last column or force finish)
	finishNow := p.ForceFinish
	next := "/test/kraepelin/questions"

	if finishNow {
		att.Status = "finished"
		att.FinishedAt = time.Now()
	}

	// Simpan ke DB TERLEBIH DAHULU agar status "finished" sudah tercatat
	// sebelum GetNextTestRedirect mengecek apakah Kraepelin sudah selesai.
	if _, err := o.Update(&att,
		"AnswersJSON", "CorrectCountsJSON",
		"TotalCorrect", "TotalErrors", "TotalSkipped",
		"Status", "FinishedAt",
	); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal menyimpan jawaban"}
		c.ServeJSON()
		return
	}

	// Setelah DB diupdate, tentukan redirect berikutnya
	if finishNow {
		var batch models.TestBatch
		if inv.BatchId != nil {
			batch.Id = *inv.BatchId
			_ = o.Read(&batch)
		}
		nextURL := GetNextTestRedirect(inv.Id, &batch)
		if nextURL != "" {
			next = nextURL
		} else {
			next = "/test/kraepelin/finish"
			// Mark invitation used
			if inv.Status != models.StatusInvitationUsed {
				inv.Status = models.StatusInvitationUsed
				inv.UsedAt = time.Now()
				_, _ = o.Update(inv, "Status", "UsedAt")
				go utils.SendTestCompletionNotification(inv.UserId, "Kraepelin")
			}
		}
	}

	c.Data["json"] = map[string]interface{}{
		"success": true,
		"column_correct": correct,
		"column_errors":  errors,
		"column_skipped": skipped,
		"next_redirect":  next,
	}
	c.ServeJSON()
}

// @router /test/kraepelin/finish [get]
func (c *KraepelinTestController) FinishPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsKraepelin(inv) {
		c.Redirect("/test", 302)
		return
	}

	o := orm.NewOrm()
	var att models.KraepelinAttempt
	if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&att); err != nil || att.Id == 0 {
		c.Redirect("/test/kraepelin/start", 302)
		return
	}

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

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["Attempt"] = att
	c.TplName = "test_kraepelin_finish.html"
}

// @router /test/kraepelin/result/excel [get]
func (c *KraepelinTestController) ExportResultExcel() {
	o := orm.NewOrm()

	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login", 302)
		return
	}
	userID := sessionUser.(int)

	invIDStr := strings.TrimSpace(c.GetString("invId"))
	var inv models.TestInvitation
	if invIDStr != "" {
		id, err := strconv.Atoi(invIDStr)
		if err != nil || id <= 0 {
			c.Redirect("/test/kraepelin/finish", 302)
			return
		}
		inv.Id = id
		if err := o.Read(&inv); err != nil {
			c.Redirect("/test/kraepelin/finish", 302)
			return
		}
		if inv.UserId == nil || *inv.UserId != userID {
			c.Redirect("/test/kraepelin/finish", 302)
			return
		}
	} else {
		sessionInv := c.GetSession("current_invitation_id")
		if sessionInv == nil {
			c.Redirect("/test", 302)
			return
		}
		inv.Id = sessionInv.(int)
		if err := o.Read(&inv); err != nil {
			c.Redirect("/test", 302)
			return
		}
	}

	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/test/kraepelin/finish", 302)
		return
	}
	var batch *models.TestBatch
	if inv.BatchId != nil {
		var b models.TestBatch
		b.Id = *inv.BatchId
		if o.Read(&b) == nil {
			batch = &b
		}
	}

	excelBytes, err := buildKraepelinResultXLSX(o, batch, &inv, &user)
	if err != nil {
		c.Redirect("/test/kraepelin/finish", 302)
		return
	}

	makeSafeName := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			return "user"
		}
		var b strings.Builder
		lastUnderscore := false
		for _, r := range s {
			isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
			if isAlphaNum {
				b.WriteRune(r)
				lastUnderscore = false
				continue
			}
			if !lastUnderscore {
				b.WriteRune('_')
				lastUnderscore = true
			}
		}
		out := strings.Trim(b.String(), "_")
		if out == "" {
			return "user"
		}
		return out
	}
	downloadName := strings.TrimSpace(user.NamaLengkap)
	if downloadName == "" {
		downloadName = strings.TrimSpace(user.Email)
	}
	if downloadName == "" {
		downloadName = strings.TrimSpace(inv.Email)
	}
	filename := fmt.Sprintf("kraepelin_result_%s.xlsx", makeSafeName(downloadName))
	c.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(excelBytes)
}

// DEV ONLY: POST /test/kraepelin/dev-autofill
func (c *KraepelinTestController) DevAutoFill() {
	if !strings.EqualFold(beego.BConfig.RunMode, "dev") {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Endpoint hanya tersedia di mode development"}
		c.ServeJSON()
		return
	}

	inv, _, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	var att models.KraepelinAttempt
	if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&att); err != nil || att.Id == 0 {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Attempt tidak ditemukan"}
		c.ServeJSON()
		return
	}

	var digits [][]int
	if strings.TrimSpace(att.DigitsJSON) == "" {
		digits = kraepelinFixedDigits()
	} else if err := json.Unmarshal([]byte(att.DigitsJSON), &digits); err != nil || len(digits) != 40 {
		digits = kraepelinFixedDigits()
	}
	if len(digits) > 40 {
		digits = digits[:40]
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	allAnswers := make([][]*int, 40)
	counts := make([]int, 40)
	totalCorrect := 0
	totalErrors := 0
	totalSkipped := 0

	for col := 0; col < 40; col++ {
		allAnswers[col] = make([]*int, 26)
		colDigits := digits[col]
		correctInCol := 0
		for i := 0; i < 26; i++ {
			expected := (colDigits[i] + colDigits[i+1]) % 10
			var ans int
			// 85% chance of correct, 15% error
			if rng.Intn(100) < 85 {
				ans = expected
				correctInCol++
				totalCorrect++
			} else {
				ans = (expected + 1 + rng.Intn(8)) % 10
				totalErrors++
			}
			allAnswers[col][i] = &ans
		}
		counts[col] = correctInCol
	}

	allAnswersJSON, _ := json.Marshal(allAnswers)
	countsJSON, _ := json.Marshal(counts)

	att.AnswersJSON = string(allAnswersJSON)
	att.CorrectCountsJSON = string(countsJSON)
	att.TotalCorrect = totalCorrect
	att.TotalErrors = totalErrors
	att.TotalSkipped = totalSkipped
	att.Status = "finished"
	att.FinishedAt = time.Now()

	// Simpan ke DB TERLEBIH DAHULU sebelum GetNextTestRedirect mengecek status Kraepelin
	if _, err := o.Update(&att,
		"AnswersJSON", "CorrectCountsJSON",
		"TotalCorrect", "TotalErrors", "TotalSkipped",
		"Status", "FinishedAt",
	); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal menyimpan jawaban dev"}
		c.ServeJSON()
		return
	}

	var redirectURL = ""
	var batch models.TestBatch
	if inv.BatchId != nil {
		batch.Id = *inv.BatchId
		_ = o.Read(&batch)
	}
	nextURL := GetNextTestRedirect(inv.Id, &batch)
	if nextURL != "" {
		redirectURL = nextURL
	} else {
		redirectURL = "/test/kraepelin/finish"
		if inv.Status != models.StatusInvitationUsed {
			inv.Status = models.StatusInvitationUsed
			inv.UsedAt = time.Now()
			_, _ = o.Update(inv, "Status", "UsedAt")
			go utils.SendTestCompletionNotification(inv.UserId, "Kraepelin")
		}
	}

	c.Data["json"] = map[string]interface{}{"success": true, "next": redirectURL}
	c.ServeJSON()
}

