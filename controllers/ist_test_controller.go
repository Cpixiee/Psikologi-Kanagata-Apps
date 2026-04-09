package controllers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/rand"
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

// istAspectRow merepresentasikan satu baris psikogram (untuk HTML & PDF).
type istAspectRow struct {
	No      int
	Nama    string
	Kurang  bool
	Cukup   bool
	Baik    bool
	SkorSW  int
	Subtest string // keterangan singkat, opsional
}

// computeAspectCategoryFromSW mengubah skor standar (SW) menjadi kategori Kurang/Cukup/Baik.
// Threshold masih generic (bisa disesuaikan dengan warna norma yang kamu punya).
func computeAspectCategoryFromSW(sw int) string {
	switch {
	case sw >= 120:
		return "Baik Sekali"
	case sw >= 110:
		return "Baik"
	case sw >= 100:
		return "Cukup Baik"
	case sw >= 90:
		return "Cukup"
	case sw >= 80:
		return "Kurang"
	default:
		return "Kurang Sekali"
	}
}

// psychogramCatIdxFromSW mengubah SW ke index kolom kategori psikogram (0..5):
// 0 Kurang Sekali, 1 Kurang, 2 Cukup, 3 Cukup Baik, 4 Baik, 5 Baik Sekali.
func psychogramCatIdxFromSW(sw int) int {
	switch {
	case sw >= 120:
		return 5
	case sw >= 110:
		return 4
	case sw >= 100:
		return 3
	case sw >= 90:
		return 2
	case sw >= 80:
		return 1
	default:
		return 0
	}
}

// buildISTAspectRows membentuk 9 aspek psikogram sesuai manual IST (kombinasi subtes).
// Contoh mapping (bisa disesuaikan):
// 1 Penalaran Konkret: SE + GE
// 2 Penalaran Verbal : SE + WA + GE
// 3 Daya Analisis    : AN
// 4 Penalaran Abstrak: ZR
// 5 Daya Ingat       : ME
// 6 Kemampuan Berhitung: RA
// 7 Analogi Angka    : ZR
// 8 Daya Bayang Konstruksional: FA
// 9 Daya Bayang Ruang: WU
func buildISTAspectRows(res *models.ISTResult) []istAspectRow {
	if res == nil {
		return nil
	}

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

	type def struct {
		no      int
		nama    string
		scoreSW int
		subtest string
	}

	defs := []def{
		{1, "Penalaran Konkret", avg(res.StdSE, res.StdGE), "SE+GE"},
		{2, "Penalaran Verbal", avg(res.StdSE, res.StdWA, res.StdGE), "SE+WA+GE"},
		{3, "Daya Analisis", res.StdAN, "AN"},
		{4, "Penalaran Abstrak", res.StdZA, "ZR"},
		{5, "Daya Ingat", res.StdME, "ME"},
		{6, "Kemampuan Berhitung", res.StdRA, "RA"},
		{7, "Analogi Angka", res.StdZA, "ZR"},
		{8, "Daya Bayang Konstruksional", res.StdFA, "FA"},
		{9, "Daya Bayang Ruang", res.StdWU, "WU"},
	}

	var rows []istAspectRow
	for _, d := range defs {
		cat := computeAspectCategoryFromSW(d.scoreSW)
		row := istAspectRow{
			No:      d.no,
			Nama:    d.nama,
			SkorSW:  d.scoreSW,
			Subtest: d.subtest,
		}
		switch cat {
		case "Baik":
			row.Baik = true
		case "Cukup":
			row.Cukup = true
		default:
			row.Kurang = true
		}
		rows = append(rows, row)
	}
	return rows
}

// ISTTestController menangani alur pengerjaan tes IST.
type ISTTestController struct {
	beego.Controller
}

func istAllowedOrder() []string {
	return []string{"SE", "WA", "AN", "GE", "RA", "ZR", "FA", "WU", "ME"}
}

func istAllowedSet() map[string]bool {
	m := make(map[string]bool)
	for _, c := range istAllowedOrder() {
		m[c] = true
	}
	// Backward-compatible: sebagian DB lama pakai "ZA" untuk subtest VI.
	// Kita treat ZA sebagai alias dari ZR, tapi flow UI tetap pakai ZR.
	m["ZA"] = true
	return m
}

func normalizeISTCode(code string) string {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "ZA" {
		return "ZR"
	}
	return code
}

// filterISTDummyQuestions menghapus soal-soal dummy dari seeder trial lama
// yang prompt-nya "Soal contoh ..." dan opsi jawabannya hanya A-E polos.
// Dipakai di halaman soal dan perhitungan progres supaya konsisten.
func filterISTDummyQuestions(questions []models.ISTQuestion) []models.ISTQuestion {
	var filtered []models.ISTQuestion
	for _, q := range questions {
		p := strings.ToLower(strings.TrimSpace(q.Prompt))
		isDummyPrompt := strings.HasPrefix(p, "soal contoh")
		isDummyOptions := strings.TrimSpace(q.OptionA) == "A" &&
			strings.TrimSpace(q.OptionB) == "B" &&
			strings.TrimSpace(q.OptionC) == "C" &&
			strings.TrimSpace(q.OptionD) == "D" &&
			strings.TrimSpace(q.OptionE) == "E"
		if isDummyPrompt && isDummyOptions {
			continue
		}
		filtered = append(filtered, q)
	}
	return filtered
}

// istQuestionRangeByCode mengembalikan rentang nomor soal 176-item IST
// untuk setiap subtest, supaya progres tidak terpengaruh soal-soal sisa
// dari seeder lama yang nomornya di luar rentang resmi.
func istQuestionRangeByCode(code string) (start, end int) {
	switch strings.ToUpper(strings.TrimSpace(code)) {
	case "SE":
		return 1, 20
	case "WA":
		return 21, 40
	case "AN":
		return 41, 60
	case "GE":
		return 61, 76
	case "RA":
		return 77, 96
	case "ZR", "ZA":
		return 97, 116
	case "FA":
		return 117, 136
	case "WU":
		return 137, 156
	case "ME":
		return 157, 176
	default:
		return 0, 0
	}
}

func normalizeISTOptionText(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "–", "-")
	s = strings.ReplaceAll(s, "—", "-")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

func toTitleCaseWords(s string) string {
	s = normalizeISTOptionText(s)
	if s == "" {
		return ""
	}
	parts := strings.Fields(s)
	for i, p := range parts {
		runes := []rune(p)
		if len(runes) == 0 {
			continue
		}
		runes[0] = unicode.ToUpper(runes[0])
		for j := 1; j < len(runes); j++ {
			runes[j] = unicode.ToLower(runes[j])
		}
		parts[i] = string(runes)
	}
	return strings.Join(parts, " ")
}

func istOptionTextByAnswer(q *models.ISTQuestion, ans string) string {
	switch strings.ToUpper(strings.TrimSpace(ans)) {
	case "A":
		return q.OptionA
	case "B":
		return q.OptionB
	case "C":
		return q.OptionC
	case "D":
		return q.OptionD
	case "E":
		return q.OptionE
	default:
		return ""
	}
}

