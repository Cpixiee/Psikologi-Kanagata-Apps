package controllers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/xuri/excelize/v2"
)

// PAPITestController menangani alur pengerjaan tes PAPI (90 item, 60 menit)
type PAPITestController struct {
	beego.Controller
}

// papiCategoryOrder adalah urutan standar 20 kode PAPI (10 Peran + 10 Kebutuhan)
var papiCategoryOrder = []string{
	// PERAN (Roles)
	"G", "L", "I", "T", "V", "S", "R", "D", "C", "E",
	// KEBUTUHAN (Needs)
	"N", "A", "P", "X", "B", "O", "Z", "K", "F", "W",
}

// papiCategoryLabel mengubah kode menjadi label deskriptif (Bahasa Indonesia)
var papiCategoryLabel = map[string]string{
	// PERAN
	"G": "Pekerja keras",
	"L": "Kepemimpinan",
	"I": "Mudah membuat keputusan",
	"T": "Tipe orang yang sibuk",
	"V": "Tipe orang yang bersemangat",
	"S": "Hubungan sosial / pergaulan luas",
	"R": "Tipe teoritis",
	"D": "Tipe orang teratur / bekerja detail",
	"C": "Mengatur / mengorganisir",
	"E": "Pengendalian emosi",
	// KEBUTUHAN
	"N": "Penyelesaian mandiri",
	"A": "Kebutuhan untuk berprestasi",
	"P": "Mengatur orang lain",
	"X": "Untuk mendapat perhatian",
	"B": "Diterima dalam kelompok",
	"O": "Hubungan akrab / intim",
	"Z": "Hasrat untuk berubah",
	"K": "Agresi",
	"F": "Mendukung / membantu atasan",
	"W": "Mengikuti aturan",
}

// papiCategoryGroup mengelompokkan setiap kode ke "Peran" atau "Kebutuhan"
var papiCategoryGroup = map[string]string{
	"G": "Peran", "L": "Peran", "I": "Peran", "T": "Peran", "V": "Peran",
	"S": "Peran", "R": "Peran", "D": "Peran", "C": "Peran", "E": "Peran",
	"N": "Kebutuhan", "A": "Kebutuhan", "P": "Kebutuhan", "X": "Kebutuhan", "B": "Kebutuhan",
	"O": "Kebutuhan", "Z": "Kebutuhan", "K": "Kebutuhan", "F": "Kebutuhan", "W": "Kebutuhan",
}

const papiTotalItems = 90
const papiTimeLimitMinutes = 30

func (c *PAPITestController) mustGetSessionInvitation() (*models.TestInvitation, *models.User, bool) {
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
	return &inv, &user, true
}

// getOrCreateSession memastikan ada satu PAPISession untuk invitation ini
func (c *PAPITestController) getOrCreateSession(inv *models.TestInvitation, user *models.User) (*models.PAPISession, error) {
	o := orm.NewOrm()
	var session models.PAPISession
	err := o.QueryTable(new(models.PAPISession)).Filter("Invitation__Id", inv.Id).One(&session)
	if err == nil && session.Id != 0 {
		return &session, nil
	}

	// Pastikan soal PAPI tersedia
	cnt, _ := o.QueryTable(new(models.PAPIQuestion)).Count()
	if cnt == 0 {
		return nil, fmt.Errorf("soal PAPI belum tersedia, hubungi admin")
	}

	session = models.PAPISession{
		Invitation:           inv,
		User:                 user,
		BatchId:              inv.BatchId,
		Status:               "in_progress",
		TimeLimitMinutes:     papiTimeLimitMinutes,
		TimeRemainingSeconds: papiTimeLimitMinutes * 60,
	}
	if _, err := o.Insert(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *PAPITestController) getAllQuestions() ([]models.PAPIQuestion, error) {
	o := orm.NewOrm()
	var qs []models.PAPIQuestion
	_, err := o.QueryTable(new(models.PAPIQuestion)).
		OrderBy("item_number").
		All(&qs)
	return qs, err
}

func (c *PAPITestController) getAnswersBySession(sessionID int) (map[int]string, error) {
	o := orm.NewOrm()
	type row struct {
		QuestionID     int    `orm:"column(question_id)"`
		SelectedOption string `orm:"column(selected_option)"`
	}
	var rows []row
	_, err := o.Raw(`
		SELECT question_id, selected_option
		FROM papi_answers
		WHERE session_id = ?
	`, sessionID).QueryRows(&rows)
	if err != nil {
		return nil, err
	}
	m := make(map[int]string, len(rows))
	for _, r := range rows {
		m[r.QuestionID] = r.SelectedOption
	}
	return m, nil
}

// =========================
// PAGE: /test/papi/start
// =========================
func (c *PAPITestController) StartPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	// Jika hasil sudah ada, arahkan ke tes berikutnya di batch (jika ada), atau ke /hasil-tes.
	o := orm.NewOrm()
	var existing models.PAPIResult
	if err := o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&existing); err == nil && existing.Id != 0 {
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
	}

	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Data["Error"] = err.Error()
	} else if session != nil {
		answers, _ := c.getAnswersBySession(session.Id)
		if len(answers) > 0 {
			c.Redirect("/test/papi/questions", 302)
			return
		}
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["TotalItems"] = papiTotalItems
	c.Data["TimeLimitMinutes"] = papiTimeLimitMinutes
	c.TplName = "test_papi_start.html"
}

