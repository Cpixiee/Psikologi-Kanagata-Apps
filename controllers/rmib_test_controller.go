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
)

// RMIBTestController menangani alur pengerjaan tes RMIB (versi pria/wanita).
type RMIBTestController struct {
	beego.Controller
}

// rmibCategoryOrder identik dengan seeder; dipakai untuk render bar chart agar urut konsisten.
var rmibCategoryOrder = []string{
	"OUT", "MEC", "COMP", "SCI", "PERS", "AEST", "MUS", "LIT", "SOC", "CLER", "PRAC", "MED",
}

// rmibCategoryLabel mengubah kode kategori jadi label panjang manusiawi.
var rmibCategoryLabel = map[string]string{
	"OUT":  "Outdoor",
	"MEC":  "Mechanical",
	"COMP": "Computational",
	"SCI":  "Scientific",
	"PERS": "Personal Contact",
	"AEST": "Aesthetic",
	"MUS":  "Musical",
	"LIT":  "Literary",
	"SOC":  "Social Service",
	"CLER": "Clerical",
	"PRAC": "Practical",
	"MED":  "Medical",
}

const rmibTotalGroups = 8
const rmibItemsPerGroup = 12

// genderVersionFromUser memetakan field user.JenisKelamin ke string "pria" / "wanita".
// Default ke "pria" jika tidak terdeteksi.
func genderVersionFromUser(u *models.User) string {
	if u == nil {
		return "pria"
	}
	switch strings.ToLower(strings.TrimSpace(string(u.JenisKelamin))) {
	case "perempuan":
		return "wanita"
	default:
		return "pria"
	}
}

func (c *RMIBTestController) mustGetSessionInvitation() (*models.TestInvitation, *models.User, bool) {
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

	return &inv, &user, true
}

