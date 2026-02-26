package controllers

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

// ISTTestController menangani alur pengerjaan tes IST.
type ISTTestController struct {
	beego.Controller
}

// mustGetSessionInvitation memastikan user punya invitation valid di session.
func (c *ISTTestController) mustGetSessionInvitation() (*models.TestInvitation, *models.User, bool) {
	userID := c.GetSession("user_id")
	invID := c.GetSession("current_invitation_id")
	batchID := c.GetSession("current_batch_id")

	if userID == nil || invID == nil || batchID == nil {
		return nil, nil, false
	}

	o := orm.NewOrm()
	var inv models.TestInvitation
	inv.Id = invID.(int)
	if err := o.Read(&inv); err != nil {
		return nil, nil, false
	}

	var user models.User
	user.Id = userID.(int)
	if err := o.Read(&user); err != nil {
		return nil, nil, false
	}

	return &inv, &user, true
}

// StartISTPage menampilkan halaman start IST: nama, tanggal lahir, tujuan, Mulai Test IST.
// @router /test/ist/start [get]
func (c *ISTTestController) StartISTPage() {
	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login?next=/test/ist/start", 302)
		return
	}

	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	o := orm.NewOrm()
	var batch models.TestBatch
	batch.Id = inv.BatchId
	_ = o.Read(&batch)

	c.Data["User"] = user
	c.Data["Batch"] = batch
	c.Data["Invitation"] = inv
	if user.TanggalLahir != nil {
		c.Data["TanggalLahir"] = user.TanggalLahir.Format("2006-01-02")
	} else {
		c.Data["TanggalLahir"] = ""
	}
	c.TplName = "test_ist_start.html"
}

// SubmitStartIST menyimpan tanggal lahir (jika diisi) dan redirect ke announcement.
// @router /test/ist/start [post]
func (c *ISTTestController) SubmitStartIST() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	tglStr := strings.TrimSpace(c.GetString("tanggal_lahir"))
	if tglStr != "" {
		t, err := time.Parse("2006-01-02", tglStr)
		if err == nil {
			user.TanggalLahir = &t
			o := orm.NewOrm()
			o.Update(user, "TanggalLahir")
		}
	}

	c.SetSession("ist_started_at", time.Now())
	c.SetSession("ist_current_subtest_index", 0)
	_ = inv
	c.Redirect("/test/ist/announcement", 302)
}

// AnnouncementPage menampilkan peraturan tes.
// @router /test/ist/announcement [get]
func (c *ISTTestController) AnnouncementPage() {
	_, _, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	c.TplName = "test_ist_announcement.html"
}

// SubtestPage menampilkan soal subtest tertentu.
// @router /test/ist/subtest/:code [get]
func (c *ISTTestController) SubtestPage() {
	code := c.Ctx.Input.Param(":code")
	if code == "" {
		c.Redirect("/test/ist/announcement", 302)
		return
	}

	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	o := orm.NewOrm()
	var sub models.ISTSubtest
	if err := o.QueryTable(new(models.ISTSubtest)).Filter("Code", code).One(&sub); err != nil {
		c.Data["Error"] = "Subtest tidak ditemukan"
		c.TplName = "test_ist_subtest.html"
		return
	}

	var questions []models.ISTQuestion
	_, err := o.QueryTable(new(models.ISTQuestion)).Filter("Subtest__Id", sub.Id).OrderBy("Number").All(&questions)
	if err != nil || len(questions) == 0 {
		c.Data["Error"] = "Belum ada soal untuk subtest ini"
		c.Data["Subtest"] = &sub
		c.TplName = "test_ist_subtest.html"
		return
	}

	c.Data["Subtest"] = &sub
	c.Data["Questions"] = questions
	c.Data["Invitation"] = inv
	c.Data["User"] = user
	c.Data["TimerMinutes"] = 5 // 5 menit per subtest untuk trial
	c.TplName = "test_ist_subtest.html"
}

// SubmitSubtestAPI menyimpan jawaban dan mengembalikan hasil (raw score, next subtest).
// @router /api/test/ist/subtest/:code [post]
func (c *ISTTestController) SubmitSubtestAPI() {
	code := c.Ctx.Input.Param(":code")
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	var payload struct {
		Answers map[string]string `json:"answers"` // qid -> "A"/"B"/...
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	var sub models.ISTSubtest
	if err := o.QueryTable(new(models.ISTSubtest)).Filter("Code", code).One(&sub); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Subtest tidak ditemukan"}
		c.ServeJSON()
		return
	}

	rawScore := 0
	for qidStr, ans := range payload.Answers {
		qid, _ := strconv.Atoi(qidStr)
		if qid <= 0 {
			continue
		}
		var q models.ISTQuestion
		q.Id = qid
		if err := o.Read(&q); err != nil {
			continue
		}
		if q.Subtest == nil || q.Subtest.Id != sub.Id {
			continue
		}
		correct := strings.EqualFold(strings.TrimSpace(ans), strings.TrimSpace(q.Correct))
		if correct {
			rawScore++
		}
		// Simpan jawaban
		istAns := models.ISTAnswer{
			Invitation:  inv,
			User:        user,
			Subtest:     &sub,
			Question:    &q,
			Answer:      strings.ToUpper(strings.TrimSpace(ans)),
			IsCorrect:   correct,
		}
		o.Insert(&istAns)
	}

	// Update / buat ISTResult dengan raw score
	var res models.ISTResult
	err := o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil {
		res = models.ISTResult{
			Invitation: inv,
			User:       user,
		}
		_, _ = o.Insert(&res)
	}

	updateFields := []string{}
	switch code {
	case "SE":
		res.RawSE = rawScore
		updateFields = append(updateFields, "RawSE")
	case "WA":
		res.RawWA = rawScore
		updateFields = append(updateFields, "RawWA")
	case "AN":
		res.RawAN = rawScore
		updateFields = append(updateFields, "RawAN")
	case "ME":
		res.RawME = rawScore
		updateFields = append(updateFields, "RawME")
	case "RA":
		res.RawRA = rawScore
		updateFields = append(updateFields, "RawRA")
	case "ZA":
		res.RawZA = rawScore
		updateFields = append(updateFields, "RawZA")
	case "FA":
		res.RawFA = rawScore
		updateFields = append(updateFields, "RawFA")
	case "WU":
		res.RawWU = rawScore
		updateFields = append(updateFields, "RawWU")
	case "GE":
		res.RawGE = rawScore
		updateFields = append(updateFields, "RawGE")
	}
	if len(updateFields) > 0 {
		o.Update(&res, updateFields...)
	}

	// Cari subtest berikutnya
	var subs []models.ISTSubtest
	o.QueryTable(new(models.ISTSubtest)).OrderBy("OrderIndex").All(&subs)
	nextCode := ""
	for i, s := range subs {
		if s.Code == code && i+1 < len(subs) {
			nextCode = subs[i+1].Code
			break
		}
	}

	c.Data["json"] = map[string]interface{}{
		"success":     true,
		"raw_score":   rawScore,
		"next_subtest": nextCode,
		"is_complete": nextCode == "",
	}
	c.ServeJSON()
}