// scoreGEQuestion computes item score for GE (61-76) based on option text groups (skor 2/1/0).
func scoreGEQuestion(qNumber int, chosenOptionText string) int {
	ch := normalizeISTOptionText(chosenOptionText)
	if ch == "" {
		return 0
	}
	type key struct {
		s2 []string
		s1 []string
	}
	keys := map[int]key{
		61: {s2: []string{"bunga", "kembang", "perdu"}, s1: []string{"tumbuh-tumbuhan", "tangkai", "harum"}},
		62: {s2: []string{"alat indera", "indera", "panca indera"}, s1: []string{"organ", "alat tubuh"}},
		63: {s2: []string{"hablur", "kristal", "zat arang"}, s1: []string{"berkilauan", "mengkilat", "bening"}},
		64: {s2: []string{"musim"}, s1: []string{"cuaca"}},
		65: {s2: []string{"pembawa berita", "alat perhubungan"}, s1: []string{"telekomunikasi", "perhubungan", "komunikasi"}},
		66: {s2: []string{"alat optik", "optik"}, s1: []string{"lensa"}},
		67: {s2: []string{"alat pencernaan"}, s1: []string{"jalan makanan", "perut", "isi perut", "pencernaan makanan"}},
		68: {s2: []string{"jumlah/kuantitas", "jumlah", "kuantitas", "penyebut jumlah", "penyertaan jumlah"}, s1: []string{"mengukur", "ukuran"}},
		69: {s2: []string{"bibit/bakal/embrio", "bibit", "bakal", "embrio", "alat pembiak", "permulaan penghidupan"}, s1: []string{"sel", "pembiakan"}},
		70: {s2: []string{"simbol", "lambang", "tanda"}, s1: []string{"nama", "tanda pengenal"}},
		71: {s2: []string{"makhluk", "organism", "organisme", "makhluk hidup"}, s1: []string{"tumbuh", "ilmu hayat", "biologi"}},
		72: {s2: []string{"wadah", "tempat pengisi", "tempat penyimpan"}, s1: []string{"alat", "tempat sesuatu", "tempat", "benda"}},
		73: {s2: []string{"pengertian waktu", "batas"}, s1: []string{"waktu", "lamanya", "masa/saat", "masa", "saat"}},
		74: {s2: []string{"kata sifat", "watak", "sifat karakter"}, s1: []string{"sifat"}},
		75: {s2: []string{"regulator harga", "pengertian ekonomi"}, s1: []string{"dagang", "pembelian", "penjualan", "niaga", "jual beli"}},
		76: {s2: []string{"pengertian ruang", "penyebut ruang"}, s1: []string{"arah", "tempat/ruang", "tempat", "ruang", "letak", "penunjuk tempat", "penentuan daerah"}},
	}
	k, ok := keys[qNumber]
	if !ok {
		return 0
	}
	for _, v := range k.s2 {
		if ch == normalizeISTOptionText(v) {
			return 2
		}
	}
	for _, v := range k.s1 {
		if ch == normalizeISTOptionText(v) {
			return 1
		}
	}
	return 0
}

func findISTSubtestByCode(o orm.Ormer, code string) (*models.ISTSubtest, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	tryCodes := []string{code}
	if code == "ZR" {
		tryCodes = append(tryCodes, "ZA")
	} else if code == "ZA" {
		tryCodes = append(tryCodes, "ZR")
	}
	for _, tc := range tryCodes {
		var sub models.ISTSubtest
		if err := o.QueryTable(new(models.ISTSubtest)).Filter("Code", tc).One(&sub); err == nil && sub.Id != 0 {
			// Untuk render template, gunakan code yang diminta user (supaya petunjuk ZR tampil meskipun DB pakai ZA).
			sub.Code = normalizeISTCode(code)
			return &sub, nil
		}
	}
	return nil, errors.New("subtest not found")
}

func getISTCurrentSubtestCode(o orm.Ormer, invID int) (string, bool, error) {
	order := istAllowedOrder()
	
	// Cek progress dari tabel ist_progress (sumber utama untuk tracking)
	var progressList []models.ISTProgress
	_, _ = o.QueryTable(new(models.ISTProgress)).
		Filter("Invitation__Id", invID).
		Filter("Status", "completed").
		All(&progressList)
	
	completedMap := make(map[string]bool)
	for _, p := range progressList {
		completedMap[p.SubtestCode] = true
	}
	
	// Cari subtest pertama yang belum completed
	for _, code := range order {
		// Normalize code (ZA -> ZR)
		normalizedCode := normalizeISTCode(code)
		
		// Jika sudah completed di progress table, skip
		if completedMap[normalizedCode] || completedMap[code] {
			continue
		}
		
		// Cek apakah subtest ada di DB
		sub, err := findISTSubtestByCode(o, code)
		if err != nil {
			continue
		}
		
		// Cek apakah ada soal untuk subtest ini
		start, end := istQuestionRangeByCode(code)
		var qs []models.ISTQuestion
		q := o.QueryTable(new(models.ISTQuestion)).Filter("Subtest__Id", sub.Id)
		if start > 0 && end > 0 {
			q = q.Filter("Number__gte", start).Filter("Number__lte", end)
		}
		_, _ = q.OrderBy("Number").All(&qs)
		qs = filterISTDummyQuestions(qs)
		
		if len(qs) == 0 {
			// Jika belum ada soal, anggap belum bisa dikerjakan -> current.
			return code, false, nil
		}
		
		// Jika belum completed di progress table, ini adalah current subtest
		return code, false, nil
	}
	
	// Semua subtest sudah completed
	return "", true, nil
}

func (c *ISTTestController) redirectToISTCurrent(invID int) {
	o := orm.NewOrm()
	current, complete, _ := getISTCurrentSubtestCode(o, invID)
	if complete || current == "" {
		c.Redirect("/test/ist/finish", 302)
		return
	}
	c.Redirect("/test/ist/instruction/"+current, 302)
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

	// Anti-back/anti-skip: kalau sudah ada progres, jangan balik ke start.
	o := orm.NewOrm()
	if current, complete, _ := getISTCurrentSubtestCode(o, inv.Id); complete {
		c.Redirect("/test/ist/finish", 302)
		return
	} else if current != "" && current != "SE" {
		c.Redirect("/test/ist/instruction/"+current, 302)
		return
	}

	var batch models.TestBatch
	if inv.BatchId != nil {
		batch.Id = *inv.BatchId
		_ = o.Read(&batch)
	}

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
	// DOB wajib untuk skoring IST (usia dipakai untuk norma)
	if tglStr == "" {
		o := orm.NewOrm()
		var batch models.TestBatch
		if inv.BatchId != nil {
			batch.Id = *inv.BatchId
			_ = o.Read(&batch)
		}
		c.Data["User"] = user
		c.Data["Batch"] = batch
		c.Data["Invitation"] = inv
		c.Data["TanggalLahir"] = ""
		c.Data["Error"] = "Tanggal lahir wajib diisi untuk skoring IST."
		c.TplName = "test_ist_start.html"
		return
	}
		t, err := time.Parse("2006-01-02", tglStr)
	if err != nil {
		o := orm.NewOrm()
		var batch models.TestBatch
		if inv.BatchId != nil {
			batch.Id = *inv.BatchId
			_ = o.Read(&batch)
		}
		c.Data["User"] = user
		c.Data["Batch"] = batch
		c.Data["Invitation"] = inv
		c.Data["TanggalLahir"] = tglStr
		c.Data["Error"] = "Format tanggal lahir tidak valid."
		c.TplName = "test_ist_start.html"
		return
	}
	user.TanggalLahir = &t
	o := orm.NewOrm()
	// Pastikan user sudah di-load dengan benar sebelum update
	var userToUpdate models.User
	userToUpdate.Id = user.Id
	if err := o.Read(&userToUpdate); err != nil {
		var batch models.TestBatch
		if inv.BatchId != nil {
			batch.Id = *inv.BatchId
			_ = o.Read(&batch)
		}
		c.Data["User"] = user
		c.Data["Batch"] = batch
		c.Data["Invitation"] = inv
		c.Data["TanggalLahir"] = tglStr
		c.Data["Error"] = "User tidak ditemukan. Silakan login ulang."
		c.TplName = "test_ist_start.html"
		return
	}
	
	// Update tanggal lahir menggunakan raw SQL untuk memastikan update berhasil
	// Gunakan placeholder PostgreSQL ($1, $2) untuk kompatibilitas yang lebih baik
	_, uerr := o.Raw("UPDATE users SET tanggal_lahir = $1 WHERE id = $2", tglStr, user.Id).Exec()
	if uerr != nil {
		// Log error untuk debugging
		logs.Error("Error updating tanggal_lahir for user %d: %v", user.Id, uerr)
		var batch models.TestBatch
		if inv.BatchId != nil {
			batch.Id = *inv.BatchId
			_ = o.Read(&batch)
		}
		c.Data["User"] = user
		c.Data["Batch"] = batch
		c.Data["Invitation"] = inv
		c.Data["TanggalLahir"] = tglStr
		c.Data["Error"] = "Gagal menyimpan tanggal lahir. Silakan coba lagi."
		c.TplName = "test_ist_start.html"
		return
	}
	
	// Update user object untuk session (setelah update berhasil)
	user.TanggalLahir = &t

	c.SetSession("ist_started_at", time.Now())
	_ = inv
	c.Redirect("/test/ist/announcement", 302)
}

