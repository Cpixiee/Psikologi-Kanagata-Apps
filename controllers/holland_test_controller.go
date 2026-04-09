package controllers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/xuri/excelize/v2"
)

// HollandTestController menangani alur pengerjaan tes Holland (RIASEC).
type HollandTestController struct {
	beego.Controller
}

func (c *HollandTestController) mustGetSessionInvitation() (*models.TestInvitation, *models.User, bool) {
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

func (c *HollandTestController) hollandQuestionsRange(startNum, endNum int) ([]models.HollandQuestion, error) {
	o := orm.NewOrm()
	var qs []models.HollandQuestion
	_, err := o.QueryTable(new(models.HollandQuestion)).
		Filter("Number__gte", startNum).
		Filter("Number__lte", endNum).
		OrderBy("Number").
		All(&qs)
	if err != nil {
		return nil, err
	}
	return qs, nil
}

type hollandAnswerRow struct {
	QuestionID int `orm:"column(question_id)" json:"question_id"`
	Value      int `orm:"column(value)" json:"value"`
}

func (c *HollandTestController) hollandAnswersMap(invID int) (map[int]int, error) {
	o := orm.NewOrm()
	var rows []hollandAnswerRow
	_, err := o.Raw(`
		SELECT a.question_id, a.value
		FROM holland_answers a
		WHERE a.invitation_id = ?
	`, invID).QueryRows(&rows)
	if err != nil {
		return nil, err
	}
	m := make(map[int]int, len(rows))
	for _, r := range rows {
		m[r.QuestionID] = r.Value
	}
	return m, nil
}

func (c *HollandTestController) hollandScores(invID int) (map[string]int, error) {
	o := orm.NewOrm()
	type row struct {
		Code  string `orm:"column(code)"`
		Value int    `orm:"column(value)"`
	}
	var rows []row
	_, err := o.Raw(`
		SELECT q.code, a.value
		FROM holland_answers a
		JOIN holland_questions q ON q.id = a.question_id
		WHERE a.invitation_id = ?
	`, invID).QueryRows(&rows)
	if err != nil {
		return nil, err
	}
	scores := map[string]int{"R": 0, "I": 0, "A": 0, "S": 0, "E": 0, "C": 0}
	for _, r := range rows {
		uc := strings.ToUpper(strings.TrimSpace(r.Code))
		if _, ok := scores[uc]; !ok {
			continue
		}
		scores[uc] += r.Value
	}
	return scores, nil
}

func (c *HollandTestController) top3FromScores(scores map[string]int) (top1, top2, top3, code string) {
	order := []string{"R", "I", "A", "S", "E", "C"}
	type pair struct {
		Code  string
		Score int
		Idx   int
	}
	ps := make([]pair, 0, len(order))
	for i, code := range order {
		ps = append(ps, pair{Code: code, Score: scores[code], Idx: i})
	}
	// Sort desc by score, tie-break by original order.
	for i := 0; i < len(ps); i++ {
		for j := i + 1; j < len(ps); j++ {
			if ps[j].Score > ps[i].Score || (ps[j].Score == ps[i].Score && ps[j].Idx < ps[i].Idx) {
				ps[i], ps[j] = ps[j], ps[i]
			}
		}
	}
	top1 = ps[0].Code
	top2 = ps[1].Code
	top3 = ps[2].Code
	code = top1 + top2 + top3
	return
}

func (c *HollandTestController) isHollandPageComplete(invID int, page int) (bool, error) {
	answersMap, err := c.hollandAnswersMap(invID)
	if err != nil {
		return false, err
	}
	var start, end int
	if page == 1 {
		start, end = 1, 35
	} else if page == 2 {
		start, end = 36, 60
	} else {
		return false, nil
	}

	qs, err := c.hollandQuestionsRange(start, end)
	if err != nil {
		return false, err
	}
	for _, q := range qs {
		if _, ok := answersMap[q.Id]; !ok {
			return false, nil
		}
	}
	return true, nil
}

// @router /test/holland/start [get]
func (c *HollandTestController) StartHollandPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	// If Holland result exists with page3 fields, go to finish.
	o := orm.NewOrm()
	var res models.HollandResult
	err := o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err == nil && res.Id != 0 {
		// If user already submitted page 3, redirect to finish.
		if strings.TrimSpace(res.DreamJob1) != "" &&
			strings.TrimSpace(res.DreamJob2) != "" &&
			strings.TrimSpace(res.DreamJob3) != "" &&
			strings.TrimSpace(res.FavoriteSubject) != "" &&
			strings.TrimSpace(res.DislikedSubject) != "" {
			c.Redirect("/test/holland/finish", 302)
			return
		}
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_holland_start.html"
}

// @router /test/holland/instruction [get]
func (c *HollandTestController) HollandInstructionPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	complete1, err := c.isHollandPageComplete(inv.Id, 1)
	if err != nil {
		c.Redirect("/test/holland/start", 302)
		return
	}
	if complete1 {
		c.Redirect("/test/holland/page2", 302)
		return
	}
	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_holland_instruction.html"
}