// getOrCreateSession memastikan ada satu RMIBSession untuk invitation ini.
func (c *RMIBTestController) getOrCreateSession(inv *models.TestInvitation, user *models.User) (*models.RMIBSession, error) {
	o := orm.NewOrm()
	var session models.RMIBSession
	err := o.QueryTable(new(models.RMIBSession)).Filter("Invitation__Id", inv.Id).One(&session)
	if err == nil && session.Id != 0 {
		return &session, nil
	}

	gv := genderVersionFromUser(user)

	// Pastikan soal untuk gender_version tersedia.
	cnt, _ := o.QueryTable(new(models.RMIBQuestion)).
		Filter("GenderVersion", gv).Count()
	if cnt == 0 {
		return nil, fmt.Errorf("soal RMIB versi %s belum tersedia, hubungi admin", gv)
	}

	session = models.RMIBSession{
		Invitation:    inv,
		User:          user,
		BatchId:       inv.BatchId,
		GenderVersion: gv,
		Status:        "in_progress",
	}
	if _, err := o.Insert(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (c *RMIBTestController) getQuestionsByGroup(genderVersion string, group int) ([]models.RMIBQuestion, error) {
	o := orm.NewOrm()
	var qs []models.RMIBQuestion
	_, err := o.QueryTable(new(models.RMIBQuestion)).
		Filter("GenderVersion", genderVersion).
		Filter("GroupNumber", group).
		OrderBy("ItemOrder").
		All(&qs)
	return qs, err
}

func (c *RMIBTestController) getAnswersBySession(sessionID int) (map[int]int, error) {
	o := orm.NewOrm()
	type row struct {
		QuestionID   int `orm:"column(question_id)"`
		SelectedRank int `orm:"column(selected_rank)"`
	}
	var rows []row
	_, err := o.Raw(`
		SELECT question_id, selected_rank
		FROM rmib_answers
		WHERE session_id = ?
	`, sessionID).QueryRows(&rows)
	if err != nil {
		return nil, err
	}
	m := make(map[int]int, len(rows))
	for _, r := range rows {
		m[r.QuestionID] = r.SelectedRank
	}
	return m, nil
}

// groupCompletion mengembalikan map[group]bool: true jika 12 jawaban valid 1-12 unik.
func (c *RMIBTestController) groupCompletion(genderVersion string, sessionID int) (map[int]bool, error) {
	completion := make(map[int]bool, rmibTotalGroups)
	for g := 1; g <= rmibTotalGroups; g++ {
		completion[g] = false
	}

	answers, err := c.getAnswersBySession(sessionID)
	if err != nil {
		return completion, err
	}

	for g := 1; g <= rmibTotalGroups; g++ {
		qs, err := c.getQuestionsByGroup(genderVersion, g)
		if err != nil || len(qs) != rmibItemsPerGroup {
			continue
		}
		seen := map[int]bool{}
		ok := true
		for _, q := range qs {
			r, exists := answers[q.Id]
			if !exists || r < 1 || r > 12 || seen[r] {
				ok = false
				break
			}
			seen[r] = true
		}
		completion[g] = ok
	}
	return completion, nil
}

// firstIncompleteGroup mengembalikan kelompok pertama yang belum lengkap (1..8) atau 0 jika semua lengkap.
func (c *RMIBTestController) firstIncompleteGroup(genderVersion string, sessionID int) (int, error) {
	completion, err := c.groupCompletion(genderVersion, sessionID)
	if err != nil {
		return 0, err
	}
	for g := 1; g <= rmibTotalGroups; g++ {
		if !completion[g] {
			return g, nil
		}
	}
	return 0, nil
}

// =========================
// PAGE: /test/rmib/start
// =========================
func (c *RMIBTestController) StartPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	// Jika hasil sudah ada, arahkan ke tes berikutnya di batch (jika ada), atau ke /hasil-tes.
	o := orm.NewOrm()
	var existing models.RMIBResult
	if err := o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).One(&existing); err == nil && existing.Id != 0 {
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
		gv := genderVersionFromUser(user)
		firstInc, _ := c.firstIncompleteGroup(gv, session.Id)
		if firstInc > 1 {
			c.Redirect(fmt.Sprintf("/test/rmib/group/%d", firstInc), 302)
			return
		} else if firstInc == 0 {
			c.Redirect("/test/rmib/summary", 302)
			return
		}
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["GenderVersion"] = genderVersionFromUser(user)
	c.TplName = "test_rmib_start.html"
}

// =========================
// PAGE: /test/rmib/instruction
// =========================
func (c *RMIBTestController) InstructionPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Redirect("/test/rmib/start", 302)
		return
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["GenderVersion"] = session.GenderVersion
	c.Data["TotalGroups"] = rmibTotalGroups
	c.Data["EstimatedMinutes"] = 25
	c.TplName = "test_rmib_instruction.html"
}

// =========================
// PAGE: /test/rmib/group/:n
// =========================
type rmibGroupItemView struct {
	ID           int
	QuestionId   int
	ItemOrder    int
	QuestionText string
	SelectedRank int // 0 jika belum dijawab
}

func (c *RMIBTestController) GroupPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	groupStr := strings.TrimSpace(c.Ctx.Input.Param(":n"))
	groupNum, err := strconv.Atoi(groupStr)
	if err != nil || groupNum < 1 || groupNum > rmibTotalGroups {
		c.Redirect("/test/rmib/start", 302)
		return
	}

	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Redirect("/test/rmib/start", 302)
		return
	}

	qs, err := c.getQuestionsByGroup(session.GenderVersion, groupNum)
	if err != nil || len(qs) != rmibItemsPerGroup {
		c.Data["Error"] = "Soal RMIB belum lengkap untuk kelompok ini."
		c.TplName = "test_rmib_group.html"
		return
	}

	answers, _ := c.getAnswersBySession(session.Id)

	items := make([]rmibGroupItemView, 0, len(qs))
	for _, q := range qs {
		items = append(items, rmibGroupItemView{
			ID:           q.Id,
			QuestionId:   q.Id,
			ItemOrder:    q.ItemOrder,
			QuestionText: q.QuestionText,
			SelectedRank: answers[q.Id],
		})
	}

	completion, _ := c.groupCompletion(session.GenderVersion, session.Id)
	completed := 0
	for g := 1; g <= rmibTotalGroups; g++ {
		if completion[g] {
			completed++
		}
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["Session"] = session
	c.Data["GroupNumber"] = groupNum
	c.Data["GroupTitle"] = qs[0].GroupTitle
	c.Data["GroupDescription"] = qs[0].GroupDescription
	c.Data["TotalGroups"] = rmibTotalGroups
	c.Data["Items"] = items
	c.Data["GenderVersion"] = session.GenderVersion
	c.Data["CompletedGroups"] = completed
	c.Data["ProgressPercent"] = int(float64(completed) / float64(rmibTotalGroups) * 100)
	c.Data["IsDev"] = strings.EqualFold(beego.BConfig.RunMode, "dev")
	if groupNum > 1 {
		c.Data["PrevGroup"] = groupNum - 1
	}
	if groupNum < rmibTotalGroups {
		c.Data["NextGroup"] = groupNum + 1
	}
	c.TplName = "test_rmib_group.html"
}

