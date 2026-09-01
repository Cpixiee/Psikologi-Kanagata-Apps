package controllers

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"
	"psikologi_apps/seeds"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

type LearningStyleTestController struct {
	beego.Controller
}

func (c *LearningStyleTestController) mustGetSessionInvitation() (*models.TestInvitation, *models.User, bool) {
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
		// allow email match fallback
		if strings.TrimSpace(inv.Email) == "" || user.Email == "" || !strings.EqualFold(inv.Email, user.Email) {
			return nil, nil, false
		}
	}

	return &inv, &user, true
}

func (c *LearningStyleTestController) ensureBatchAllowsVAK(inv *models.TestInvitation) bool {
	if inv == nil || inv.BatchId == nil {
		return false
	}
	o := orm.NewOrm()
	var batch models.TestBatch
	batch.Id = *inv.BatchId
	if err := o.Read(&batch); err != nil {
		return false
	}
	return batch.EnableLearningStyle
}

// @router /test/learning-style/start [get]
func (c *LearningStyleTestController) StartPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsVAK(inv) {
		c.Redirect("/test", 302)
		return
	}

	// If already finished, check next test in batch.
	o := orm.NewOrm()
	var res models.LearningStyleResult
	if err := o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res); err == nil && res.Id != 0 {
		if res.ScoreVisual+res.ScoreAuditory+res.ScoreKinesthetic > 0 && strings.TrimSpace(res.DominantType) != "" {
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
	}

	// Checkpoint resume logic: if user already answered some questions, jump directly to questions page
	ansMap, _ := c.answersMap(inv.Id)
	if len(ansMap) > 0 {
		c.Redirect("/test/learning-style/questions", 302)
		return
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_learning_style_start.html"
}

// @router /test/learning-style/start [post]
func (c *LearningStyleTestController) SubmitStart() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsVAK(inv) {
		c.Redirect("/test", 302)
		return
	}

	name := strings.TrimSpace(c.GetString("name"))
	ageStr := strings.TrimSpace(c.GetString("age"))
	institution := strings.TrimSpace(c.GetString("institution"))
	gender := strings.TrimSpace(c.GetString("gender"))

	age, _ := strconv.Atoi(ageStr)
	if name == "" || age <= 0 || institution == "" || (gender != "laki-laki" && gender != "perempuan") {
		c.Data["Error"] = "Mohon lengkapi biodata dengan benar."
		c.Data["User"] = user
		c.Data["Invitation"] = inv
		c.TplName = "test_learning_style_start.html"
		return
	}

	o := orm.NewOrm()

	// Upsert draft result row so metadata persists even if user refreshes.
	var res models.LearningStyleResult
	err := o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		res = models.LearningStyleResult{
			Invitation:  inv,
			User:        user,
			TestDate:    time.Now(),
			TestName:    name,
			TestAge:     age,
			TestInstitution: institution,
			TestGender:  gender,
			InterpretationVisual:      seeds.LearningStyleInterpretationVisual(),
			InterpretationAuditory:    seeds.LearningStyleInterpretationAuditory(),
			InterpretationKinesthetic: seeds.LearningStyleInterpretationKinesthetic(),
		}
		if _, ierr := o.Insert(&res); ierr != nil {
			c.Data["Error"] = "Gagal menyimpan biodata. Silakan coba lagi."
			c.Data["User"] = user
			c.Data["Invitation"] = inv
			c.TplName = "test_learning_style_start.html"
			return
		}
	} else {
		res.TestName = name
		res.TestAge = age
		res.TestInstitution = institution
		res.TestGender = gender
		if res.TestDate.IsZero() {
			res.TestDate = time.Now()
		}
		if strings.TrimSpace(res.InterpretationVisual) == "" {
			res.InterpretationVisual = seeds.LearningStyleInterpretationVisual()
		}
		if strings.TrimSpace(res.InterpretationAuditory) == "" {
			res.InterpretationAuditory = seeds.LearningStyleInterpretationAuditory()
		}
		if strings.TrimSpace(res.InterpretationKinesthetic) == "" {
			res.InterpretationKinesthetic = seeds.LearningStyleInterpretationKinesthetic()
		}
		_, _ = o.Update(&res,
			"TestName", "TestAge", "TestInstitution", "TestGender", "TestDate",
			"InterpretationVisual", "InterpretationAuditory", "InterpretationKinesthetic",
		)
	}

	c.Redirect("/test/learning-style/instruction", 302)
}

// @router /test/learning-style/instruction [get]
func (c *LearningStyleTestController) InstructionPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsVAK(inv) {
		c.Redirect("/test", 302)
		return
	}

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.TplName = "test_learning_style_instruction.html"
}