// AnnouncementPage menampilkan peraturan tes.
// @router /test/ist/announcement [get]
func (c *ISTTestController) AnnouncementPage() {
	inv, _, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	// Jika sudah ada progres, redirect ke subtest current.
	o := orm.NewOrm()
	if current, complete, _ := getISTCurrentSubtestCode(o, inv.Id); complete {
		c.Redirect("/test/ist/finish", 302)
		return
	} else if current != "" && current != "SE" {
		c.Redirect("/test/ist/instruction/"+current, 302)
		return
	}
	c.TplName = "test_ist_announcement.html"
}

// InstructionPage menampilkan petunjuk kelompok soal sebelum masuk ke soal.
// @router /test/ist/instruction/:code [get]
func (c *ISTTestController) InstructionPage() {
	code := normalizeISTCode(c.Ctx.Input.Param(":code"))
	if code == "" {
		c.Redirect("/test/ist/announcement", 302)
		return
	}
	allowed := istAllowedSet()
	if !allowed[code] {
		c.Redirect("/test/ist/announcement", 302)
		return
	}

	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	o := orm.NewOrm()
	// Anti-back/anti-skip: user idealnya hanya boleh akses subtest current.
	// Namun untuk menghindari kasus "terkunci" di subtest sebelumnya karena
	// perbedaan data, kita izinkan akses ke subtest berikutnya (next step)
	// meskipun current masih terdeteksi di subtest sebelumnya.
	if current, complete, _ := getISTCurrentSubtestCode(o, inv.Id); complete {
		c.Redirect("/test/ist/finish", 302)
		return
	} else if current != "" && code != current {
		order := istAllowedOrder()
		currentIdx, codeIdx := -1, -1
		for i, v := range order {
			if v == current {
				currentIdx = i
			}
			if v == code {
				codeIdx = i
			}
		}
		// Jika user mencoba lompat jauh (bukan tepat 1 langkah setelah current),
		// paksa kembali ke current. Kalau hanya satu langkah berikutnya, izinkan.
		if !(currentIdx >= 0 && codeIdx == currentIdx+1) {
			c.Redirect("/test/ist/instruction/"+current, 302)
			return
		}
	}

	sub, err := findISTSubtestByCode(o, code)
	if err != nil {
		c.Redirect("/test/ist/announcement", 302)
		return
	}

	c.Data["Subtest"] = sub
	c.Data["Invitation"] = inv
	c.Data["User"] = user
	c.TplName = "test_ist_instruction.html"
}

// SubtestPage menampilkan soal subtest tertentu.
// @router /test/ist/subtest/:code [get]
func (c *ISTTestController) SubtestPage() {
	code := normalizeISTCode(c.Ctx.Input.Param(":code"))
	if code == "" {
		c.Redirect("/test/ist/announcement", 302)
		return
	}
	allowed := istAllowedSet()
	if !allowed[code] {
		c.Redirect("/test/ist/announcement", 302)
		return
	}

	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}

	o := orm.NewOrm()
	// Anti-back/anti-skip: sama logika dengan InstructionPage.
	if current, complete, _ := getISTCurrentSubtestCode(o, inv.Id); complete {
		c.Redirect("/test/ist/finish", 302)
		return
	} else if current != "" && code != current {
		order := istAllowedOrder()
		currentIdx, codeIdx := -1, -1
		for i, v := range order {
			if v == current {
				currentIdx = i
			}
			if v == code {
				codeIdx = i
			}
		}
		if !(currentIdx >= 0 && codeIdx == currentIdx+1) {
			c.Redirect("/test/ist/instruction/"+current, 302)
			return
		}
	}

	sub, err := findISTSubtestByCode(o, code)
	if err != nil {
		c.Data["Error"] = "Subtest tidak ditemukan"
		c.TplName = "test_ist_subtest.html"
		return
	}

	var questions []models.ISTQuestion
	_, err = o.QueryTable(new(models.ISTQuestion)).Filter("Subtest__Id", sub.Id).OrderBy("Number").All(&questions)
	if err != nil || len(questions) == 0 {
		c.Data["Error"] = "Belum ada soal untuk subtest ini"
		c.Data["Subtest"] = sub
		c.TplName = "test_ist_subtest.html"
		return
	}

	// Filter out soal dummy dari seeder trial lama ("Soal contoh 1/2 untuk XX...").
	// Jangan filter berdasarkan nomor, karena SE no 1-2 adalah soal asli.
	questions = filterISTDummyQuestions(questions)

	c.Data["Subtest"] = sub
	c.Data["Questions"] = questions
	c.Data["Invitation"] = inv
	c.Data["User"] = user
	c.Data["TimerMinutes"] = 20 // 20 menit per subtest (final)
	c.TplName = "test_ist_subtest.html"
}