// =========================
// API: POST /api/test/rmib/answer (auto-save UPSERT 1 jawaban)
// Body: { question_id, selected_rank }
// =========================
func (c *RMIBTestController) SaveAnswerAPI() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	var payload struct {
		QuestionID   int `json:"question_id"`
		SelectedRank int `json:"selected_rank"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}
	if payload.QuestionID <= 0 || payload.SelectedRank < 1 || payload.SelectedRank > 12 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "question_id / selected_rank tidak valid"}
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

	// Validasi: question harus milik gender_version yang sesuai.
	var q models.RMIBQuestion
	q.Id = payload.QuestionID
	if err := o.Read(&q); err != nil || q.GenderVersion != session.GenderVersion {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal tidak ditemukan untuk versi gender Anda"}
		c.ServeJSON()
		return
	}

	// UPSERT (PostgreSQL).
	_, err = o.Raw(`
		INSERT INTO rmib_answers (session_id, group_number, question_id, selected_rank, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT (session_id, question_id)
		DO UPDATE SET selected_rank = EXCLUDED.selected_rank, updated_at = CURRENT_TIMESTAMP
	`, session.Id, q.GroupNumber, q.Id, payload.SelectedRank).Exec()
	if err != nil {
		logs.Error("RMIB SaveAnswerAPI upsert error: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal menyimpan jawaban"}
		c.ServeJSON()
		return
	}

	c.Data["json"] = map[string]interface{}{"success": true}
	c.ServeJSON()
}

// =========================
// API: POST /api/test/rmib/group/:n (validasi server saat klik Next)
// Body: { answers: { question_id: rank } }
// =========================
func (c *RMIBTestController) SubmitGroupAPI() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	groupStr := strings.TrimSpace(c.Ctx.Input.Param(":n"))
	groupNum, err := strconv.Atoi(groupStr)
	if err != nil || groupNum < 1 || groupNum > rmibTotalGroups {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Nomor kelompok tidak valid"}
		c.ServeJSON()
		return
	}

	var payload struct {
		Answers map[string]int `json:"answers"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
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

	qs, err := c.getQuestionsByGroup(session.GenderVersion, groupNum)
	if err != nil || len(qs) != rmibItemsPerGroup {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal kelompok belum lengkap"}
		c.ServeJSON()
		return
	}

	// Validasi 12 jawaban unik 1..12 dan sesuai daftar question_id kelompok.
	idsByGroup := map[int]bool{}
	for _, q := range qs {
		idsByGroup[q.Id] = true
	}
	if len(payload.Answers) != rmibItemsPerGroup {
		c.Ctx.Output.SetStatus(422)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Semua 12 ranking harus terisi."}
		c.ServeJSON()
		return
	}
	seen := map[int]bool{}
	parsed := make(map[int]int, rmibItemsPerGroup)
	for kID, rank := range payload.Answers {
		qid, e := strconv.Atoi(kID)
		if e != nil || !idsByGroup[qid] {
			c.Ctx.Output.SetStatus(422)
			c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal di luar kelompok ini."}
			c.ServeJSON()
			return
		}
		if rank < 1 || rank > 12 {
			c.Ctx.Output.SetStatus(422)
			c.Data["json"] = map[string]interface{}{"success": false, "message": "Ranking harus 1 sampai 12."}
			c.ServeJSON()
			return
		}
		if seen[rank] {
			c.Ctx.Output.SetStatus(422)
			c.Data["json"] = map[string]interface{}{"success": false, "message": "Setiap angka hanya boleh digunakan satu kali dalam satu kelompok."}
			c.ServeJSON()
			return
		}
		seen[rank] = true
		parsed[qid] = rank
	}

	// UPSERT semua jawaban kelompok ini dalam transaksi.
	o := orm.NewOrm()
	tx, err := o.Begin()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal mulai transaksi"}
		c.ServeJSON()
		return
	}
	for qid, rank := range parsed {
		if _, err := tx.Raw(`
			INSERT INTO rmib_answers (session_id, group_number, question_id, selected_rank, updated_at)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT (session_id, question_id)
			DO UPDATE SET selected_rank = EXCLUDED.selected_rank, updated_at = CURRENT_TIMESTAMP
		`, session.Id, groupNum, qid, rank).Exec(); err != nil {
			_ = tx.Rollback()
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal menyimpan jawaban kelompok"}
			c.ServeJSON()
			return
		}
	}
	if err := tx.Commit(); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal commit transaksi"}
		c.ServeJSON()
		return
	}

	next := "/test/rmib/finish"
	if groupNum < rmibTotalGroups {
		next = fmt.Sprintf("/test/rmib/group/%d", groupNum+1)
	} else {
		if nextURL, err := c.finalizeRMIB(inv, user, session); err == nil && nextURL != "" {
			next = nextURL
		}
	}
	c.Data["json"] = map[string]interface{}{
		"success":       true,
		"next_redirect": next,
	}
	c.ServeJSON()
}