// @router /test/holland/page1 [get]
func (c *HollandTestController) HollandPage1() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	complete1, err := c.isHollandPageComplete(inv.Id, 1)
	if err == nil && complete1 {
		c.Redirect("/test/holland/page2", 302)
		return
	}

	qs, err := c.hollandQuestionsRange(1, 35)
	if err != nil {
		c.Data["Error"] = "Gagal memuat aktivitas Holland page 1"
		c.Data["User"] = user
		c.Data["Invitation"] = inv
		c.TplName = "test_holland_page1.html"
		return
	}

	c.Data["Questions"] = qs
	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_holland_page1.html"
}

// @router /test/holland/page2 [get]
func (c *HollandTestController) HollandPage2() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	complete1, err := c.isHollandPageComplete(inv.Id, 1)
	if err != nil || !complete1 {
		c.Redirect("/test/holland/page1", 302)
		return
	}

	complete2, err := c.isHollandPageComplete(inv.Id, 2)
	if err == nil && complete2 {
		c.Redirect("/test/holland/page3", 302)
		return
	}

	qs, err := c.hollandQuestionsRange(36, 60)
	if err != nil {
		c.Data["Error"] = "Gagal memuat aktivitas Holland page 2"
		c.Data["User"] = user
		c.Data["Invitation"] = inv
		c.TplName = "test_holland_page2.html"
		return
	}

	c.Data["Questions"] = qs
	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_holland_page2.html"
}

// @router /test/holland/page3 [get]
func (c *HollandTestController) HollandPage3() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	complete1, err := c.isHollandPageComplete(inv.Id, 1)
	if err != nil || !complete1 {
		c.Redirect("/test/holland/page1", 302)
		return
	}
	complete2, err := c.isHollandPageComplete(inv.Id, 2)
	if err != nil || !complete2 {
		c.Redirect("/test/holland/page2", 302)
		return
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_holland_page3.html"
}

// @router /test/holland/finish [get]
func (c *HollandTestController) HollandFinishPage() {
	inv, _, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	o := orm.NewOrm()
	var res models.HollandResult
	err := o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		c.Redirect("/test/holland/page3", 302)
		return
	}
	c.Data["InvitationId"] = inv.Id
	c.Data["Result"] = res
	c.TplName = "test_holland_finish.html"
}

// @router /api/test/holland/page1 [post]
func (c *HollandTestController) SubmitPage1API() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	qs, err := c.hollandQuestionsRange(1, 35)
	if err != nil || len(qs) == 0 {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal page 1 belum tersedia"}
		c.ServeJSON()
		return
	}

	type payload struct {
		Answers     map[string]int `json:"answers"`
		ForceSubmit bool           `json:"force_submit"`
	}
	var p payload
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &p); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}

	answersMap := p.Answers
	if !p.ForceSubmit {
		for _, q := range qs {
			v, ok := answersMap[strconv.Itoa(q.Id)]
			if !ok || v < 0 || v > 4 {
				c.Ctx.Output.SetStatus(422)
				c.Data["json"] = map[string]interface{}{
					"success":        false,
					"message":        "Jawaban belum lengkap",
					"missing_question_id": q.Id,
				}
				c.ServeJSON()
				return
			}
		}
	}

	tx, err := orm.NewOrm().Begin()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal memulai transaksi"}
		c.ServeJSON()
		return
	}

	rollback := func(msg string) {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": msg}
		c.ServeJSON()
	}

	// Hapus jawaban lama page 1 agar resubmit aman.
	for _, q := range qs {
		if _, err := tx.QueryTable(new(models.HollandAnswer)).
			Filter("Invitation__Id", inv.Id).
			Filter("Question__Id", q.Id).
			Delete(); err != nil {
			rollback("Gagal menghapus jawaban page 1")
			return
		}
	}

	for _, q := range qs {
		v, ok := answersMap[strconv.Itoa(q.Id)]
		if !ok {
			continue
		}
		if v < 0 || v > 4 {
			continue
		}

		qCopy := q
		ans := models.HollandAnswer{
			Invitation: inv,
			User:       user,
			Question:   &qCopy,
			Value:      v,
		}
		if _, err := tx.Insert(&ans); err != nil {
			rollback("Gagal menyimpan jawaban page 1")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		rollback("Gagal commit transaksi page 1")
		return
	}

	c.Data["json"] = map[string]interface{}{
		"success":     true,
		"next_redirect": "/test/holland/page2",
	}
	c.ServeJSON()
}