// SubmitSubtestAPI menyimpan jawaban dan mengembalikan hasil (raw score, next subtest).
// @router /api/test/ist/subtest/:code [post]
func (c *ISTTestController) SubmitSubtestAPI() {
	code := normalizeISTCode(c.Ctx.Input.Param(":code"))
	allowed := istAllowedSet()
	if !allowed[code] {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Subtest tidak ditemukan"}
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

	var payload struct {
		Answers      map[string]string `json:"answers"`       // qid -> "A"/"B"/...
		ForceSubmit  bool              `json:"force_submit"`  // true jika auto-submit karena pelanggaran/keluar fullscreen
		ViolationSrc string            `json:"violation_src"` // optional: sumber pemicu force submit (debug)
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &payload); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Format tidak valid"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	// Anti-back/anti-skip: submission idealnya hanya untuk subtest current.
	// Namun untuk mencegah peserta "terkunci" karena perbedaan data progres,
	// kita izinkan juga submit untuk subtest yang persis satu langkah
	// setelah current (next subtest dalam urutan resmi).
	if current, complete, _ := getISTCurrentSubtestCode(o, inv.Id); complete {
		c.Ctx.Output.SetStatus(409)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Tes sudah selesai", "finish_redirect": "/test/ist/finish"}
		c.ServeJSON()
		return
	} else if current != "" && code != current {
		order := istAllowedOrder()
		currentIdx, codeIdx := -1, -1
		for i, v := range order {
			if v == current {
				currentIdx = i
			}
			if v == code {
				codeIdx = i
			}
		}
		// Jika code bukan current dan juga bukan tepat 1 langkah setelahnya,
		// anggap percobaan lompat subtest dan paksa kembali ke current.
		if !(currentIdx >= 0 && codeIdx == currentIdx+1) {
			c.Ctx.Output.SetStatus(409)
			c.Data["json"] = map[string]interface{}{"success": false, "message": "Tidak boleh lompat subtest", "redirect": "/test/ist/instruction/" + current}
			c.ServeJSON()
			return
		}
	}

	sub, err := findISTSubtestByCode(o, code)
	if err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Subtest tidak ditemukan"}
		c.ServeJSON()
		return
	}

	// Ambil daftar soal untuk validasi wajib terjawab.
	// Penting: gunakan rentang resmi 176-item + filter dummy yang sama
	// dengan getISTCurrentSubtestCode supaya penentuan progres konsisten.
	var questions []models.ISTQuestion
	start, end := istQuestionRangeByCode(code)
	q := o.QueryTable(new(models.ISTQuestion)).Filter("Subtest__Id", sub.Id)
	if start > 0 && end > 0 {
		q = q.Filter("Number__gte", start).Filter("Number__lte", end)
	}
	_, _ = q.OrderBy("Number").All(&questions)
	questions = filterISTDummyQuestions(questions)
	if len(questions) == 0 {
		c.Ctx.Output.SetStatus(422)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Soal subtest belum tersedia"}
		c.ServeJSON()
		return
	}

	// Validasi: default (normal) semua soal yang tampil harus dijawab.
	// Jika force_submit (mis. anti-cheat), izinkan jawaban parsial (auto-submit apa yang sudah dikerjakan).
	if !payload.ForceSubmit {
		for _, q := range questions {
			if payload.Answers == nil {
				c.Ctx.Output.SetStatus(422)
				c.Data["json"] = map[string]interface{}{
					"success":        false,
					"message":        "Jawaban nomor " + strconv.Itoa(q.Number) + " belum dikerjakan",
					"missing_number": q.Number,
				}
				c.ServeJSON()
				return
			}
			if ans, ok := payload.Answers[strconv.Itoa(q.Id)]; !ok || strings.TrimSpace(ans) == "" {
				c.Ctx.Output.SetStatus(422)
				c.Data["json"] = map[string]interface{}{
					"success":        false,
					"message":        "Jawaban nomor " + strconv.Itoa(q.Number) + " belum dikerjakan",
					"missing_number": q.Number,
				}
				c.ServeJSON()
				return
			}
		}
	}

	// Simpan jawaban secara atomik (hapus lama -> insert baru) agar tidak ada state setengah jalan.
	tx, err := o.Begin()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal memulai transaksi penyimpanan jawaban"}
		c.ServeJSON()
		return
	}
	rollback := func(msg string, err error) {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(500)
		if err != nil {
			c.Data["json"] = map[string]interface{}{"success": false, "message": msg, "error": err.Error()}
		} else {
			c.Data["json"] = map[string]interface{}{"success": false, "message": msg}
		}
		c.ServeJSON()
	}

	// Hapus jawaban lama untuk subtest ini (agar boleh resubmit tanpa error UNIQUE)
	if _, derr := tx.QueryTable(new(models.ISTAnswer)).
		Filter("Invitation__Id", inv.Id).
		Filter("Subtest__Id", sub.Id).
		Delete(); derr != nil {
		rollback("Gagal menghapus jawaban lama", derr)
		return
	}

	rawScore := 0
	gePoints := 0
	for i := range questions {
		q := &questions[i]
		if payload.Answers == nil {
			// Mode force_submit tanpa jawaban apapun.
			continue
		}
		ans, ok := payload.Answers[strconv.Itoa(q.Id)]
		if !ok || strings.TrimSpace(ans) == "" {
			// Pada mode force_submit, soal tidak wajib dijawab, skip.
			// Pada mode normal, kondisi ini tidak mungkin terjadi karena sudah divalidasi di atas.
			continue
		}
		ansRaw := strings.TrimSpace(ans)
		ansNorm := strings.ToUpper(ansRaw)
		correct := false
		score := 0
		storedAnswer := ansNorm
		if code == "GE" {
			// GE menerima jawaban teks bebas.
			// Backward-compatible: jika client lama masih mengirim A-E, konversi ke teks opsi.
			textAns := ansRaw
			if ansNorm == "A" || ansNorm == "B" || ansNorm == "C" || ansNorm == "D" || ansNorm == "E" {
				textAns = istOptionTextByAnswer(q, ansNorm)
			}
			score = scoreGEQuestion(q.Number, textAns)
			correct = score > 0
			gePoints += score
			storedAnswer = toTitleCaseWords(textAns)
		} else {
			correct = strings.EqualFold(ansNorm, strings.TrimSpace(q.Correct))
			if correct {
				score = 1
				rawScore++
			}
		}
		// Simpan jawaban
		istAns := models.ISTAnswer{
			Invitation: inv,
			User:       user,
			Subtest:    sub,
			Question:   q,
			Answer:     storedAnswer,
			Score:      score,
			IsCorrect:  correct,
		}
		if _, ierr := tx.Insert(&istAns); ierr != nil {
			rollback("Gagal menyimpan jawaban. Pastikan migrasi database sudah dijalankan.", ierr)
			return
		}
	}

	if code == "GE" {
		raw := int(math.Round(float64(gePoints) / 1.6))
		if raw < 0 {
			raw = 0
		}
		if raw > 20 {
			raw = 20
		}
		rawScore = raw
	}

	if err := tx.Commit(); err != nil {
		rollback("Gagal menyimpan jawaban", err)
		return
	}

	// Update / buat ISTResult dengan raw score
	// PASTIKAN result selalu ada sebelum update raw scores
	var res models.ISTResult
	err = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		// Result belum ada, buat baru dengan berbagai metode untuk memastikan berhasil
		logs.Info("Creating IST result for invitation %d (user_id=%d)", inv.Id, user.Id)
		
		// Metode 1: Coba dengan ORM
		res = models.ISTResult{
			Invitation: inv,
			User:       user,
		}
		num, ierr := o.Insert(&res)
		if ierr != nil || res.Id == 0 {
			logs.Warning("ORM Insert failed for invitation %d: %v, trying raw SQL", inv.Id, ierr)
			
			// Metode 2: Coba dengan raw SQL langsung
			rawErr := o.Raw("INSERT INTO ist_results (invitation_id, user_id, created_at) VALUES ($1, $2, NOW()) RETURNING id", inv.Id, user.Id).QueryRow(&res.Id)
			if rawErr != nil {
				logs.Error("Raw SQL Insert also failed for invitation %d: %v", inv.Id, rawErr)
				// Metode 3: Coba dengan INSERT ... ON CONFLICT DO NOTHING
				conflictErr := o.Raw(`
					INSERT INTO ist_results (invitation_id, user_id, created_at) 
					VALUES ($1, $2, NOW()) 
					ON CONFLICT (invitation_id) DO UPDATE SET user_id = EXCLUDED.user_id
					RETURNING id
				`, inv.Id, user.Id).QueryRow(&res.Id)
				if conflictErr != nil {
					logs.Error("All insert methods failed for invitation %d: %v", inv.Id, conflictErr)
					// Return error atau continue dengan result kosong?
					// Untuk sekarang, continue saja dan log error
				} else {
					logs.Info("Created IST result with ON CONFLICT for invitation %d (id=%d)", inv.Id, res.Id)
				}
			} else {
				logs.Info("Created IST result with raw SQL for invitation %d (id=%d)", inv.Id, res.Id)
			}
			
			// Reload result setelah insert
			if res.Id > 0 {
				err = o.QueryTable(new(models.ISTResult)).Filter("Id", res.Id).One(&res)
				if err != nil {
					logs.Error("Failed to reload IST result after insert for invitation %d: %v", inv.Id, err)
				}
			}
		} else {
			logs.Info("Created new IST result with ORM for invitation %d (id=%d, rows=%d)", inv.Id, res.Id, num)
		}
		
		// Pastikan result sudah ada sebelum lanjut
		if res.Id == 0 {
			logs.Error("CRITICAL: IST result still not created for invitation %d after all attempts", inv.Id)
			// Tetap lanjutkan, mungkin bisa dibuat nanti saat test selesai
		}
	} else {
		logs.Debug("IST result already exists for invitation %d (id=%d)", inv.Id, res.Id)
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
	case "ZR":
		// DB kolom bernama raw_za (legacy), tapi flow subtest VI pakai ZR.
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
		_, _ = o.Update(&res, updateFields...)
	}

	// Catat progress di tabel ist_progress (untuk tracking & export)
	// Normalize code untuk konsistensi (ZA -> ZR)
	normalizedCode := normalizeISTCode(code)
	var progress models.ISTProgress
	err = o.QueryTable(new(models.ISTProgress)).
		Filter("Invitation__Id", inv.Id).
		Filter("SubtestCode", normalizedCode).
		One(&progress)
	if err != nil {
		// Belum ada, insert baru
		progress = models.ISTProgress{
			Invitation:  inv,
			SubtestCode: normalizedCode,
			Status:      "completed",
		}
		_, _ = o.Insert(&progress)
	} else {
		// Sudah ada, update status dan completed_at
		progress.Status = "completed"
		progress.CompletedAt = time.Now()
		_, _ = o.Update(&progress, "Status", "CompletedAt")
	}

	// Cari subtest berikutnya berdasarkan urutan IST 176 soal (bukan dari DB),
	// supaya tidak nyasar ke subtest lain (mis. ZA) dan setelah 176 benar-benar selesai.
	order := istAllowedOrder()
	nextCode := ""
	for i := range order {
		if order[i] == code {
			if i+1 < len(order) {
				nextCode = order[i+1]
			}
			break
		}
	}

	// Jika ada next subtest, redirect ke instruction page dulu
	if nextCode != "" {
		c.Data["json"] = map[string]interface{}{
			"success":          true,
			"raw_score":        rawScore,
			"next_subtest":     nextCode,
			"is_complete":      false,
			"next_instruction": "/test/ist/instruction/" + nextCode,
		}
		c.ServeJSON()
		return
	}

	// Selesai (ME): hitung standard score + IQ lalu tandai undangan used.
	// Age sangat penting untuk perhitungan IQ yang akurat karena norma berbeda untuk setiap usia
	// Age dihitung pada saat test dikerjakan (bukan age sekarang)
	age := 0
	if user.TanggalLahir != nil {
		// Gunakan waktu sekarang sebagai waktu test (karena test baru saja selesai)
		testTime := time.Now()
		age = utils.AgeYears(*user.TanggalLahir, testTime)
	}
	if age <= 0 {
		// Jika age tidak valid, tidak bisa menghitung IQ
		// Tapi tetap simpan raw scores
		logs.Warning("Age tidak valid untuk user %d (age: %d), tidak bisa menghitung IQ", user.Id, age)
	} else {
		// Hitung standard scores dan IQ berdasarkan age
		// Setiap subtest dan TotalStandardScore menggunakan age untuk mencari norma yang tepat
		updatedRes, err := utils.EnsureISTStandardAndIQScores(o, &res, age)
		if err != nil {
			logs.Error("Error calculating IST scores for user %d, age %d: %v", user.Id, age, err)
		} else {
			res = *updatedRes
		}
	}
	
	// Update result dengan standard scores dan IQ
	num, uerr := o.Update(&res,
		"StdSE", "StdWA", "StdAN", "StdGE", "StdRA", "StdZA", "StdFA", "StdWU", "StdME",
		"TotalStandardScore", "IQ", "IQCategory",
	)
	if uerr != nil {
		logs.Error("Error updating IST result for invitation %d: %v", inv.Id, uerr)
	} else if num == 0 {
		logs.Warning("No rows updated for IST result invitation %d", inv.Id)
	} else {
		logs.Info("Successfully updated IST result for invitation %d: IQ=%d, TotalSS=%d", inv.Id, res.IQ, res.TotalStandardScore)
	}

	if inv.Status != models.StatusInvitationUsed {
		inv.Status = models.StatusInvitationUsed
		inv.UsedAt = time.Now()
		_, _ = o.Update(inv, "Status", "UsedAt")
	}

	c.Data["json"] = map[string]interface{}{
		"success":         true,
		"raw_score":       rawScore,
		"next_subtest":    nextCode,
		"is_complete":     nextCode == "",
		// Setelah selesai, arahkan peserta ke halaman profile untuk cek IQ.
		"finish_redirect": "/profile",
	}
	c.ServeJSON()
}