// =========================
// PAGE: /test/papi/instruction
// =========================
func (c *PAPITestController) InstructionPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	_, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Redirect("/test/papi/start", 302)
		return
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["TotalItems"] = papiTotalItems
	c.Data["TimeLimitMinutes"] = papiTimeLimitMinutes
	c.TplName = "test_papi_instruction.html"
}

// =========================
// PAGE: /test/papi/questions
// =========================
type papiQuestionView struct {
	ID             int
	ItemNumber     int
	OptionA        string
	OptionB        string
	CategoryA      string
	CategoryB      string
	SelectedOption string // "" jika belum dijawab
}

func (c *PAPITestController) QuestionsPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Redirect("/test/papi/start", 302)
		return
	}

	// Cek apakah session sudah expired
	if session.Status == "expired" {
		c.Redirect("/test/papi/expired", 302)
		return
	}

	qs, err := c.getAllQuestions()
	if err != nil || len(qs) != papiTotalItems {
		c.Data["Error"] = "Soal PAPI belum lengkap."
		c.TplName = "test_papi_questions.html"
		return
	}

	answers, _ := c.getAnswersBySession(session.Id)

	items := make([]papiQuestionView, 0, len(qs))
	for _, q := range qs {
		items = append(items, papiQuestionView{
			ID:             q.Id,
			ItemNumber:     q.ItemNumber,
			OptionA:        q.OptionA,
			OptionB:        q.OptionB,
			CategoryA:      q.CategoryA,
			CategoryB:      q.CategoryB,
			SelectedOption: answers[q.Id],
		})
	}

	// Hitung progress
	answered := 0
	for _, ans := range answers {
		if ans != "" {
			answered++
		}
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["Session"] = session
	c.Data["Items"] = items
	c.Data["TotalItems"] = papiTotalItems
	c.Data["Answered"] = answered
	c.Data["ProgressPercent"] = int(float64(answered) / float64(papiTotalItems) * 100)
	c.Data["IsDev"] = strings.EqualFold(beego.BConfig.RunMode, "dev")
	c.TplName = "test_papi_questions.html"
}