type learningStyleAnswerExportRow struct {
	QuestionID int `orm:"column(question_id)"`
	Yes        int `orm:"column(answer_yes)"`
	No         int `orm:"column(answer_no)"`
}

func (c *LearningStyleTestController) answersMap(invID int) (map[int][2]int, error) {
	o := orm.NewOrm()
	var rows []learningStyleAnswerExportRow
	_, err := o.Raw(`
		SELECT question_id, answer_yes, answer_no
		FROM learning_style_answers
		WHERE invitation_id = ?
	`, invID).QueryRows(&rows)
	if err != nil {
		return nil, err
	}
	m := make(map[int][2]int, len(rows))
	for _, r := range rows {
		m[r.QuestionID] = [2]int{r.Yes, r.No}
	}
	return m, nil
}

// @router /test/learning-style/questions [get]
func (c *LearningStyleTestController) QuestionsPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsVAK(inv) {
		c.Redirect("/test", 302)
		return
	}

	o := orm.NewOrm()
	var qs []models.LearningStyleQuestion
	_, err := o.QueryTable(new(models.LearningStyleQuestion)).OrderBy("Number").All(&qs)
	if err != nil || len(qs) == 0 {
		c.Data["Error"] = "Soal tes gaya belajar belum tersedia."
		c.Data["User"] = user
		c.Data["Invitation"] = inv
		c.TplName = "test_learning_style_questions.html"
		return
	}

	ansMap, _ := c.answersMap(inv.Id)

	// Load result metadata (biodata)
	var res models.LearningStyleResult
	_ = o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res)

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["Questions"] = qs
	c.Data["AnswersMap"] = ansMap
	c.Data["Result"] = res
	c.Data["IsDev"] = strings.EqualFold(beego.BConfig.RunMode, "dev")
	c.TplName = "test_learning_style_questions.html"
}

type submitPayload struct {
	Answers     map[string]struct {
		Yes int `json:"yes"`
		No  int `json:"no"`
	} `json:"answers"`
	ForceSubmit bool `json:"force_submit"`
}