// ReportViolationAPI mencatat pelanggaran anti-cheat selama ujian berlangsung.
// Menggunakan session supaya tidak butuh migrasi DB.
// @router /api/test/ist/violation [post]
func (c *ISTTestController) ReportViolationAPI() {
	_, _, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	type req struct {
		Type string `json:"type"` // blur|hidden|exit_fullscreen|keydown|copy|paste|...
		Meta string `json:"meta"` // optional: detail tambahan
	}
	var r req
	_ = json.Unmarshal(c.Ctx.Input.RequestBody, &r)

	const limit = 3

	curAny := c.GetSession("ist_violation_count")
	cur := 0
	if curAny != nil {
		if v, ok := curAny.(int); ok {
			cur = v
		}
	}
	cur++
	c.SetSession("ist_violation_count", cur)
	logs.Warning("IST anti-cheat violation: count=%d type=%s meta=%s ip=%s ua=%s", cur, r.Type, r.Meta, c.Ctx.Input.IP(), c.Ctx.Input.UserAgent())

	c.Data["json"] = map[string]interface{}{
		"success":      true,
		"count":        cur,
		"limit":        limit,
		"force_submit": cur >= limit,
	}
	c.ServeJSON()
}

// FinishPage: halaman setelah IST selesai.
// @router /test/ist/finish [get]
func (c *ISTTestController) FinishPage() {
	inv, _, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Redirect("/test", 302)
		return
	}
	// Jika belum selesai, jangan bisa "skip" ke finish page.
	o := orm.NewOrm()
	if current, complete, _ := getISTCurrentSubtestCode(o, inv.Id); !complete && current != "" {
		c.Redirect("/test/ist/instruction/"+current, 302)
		return
	}
	
	// Cek apakah result sudah ada, jika sudah ada langsung redirect ke result
	var res models.ISTResult
	err := o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err == nil && res.Id != 0 {
		// Result sudah ada, langsung redirect ke profile (peserta cek IQ di sana)
		c.Redirect("/profile", 302)
		return
	}
	
	// Jika belum ada result, tampilkan halaman finish
	c.TplName = "test_ist_finish.html"
}

// ResultPage menampilkan ringkasan hasil IST untuk peserta (IQ, kategori, grafik, psikogram sederhana).
// @router /test/ist/result [get]
func (c *ISTTestController) ResultPage() {
	// Bisa diakses via:
	// - Session invitation (alur tes)
	// - Query ?invId=123 dari halaman profil (pastikan milik user login)
	o := orm.NewOrm()

	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login", 302)
		return
	}
	userID := sessionUser.(int)

	// Prioritas: invId dari query (untuk halaman profil)
	invIDStr := strings.TrimSpace(c.GetString("invId"))
	var inv models.TestInvitation
	if invIDStr != "" {
		if id, err := strconv.Atoi(invIDStr); err == nil && id > 0 {
			inv.Id = id
			if err := o.Read(&inv); err != nil {
				c.Redirect("/profile", 302)
				return
			}
			// Pastikan undangan milik user login
			if inv.UserId == nil || *inv.UserId != userID {
				c.Redirect("/profile", 302)
				return
			}
		}
	}

	// Fallback: pakai session invitation
	if inv.Id == 0 {
		sessionInv := c.GetSession("current_invitation_id")
		if sessionInv == nil {
			c.Redirect("/profile", 302)
			return
		}
		inv.Id = sessionInv.(int)
		if err := o.Read(&inv); err != nil {
			c.Redirect("/profile", 302)
			return
		}
	}

	// Load user
	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/profile", 302)
		return
	}

	// Pastikan tes sudah selesai
	current, complete, _ := getISTCurrentSubtestCode(o, inv.Id)
	if !complete && current != "" {
		c.Redirect("/test/ist/instruction/"+current, 302)
		return
	}

	var res models.ISTResult
	err := o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		// Tidak ada result
		if complete {
			// Test sudah selesai tapi result belum dibuat, tampilkan pesan di finish page
			c.Data["Error"] = "Hasil tes sedang diproses. Silakan refresh halaman ini dalam beberapa saat."
			c.TplName = "test_ist_finish.html"
			return
		}
		// Test belum selesai, redirect ke instruction
		if current != "" {
			c.Redirect("/test/ist/instruction/"+current, 302)
		} else {
			c.Redirect("/test/ist/finish", 302)
		}
		return
	}

	// Pastikan standar & IQ terisi (kalau norma baru diisi belakangan)
	age := 0
	if user.TanggalLahir != nil {
		age = utils.AgeYears(*user.TanggalLahir, time.Now())
	}
	if age > 0 {
		updatedRes, err := utils.EnsureISTStandardAndIQScores(o, &res, age)
		if err == nil && updatedRes != nil {
			// Update hanya jika ada perubahan (biar tidak nulis DB terus)
			changed := res.TotalStandardScore != updatedRes.TotalStandardScore || res.IQ != updatedRes.IQ || res.IQCategory != updatedRes.IQCategory ||
				res.StdSE != updatedRes.StdSE || res.StdWA != updatedRes.StdWA || res.StdAN != updatedRes.StdAN || res.StdGE != updatedRes.StdGE ||
				res.StdRA != updatedRes.StdRA || res.StdZA != updatedRes.StdZA || res.StdFA != updatedRes.StdFA || res.StdWU != updatedRes.StdWU || res.StdME != updatedRes.StdME
			res = *updatedRes
			if changed {
				_, _ = o.Update(&res,
					"StdSE", "StdWA", "StdAN", "StdGE", "StdRA", "StdZA", "StdFA", "StdWU", "StdME",
					"TotalStandardScore", "IQ", "IQCategory",
				)
			}
		}
	}

	subtests := []string{"SE", "WA", "AN", "GE", "RA", "ZR", "FA", "WU", "ME"}
	stdScores := []int{
		res.StdSE, res.StdWA, res.StdAN, res.StdGE,
		res.StdRA, res.StdZA, res.StdFA, res.StdWU, res.StdME,
	}

	aspekList := buildISTAspectRows(&res)

	c.Data["User"] = user
	c.Data["Invitation"] = inv
	c.Data["Result"] = res
	c.Data["Age"] = age
	c.Data["Subtests"] = subtests
	c.Data["StdScores"] = stdScores
	c.Data["AspekList"] = aspekList
	c.TplName = "test_ist_result.html"
}