// =========================
// API: POST /api/test/papi/answer (auto-save UPSERT 1 jawaban)
// Body: { question_id, selected_option }
// =========================
func (c *PAPITestController) SaveAnswerAPI() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	var payload struct {
		QuestionID     int    `json:"question_id"`
		SelectedOption string `json:"selected_option"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}
	if payload.QuestionID <= 0 || (payload.SelectedOption != "A" && payload.SelectedOption != "B") {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "question_id / selected_option tidak valid"}
		c.ServeJSON()
		return
	}

	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": err.Error()}
		c.ServeJSON()
		return
	}

	// Cek apakah session sudah expired
	if session.Status == "expired" {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi telah berakhir"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	// Validasi: question harus ada
	var q models.PAPIQuestion
	q.Id = payload.QuestionID
	if err := o.Read(&q); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal tidak ditemukan"}
		c.ServeJSON()
		return
	}

	// UPSERT (PostgreSQL)
	_, err = o.Raw(`
		INSERT INTO papi_answers (session_id, question_id, selected_option, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (session_id, question_id)
		DO UPDATE SET selected_option = EXCLUDED.selected_option, updated_at = CURRENT_TIMESTAMP
	`, session.Id, q.Id, payload.SelectedOption).Exec()
	if err != nil {
		logs.Error("PAPI SaveAnswerAPI upsert error: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal menyimpan jawaban"}
		c.ServeJSON()
		return
	}

	c.Data["json"] = map[string]interface{}{"success": true}
	c.ServeJSON()
}

// =========================
// API: POST /test/papi/submit (finalisasi & hitung skor)
// =========================
func (c *PAPITestController) SubmitFinal() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Redirect("/test/papi/start", 302)
		return
	}

	// Cek apakah semua jawaban sudah terisi
	answers, err := c.getAnswersBySession(session.Id)
	if err != nil || len(answers) != papiTotalItems {
		c.Redirect("/test/papi/questions", 302)
		return
	}

	for _, ans := range answers {
		if ans == "" {
			c.Redirect("/test/papi/questions", 302)
			return
		}
	}

	nextURL, err := c.finalizePAPI(inv, user, session)
	if err != nil {
		logs.Error("PAPI SubmitFinal finalize error: %v", err)
		c.Redirect("/test/papi/questions", 302)
		return
	}

	if nextURL != "" {
		c.Redirect(nextURL, 302)
	} else {
		c.Redirect("/hasil-tes", 302)
	}
}

// finalizePAPI menghitung skor + UPSERT PAPIResult + tandai session/invitation selesai
func (c *PAPITestController) finalizePAPI(inv *models.TestInvitation, user *models.User, session *models.PAPISession) (string, error) {
	o := orm.NewOrm()

	type answerRow struct {
		CategoryA string `orm:"column(category_a)"`
		CategoryB string `orm:"column(category_b)"`
		Option    string `orm:"column(selected_option)"`
	}
	var rows []answerRow
	if _, err := o.Raw(`
		SELECT q.category_a, q.category_b, a.selected_option
		FROM papi_answers a
		JOIN papi_questions q ON q.id = a.question_id
		WHERE a.session_id = ?
	`, session.Id).QueryRows(&rows); err != nil {
		return "", err
	}

	// Hitung skor per kategori
	scoreMap := map[string]int{}
	for _, code := range papiCategoryOrder {
		scoreMap[code] = 0
	}

	for _, r := range rows {
		var selectedCategory string
		switch r.Option {
		case "A":
			selectedCategory = r.CategoryA
		case "B":
			selectedCategory = r.CategoryB
		}
		if selectedCategory != "" {
			scoreMap[selectedCategory]++
		}
	}

	type catScore struct {
		Code  string
		Score int
		Idx   int
	}
	ranked := make([]catScore, 0, len(papiCategoryOrder))
	for i, code := range papiCategoryOrder {
		ranked = append(ranked, catScore{Code: code, Score: scoreMap[code], Idx: i})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score > ranked[j].Score // PAPI: skor lebih tinggi = lebih dominan
		}
		return ranked[i].Idx < ranked[j].Idx
	})

	type resultEntry struct {
		Label string `json:"label"`
		Score int    `json:"score"`
		Rank  int    `json:"rank"`
	}
	resultMap := map[string]resultEntry{}
	for i, cs := range ranked {
		resultMap[cs.Code] = resultEntry{
			Label: papiCategoryLabel[cs.Code],
			Score: cs.Score,
			Rank:  i + 1,
		}
	}
	resJSON, _ := json.Marshal(resultMap)

	dominant := ranked[0].Code
	topCategories := []string{ranked[0].Code, ranked[1].Code, ranked[2].Code}
	topCategoriesJSON, _ := json.Marshal(topCategories)

	// Cari top Peran dan top Kebutuhan terpisah (karena PAPI membedakan keduanya)
	var topPeran, topKeb *catScore
	for i := range ranked {
		if topPeran == nil && papiCategoryGroup[ranked[i].Code] == "Peran" {
			topPeran = &ranked[i]
		}
		if topKeb == nil && papiCategoryGroup[ranked[i].Code] == "Kebutuhan" {
			topKeb = &ranked[i]
		}
		if topPeran != nil && topKeb != nil {
			break
		}
	}

	interpParts := []string{
		"Berdasarkan hasil tes PAPI, Anda memiliki kecenderungan tertinggi pada:",
		fmt.Sprintf("1) %s (%s, skor %d)", papiCategoryLabel[ranked[0].Code], papiCategoryGroup[ranked[0].Code], ranked[0].Score),
		fmt.Sprintf("2) %s (%s, skor %d)", papiCategoryLabel[ranked[1].Code], papiCategoryGroup[ranked[1].Code], ranked[1].Score),
		fmt.Sprintf("3) %s (%s, skor %d)", papiCategoryLabel[ranked[2].Code], papiCategoryGroup[ranked[2].Code], ranked[2].Score),
	}
	if topPeran != nil {
		interpParts = append(interpParts, fmt.Sprintf("\nPeran dominan: %s (%s, skor %d)", topPeran.Code, papiCategoryLabel[topPeran.Code], topPeran.Score))
	}
	if topKeb != nil {
		interpParts = append(interpParts, fmt.Sprintf("Kebutuhan dominan: %s (%s, skor %d)", topKeb.Code, papiCategoryLabel[topKeb.Code], topKeb.Score))
	}
	interpParts = append(interpParts, "\nSemakin tinggi skor suatu kategori, semakin dominan aspek tersebut dalam kepribadian Anda.")
	interp := strings.Join(interpParts, "\n")

	// Hitung waktu yang diperlukan
	timeTaken := int(time.Since(session.StartedAt).Minutes())
	if timeTaken < 1 {
		timeTaken = 1
	}

	var existing models.PAPIResult
	err := o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&existing)
	if err != nil || existing.Id == 0 {
		newRes := models.PAPIResult{
			Invitation:     inv,
			User:           user,
			ResultJSON:     string(resJSON),
			DominantCategory: dominant,
			TopCategories:  string(topCategoriesJSON),
			Interpretation: interp,
			CompletedAt:    time.Now(),
			TimeTakenMinutes: timeTaken,
		}
		if _, err := o.Insert(&newRes); err != nil {
			return "", err
		}
	} else {
		existing.ResultJSON = string(resJSON)
		existing.DominantCategory = dominant
		existing.TopCategories = string(topCategoriesJSON)
		existing.Interpretation = interp
		existing.CompletedAt = time.Now()
		existing.TimeTakenMinutes = timeTaken
		if _, err := o.Update(&existing,
			"ResultJSON", "DominantCategory", "TopCategories",
			"Interpretation", "CompletedAt", "TimeTakenMinutes",
		); err != nil {
			return "", err
		}
	}

	session.Status = "finished"
	session.CompletedAt = time.Now()
	_, _ = o.Update(session, "Status", "CompletedAt")

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
		if inv.Status != models.StatusInvitationUsed {
			inv.Status = models.StatusInvitationUsed
			inv.UsedAt = time.Now()
			_, _ = o.Update(inv, "Status", "UsedAt")
			go utils.SendTestCompletionNotification(inv.UserId, "PAPI Kostick")
		}
	}

	return redirectURL, nil
}

// =========================
// PAGE: /test/papi/finish
// Tidak ada halaman finish/result terpisah; selalu lempar ke /profile/papi.
// =========================
func (c *PAPITestController) FinishPage() {
	c.Redirect("/profile/papi", 302)
}

// =========================
// PAGE: /test/papi/result (bar chart + tabel detail)
// =========================
type papiChartRow struct {
	Code        string
	Label       string
	Score       int
	Rank        int
	BarPercent  int
	IsTop3      bool
	Category    string
	CategoryCSS string
}

func papiCategoryLabelFromRank(rank int) (string, string) {
	// PAPI memiliki 20 kategori (10 Peran + 10 Kebutuhan)
	switch {
	case rank == 0:
		return "-", "bg-secondary"
	case rank <= 3:
		return "Sangat Dominan", "bg-success"
	case rank <= 7:
		return "Dominan", "bg-primary"
	case rank <= 13:
		return "Sedang", "bg-info"
	case rank <= 17:
		return "Rendah", "bg-warning"
	default:
		return "Sangat Rendah", "bg-secondary"
	}
}

// ResultPage tidak lagi merender halaman hasil terpisah.
// Hasil PAPI ditampilkan di halaman profil (/profile/papi).
func (c *PAPITestController) ResultPage() {
	c.Redirect("/profile/papi", 302)
}

// =========================
// DEV ONLY: POST /test/papi/dev-autofill
// Mengisi 90 jawaban secara acak (A/B) lalu langsung finalize.
// Hanya aktif jika RunMode = "dev".
// =========================
func (c *PAPITestController) DevAutoFill() {
	if !strings.EqualFold(beego.BConfig.RunMode, "dev") {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Endpoint hanya tersedia di mode development"}
		c.ServeJSON()
		return
	}

	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}
	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": err.Error()}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	options := []string{"A", "B"}

	qs, err := c.getAllQuestions()
	if err != nil || len(qs) != papiTotalItems {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal PAPI belum lengkap"}
		c.ServeJSON()
		return
	}

	for _, q := range qs {
		selectedOption := options[rng.Intn(len(options))]
		if _, err := o.Raw(`
			INSERT INTO papi_answers (session_id, question_id, selected_option, updated_at)
			VALUES (?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT (session_id, question_id)
			DO UPDATE SET selected_option = EXCLUDED.selected_option, updated_at = CURRENT_TIMESTAMP
		`, session.Id, q.Id, selectedOption).Exec(); err != nil {
			logs.Error("PAPI DevAutoFill upsert error: %v", err)
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal menyimpan jawaban acak"}
			c.ServeJSON()
			return
		}
	}

	// Jalankan finalisasi (perhitungan + simpan PAPIResult).
	nextURL, err := c.finalizePAPI(inv, user, session)
	if err != nil {
		logs.Error("PAPI DevAutoFill finalize error: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal finalisasi"}
		c.ServeJSON()
		return
	}

	if nextURL == "" {
		nextURL = "/profile/papi"
	}

	c.Data["json"] = map[string]interface{}{"success": true, "next": nextURL}
	c.ServeJSON()
}

// =========================
// PAGE: /test/papi/result/excel
// =========================
func (c *PAPITestController) ExportResultExcel() {
	invIDStr := c.GetString("invId")
	if invIDStr == "" {
		c.Ctx.Output.SetStatus(400)
		c.Ctx.WriteString("invId parameter required")
		return
	}

	invID, err := strconv.Atoi(invIDStr)
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Ctx.WriteString("Invalid invId")
		return
	}

	o := orm.NewOrm()
	var inv models.TestInvitation
	inv.Id = invID
	if err := o.Read(&inv); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Ctx.WriteString("Invitation not found")
		return
	}

	var res models.PAPIResult
	if err := o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", invID).One(&res); err != nil || res.Id == 0 {
		c.Ctx.Output.SetStatus(404)
		c.Ctx.WriteString("PAPI result not found")
		return
	}

	// Load user untuk header NISN/NIP, Kelas, Jurusan
	var user models.User
	if inv.UserId != nil {
		user.Id = *inv.UserId
		_ = o.Read(&user)
	}

	// Parse result JSON
	type resultEntry struct {
		Label string `json:"label"`
		Score int    `json:"score"`
		Rank  int    `json:"rank"`
	}
	parsed := map[string]resultEntry{}
	if err := json.Unmarshal([]byte(res.ResultJSON), &parsed); err != nil {
		logs.Error("PAPI ExportResultExcel parse error: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Ctx.WriteString("Error parsing result data")
		return
	}

	// Ambil semua jawaban + soal untuk detail dan worksheet
	type detailRow struct {
		ItemNumber int
		OptionA    string
		OptionB    string
		CategoryA  string
		CategoryB  string
		Selected   string // "A" atau "B" (atau "" jika kosong)
	}
	var details []detailRow
	if _, derr := o.Raw(`
		SELECT q.item_number, q.option_a, q.option_b, q.category_a, q.category_b,
		       COALESCE(a.selected_option, '') AS selected
		FROM papi_questions q
		LEFT JOIN papi_answers a ON a.question_id = q.id
		    AND a.session_id = (SELECT id FROM papi_sessions WHERE invitation_id = ? LIMIT 1)
		ORDER BY q.item_number
	`, invID).QueryRows(&details); derr != nil {
		logs.Error("PAPI ExportResultExcel detail query error: %v", derr)
	}

	// ===== Buat Excel dengan beberapa sheet =====
	f := excelize.NewFile()

	// Style umum
	titleStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2E75B6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	cellStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "left", Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	centerStyle, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	highlightStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "1F4E79"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#FFE699"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})

	// =========================
	// SHEET 1: Ringkasan
	// =========================
	sumSheet := "Ringkasan"
	f.SetSheetName("Sheet1", sumSheet)

	// Identitas
	f.MergeCell(sumSheet, "A1", "F1")
	f.SetCellValue(sumSheet, "A1", "HASIL TES PAPI (Personality and Preference Inventory)")
	f.SetCellStyle(sumSheet, "A1", "F1", titleStyle)
	f.SetRowHeight(sumSheet, 1, 26)

	nisnNip := strings.TrimSpace(user.NISN)
	idLabel := "NISN"
	if nisnNip == "" && strings.TrimSpace(user.NIP) != "" {
		nisnNip = user.NIP
		idLabel = "NIP"
	}
	if nisnNip == "" {
		idLabel = "NISN/NIP"
	}
	nama := strings.TrimSpace(user.NamaLengkap)
	if nama == "" {
		nama = inv.Email
	}
	infoRows := [][]string{
		{"Nama Peserta", nama},
		{idLabel, nisnNip},
		{"Kelas", user.Kelas},
		{"Jurusan", user.Jurusan},
		{"Email Peserta", inv.Email},
		{"Tanggal Tes", res.CompletedAt.Format("02 January 2006, 15:04")},
		{"Waktu Pengerjaan", fmt.Sprintf("%d menit", res.TimeTakenMinutes)},
		{"Kategori Dominan", fmt.Sprintf("%s - %s", res.DominantCategory, papiCategoryLabel[res.DominantCategory])},
	}
	for i, ir := range infoRows {
		r := i + 3
		f.SetCellValue(sumSheet, fmt.Sprintf("A%d", r), ir[0])
		f.MergeCell(sumSheet, fmt.Sprintf("B%d", r), fmt.Sprintf("F%d", r))
		f.SetCellValue(sumSheet, fmt.Sprintf("B%d", r), ir[1])
	}

	// PERAN section (shifted to accommodate 4 baris identitas tambahan)
	peranHeaderRow := 3 + len(infoRows) + 1
	f.MergeCell(sumSheet, fmt.Sprintf("A%d", peranHeaderRow), fmt.Sprintf("F%d", peranHeaderRow))
	f.SetCellValue(sumSheet, fmt.Sprintf("A%d", peranHeaderRow), "PERAN (Roles)")
	f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", peranHeaderRow), fmt.Sprintf("F%d", peranHeaderRow), titleStyle)

	tblHeaderRow := peranHeaderRow + 1
	f.SetCellValue(sumSheet, fmt.Sprintf("A%d", tblHeaderRow), "No")
	f.SetCellValue(sumSheet, fmt.Sprintf("B%d", tblHeaderRow), "Kode")
	f.SetCellValue(sumSheet, fmt.Sprintf("C%d", tblHeaderRow), "Deskripsi")
	f.SetCellValue(sumSheet, fmt.Sprintf("D%d", tblHeaderRow), "Skor")
	f.SetCellValue(sumSheet, fmt.Sprintf("E%d", tblHeaderRow), "Ranking")
	f.SetCellValue(sumSheet, fmt.Sprintf("F%d", tblHeaderRow), "Level")
	f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", tblHeaderRow), fmt.Sprintf("F%d", tblHeaderRow), headerStyle)

	peranCodes := []string{"G", "L", "I", "T", "V", "S", "R", "D", "C", "E"}
	kebCodes := []string{"N", "A", "P", "X", "B", "O", "Z", "K", "F", "W"}

	rowi := tblHeaderRow + 1
	for i, code := range peranCodes {
		entry := parsed[code]
		level := "Rendah"
		if entry.Rank == 1 {
			level = "Sangat Dominan"
		} else if entry.Rank <= 3 {
			level = "Dominan"
		} else if entry.Rank <= 7 {
			level = "Sedang"
		}
		f.SetCellValue(sumSheet, fmt.Sprintf("A%d", rowi), i+1)
		f.SetCellValue(sumSheet, fmt.Sprintf("B%d", rowi), code)
		f.SetCellValue(sumSheet, fmt.Sprintf("C%d", rowi), papiCategoryLabel[code])
		f.SetCellValue(sumSheet, fmt.Sprintf("D%d", rowi), entry.Score)
		f.SetCellValue(sumSheet, fmt.Sprintf("E%d", rowi), entry.Rank)
		f.SetCellValue(sumSheet, fmt.Sprintf("F%d", rowi), level)
		f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("A%d", rowi), centerStyle)
		f.SetCellStyle(sumSheet, fmt.Sprintf("B%d", rowi), fmt.Sprintf("B%d", rowi), centerStyle)
		f.SetCellStyle(sumSheet, fmt.Sprintf("C%d", rowi), fmt.Sprintf("C%d", rowi), cellStyle)
		f.SetCellStyle(sumSheet, fmt.Sprintf("D%d", rowi), fmt.Sprintf("E%d", rowi), centerStyle)
		f.SetCellStyle(sumSheet, fmt.Sprintf("F%d", rowi), fmt.Sprintf("F%d", rowi), cellStyle)
		rowi++
	}

	// Total Peran
	totPeran := 0
	for _, c := range peranCodes {
		totPeran += parsed[c].Score
	}
	f.MergeCell(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("C%d", rowi))
	f.SetCellValue(sumSheet, fmt.Sprintf("A%d", rowi), "Total Peran")
	f.SetCellValue(sumSheet, fmt.Sprintf("D%d", rowi), totPeran)
	f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("F%d", rowi), highlightStyle)
	rowi += 2

	// KEBUTUHAN section
	f.MergeCell(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("F%d", rowi))
	f.SetCellValue(sumSheet, fmt.Sprintf("A%d", rowi), "KEBUTUHAN (Needs)")
	f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("F%d", rowi), titleStyle)
	rowi++

	tblHeaderRow2 := rowi
	f.SetCellValue(sumSheet, fmt.Sprintf("A%d", rowi), "No")
	f.SetCellValue(sumSheet, fmt.Sprintf("B%d", rowi), "Kode")
	f.SetCellValue(sumSheet, fmt.Sprintf("C%d", rowi), "Deskripsi")
	f.SetCellValue(sumSheet, fmt.Sprintf("D%d", rowi), "Skor")
	f.SetCellValue(sumSheet, fmt.Sprintf("E%d", rowi), "Ranking")
	f.SetCellValue(sumSheet, fmt.Sprintf("F%d", rowi), "Level")
	f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", tblHeaderRow2), fmt.Sprintf("F%d", tblHeaderRow2), headerStyle)
	rowi++

	for i, code := range kebCodes {
		entry := parsed[code]
		level := "Rendah"
		if entry.Rank == 1 {
			level = "Sangat Dominan"
		} else if entry.Rank <= 3 {
			level = "Dominan"
		} else if entry.Rank <= 7 {
			level = "Sedang"
		}
		f.SetCellValue(sumSheet, fmt.Sprintf("A%d", rowi), i+1)
		f.SetCellValue(sumSheet, fmt.Sprintf("B%d", rowi), code)
		f.SetCellValue(sumSheet, fmt.Sprintf("C%d", rowi), papiCategoryLabel[code])
		f.SetCellValue(sumSheet, fmt.Sprintf("D%d", rowi), entry.Score)
		f.SetCellValue(sumSheet, fmt.Sprintf("E%d", rowi), entry.Rank)
		f.SetCellValue(sumSheet, fmt.Sprintf("F%d", rowi), level)
		f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("A%d", rowi), centerStyle)
		f.SetCellStyle(sumSheet, fmt.Sprintf("B%d", rowi), fmt.Sprintf("B%d", rowi), centerStyle)
		f.SetCellStyle(sumSheet, fmt.Sprintf("C%d", rowi), fmt.Sprintf("C%d", rowi), cellStyle)
		f.SetCellStyle(sumSheet, fmt.Sprintf("D%d", rowi), fmt.Sprintf("E%d", rowi), centerStyle)
		f.SetCellStyle(sumSheet, fmt.Sprintf("F%d", rowi), fmt.Sprintf("F%d", rowi), cellStyle)
		rowi++
	}

	totKeb := 0
	for _, c := range kebCodes {
		totKeb += parsed[c].Score
	}
	f.MergeCell(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("C%d", rowi))
	f.SetCellValue(sumSheet, fmt.Sprintf("A%d", rowi), "Total Kebutuhan")
	f.SetCellValue(sumSheet, fmt.Sprintf("D%d", rowi), totKeb)
	f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("F%d", rowi), highlightStyle)
	rowi += 2

	// Grand total
	f.MergeCell(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("C%d", rowi))
	f.SetCellValue(sumSheet, fmt.Sprintf("A%d", rowi), "Grand Total (Peran + Kebutuhan)")
	f.SetCellValue(sumSheet, fmt.Sprintf("D%d", rowi), totPeran+totKeb)
	f.SetCellStyle(sumSheet, fmt.Sprintf("A%d", rowi), fmt.Sprintf("F%d", rowi), highlightStyle)

	f.SetColWidth(sumSheet, "A", "A", 6)
	f.SetColWidth(sumSheet, "B", "B", 8)
	f.SetColWidth(sumSheet, "C", "C", 38)
	f.SetColWidth(sumSheet, "D", "D", 10)
	f.SetColWidth(sumSheet, "E", "E", 10)
	f.SetColWidth(sumSheet, "F", "F", 18)

	// =========================
	// SHEET 2: Worksheet (visual mirip form PAPI)
	// =========================
	wsName := "Worksheet PAPI"
	if _, err := f.NewSheet(wsName); err == nil {
		// Header peran
		f.MergeCell(wsName, "A1", "L1")
		f.SetCellValue(wsName, "A1", "PERAN")
		f.SetCellStyle(wsName, "A1", "L1", titleStyle)

		// Row 2: kode peran
		f.SetCellValue(wsName, "A2", "Total")
		for i, c := range peranCodes {
			col := string(rune('B' + i))
			f.SetCellValue(wsName, col+"2", c)
		}
		f.SetCellValue(wsName, "L2", "")
		// Row 3: deskripsi peran
		for i, c := range peranCodes {
			col := string(rune('B' + i))
			f.SetCellValue(wsName, col+"3", papiCategoryLabel[c])
		}
		// Row 4: skor peran
		f.SetCellValue(wsName, "A4", totPeran)
		for i, c := range peranCodes {
			col := string(rune('B' + i))
			f.SetCellValue(wsName, col+"4", parsed[c].Score)
		}
		f.SetCellStyle(wsName, "A2", "L4", headerStyle)

		// Row 5+: per-item dengan kode terpilih ditandai
		f.SetCellValue(wsName, "A5", "No")
		f.SetCellValue(wsName, "B5", "Pilihan")
		f.SetCellValue(wsName, "C5", "Kode (peran/kebutuhan)")
		f.MergeCell(wsName, "C5", "L5")
		f.SetCellStyle(wsName, "A5", "L5", headerStyle)
		wr := 6
		for _, d := range details {
			f.SetCellValue(wsName, fmt.Sprintf("A%d", wr), d.ItemNumber)
			pickedCode := ""
			pickedLabel := ""
			if d.Selected == "A" {
				pickedCode = d.CategoryA
				pickedLabel = "Atas (A)"
			} else if d.Selected == "B" {
				pickedCode = d.CategoryB
				pickedLabel = "Bawah (B)"
			} else {
				pickedLabel = "(kosong)"
			}
			f.SetCellValue(wsName, fmt.Sprintf("B%d", wr), pickedLabel)
			combo := fmt.Sprintf("Atas=%s | Bawah=%s | Skor masuk: %s",
				d.CategoryA, d.CategoryB, pickedCode)
			f.MergeCell(wsName, fmt.Sprintf("C%d", wr), fmt.Sprintf("L%d", wr))
			f.SetCellValue(wsName, fmt.Sprintf("C%d", wr), combo)
			f.SetCellStyle(wsName, fmt.Sprintf("A%d", wr), fmt.Sprintf("B%d", wr), centerStyle)
			f.SetCellStyle(wsName, fmt.Sprintf("C%d", wr), fmt.Sprintf("L%d", wr), cellStyle)
			wr++
		}

		// Footer Kebutuhan
		wr++
		f.MergeCell(wsName, fmt.Sprintf("A%d", wr), fmt.Sprintf("L%d", wr))
		f.SetCellValue(wsName, fmt.Sprintf("A%d", wr), "KEBUTUHAN")
		f.SetCellStyle(wsName, fmt.Sprintf("A%d", wr), fmt.Sprintf("L%d", wr), titleStyle)
		wr++
		f.SetCellValue(wsName, fmt.Sprintf("A%d", wr), "Total")
		for i, c := range kebCodes {
			col := string(rune('B' + i))
			f.SetCellValue(wsName, col+fmt.Sprintf("%d", wr), c)
		}
		f.SetCellStyle(wsName, fmt.Sprintf("A%d", wr), fmt.Sprintf("L%d", wr), headerStyle)
		wr++
		for i, c := range kebCodes {
			col := string(rune('B' + i))
			f.SetCellValue(wsName, col+fmt.Sprintf("%d", wr), papiCategoryLabel[c])
		}
		f.SetCellStyle(wsName, fmt.Sprintf("A%d", wr), fmt.Sprintf("L%d", wr), headerStyle)
		wr++
		f.SetCellValue(wsName, fmt.Sprintf("A%d", wr), totKeb)
		for i, c := range kebCodes {
			col := string(rune('B' + i))
			f.SetCellValue(wsName, col+fmt.Sprintf("%d", wr), parsed[c].Score)
		}
		f.SetCellStyle(wsName, fmt.Sprintf("A%d", wr), fmt.Sprintf("L%d", wr), headerStyle)

		f.SetColWidth(wsName, "A", "A", 8)
		f.SetColWidth(wsName, "B", "B", 14)
		f.SetColWidth(wsName, "C", "L", 12)
	}

	// =========================
	// SHEET 3: Detail Jawaban
	// =========================
	detSheet := "Detail Jawaban"
	if _, err := f.NewSheet(detSheet); err == nil {
		f.MergeCell(detSheet, "A1", "G1")
		f.SetCellValue(detSheet, "A1", "DETAIL JAWABAN PER ITEM")
		f.SetCellStyle(detSheet, "A1", "G1", titleStyle)

		f.SetCellValue(detSheet, "A2", "No")
		f.SetCellValue(detSheet, "B2", "Pernyataan A (Atas)")
		f.SetCellValue(detSheet, "C2", "Pernyataan B (Bawah)")
		f.SetCellValue(detSheet, "D2", "Kode A")
		f.SetCellValue(detSheet, "E2", "Kode B")
		f.SetCellValue(detSheet, "F2", "Pilihan")
		f.SetCellValue(detSheet, "G2", "Skor Masuk")
		f.SetCellStyle(detSheet, "A2", "G2", headerStyle)

		dr := 3
		for _, d := range details {
			pickedCode := ""
			pickLabel := "-"
			if d.Selected == "A" {
				pickedCode = d.CategoryA
				pickLabel = "A"
			} else if d.Selected == "B" {
				pickedCode = d.CategoryB
				pickLabel = "B"
			}
			f.SetCellValue(detSheet, fmt.Sprintf("A%d", dr), d.ItemNumber)
			f.SetCellValue(detSheet, fmt.Sprintf("B%d", dr), d.OptionA)
			f.SetCellValue(detSheet, fmt.Sprintf("C%d", dr), d.OptionB)
			f.SetCellValue(detSheet, fmt.Sprintf("D%d", dr), d.CategoryA)
			f.SetCellValue(detSheet, fmt.Sprintf("E%d", dr), d.CategoryB)
			f.SetCellValue(detSheet, fmt.Sprintf("F%d", dr), pickLabel)
			f.SetCellValue(detSheet, fmt.Sprintf("G%d", dr), pickedCode)
			f.SetCellStyle(detSheet, fmt.Sprintf("A%d", dr), fmt.Sprintf("A%d", dr), centerStyle)
			f.SetCellStyle(detSheet, fmt.Sprintf("B%d", dr), fmt.Sprintf("C%d", dr), cellStyle)
			f.SetCellStyle(detSheet, fmt.Sprintf("D%d", dr), fmt.Sprintf("G%d", dr), centerStyle)
			dr++
		}

		f.SetColWidth(detSheet, "A", "A", 6)
		f.SetColWidth(detSheet, "B", "C", 50)
		f.SetColWidth(detSheet, "D", "G", 10)
	}

	// Set sheet aktif ke Ringkasan
	if idx, err := f.GetSheetIndex(sumSheet); err == nil {
		f.SetActiveSheet(idx)
	}

	// Set filename
	filename := fmt.Sprintf("Hasil_PAPI_%s_%s.xlsx", 
		strings.ReplaceAll(inv.Email, "@", "_"), 
		res.CompletedAt.Format("20060102_150405"))

	c.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.Output.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Ctx.Output.Header("Content-Transfer-Encoding", "binary")

	if err := f.Write(c.Ctx.ResponseWriter); err != nil {
		logs.Error("PAPI ExportResultExcel write error: %v", err)
	}
}