// @router /api/test/learning-style/submit [post]
func (c *LearningStyleTestController) SubmitAnswersAPI() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}
	if !c.ensureBatchAllowsVAK(inv) {
		c.Ctx.Output.SetStatus(409)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Batch tidak mengaktifkan Tes Gaya Belajar"}
		c.ServeJSON()
		return
	}

	var p submitPayload
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &p); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	var qs []models.LearningStyleQuestion
	_, err := o.QueryTable(new(models.LearningStyleQuestion)).OrderBy("Number").All(&qs)
	if err != nil || len(qs) != 36 {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal belum lengkap"}
		c.ServeJSON()
		return
	}

	// Validation: all questions must be answered (binary)
	if !p.ForceSubmit {
		for _, q := range qs {
			key := strconv.Itoa(q.Id)
			a, ok := p.Answers[key]
			if !ok {
				c.Ctx.Output.SetStatus(422)
				c.Data["json"] = map[string]interface{}{
					"success":             false,
					"message":             "Jawaban belum lengkap",
					"missing_question_id": q.Id,
				}
				c.ServeJSON()
				return
			}
			validBinary := (a.Yes == 1 && a.No == 0) || (a.Yes == 0 && a.No == 1)
			if !validBinary {
				c.Ctx.Output.SetStatus(422)
				c.Data["json"] = map[string]interface{}{
					"success":             false,
					"message":             "Jawaban harus diisi tepat satu pilihan (Ya atau Tidak)",
					"missing_question_id": q.Id,
				}
				c.ServeJSON()
				return
			}
		}
	}

	tx, err := o.Begin()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal memulai transaksi"}
		c.ServeJSON()
		return
	}

	rollback := func(status int, msg string) {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(status)
		c.Data["json"] = map[string]interface{}{"success": false, "message": msg}
		c.ServeJSON()
	}

	// Delete old answers for invitation
	if _, derr := tx.QueryTable(new(models.LearningStyleAnswer)).
		Filter("Invitation__Id", inv.Id).
		Delete(); derr != nil {
		rollback(500, "Gagal menghapus jawaban lama")
		return
	}

	// Insert new answers + compute scores from YES only
	scoreV, scoreA, scoreK := 0, 0, 0
	for _, q := range qs {
		key := strconv.Itoa(q.Id)
		a, ok := p.Answers[key]
		if !ok {
			continue
		}
		yes := a.Yes
		no := a.No
		validBinary := (yes == 1 && no == 0) || (yes == 0 && no == 1)
		if !validBinary {
			continue
		}

		qCopy := q
		ans := models.LearningStyleAnswer{
			Invitation: inv,
			User:       user,
			Question:   &qCopy,
			AnswerYes:  yes,
			AnswerNo:   no,
		}
		if _, ierr := tx.Insert(&ans); ierr != nil {
			rollback(500, "Gagal menyimpan jawaban")
			return
		}

		if yes == 1 {
			switch strings.ToUpper(strings.TrimSpace(q.Dimension)) {
			case "V":
				scoreV++
			case "A":
				scoreA++
			case "K":
				scoreK++
			}
		}
	}

	// Determine dominant type (tie-break: Visual > Auditori > Kinestetik)
	dominant := "Visual"
	max := scoreV
	if scoreA > max {
		max = scoreA
		dominant = "Auditori"
	}
	if scoreK > max {
		max = scoreK
		dominant = "Kinestetik"
	}

	// Upsert result row and finalize
	var res models.LearningStyleResult
	rerr := tx.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if rerr != nil || res.Id == 0 {
		res = models.LearningStyleResult{
			Invitation:  inv,
			User:        user,
			TestDate:    time.Now(),
			InterpretationVisual:      seeds.LearningStyleInterpretationVisual(),
			InterpretationAuditory:    seeds.LearningStyleInterpretationAuditory(),
			InterpretationKinesthetic: seeds.LearningStyleInterpretationKinesthetic(),
		}
		if _, ierr := tx.Insert(&res); ierr != nil {
			rollback(500, "Gagal menyimpan hasil")
			return
		}
	}
	res.ScoreVisual = scoreV
	res.ScoreAuditory = scoreA
	res.ScoreKinesthetic = scoreK
	res.DominantType = dominant
	res.TestDate = time.Now()
	// Always enforce canonical interpretation text so output stays consistent with template.
	res.InterpretationVisual = seeds.LearningStyleInterpretationVisual()
	res.InterpretationAuditory = seeds.LearningStyleInterpretationAuditory()
	res.InterpretationKinesthetic = seeds.LearningStyleInterpretationKinesthetic()

	_, _ = tx.Update(&res,
		"TestName", "TestAge", "TestInstitution", "TestGender", "TestDate",
		"ScoreVisual", "ScoreAuditory", "ScoreKinesthetic", "DominantType",
		"InterpretationVisual", "InterpretationAuditory", "InterpretationKinesthetic",
	)

	// Commit transaksi TERLEBIH DAHULU sebelum GetNextTestRedirect.
	// GetNextTestRedirect menggunakan orm.NewOrm() (koneksi baru) yang tidak bisa
	// melihat data dalam transaksi yang belum committed.
	if err := tx.Commit(); err != nil {
		rollback(500, "Gagal menyimpan jawaban")
		return
	}

	var finishRedirect = "/hasil-tes"
	var batch models.TestBatch
	if inv.BatchId != nil {
		batch.Id = *inv.BatchId
		_ = orm.NewOrm().Read(&batch)
	}
	nextURL := GetNextTestRedirect(inv.Id, &batch)

	// Mark invitation used
	justCompleted := false
	if nextURL != "" {
		finishRedirect = nextURL
	} else {
		if inv.Status != models.StatusInvitationUsed {
			inv.Status = models.StatusInvitationUsed
			inv.UsedAt = time.Now()
			_, _ = orm.NewOrm().Update(inv, "Status", "UsedAt")
			justCompleted = true
		}
	}

	if justCompleted {
		go utils.SendTestCompletionNotification(inv.UserId, "Gaya Belajar")
	}

	c.Data["json"] = map[string]interface{}{
		"success":         true,
		"finish_redirect": finishRedirect,
	}
	c.ServeJSON()
}

// @router /test/learning-style/finish [get]
func (c *LearningStyleTestController) FinishPage() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	if !c.ensureBatchAllowsVAK(inv) {
		c.Redirect("/test", 302)
		return
	}

	o := orm.NewOrm()
	var res models.LearningStyleResult
	err := o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		c.Redirect("/test/learning-style/start", 302)
		return
	}
	_ = user
	_ = res
	c.Redirect("/profile/learning-style", 302)
}

// @router /test/learning-style/result/excel [get]
func (c *LearningStyleTestController) ExportResultExcel() {
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
			c.Redirect("/test/learning-style/finish", 302)
			return
		}
		inv.Id = id
		if err := o.Read(&inv); err != nil {
			c.Redirect("/test/learning-style/finish", 302)
			return
		}
		if inv.UserId == nil || *inv.UserId != userID {
			c.Redirect("/test/learning-style/finish", 302)
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
		c.Redirect("/test/learning-style/finish", 302)
		return
	}

	excelBytes, err := buildLearningStyleResultXLSX(o, nil, &inv, &user)
	if err != nil {
		c.Redirect("/test/learning-style/finish", 302)
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
		downloadName = strings.TrimSpace(inv.Email)
	}
	filename := fmt.Sprintf("learning_style_result_%s.xlsx", makeSafeName(downloadName))
	c.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(excelBytes)
}