// ExportResultPDF mengunduh laporan IST dalam bentuk PDF dengan psikogram ringkas.
// @router /test/ist/result/pdf [get]
func (c *ISTTestController) ExportResultPDF() {
	o := orm.NewOrm()

	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login", 302)
		return
	}
	userID := sessionUser.(int)

	// Support query invId untuk akses dari profil
	invIDStr := strings.TrimSpace(c.GetString("invId"))
	var inv models.TestInvitation
	if invIDStr != "" {
		if id, err := strconv.Atoi(invIDStr); err == nil && id > 0 {
			inv.Id = id
			if err := o.Read(&inv); err != nil {
				c.Redirect("/profile", 302)
				return
			}
			if inv.UserId == nil || *inv.UserId != userID {
				c.Redirect("/profile", 302)
				return
			}
		}
	}
	if inv.Id == 0 {
		sessionInv := c.GetSession("current_invitation_id")
		if sessionInv == nil {
			c.Redirect("/profile", 302)
			return
		}
		inv.Id = sessionInv.(int)
		if err := o.Read(&inv); err != nil {
			c.Redirect("/profile", 302)
			return
		}
	}

	// Load user
	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/profile", 302)
		return
	}

	var res models.ISTResult
	err := o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		c.Redirect("/profile", 302)
		return
	}

	age := 0
	if user.TanggalLahir != nil {
		age = utils.AgeYears(*user.TanggalLahir, time.Now())
	}
	if age > 0 {
		if updatedRes, err := utils.EnsureISTStandardAndIQScores(o, &res, age); err == nil && updatedRes != nil {
			changed := res.TotalStandardScore != updatedRes.TotalStandardScore || res.IQ != updatedRes.IQ || res.IQCategory != updatedRes.IQCategory ||
				res.StdSE != updatedRes.StdSE || res.StdWA != updatedRes.StdWA || res.StdAN != updatedRes.StdAN || res.StdGE != updatedRes.StdGE ||
				res.StdRA != updatedRes.StdRA || res.StdZA != updatedRes.StdZA || res.StdFA != updatedRes.StdFA || res.StdWU != updatedRes.StdWU || res.StdME != updatedRes.StdME
			res = *updatedRes
			if changed {
				_, _ = o.Update(&res,
					"StdSE", "StdWA", "StdAN", "StdGE", "StdRA", "StdZA", "StdFA", "StdWU", "StdME",
					"TotalStandardScore", "IQ", "IQCategory",
				)
			}
		}
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Laporan Hasil IST", false)
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 8, "LAPORAN HASIL TES IST")
	pdf.Ln(10)

	// Identitas
	pdf.SetFont("Arial", "", 11)
	nama := user.NamaLengkap
	if nama == "" {
		nama = inv.Email
	}
	dob := ""
	if user.TanggalLahir != nil {
		dob = user.TanggalLahir.Format("02-01-2006")
	}
	pdf.CellFormat(40, 6, "Nama", "", 0, "L", false, 0, "")
	pdf.CellFormat(5, 6, ":", "", 0, "L", false, 0, "")
	pdf.Cell(0, 6, nama)
	pdf.Ln(6)
	pdf.CellFormat(40, 6, "Tanggal Lahir", "", 0, "L", false, 0, "")
	pdf.CellFormat(5, 6, ":", "", 0, "L", false, 0, "")
	pdf.Cell(0, 6, dob)
	pdf.Ln(6)
	pdf.CellFormat(40, 6, "Usia", "", 0, "L", false, 0, "")
	pdf.CellFormat(5, 6, ":", "", 0, "L", false, 0, "")
	if age > 0 {
		pdf.Cell(0, 6, fmt.Sprintf("%d tahun", age))
	}
	pdf.Ln(10)

	// A. Kecerdasan Umum
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 7, "A. KECERDASAN UMUM (Skala IST)")
	pdf.Ln(8)
	pdf.SetFont("Arial", "", 11)
	pdf.Cell(0, 6, fmt.Sprintf("IQ = %d   (%s)", res.IQ, res.IQCategory))
	pdf.Ln(10)

	// B. Kemampuan Khusus (psikogram)
	pdf.SetFont("Arial", "B", 12)
	pdf.Cell(0, 7, "B. KEMAMPUAN KHUSUS")
	pdf.Ln(8)

	// Psikogram table layout mirror dengan tampilan di profil (6 kolom kategori)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	pdf.SetLineWidth(0.4)
	// kolom: No | Aspek | Uraian | 6 kolom kategori
	colW := []float64{8, 45, 82, 10, 10, 10, 10, 10, 10}

	// Baris 1: ASPEK PSIKOLOGIS | KATEGORI
	pdf.CellFormat(colW[0], 7, "", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[1]+colW[2], 7, "ASPEK PSIKOLOGIS", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[3]+colW[4]+colW[5]+colW[6]+colW[7]+colW[8], 7, "KATEGORI", "1", 0, "C", true, 0, "")
	pdf.Ln(-1)

	// Baris 2: No | (kosong) | Kurang | Cukup | Baik (masing-masing 2 kolom)
	pdf.CellFormat(colW[0], 7, "No", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[1]+colW[2], 7, "", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[3]+colW[4], 7, "Kurang", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[5]+colW[6], 7, "Cukup", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[7]+colW[8], 7, "Baik", "1", 0, "C", true, 0, "")
	pdf.Ln(-1)

	// Baris 3: 6 label kategori detail (font sedikit lebih kecil supaya muat)
	pdf.SetFont("Arial", "B", 8)
	pdf.CellFormat(colW[0], 7, "", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[1]+colW[2], 7, "", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[3], 7, "Kurang Sekali", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[4], 7, "Kurang", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[5], 7, "Cukup", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[6], 7, "Cukup Baik", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[7], 7, "Baik", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[8], 7, "Baik Sekali", "1", 0, "C", true, 0, "")
	pdf.Ln(-1)

	// Baris 4: angka 1-6
	pdf.SetFont("Arial", "B", 9)
	pdf.CellFormat(colW[0], 7, "", "1", 0, "C", true, 0, "")
	pdf.CellFormat(colW[1]+colW[2], 7, "", "1", 0, "C", true, 0, "")
	for i := 1; i <= 6; i++ {
		pdf.CellFormat(colW[2+i], 7, strconv.Itoa(i), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Data aspek – sama urutan & nama seperti buildISTAspectRows / tampilan profil
	type aspekDef struct {
		No   int
		Nama string
		Skor int
	}

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

	aspeks := []aspekDef{
		{1, "Penalaran Konkret", avg(res.StdSE, res.StdGE)},
		{2, "Penalaran Verbal", avg(res.StdSE, res.StdWA, res.StdGE)},
		{3, "Daya Analisis", res.StdAN},
		{4, "Penalaran Abstrak", res.StdZA},
		{5, "Daya Ingat", res.StdME},
		{6, "Kemampuan Berhitung", res.StdRA},
		{7, "Analogi Angka", res.StdZA},
		{8, "Daya Bayang Konstruksional", res.StdFA},
		{9, "Daya Bayang Ruang", res.StdWU},
	}

	// Tentukan kategori psikogram dari skor SW aspek.
	getCatIdx := func(sw int) int { return psychogramCatIdxFromSW(sw) }

	pdf.SetFont("Arial", "", 9)
	pdf.SetFillColor(255, 255, 255)
	for _, a := range aspeks {
		pdf.CellFormat(colW[0], 7, strconv.Itoa(a.No), "1", 0, "C", false, 0, "")
		pdf.CellFormat(colW[1], 7, a.Nama, "1", 0, "L", false, 0, "")
		// uraian dikosongkan di PDF singkat (sama seperti contoh yang kamu kirim)
		pdf.CellFormat(colW[2], 7, "", "1", 0, "L", false, 0, "")

		catIdx := getCatIdx(a.Skor)
		for i := 0; i < 6; i++ {
			val := ""
			if i == catIdx {
				val = "v"
			}
			pdf.CellFormat(colW[3+i], 7, val, "1", 0, "C", false, 0, "")
		}
		pdf.Ln(-1)
	}

	filename := fmt.Sprintf("IST_Result_%d.pdf", inv.Id)
	c.Ctx.Output.Header("Content-Type", "application/pdf")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_ = pdf.Output(c.Ctx.ResponseWriter)
}

// ExportResultExcel mengunduh laporan IST dalam bentuk Excel (XLSX) dengan layout psikogram seperti standar.
// @router /test/ist/result/excel [get]
func (c *ISTTestController) ExportResultExcel() {
	o := orm.NewOrm()

	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login", 302)
		return
	}
	userID := sessionUser.(int)

	// Support query invId untuk akses dari profil
	invIDStr := strings.TrimSpace(c.GetString("invId"))
	var inv models.TestInvitation
	if invIDStr != "" {
		if id, err := strconv.Atoi(invIDStr); err == nil && id > 0 {
			inv.Id = id
			if err := o.Read(&inv); err != nil {
				c.Redirect("/profile", 302)
				return
			}
			if inv.UserId == nil || *inv.UserId != userID {
				c.Redirect("/profile", 302)
				return
			}
		}
	}
	if inv.Id == 0 {
		sessionInv := c.GetSession("current_invitation_id")
		if sessionInv == nil {
			c.Redirect("/profile", 302)
			return
		}
		inv.Id = sessionInv.(int)
		if err := o.Read(&inv); err != nil {
			c.Redirect("/profile", 302)
			return
		}
	}

	// Load user
	var user models.User
	user.Id = userID
	if err := o.Read(&user); err != nil {
		c.Redirect("/profile", 302)
		return
	}

	// Load result
	var res models.ISTResult
	err := o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
	if err != nil || res.Id == 0 {
		c.Redirect("/profile", 302)
		return
	}

	age := 0
	if user.TanggalLahir != nil {
		age = utils.AgeYears(*user.TanggalLahir, time.Now())
	}
	if age > 0 {
		if updatedRes, err := utils.EnsureISTStandardAndIQScores(o, &res, age); err == nil && updatedRes != nil {
			changed := res.TotalStandardScore != updatedRes.TotalStandardScore || res.IQ != updatedRes.IQ || res.IQCategory != updatedRes.IQCategory ||
				res.StdSE != updatedRes.StdSE || res.StdWA != updatedRes.StdWA || res.StdAN != updatedRes.StdAN || res.StdGE != updatedRes.StdGE ||
				res.StdRA != updatedRes.StdRA || res.StdZA != updatedRes.StdZA || res.StdFA != updatedRes.StdFA || res.StdWU != updatedRes.StdWU || res.StdME != updatedRes.StdME
			res = *updatedRes
			if changed {
				_, _ = o.Update(&res,
					"StdSE", "StdWA", "StdAN", "StdGE", "StdRA", "StdZA", "StdFA", "StdWU", "StdME",
					"TotalStandardScore", "IQ", "IQCategory",
				)
			}
		}
	}

	f := excelize.NewFile()
	sheet := "IST"
	f.SetSheetName("Sheet1", sheet)

	// Styles
	borderAll := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
	styleTitle, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true, Size: 14}})
	styleBold, _ := f.NewStyle(&excelize.Style{Font: &excelize.Font{Bold: true}})
	styleHeaderGray, _ := f.NewStyle(&excelize.Style{
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
	styleCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    borderAll,
	})

	// Column widths
	_ = f.SetColWidth(sheet, "A", "A", 4)
	_ = f.SetColWidth(sheet, "B", "B", 28)
	_ = f.SetColWidth(sheet, "C", "C", 70)
	_ = f.SetColWidth(sheet, "D", "I", 12)

	// Title
	_ = f.MergeCell(sheet, "A1", "I1")
	_ = f.SetCellValue(sheet, "A1", "LAPORAN HASIL TES IST")
	_ = f.SetCellStyle(sheet, "A1", "A1", styleTitle)

	// Identity
	nama := user.NamaLengkap
	if nama == "" {
		nama = inv.Email
	}
	dob := ""
	if user.TanggalLahir != nil {
		dob = user.TanggalLahir.Format("02-01-2006")
	}
	_ = f.SetCellValue(sheet, "A3", "Nama")
	_ = f.SetCellValue(sheet, "C3", nama)
	_ = f.SetCellValue(sheet, "A4", "Tanggal Lahir")
	_ = f.SetCellValue(sheet, "C4", dob)
	_ = f.SetCellValue(sheet, "A5", "Usia")
	if age > 0 {
		_ = f.SetCellValue(sheet, "C5", fmt.Sprintf("%d tahun", age))
	}
	_ = f.SetCellStyle(sheet, "A3", "A5", styleBold)

	// A. Kecerdasan umum
	_ = f.SetCellValue(sheet, "A7", "A. KECERDASAN UMUM (Skala IST)")
	_ = f.SetCellStyle(sheet, "A7", "A7", styleBold)
	_ = f.SetCellValue(sheet, "A8", fmt.Sprintf("IQ = %d   (%s)", res.IQ, res.IQCategory))

	// B. Kemampuan khusus
	_ = f.SetCellValue(sheet, "A10", "B. KEMAMPUAN KHUSUS")
	_ = f.SetCellStyle(sheet, "A10", "A10", styleBold)

	// Header tabel psikogram (mirip screenshot)
	_ = f.MergeCell(sheet, "A12", "A15")
	_ = f.MergeCell(sheet, "B12", "B15")
	_ = f.MergeCell(sheet, "C12", "C15")
	_ = f.MergeCell(sheet, "D12", "I12")
	_ = f.SetCellValue(sheet, "A12", "No")
	_ = f.SetCellValue(sheet, "B12", "ASPEK PSIKOLOGIS")
	_ = f.SetCellValue(sheet, "C12", "")
	_ = f.SetCellValue(sheet, "D12", "KATEGORI")
	_ = f.SetCellStyle(sheet, "A12", "I12", styleHeaderGray)
	_ = f.SetCellStyle(sheet, "A12", "C15", styleHeaderGray)

	_ = f.MergeCell(sheet, "D13", "E13")
	_ = f.MergeCell(sheet, "F13", "G13")
	_ = f.MergeCell(sheet, "H13", "I13")
	_ = f.SetCellValue(sheet, "D13", "Kurang")
	_ = f.SetCellValue(sheet, "F13", "Cukup")
	_ = f.SetCellValue(sheet, "H13", "Baik")
	_ = f.SetCellStyle(sheet, "D13", "I13", styleHeaderBlue)

	_ = f.SetCellValue(sheet, "D14", "Kurang Sekali")
	_ = f.SetCellValue(sheet, "E14", "Kurang")
	_ = f.SetCellValue(sheet, "F14", "Cukup")
	_ = f.SetCellValue(sheet, "G14", "Cukup Baik")
	_ = f.SetCellValue(sheet, "H14", "Baik")
	_ = f.SetCellValue(sheet, "I14", "Baik Sekali")
	_ = f.SetCellStyle(sheet, "D14", "I14", styleHeaderBlue)

	_ = f.SetCellValue(sheet, "D15", 1)
	_ = f.SetCellValue(sheet, "E15", 2)
	_ = f.SetCellValue(sheet, "F15", 3)
	_ = f.SetCellValue(sheet, "G15", 4)
	_ = f.SetCellValue(sheet, "H15", 5)
	_ = f.SetCellValue(sheet, "I15", 6)
	_ = f.SetCellStyle(sheet, "D15", "I15", styleHeaderBlue)

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
	type rowDef struct {
		no   int
		nama string
		desc string
		skor int
	}
	rows := []rowDef{
		{1, "Penalaran Konkret", "Kemampuan berpikir praktis, sesuai kenyataan dan mengambil keputusan secara mandiri berdasarkan data maupun situasi serta kondisi yang ada.", avg(res.StdSE, res.StdGE)},
		{2, "Penalaran Verbal", "Kemampuan berpikir logis dalam penggunaan bahasa terkait informasi, instruksi maupun literasi.", avg(res.StdSE, res.StdWA, res.StdGE)},
		{3, "Daya Analisis", "Kemampuan melakukan pengkajian suatu peristiwa, objek, informasi maupun hubungan serta sebab akibat dalam penyelesaian persoalan atau masalah.", res.StdAN},
		{4, "Penalaran Abstrak", "Kemampuan memahami dan membayangkan suatu objek yang tidak nyata/abstrak.", res.StdZA},
		{5, "Daya Ingat", "Kemampuan mengingat/menghafal suatu informasi maupun objek.", res.StdME},
		{6, "Kemampuan Berhitung", "Kemampuan memahami konsep dasar berhitung dan mengolah angka secara praktis.", res.StdRA},
		{7, "Analogi Angka", "Kemampuan memahami analogi angka dengan analisis mendalam sesuai pola khusus secara teoritis.", res.StdZA},
		{8, "Daya Bayang Konstruksional", "Kemampuan memahami, mengingat dan membayangkan maupun menciptakan kreasi konstruksi secara teknis dengan berpikir secara menyeluruh mengenai suatu objek, bangunan maupun lokasi.", res.StdFA},
		{9, "Daya Bayang Ruang", "Kemampuan memahami dan membayangkan suatu ruang tiga dimensi, berpikir fleksibel dan kreatif.", res.StdWU},
	}
	// Tentukan kategori psikogram dari skor SW aspek.
	getCatIdx := func(sw int) int { return psychogramCatIdxFromSW(sw) }

	startRow := 16
	for i, r := range rows {
		top := startRow + i*2
		bot := top + 1
		_ = f.MergeCell(sheet, fmt.Sprintf("A%d", top), fmt.Sprintf("A%d", bot))
		_ = f.MergeCell(sheet, fmt.Sprintf("B%d", top), fmt.Sprintf("B%d", bot))
		_ = f.MergeCell(sheet, fmt.Sprintf("C%d", top), fmt.Sprintf("C%d", bot))
		for col := 'D'; col <= 'I'; col++ {
			_ = f.MergeCell(sheet, fmt.Sprintf("%c%d", col, top), fmt.Sprintf("%c%d", col, bot))
		}
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", top), r.no)
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", top), r.nama)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", top), r.desc)
		cat := getCatIdx(r.skor)
		for j := 0; j < 6; j++ {
			cell := fmt.Sprintf("%c%d", 'D'+j, top)
			if j == cat {
				_ = f.SetCellValue(sheet, cell, "√")
			}
		}
		_ = f.SetRowHeight(sheet, top, 34)
		_ = f.SetRowHeight(sheet, bot, 34)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", top), fmt.Sprintf("A%d", bot), styleCenter)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("B%d", top), fmt.Sprintf("B%d", bot), styleBody)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("C%d", top), fmt.Sprintf("C%d", bot), styleBody)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("D%d", top), fmt.Sprintf("I%d", bot), styleCenter)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		c.Redirect("/profile", 302)
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
	filename := fmt.Sprintf("ist_result_%s.xlsx", makeSafeName(downloadName))
	c.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+filename+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(buf.Bytes())
}