// @router /api/test/holland/page2 [post]
func (c *HollandTestController) SubmitPage2API() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	qs, err := c.hollandQuestionsRange(36, 60)
	if err != nil || len(qs) == 0 {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal page 2 belum tersedia"}
		c.ServeJSON()
		return
	}

	type payload struct {
		Answers     map[string]int `json:"answers"`
		ForceSubmit bool           `json:"force_submit"`
	}
	var p payload
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &p); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}

	answersMap := p.Answers
	if !p.ForceSubmit {
		for _, q := range qs {
			v, ok := answersMap[strconv.Itoa(q.Id)]
			if !ok || v < 0 || v > 4 {
				c.Ctx.Output.SetStatus(422)
				c.Data["json"] = map[string]interface{}{
					"success":        false,
					"message":        "Jawaban belum lengkap",
					"missing_question_id": q.Id,
				}
				c.ServeJSON()
				return
			}
		}
	}

	tx, err := orm.NewOrm().Begin()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal memulai transaksi"}
		c.ServeJSON()
		return
	}

	rollback := func(msg string) {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": msg}
		c.ServeJSON()
	}

	for _, q := range qs {
		if _, err := tx.QueryTable(new(models.HollandAnswer)).
			Filter("Invitation__Id", inv.Id).
			Filter("Question__Id", q.Id).
			Delete(); err != nil {
			rollback("Gagal menghapus jawaban page 2")
			return
		}
	}

	for _, q := range qs {
		v, ok := answersMap[strconv.Itoa(q.Id)]
		if !ok {
			continue
		}
		if v < 0 || v > 4 {
			continue
		}
		qCopy := q
		ans := models.HollandAnswer{
			Invitation: inv,
			User:       user,
			Question:   &qCopy,
			Value:      v,
		}
		if _, err := tx.Insert(&ans); err != nil {
			rollback("Gagal menyimpan jawaban page 2")
			return
		}
	}

	if err := tx.Commit(); err != nil {
		rollback("Gagal commit transaksi page 2")
		return
	}

	c.Data["json"] = map[string]interface{}{
		"success":       true,
		"next_redirect": "/test/holland/page3",
	}
	c.ServeJSON()
}