// =========================
// PAGE: /test/rmib/summary
// =========================
func (c *RMIBTestController) SummaryPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Redirect("/test/rmib/start", 302)
		return
	}

	completion, _ := c.groupCompletion(session.GenderVersion, session.Id)

	type groupRow struct {
		GroupNumber int
		Title       string
		Completed   bool
	}
	o := orm.NewOrm()
	rows := make([]groupRow, 0, rmibTotalGroups)
	for g := 1; g <= rmibTotalGroups; g++ {
		var sample models.RMIBQuestion
		_ = o.QueryTable(new(models.RMIBQuestion)).
			Filter("GenderVersion", session.GenderVersion).
			Filter("GroupNumber", g).
			Limit(1).One(&sample)
		title := sample.GroupTitle
		if title == "" {
			title = fmt.Sprintf("Kelompok %d", g)
		}
		rows = append(rows, groupRow{
			GroupNumber: g,
			Title:       title,
			Completed:   completion[g],
		})
	}

	allComplete := true
	for g := 1; g <= rmibTotalGroups; g++ {
		if !completion[g] {
			allComplete = false
			break
		}
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["Groups"] = rows
	c.Data["AllComplete"] = allComplete
	c.Data["GenderVersion"] = session.GenderVersion
	c.TplName = "test_rmib_summary.html"
}

// =========================
// API: POST /test/rmib/submit (finalisasi & hitung skor)
// =========================
func (c *RMIBTestController) SubmitFinal() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	session, err := c.getOrCreateSession(inv, user)
	if err != nil {
		c.Redirect("/test/rmib/start", 302)
		return
	}

	completion, err := c.groupCompletion(session.GenderVersion, session.Id)
	if err != nil {
		c.Redirect("/test/rmib/summary", 302)
		return
	}
	for g := 1; g <= rmibTotalGroups; g++ {
		if !completion[g] {
			c.Redirect(fmt.Sprintf("/test/rmib/group/%d", g), 302)
			return
		}
	}

	nextURL, err := c.finalizeRMIB(inv, user, session)
	if err != nil {
		logs.Error("RMIB SubmitFinal finalize error: %v", err)
		c.Redirect("/test/rmib/summary", 302)
		return
	}

	if nextURL != "" {
		c.Redirect(nextURL, 302)
	} else {
		c.Redirect("/hasil-tes", 302)
	}
}

