package controllers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/core/logs"
	"github.com/xuri/excelize/v2"
)

// KraepelinTestController menangani alur pengerjaan tes Kraepelin.
// Catatan: versi pertama ini fokus pada flow lengkap + penyimpanan data + export Excel,
// lalu scoring/faktor bisa disempurnakan bertahap agar sesuai template.
type KraepelinTestController struct {
	beego.Controller
}

func (c *KraepelinTestController) mustGetSessionInvitation() (*models.TestInvitation, *models.User, bool) {
	userIDAny := c.GetSession("user_id")
	invIDAny := c.GetSession("current_invitation_id")
	if userIDAny == nil || invIDAny == nil {
		return nil, nil, false
	}
	userID, ok := userIDAny.(int)
	if !ok || userID == 0 {
		return nil, nil, false
	}
	invID, ok := invIDAny.(int)
	if !ok || invID == 0 {
		return nil, nil, false
	}

	o := orm.NewOrm()
	var inv models.TestInvitation
	inv.Id = invID
	if err := o.Read(&inv); err != nil {
		return nil, nil, false
	}
	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		return nil, nil, false
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

	// If already finished, go to finish page.
	o := orm.NewOrm()
	var att models.KraepelinAttempt
	if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&att); err == nil && att.Id != 0 {
		if att.Status == "finished" {
			c.Redirect("/test/kraepelin/finish", 302)
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
		c.Redirect("/test/kraepelin/finish", 302)
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
		rawIdx = 1
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

	// Finish if requested or last column
	finishNow := p.ForceFinish || p.ColumnIndex == 39
	if finishNow {
		att.Status = "finished"
		att.FinishedAt = time.Now()
		// Mark invitation used
		if inv.Status != models.StatusInvitationUsed {
			inv.Status = models.StatusInvitationUsed
			inv.UsedAt = time.Now()
			_, _ = o.Update(inv, "Status", "UsedAt")
		}
	}

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

	next := "/test/kraepelin/questions"
	if finishNow {
		next = "/test/kraepelin/finish"
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

	var att models.KraepelinAttempt
	if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).One(&att); err != nil || att.Id == 0 {
		c.Redirect("/test/kraepelin/finish", 302)
		return
	}

	// Parse counts (raw app: 40 kolom).
	counts := make([]int, 40)
	if strings.TrimSpace(att.CorrectCountsJSON) != "" {
		_ = json.Unmarshal([]byte(att.CorrectCountsJSON), &counts)
	}
	if len(counts) != 40 {
		counts = make([]int, 40)
	}

	// Faktor-faktor
	maxY := 0
	minY := 0
	if len(counts) > 0 {
		maxY = counts[0]
		minY = counts[0]
		for _, v := range counts {
			if v > maxY {
				maxY = v
			}
			if v < minY {
				minY = v
			}
		}
	}
	// Garis setimbang: (puncak tertinggi + puncak terendah)/2
	balanceLine := float64(maxY+minY) / 2.0
	aboveCount := 0
	onLineCount := 0
	for _, v := range counts {
		if float64(v) > balanceLine {
			aboveCount++
		}
		if float64(v) == balanceLine {
			onLineCount++
		}
	}
	// Panker: (2 * jumlah angka di atas garis setimbang - angka di garis setimbang) / 40
	// "angka di garis setimbang" ditafsirkan sebagai jumlah titik yang tepat di garis setimbang.
	panker := (2.0*float64(aboveCount) - float64(onLineCount)) / 40.0
	// Janker: skor tertinggi - skor terendah
	janker := maxY - minY
	// Hanker: y(0), y(40), y(40)-y(0)
	y0 := 0
	yLast := 0
	if len(counts) > 0 {
		y0 = counts[0]
		yLast = counts[len(counts)-1]
	}
	hankerDiff := yLast - y0
	// Tianker: sum of error + sum of skippeds
	tianker := att.TotalErrors + att.TotalSkipped

	f := excelize.NewFile()
	sheet := "Kraepelin"
	f.SetSheetName(f.GetSheetName(0), sheet)
	showGridLines := false
	_ = f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGridLines})

	_ = f.SetColWidth(sheet, "A", "A", 10)
	_ = f.SetColWidth(sheet, "B", "B", 12)
	_ = f.SetColWidth(sheet, "D", "E", 20)
	_ = f.SetColWidth(sheet, "G", "N", 10)
	_ = f.SetColWidth(sheet, "C", "C", 4)
	_ = f.SetColWidth(sheet, "F", "F", 4)

	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	labelStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left"},
	})
	valueStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left"},
	})
	xyHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#C8A2FF"}},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	xColStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 10},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#F8CBED"}},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	yColStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 10},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	boxTitleStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 10},
		Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#F2F2F2"}},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	boxCellStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Size: 10},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
		Alignment: &excelize.Alignment{Horizontal: "left"},
	})

	// Header biodata sederhana (akan disempurnakan supaya mirip template gambar)
	_ = f.SetCellValue(sheet, "A1", "TES KRAEPELIN")
	_ = f.MergeCell(sheet, "A1", "E1")
	_ = f.SetCellStyle(sheet, "A1", "E1", titleStyle)
	_ = f.SetCellValue(sheet, "A3", "Nama")
	_ = f.SetCellValue(sheet, "B3", att.TestName)
	_ = f.SetCellValue(sheet, "A4", "Jenis kelamin")
	_ = f.SetCellValue(sheet, "B4", att.TestGender)
	_ = f.SetCellValue(sheet, "A5", "Pendidikan")
	_ = f.SetCellValue(sheet, "B5", att.TestEducation)
	_ = f.SetCellValue(sheet, "A6", "Jurusan")
	_ = f.SetCellValue(sheet, "B6", att.TestMajor)
	_ = f.SetCellValue(sheet, "A7", "Tanggal tes")
	_ = f.SetCellValue(sheet, "B7", att.TestDate.Format("2006-01-02 15:04"))
	_ = f.SetCellValue(sheet, "A8", "Tester")
	_ = f.SetCellValue(sheet, "B8", att.Tester)
	_ = f.SetCellStyle(sheet, "A3", "A8", labelStyle)
	_ = f.SetCellStyle(sheet, "B3", "B8", valueStyle)

	// Table input (x=1..40, y=benar per kolom) sesuai jumlah raw data tes.
	startRow := 11
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", startRow), "x")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", startRow), "y")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", startRow), fmt.Sprintf("B%d", startRow), xyHeaderStyle)
	for i := 0; i < 40; i++ {
		r := startRow + 1 + i
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", r), i+1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", r), counts[i])
	}
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", startRow+1), fmt.Sprintf("A%d", startRow+40), xColStyle)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("B%d", startRow+1), fmt.Sprintf("B%d", startRow+40), yColStyle)

	// Kode pendidikan boxes (template-like static reference, 4 blok).
	writeEduBox := func(title string, start int, rows []string) {
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", start), title)
		_ = f.MergeCell(sheet, fmt.Sprintf("D%d", start), fmt.Sprintf("E%d", start))
		_ = f.SetCellStyle(sheet, fmt.Sprintf("D%d", start), fmt.Sprintf("E%d", start), boxTitleStyle)
		for i, v := range rows {
			r := start + 1 + i
			_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", r), v)
			_ = f.MergeCell(sheet, fmt.Sprintf("D%d", r), fmt.Sprintf("E%d", r))
		}
		_ = f.SetCellStyle(sheet, fmt.Sprintf("D%d", start+1), fmt.Sprintf("E%d", start+len(rows)), boxCellStyle)
	}
	writeEduBox("Kode pendidikan Panker", 11, []string{
		"1  SMEA", "2  STM", "3  SMA IPA-IPS", "4  Sarjana muda ilmu sosial",
		"5  Sarjana muda ilmu sosial (P)", "6  Sarjana muda ilmu eksakta",
		"7  Sarjana ilmu sosial", "8  Sarjana ilmu eksakta",
	})
	writeEduBox("Kode pendidikan Janker", 21, []string{
		"1  SMEA", "2  STM", "3  SMA IPA-IPS", "4  Sarjana muda IPA-IPS", "5  Sarjana IPA-IPS",
	})
	writeEduBox("Kode pendidikan Hanker", 28, []string{
		"1  SMEA", "2  STM", "3  SMA IPA-IPS", "4  Sarjana muda IPS (L)",
		"5  Sarjana muda IPS (P)", "6  Sarjana IPA", "7  Sarjana IPS",
	})
	writeEduBox("Kode pendidikan Tianker", 37, []string{
		"1  SMEA", "2  STM", "3  SMA IPA", "4  SMA IPS", "5  SMA (L)",
		"6  SMA (P)", "7  Sarjana muda IPA-IPS (L/P)", "8  Sarjana IPA-IPS",
	})

	// Ringkasan atas: hanya Sum of Error / Sum of Skippeds (tanpa daftar faktor dobel).
	_ = f.SetCellValue(sheet, "H3", "Sum of Error")
	_ = f.SetCellValue(sheet, "I3", att.TotalErrors)
	_ = f.SetCellValue(sheet, "H4", "Sum of Skippeds")
	_ = f.SetCellValue(sheet, "I4", att.TotalSkipped)
	_ = f.SetCellStyle(sheet, "H3", "H4", labelStyle)
	_ = f.SetCellStyle(sheet, "I3", "I4", yColStyle)

	// Blok analisis sederhana (layout mirip template).
	// Letakkan blok panker dkk di bawah sum, dengan jarak agar tidak terlalu rapat.
	_ = f.SetCellValue(sheet, "H10", "Pembulatan")
	_ = f.SetCellValue(sheet, "I10", "Analisis")
	analysisHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	_ = f.SetCellStyle(sheet, "H10", "I10", analysisHeaderStyle)
	_ = f.SetCellValue(sheet, "G11", "Panker")
	_ = f.SetCellValue(sheet, "G12", "Janker")
	_ = f.SetCellValue(sheet, "G13", "Hanker y(40)-y(0)")
	_ = f.SetCellValue(sheet, "G14", "Tianker")
	_ = f.SetCellValue(sheet, "H11", fmt.Sprintf("%.2f", panker))
	_ = f.SetCellValue(sheet, "H12", fmt.Sprintf("%.2f", float64(janker)))
	_ = f.SetCellValue(sheet, "H13", fmt.Sprintf("%.2f", float64(hankerDiff)))
	_ = f.SetCellValue(sheet, "H14", fmt.Sprintf("%d", tianker))
	_ = f.SetCellValue(sheet, "I11", "(speed factor)")
	_ = f.SetCellValue(sheet, "I12", "(rhitme factor)")
	_ = f.SetCellValue(sheet, "I13", "(ausdeur factor)")
	_ = f.SetCellValue(sheet, "I14", "(tianker)")
	redValueStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "#000000"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#FF0000"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	_ = f.SetCellStyle(sheet, "H11", "H14", redValueStyle)
	_ = f.SetCellStyle(sheet, "G11", "G14", labelStyle)
	_ = f.SetCellStyle(sheet, "I11", "I14", valueStyle)

	// Pastikan tidak ada placeholder tombol analisis dari template lama.
	_ = f.SetCellValue(sheet, "L6", "")
	_ = f.SetCellValue(sheet, "L7", "")
	_ = f.SetCellValue(sheet, "L8", "")
	_ = f.SetCellValue(sheet, "L9", "")
	_ = f.SetCellValue(sheet, "G6", "")
	_ = f.SetCellValue(sheet, "G7", "")
	_ = f.SetCellValue(sheet, "G8", "")
	_ = f.SetCellValue(sheet, "G9", "")
	_ = f.SetCellValue(sheet, "H6", "")
	_ = f.SetCellValue(sheet, "H7", "")
	_ = f.SetCellValue(sheet, "H8", "")
	_ = f.SetCellValue(sheet, "H9", "")
	_ = f.SetCellValue(sheet, "I6", "")
	_ = f.SetCellValue(sheet, "I7", "")
	_ = f.SetCellValue(sheet, "I8", "")
	_ = f.SetCellValue(sheet, "I9", "")
	cleanStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "left"},
	})
	_ = f.SetCellStyle(sheet, "G6", "I9", cleanStyle)

	// Add line chart langsung di sheet utama agar user langsung lihat seperti template.
	categories := fmt.Sprintf("%s!$A$12:$A$51", sheet)
	values := fmt.Sprintf("%s!$B$12:$B$51", sheet)
	_ = f.SetCellValue(sheet, "D46", "GRAFIK HASIL TES KRAEPELIN")
	_ = f.MergeCell(sheet, "D46", "N46")
	_ = f.SetCellStyle(sheet, "D46", "N46", titleStyle)
	xMin := 1.0
	xMax := 50.0
	xMajor := 1.0
	yMin := 0.0
	yMax := 35.0
	yMajor := 1.0
	chartErr := f.AddChart(sheet, "C47", &excelize.Chart{
		Type: excelize.Line,
		Series: []excelize.ChartSeries{
			{
				Name:       "Nilai y",
				Categories: categories,
				Values:     values,
				Marker: excelize.ChartMarker{
					Symbol: "none",
					Size:   0,
				},
			},
		},
		XAxis: excelize.ChartAxis{
			Minimum:   &xMin,
			Maximum:   &xMax,
			MajorUnit: xMajor,
		},
		YAxis: excelize.ChartAxis{
			Minimum:   &yMin,
			Maximum:   &yMax,
			MajorUnit: yMajor,
		},
		Legend: excelize.ChartLegend{Position: "bottom"},
	})
	if chartErr != nil {
		// Fallback chart lebih sederhana untuk kompatibilitas excelize versi lama.
		_ = f.AddChart(sheet, "C47", &excelize.Chart{
			Type: excelize.Line,
			Series: []excelize.ChartSeries{
				{
					Name:       "Nilai y",
					Categories: categories,
					Values:     values,
					Marker: excelize.ChartMarker{
						Symbol: "none",
						Size:   0,
					},
				},
			},
			XAxis: excelize.ChartAxis{
				Minimum:   &xMin,
				Maximum:   &xMax,
				MajorUnit: xMajor,
			},
			YAxis: excelize.ChartAxis{
				Minimum:   &yMin,
				Maximum:   &yMax,
				MajorUnit: yMajor,
			},
		})
	}

	buf, err := f.WriteToBuffer()
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
	downloadName := strings.TrimSpace(att.TestName)
	if downloadName == "" {
		downloadName = strings.TrimSpace(user.NamaLengkap)
	}
	if downloadName == "" {
		downloadName = strings.TrimSpace(user.Email)
	}
	if downloadName == "" {
		downloadName = strings.TrimSpace(inv.Email)
	}
	filename := fmt.Sprintf("kraepelin_result_%s.xlsx", makeSafeName(downloadName))
	c.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(buf.Bytes())
}