// @router /api/test/holland/page3 [post]
func (c *HollandTestController) SubmitPage3API() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	// Page 1 & 2 should be complete before page 3.
	complete1, err := c.isHollandPageComplete(inv.Id, 1)
	if err != nil || !complete1 {
		c.Ctx.Output.SetStatus(409)
		c.Data["json"] = map[string]interface{}{
			"success":        false,
			"message":        "Aktivitas page 1 belum lengkap",
			"redirect":       "/test/holland/page1",
		}
		c.ServeJSON()
		return
	}
	complete2, err := c.isHollandPageComplete(inv.Id, 2)
	if err != nil || !complete2 {
		c.Ctx.Output.SetStatus(409)
		c.Data["json"] = map[string]interface{}{
			"success":        false,
			"message":        "Aktivitas page 2 belum lengkap",
			"redirect":       "/test/holland/page2",
		}
		c.ServeJSON()
		return
	}

	type payload struct {
		DreamJob1       string `json:"dream_job_1"`
		DreamJob2       string `json:"dream_job_2"`
		DreamJob3       string `json:"dream_job_3"`
		FavoriteSubject string `json:"favorite_subject"`
		DislikedSubject string `json:"disliked_subject"`
		ForceSubmit     bool   `json:"force_submit"`
	}
	var p payload
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &p); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}

	if !p.ForceSubmit {
		if strings.TrimSpace(p.DreamJob1) == "" ||
			strings.TrimSpace(p.DreamJob2) == "" ||
			strings.TrimSpace(p.DreamJob3) == "" ||
			strings.TrimSpace(p.FavoriteSubject) == "" ||
			strings.TrimSpace(p.DislikedSubject) == "" {
			c.Ctx.Output.SetStatus(422)
			c.Data["json"] = map[string]interface{}{
				"success": false,
				"message": "Isian page 3 belum lengkap",
			}
			c.ServeJSON()
			return
		}
	}

	// Compute scores.
	scores, err := c.hollandScores(inv.Id)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal hitung skor Holland"}
		c.ServeJSON()
		return
	}
	top1, top2, top3, code := c.top3FromScores(scores)

	// Load descriptions for top3 to build interpretation text.
	o := orm.NewOrm()
	descMap := make(map[string]models.HollandDescription)
	{
		var descs []models.HollandDescription
		_, derr := o.QueryTable(new(models.HollandDescription)).All(&descs)
		if derr == nil {
			for _, d := range descs {
				uc := strings.ToUpper(strings.TrimSpace(d.Code))
				descMap[uc] = d
			}
		}
	}

	var interpParts []string
	for _, dc := range []string{top1, top2, top3} {
		if d, ok := descMap[dc]; ok && strings.TrimSpace(d.Description) != "" {
			interpParts = append(interpParts, fmt.Sprintf("%s: %s", d.Title, d.Description))
		}
	}
	if len(interpParts) == 0 {
		interpParts = []string{"Interpretasi Holland sedang diproses."}
	}

	interpExtra := fmt.Sprintf(
		"3 Pekerjaan Impian:\n1) %s\n2) %s\n3) %s\n\nSubjek Favorit: %s\nSubjek Paling Tidak Disukai: %s",
		strings.TrimSpace(p.DreamJob1),
		strings.TrimSpace(p.DreamJob2),
		strings.TrimSpace(p.DreamJob3),
		strings.TrimSpace(p.FavoriteSubject),
		strings.TrimSpace(p.DislikedSubject),
	)

	interp := strings.TrimSpace(strings.Join(interpParts, "\n\n")) + "\n\n" + interpExtra

	// Save / upsert HollandResult.
	var res models.HollandResult
	err = o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		res = models.HollandResult{
			Invitation: inv,
			User:       user,
		}
		if _, ierr := o.Insert(&res); ierr != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal buat Holland result"}
			c.ServeJSON()
			return
		}
	}

	res.ScoreR = scores["R"]
	res.ScoreI = scores["I"]
	res.ScoreA = scores["A"]
	res.ScoreS = scores["S"]
	res.ScoreE = scores["E"]
	res.ScoreC = scores["C"]
	res.Top1 = top1
	res.Top2 = top2
	res.Top3 = top3
	res.Code = code
	res.Interpretation = interp
	res.DreamJob1 = strings.TrimSpace(p.DreamJob1)
	res.DreamJob2 = strings.TrimSpace(p.DreamJob2)
	res.DreamJob3 = strings.TrimSpace(p.DreamJob3)
	res.FavoriteSubject = strings.TrimSpace(p.FavoriteSubject)
	res.DislikedSubject = strings.TrimSpace(p.DislikedSubject)

	// Ensure createdAt already exists; update fields.
	_, _ = o.Update(&res,
		"score_r", "score_i", "score_a", "score_s", "score_e", "score_c",
		"top1", "top2", "top3", "code", "interpretation",
		"dream_job_1", "dream_job_2", "dream_job_3",
		"favorite_subject", "disliked_subject",
	)

	// Mark invitation used (like IST flow).
	if inv.Status != models.StatusInvitationUsed {
		inv.Status = models.StatusInvitationUsed
		inv.UsedAt = time.Now()
		_, _ = o.Update(inv, "Status", "UsedAt")
	}

	// Optional: mark any derived summary if needed in the future.
	_ = utils.AgeYears // keep utils imported for potential future expansion (no-op)

	c.Data["json"] = map[string]interface{}{
		"success":         true,
		"finish_redirect": "/profile/holland",
	}
	c.ServeJSON()
}