// DEV ONLY: POST /test/learning-style/dev-autofill
func (c *LearningStyleTestController) DevAutoFill() {
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

	o := orm.NewOrm()
	var qs []models.LearningStyleQuestion
	_, err := o.QueryTable(new(models.LearningStyleQuestion)).OrderBy("Number").All(&qs)
	if err != nil || len(qs) != 36 {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal belum lengkap"}
		c.ServeJSON()
		return
	}

	tx, err := o.Begin()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal memulai transaksi"}
		c.ServeJSON()
		return
	}

	rollback := func(status int, msg string) {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(status)
		c.Data["json"] = map[string]interface{}{"success": false, "message": msg}
		c.ServeJSON()
	}

	// Hapus jawaban lama
	if _, derr := tx.QueryTable(new(models.LearningStyleAnswer)).Filter("Invitation__Id", inv.Id).Delete(); derr != nil {
		rollback(500, "Gagal menghapus jawaban lama")
		return
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	scoreV, scoreA, scoreK := 0, 0, 0

	for _, q := range qs {
		yes := 0
		no := 1
		if rng.Intn(2) == 1 {
			yes = 1
			no = 0
		}

		qCopy := q
		ans := models.LearningStyleAnswer{
			Invitation: inv,
			User:       user,
			Question:   &qCopy,
			AnswerYes:  yes,
			AnswerNo:   no,
		}
		if _, ierr := tx.Insert(&ans); ierr != nil {
			rollback(500, "Gagal menyimpan jawaban")
			return
		}

		if yes == 1 {
			switch strings.ToUpper(strings.TrimSpace(q.Dimension)) {
			case "V":
				scoreV++
			case "A":
				scoreA++
			case "K":
				scoreK++
			}
		}
	}

	dominant := "Visual"
	max := scoreV
	if scoreA > max {
		max = scoreA
		dominant = "Auditori"
	}
	if scoreK > max {
		max = scoreK
		dominant = "Kinestetik"
	}

	var res models.LearningStyleResult
	rerr := tx.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if rerr != nil || res.Id == 0 {
		res = models.LearningStyleResult{
			Invitation:  inv,
			User:        user,
			TestDate:    time.Now(),
			TestName:    user.NamaLengkap,
			TestAge:     17,
			TestInstitution: "SMA Negeri 1",
			TestGender:  "laki-laki",
			InterpretationVisual:      seeds.LearningStyleInterpretationVisual(),
			InterpretationAuditory:    seeds.LearningStyleInterpretationAuditory(),
			InterpretationKinesthetic: seeds.LearningStyleInterpretationKinesthetic(),
		}
		if _, ierr := tx.Insert(&res); ierr != nil {
			rollback(500, "Gagal menyimpan hasil")
			return
		}
	}

	res.ScoreVisual = scoreV
	res.ScoreAuditory = scoreA
	res.ScoreKinesthetic = scoreK
	res.DominantType = dominant
	res.TestDate = time.Now()
	if strings.TrimSpace(res.TestName) == "" {
		res.TestName = user.NamaLengkap
	}
	if res.TestAge <= 0 {
		res.TestAge = 17
	}
	if strings.TrimSpace(res.TestInstitution) == "" {
		res.TestInstitution = "SMA Negeri 1"
	}
	if strings.TrimSpace(res.TestGender) == "" {
		res.TestGender = "laki-laki"
	}

	_, _ = tx.Update(&res,
		"TestName", "TestAge", "TestInstitution", "TestGender", "TestDate",
		"ScoreVisual", "ScoreAuditory", "ScoreKinesthetic", "DominantType",
		"InterpretationVisual", "InterpretationAuditory", "InterpretationKinesthetic",
	)

	// Commit transaksi TERLEBIH DAHULU sebelum GetNextTestRedirect.
	if err := tx.Commit(); err != nil {
		rollback(500, "Gagal menyimpan jawaban")
		return
	}

	var finishRedirect = "/hasil-tes"
	var batch models.TestBatch
	if inv.BatchId != nil {
		batch.Id = *inv.BatchId
		_ = orm.NewOrm().Read(&batch)
	}
	nextURL := GetNextTestRedirect(inv.Id, &batch)

	justCompleted := false
	if nextURL != "" {
		finishRedirect = nextURL
	} else {
		if inv.Status != models.StatusInvitationUsed {
			inv.Status = models.StatusInvitationUsed
			inv.UsedAt = time.Now()
			_, _ = orm.NewOrm().Update(inv, "Status", "UsedAt")
			justCompleted = true
		}
	}

	if justCompleted {
		go utils.SendTestCompletionNotification(inv.UserId, "Gaya Belajar")
	}

	c.Data["json"] = map[string]interface{}{"success": true, "next": finishRedirect}
	c.ServeJSON()
}