// finalizeRMIB menghitung skor + UPSERT RMIBResult + tandai session/invitation selesai.
// Helper ini dipakai oleh SubmitFinal & DevAutoFill.
func (c *RMIBTestController) finalizeRMIB(inv *models.TestInvitation, user *models.User, session *models.RMIBSession) (string, error) {
	o := orm.NewOrm()

	type rankRow struct {
		Category string `orm:"column(category_code)"`
		Rank     int    `orm:"column(selected_rank)"`
	}
	var rows []rankRow
	if _, err := o.Raw(`
		SELECT q.category_code, a.selected_rank
		FROM rmib_answers a
		JOIN rmib_questions q ON q.id = a.question_id
		WHERE a.session_id = ?
	`, session.Id).QueryRows(&rows); err != nil {
		return "", err
	}

	scoreMap := map[string]int{}
	for _, code := range rmibCategoryOrder {
		scoreMap[code] = 0
	}
	for _, r := range rows {
		scoreMap[r.Category] += r.Rank
	}

	type catScore struct {
		Code  string
		Score int
		Idx   int
	}
	ranked := make([]catScore, 0, len(rmibCategoryOrder))
	for i, code := range rmibCategoryOrder {
		ranked = append(ranked, catScore{Code: code, Score: scoreMap[code], Idx: i})
	}
	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].Score != ranked[j].Score {
			return ranked[i].Score < ranked[j].Score
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
			Label: rmibCategoryLabel[cs.Code],
			Score: cs.Score,
			Rank:  i + 1,
		}
	}
	resJSON, _ := json.Marshal(resultMap)

	top1 := ranked[0].Code
	top2 := ranked[1].Code
	top3 := ranked[2].Code

	interp := fmt.Sprintf(
		"Berdasarkan urutan minat, Anda memiliki kecenderungan tertinggi pada:\n1) %s\n2) %s\n3) %s\n\n"+
			"Semakin kecil skor total ranking suatu kategori, semakin tinggi minat Anda terhadap kategori tersebut.",
		rmibCategoryLabel[top1], rmibCategoryLabel[top2], rmibCategoryLabel[top3],
	)

	var existing models.RMIBResult
	err := o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).One(&existing)
	if err != nil || existing.Id == 0 {
		newRes := models.RMIBResult{
			Invitation:       inv,
			User:             user,
			GenderVersion:    session.GenderVersion,
			ResultJSON:       string(resJSON),
			DominantCategory: top1,
			Top1:             top1,
			Top2:             top2,
			Top3:             top3,
			Interpretation:   interp,
			CompletedAt:      time.Now(),
		}
		if _, err := o.Insert(&newRes); err != nil {
			return "", err
		}
	} else {
		existing.GenderVersion = session.GenderVersion
		existing.ResultJSON = string(resJSON)
		existing.DominantCategory = top1
		existing.Top1 = top1
		existing.Top2 = top2
		existing.Top3 = top3
		existing.Interpretation = interp
		existing.CompletedAt = time.Now()
		if _, err := o.Update(&existing,
			"GenderVersion", "ResultJSON", "DominantCategory",
			"Top1", "Top2", "Top3", "Interpretation", "CompletedAt",
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
			go utils.SendTestCompletionNotification(inv.UserId, "RMIB")
		}
	}

	return redirectURL, nil
}