// @router /test/holland/result/excel [get]
func (c *HollandTestController) ExportResultExcel() {
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
			c.Redirect("/profile/holland", 302)
			return
		}
		inv.Id = id
		if err := o.Read(&inv); err != nil {
			c.Redirect("/profile/holland", 302)
			return
		}
		if inv.UserId == nil || *inv.UserId != userID {
			c.Redirect("/profile/holland", 302)
			return
		}
	} else {
		sessionInv := c.GetSession("current_invitation_id")
		if sessionInv == nil {
			c.Redirect("/profile/holland", 302)
			return
		}
		inv.Id = sessionInv.(int)
		if err := o.Read(&inv); err != nil {
			c.Redirect("/profile/holland", 302)
			return
		}
	}

	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/profile/holland", 302)
		return
	}

	var res models.HollandResult
	err := o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		c.Redirect("/profile/holland", 302)
		return
	}

	qsPage1, err := c.hollandQuestionsRange(1, 35)
	if err != nil {
		c.Redirect("/profile/holland", 302)
		return
	}
	qsPage2, err := c.hollandQuestionsRange(36, 60)
	if err != nil {
		c.Redirect("/profile/holland", 302)
		return
	}

	answersMap, err := c.hollandAnswersMap(inv.Id)
	if err != nil {
		c.Redirect("/profile/holland", 302)
		return
	}

	completePage1 := true
	for _, q := range qsPage1 {
		v, ok := answersMap[q.Id]
		if !ok || v < 0 || v > 4 {
			completePage1 = false
			break
		}
	}
	completePage2 := true
	for _, q := range qsPage2 {
		v, ok := answersMap[q.Id]
		if !ok || v < 0 || v > 4 {
			completePage2 = false
			break
		}
	}
	completeText := strings.TrimSpace(res.DreamJob1) != "" &&
		strings.TrimSpace(res.DreamJob2) != "" &&
		strings.TrimSpace(res.DreamJob3) != "" &&
		strings.TrimSpace(res.FavoriteSubject) != "" &&
		strings.TrimSpace(res.DislikedSubject) != ""

	complete := completePage1 && completePage2 && completeText

	f := excelize.NewFile()
	sheet := "Holland"
	f.SetSheetName(f.GetSheetName(0), sheet)
	showGridLines := false
	_ = f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGridLines})

	// Styles
	borderAll := []excelize.Border{
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
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#E6E6E6"}, Pattern: 1},
		Border:    borderAll,
	})
	styleHeaderBlue, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#CFE2F3"}, Pattern: 1},
		Border:    borderAll,
	})
	styleBody, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Border:    borderAll,
	})
	styleInputLine, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
		},
	})
	styleCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    borderAll,
	})
	styleCompleteGreen, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#1B5E20"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#C6EFCE"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll,
	})
	styleCompleteRed, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#8B0000"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#FFC7CE"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll,
	})

	// Column widths
	_ = f.SetColWidth(sheet, "A", "A", 68)
	_ = f.SetColWidth(sheet, "B", "F", 12)

	// Title
	_ = f.MergeCell(sheet, "A1", "F1")
	_ = f.SetCellValue(sheet, "A1", "HASIL TES HOLLAND (RIASEC)")
	_ = f.SetCellStyle(sheet, "A1", "A1", styleTitle)

	nama := user.NamaLengkap
	if strings.TrimSpace(nama) == "" {
		nama = inv.Email
	}
	email := user.Email
	if strings.TrimSpace(email) == "" {
		email = inv.Email
	}

	_ = f.SetCellValue(sheet, "A3", "Nama")
	_ = f.SetCellValue(sheet, "B3", ":")
	_ = f.SetCellValue(sheet, "C3", nama)

	_ = f.SetCellValue(sheet, "A4", "Email")
	_ = f.SetCellValue(sheet, "B4", ":")
	_ = f.SetCellValue(sheet, "C4", email)

	labelRow := 6
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", labelRow), fmt.Sprintf("F%d", labelRow))
	if complete {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", labelRow), "Jawaban lengkap")
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", labelRow), fmt.Sprintf("F%d", labelRow), styleCompleteGreen)
	} else {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", labelRow), "Jawaban tidak lengkap")
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", labelRow), fmt.Sprintf("F%d", labelRow), styleCompleteRed)
	}

	writeTable := func(startRow int, qs []models.HollandQuestion) int {
		// Header
		headerRow := startRow
		_ = f.MergeCell(sheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("A%d", headerRow+1))
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", headerRow), "Aktifitas")
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("A%d", headerRow+1), styleHeader)

		labels := []string{"0", "1", "2", "3", "4"}
		cols := []string{"B", "C", "D", "E", "F"}
		for i := 0; i < 5; i++ {
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", cols[i], headerRow), labels[i])
			_ = f.SetCellStyle(sheet,
				fmt.Sprintf("%s%d", cols[i], headerRow),
				fmt.Sprintf("%s%d", cols[i], headerRow),
				styleHeaderBlue,
			)
			// second header row left intentionally empty (so cells keep borders)
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", cols[i], headerRow+1), "")
			_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", cols[i], headerRow+1), fmt.Sprintf("%s%d", cols[i], headerRow+1), styleHeaderBlue)
		}

		row := headerRow + 2
		for _, q := range qs {
			_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), q.Prompt)
			_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), styleBody)

			// Put marker in selected column.
			// Exact mapping value -> column is: 0->B, 1->C, ... 4->F.
			v, ok := answersMap[q.Id]
			if ok && v >= 0 && v <= 4 {
				col := "B"
				switch v {
				case 0:
					col = "B"
				case 1:
					col = "C"
				case 2:
					col = "D"
				case 3:
					col = "E"
				case 4:
					col = "F"
				}
				_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", col, row), "X")
				_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", col, row), fmt.Sprintf("%s%d", col, row), styleCenter)
			}
			// Ensure borders exist for empty cells too.
			for _, col := range []string{"B", "C", "D", "E", "F"} {
				_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", col, row), fmt.Sprintf("%s%d", col, row), styleBody)
			}
			row++
		}
		// Table end row index returned.
		return row
	}

	// Page 1 table
	nextRow := labelRow + 1
	nextRow = writeTable(nextRow, qsPage1) + 1

	// Page 2 table header separation label
	sepRow := nextRow
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", sepRow), fmt.Sprintf("F%d", sepRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", sepRow), "AKTIVITAS 2")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", sepRow), fmt.Sprintf("F%d", sepRow), styleHeaderBlue)
	nextRow = sepRow + 1
	nextRow = writeTable(nextRow, qsPage2) + 1

	// Page 3 fields (formatted block)
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Silakan tuliskan 3 pekerjaan impian Anda:")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow), styleHeaderBlue)
	nextRow++

	writeLabeledValue := func(row int, label string, value string) int {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), label)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), styleCenter)
		_ = f.MergeCell(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("F%d", row))
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), strings.TrimSpace(value))
		_ = f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("F%d", row), styleInputLine)
		_ = f.SetRowHeight(sheet, row, 24)
		return row + 1
	}
	nextRow = writeLabeledValue(nextRow, "1.", res.DreamJob1)
	nextRow = writeLabeledValue(nextRow, "2.", res.DreamJob2)
	nextRow = writeLabeledValue(nextRow, "3.", res.DreamJob3)
	nextRow++

	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Silakan tuliskan subjek favorit Anda:")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow), styleHeaderBlue)
	nextRow++
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), strings.TrimSpace(res.FavoriteSubject))
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow), styleInputLine)
	_ = f.SetRowHeight(sheet, nextRow, 24)
	nextRow++

	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Silakan tuliskan subjek yang paling tidak Anda sukai:")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow), styleHeaderBlue)
	nextRow++
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), strings.TrimSpace(res.DislikedSubject))
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow), styleInputLine)
	_ = f.SetRowHeight(sheet, nextRow, 24)
	nextRow += 2

	// Top codes + scores
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Kode RIASEC (Top 3): "+res.Code)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow), styleBody)
	nextRow++
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("Skor R=%d, I=%d, A=%d, S=%d, E=%d, C=%d", res.ScoreR, res.ScoreI, res.ScoreA, res.ScoreS, res.ScoreE, res.ScoreC))
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow), styleBody)

	// Write xlsx
	buf, err := f.WriteToBuffer()
	if err != nil {
		c.Redirect("/profile/holland", 302)
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
	filename := fmt.Sprintf("holland_result_%s.xlsx", makeSafeName(downloadName))
	c.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(buf.Bytes())
}

// styleBoldIfPossible is a tiny helper to avoid duplicating style definitions.
// If it fails, it falls back to styleBody.
func styleBoldIfPossible(f *excelize.File, styleBody int) int {
	// We don't need a unique "bold" style; keeping this as a placeholder helper
	// to keep compile safe even if style changes later.
	return styleBody
}