// AutoCompleteAllSubtestsAPI: DEV HELPER - Auto-complete semua subtest IST dengan jawaban random.
// HAPUS SEBELUM PRODUCTION! Hanya untuk mempermudah testing selama development.
// @router /api/dev/ist/auto-complete [post]
func (c *ISTTestController) AutoCompleteAllSubtestsAPI() {
	inv, user, ok := c.mustGetSessionInvitation()
	if !ok {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Sesi tidak valid"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	order := istAllowedOrder()
	options := []string{"A", "B", "C", "D", "E"}

	// Loop semua subtest dalam urutan
	for _, code := range order {
		sub, err := findISTSubtestByCode(o, code)
		if err != nil {
			continue
		}

		// Ambil soal dengan filter range resmi + dummy (sama seperti SubmitSubtestAPI)
		start, end := istQuestionRangeByCode(code)
		var questions []models.ISTQuestion
		q := o.QueryTable(new(models.ISTQuestion)).Filter("Subtest__Id", sub.Id)
		if start > 0 && end > 0 {
			q = q.Filter("Number__gte", start).Filter("Number__lte", end)
		}
		_, _ = q.OrderBy("Number").All(&questions)
		questions = filterISTDummyQuestions(questions)
		if len(questions) == 0 {
			continue
		}

		// Hapus jawaban lama untuk subtest ini
		_, _ = o.QueryTable(new(models.ISTAnswer)).Filter("Invitation__Id", inv.Id).Filter("Subtest__Id", sub.Id).Delete()

		// Generate jawaban random untuk setiap soal
		rawScore := 0
		gePoints := 0
		rand.Seed(time.Now().UnixNano())
		for i := range questions {
			q := &questions[i]
			// Random pilih A-E
			randomAns := options[rand.Intn(len(options))]
			correct := false
			score := 0
			if code == "GE" {
				optText := istOptionTextByAnswer(q, randomAns)
				score = scoreGEQuestion(q.Number, optText)
				correct = score > 0
				gePoints += score
			} else {
				correct = strings.EqualFold(randomAns, strings.TrimSpace(q.Correct))
				if correct {
					score = 1
					rawScore++
				}
			}

			// Simpan jawaban
			istAns := models.ISTAnswer{
				Invitation: inv,
				User:       user,
				Subtest:    sub,
				Question:   q,
				Answer:     randomAns,
				Score:      score,
				IsCorrect:  correct,
			}
			_, _ = o.Insert(&istAns)
		}
		if code == "GE" {
			raw := int(math.Round(float64(gePoints) / 1.6))
			if raw < 0 {
				raw = 0
			}
			if raw > 20 {
				raw = 20
			}
			rawScore = raw
		}

		// Update ISTResult dengan raw score
		var res models.ISTResult
		err = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
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
		case "ZR":
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
			_, _ = o.Update(&res, updateFields...)
		}

		// Catat progress di tabel ist_progress
		normalizedCode := normalizeISTCode(code)
		var progress models.ISTProgress
		err = o.QueryTable(new(models.ISTProgress)).
			Filter("Invitation__Id", inv.Id).
			Filter("SubtestCode", normalizedCode).
			One(&progress)
		if err != nil {
			progress = models.ISTProgress{
				Invitation:  inv,
				SubtestCode: normalizedCode,
				Status:      "completed",
			}
			_, _ = o.Insert(&progress)
		} else {
			progress.Status = "completed"
			progress.CompletedAt = time.Now()
			_, _ = o.Update(&progress, "Status", "CompletedAt")
		}
	}

	// Setelah semua selesai, hitung standard score + IQ
	var res models.ISTResult
	_ = o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res)
	age := 0
	if user.TanggalLahir != nil {
		age = utils.AgeYears(*user.TanggalLahir, time.Now())
	}
	_, _ = utils.EnsureISTStandardAndIQScores(o, &res, age)
	_, _ = o.Update(&res,
		"StdSE", "StdWA", "StdAN", "StdGE", "StdRA", "StdZA", "StdFA", "StdWU", "StdME",
		"TotalStandardScore", "IQ", "IQCategory",
	)

	if inv.Status != models.StatusInvitationUsed {
		inv.Status = models.StatusInvitationUsed
		inv.UsedAt = time.Now()
		_, _ = o.Update(inv, "Status", "UsedAt")
	}

	c.Data["json"] = map[string]interface{}{
		"success":         true,
		"message":         "Semua subtest IST telah diisi dengan jawaban random",
		"finish_redirect": "/test/ist/finish",
	}
	c.ServeJSON()
}