// =========================
// DEV ONLY: POST /test/rmib/dev-autofill
// Mengisi acak (permutasi 1..12) seluruh jawaban kelompok yang masih kosong,
// lalu menjalankan finalisasi seperti SubmitFinal. Hanya tersedia ketika
// beego RunMode = "dev".
// =========================
func (c *RMIBTestController) DevAutoFill() {
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

	for g := 1; g <= rmibTotalGroups; g++ {
		qs, err := c.getQuestionsByGroup(session.GenderVersion, g)
		if err != nil || len(qs) != rmibItemsPerGroup {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = map[string]interface{}{"success": false, "message": fmt.Sprintf("Soal kelompok %d belum lengkap", g)}
			c.ServeJSON()
			return
		}
		ranks := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}
		rng.Shuffle(len(ranks), func(i, j int) { ranks[i], ranks[j] = ranks[j], ranks[i] })

		for idx, q := range qs {
			if _, err := o.Raw(`
				INSERT INTO rmib_answers (session_id, group_number, question_id, selected_rank, updated_at)
				VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
				ON CONFLICT (session_id, question_id)
				DO UPDATE SET selected_rank = EXCLUDED.selected_rank, updated_at = CURRENT_TIMESTAMP
			`, session.Id, g, q.Id, ranks[idx]).Exec(); err != nil {
				logs.Error("RMIB DevAutoFill upsert error: %v", err)
				c.Ctx.Output.SetStatus(500)
				c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal menyimpan jawaban acak"}
				c.ServeJSON()
				return
			}
		}
	}

	// Jalankan finalisasi (perhitungan + simpan RMIBResult).
	nextURL, err := c.finalizeRMIB(inv, user, session)
	if err != nil {
		logs.Error("RMIB DevAutoFill finalize error: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal finalisasi"}
		c.ServeJSON()
		return
	}

	if nextURL == "" {
		nextURL = "/profile/rmib"
	}

	c.Data["json"] = map[string]interface{}{"success": true, "next": nextURL}
	c.ServeJSON()
}

// =========================
// PAGE: /test/rmib/finish
// Tidak ada halaman finish/result terpisah; selalu lempar ke /profile/rmib.
// =========================
func (c *RMIBTestController) FinishPage() {
	c.Redirect("/profile/rmib", 302)
}

// ResultPage tidak lagi merender halaman hasil terpisah.
// Hasil RMIB ditampilkan di halaman profil (/profile/rmib).
func (c *RMIBTestController) ResultPage() {
	c.Redirect("/profile/rmib", 302)
}

// =========================
// GET: /test/rmib/result/excel  (export hasil pribadi)
// =========================
// categoryLabelFromRank memetakan peringkat (1..12) ke label tingkat minat untuk
// keperluan tampilan/eksport. Rank <=0 dianggap belum diranking.
func categoryLabelFromRank(rank int) (string, string) {
	switch {
	case rank <= 0:
		return "-", ""
	case rank <= 3:
		return "Sangat Tinggi", "top3"
	case rank <= 6:
		return "Tinggi", "high"
	case rank <= 9:
		return "Sedang", "mid"
	default:
		return "Rendah", "low"
	}
}

func (c *RMIBTestController) ExportResultExcel() {
	o := orm.NewOrm()

	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login", 302)
		return
	}
	userID := sessionUser.(int)

	// Admin diizinkan mengakses invitation milik user lain (untuk Lihat Detail di panel admin).
	roleVal, _ := c.GetSession("user_role").(string)
	isAdmin := roleVal == string(models.RoleAdmin)

	var inv models.TestInvitation
	invIDStr := strings.TrimSpace(c.GetString("invId"))
	if invIDStr != "" {
		id, err := strconv.Atoi(invIDStr)
		if err != nil || id <= 0 {
			c.Redirect("/profile/rmib", 302)
			return
		}
		inv.Id = id
		if err := o.Read(&inv); err != nil {
			c.Redirect("/profile/rmib", 302)
			return
		}
		if !isAdmin && (inv.UserId == nil || *inv.UserId != userID) {
			c.Redirect("/profile/rmib", 302)
			return
		}
		// Untuk admin, gunakan user_id pemilik invitation supaya nama di Excel benar.
		if isAdmin && inv.UserId != nil {
			userID = *inv.UserId
		}
	} else {
		sessionInv := c.GetSession("current_invitation_id")
		if sessionInv == nil {
			c.Redirect("/profile/rmib", 302)
			return
		}
		inv.Id = sessionInv.(int)
		if err := o.Read(&inv); err != nil {
			c.Redirect("/profile/rmib", 302)
			return
		}
	}

	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/profile/rmib", 302)
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

	excelBytes, err := buildRMIBResultXLSX(o, batch, &inv, &user)
	if err != nil {
		c.Redirect("/profile/rmib", 302)
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
	filename := fmt.Sprintf("rmib_result_%s.xlsx", makeSafeName(downloadName))
	c.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(excelBytes)
}
