package controllers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/xuri/excelize/v2"
)

// RankingController menangani export "Top 10 / Bottom 10" untuk tiap alat tes
// dalam satu batch. Tombol ini muncul di samping tombol "Download .zip" di
// halaman admin manage batch.
type RankingController struct {
	beego.Controller
}

func (c *RankingController) verifyAdmin() bool {
	userRole := c.GetSession("user_role")
	roleStr, _ := userRole.(string)
	if roleStr != string(models.RoleAdmin) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Akses ditolak"}
		c.ServeJSON()
		return false
	}
	return true
}

// participantInfo merangkum identitas peserta yang dipakai di setiap baris ranking.
type participantInfo struct {
	Name    string
	NISNNIP string
	Kelas   string
	Jurusan string
}

func makeParticipantInfo(u *models.User) participantInfo {
	if u == nil {
		return participantInfo{Name: "-"}
	}
	id := strings.TrimSpace(u.NISN)
	if id == "" {
		id = strings.TrimSpace(u.NIP)
	}
	name := u.NamaLengkap
	if strings.TrimSpace(name) == "" {
		name = "-"
	}
	return participantInfo{
		Name:    name,
		NISNNIP: id,
		Kelas:   u.Kelas,
		Jurusan: u.Jurusan,
	}
}

// dimensionRanking menyimpan satu kolom ranking (1 dimensi/aspek tes).
// Entries DIASUMSIKAN sudah terurut: index 0 = paling unggul, terakhir = paling terendah.
type dimensionRanking struct {
	Code           string
	Label          string
	Entries        []rankEntry
	HigherIsBetter bool // RMIB false (skor kecil = minat tinggi)
}

type rankEntry struct {
	Info  participantInfo
	Score float64
}

func formatScore(v float64) interface{} {
	if v == float64(int64(v)) {
		return int64(v)
	}
	return v
}

func sortDescByScore(entries []rankEntry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Score > entries[j].Score })
}
func sortAscByScore(entries []rankEntry) {
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Score < entries[j].Score })
}

// renderRankingXLSX menulis 1 sheet berisi banyak dimensi side-by-side.
// Tiap dimensi lebar 6 kolom + 1 kolom gap.
//   row layout per dim:
//     row 4   : dim header
//     row 5   : "TOP 10" banner
//     row 6   : table header
//     row 7-16: top 10 data
//     row 18  : "BOTTOM 10" banner
//     row 19  : table header
//     row 20-29: bottom 10 data
func renderRankingXLSX(batch *models.TestBatch, testTitle string, dims []dimensionRanking) ([]byte, error) {
	f := excelize.NewFile()
	sheet := "Ranking"
	f.SetSheetName(f.GetSheetName(0), sheet)
	showGrid := false
	_ = f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGrid})

	border := []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
	styleTitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 16, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styleSubtitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Italic: true, Size: 11},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styleDimHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2E75B6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border,
	})
	styleSectionTop, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2E7D32"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	styleSectionBottom, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#C62828"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	styleTableHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#D9E1F2"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border,
	})
	styleCell, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		Border:    border,
	})
	styleCellCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})

	totalCols := 6
	if len(dims) > 0 {
		totalCols = len(dims)*7 - 1
	}
	lastColName, _ := excelize.ColumnNumberToName(totalCols)

	// Header batch
	_ = f.MergeCell(sheet, "A1", lastColName+"1")
	_ = f.SetCellValue(sheet, "A1", testTitle+" — Ranking Top 10 / Bottom 10")
	_ = f.SetCellStyle(sheet, "A1", lastColName+"1", styleTitle)
	_ = f.SetRowHeight(sheet, 1, 28)

	subtitle := fmt.Sprintf("Batch: %s  |  Tanggal Export: %s", batch.Name, time.Now().Format("02 January 2006 15:04"))
	_ = f.MergeCell(sheet, "A2", lastColName+"2")
	_ = f.SetCellValue(sheet, "A2", subtitle)
	_ = f.SetCellStyle(sheet, "A2", lastColName+"2", styleSubtitle)

	const startRow = 4

	for di, dim := range dims {
		startCol := 1 + di*7
		c1, _ := excelize.ColumnNumberToName(startCol)
		c2, _ := excelize.ColumnNumberToName(startCol + 1)
		c3, _ := excelize.ColumnNumberToName(startCol + 2)
		c4, _ := excelize.ColumnNumberToName(startCol + 3)
		c5, _ := excelize.ColumnNumberToName(startCol + 4)
		c6, _ := excelize.ColumnNumberToName(startCol + 5)

		_ = f.SetColWidth(sheet, c1, c1, 6)
		_ = f.SetColWidth(sheet, c2, c2, 28)
		_ = f.SetColWidth(sheet, c3, c3, 16)
		_ = f.SetColWidth(sheet, c4, c4, 14)
		_ = f.SetColWidth(sheet, c5, c5, 16)
		_ = f.SetColWidth(sheet, c6, c6, 10)

		// Dim header
		row := startRow
		_ = f.MergeCell(sheet, fmt.Sprintf("%s%d", c1, row), fmt.Sprintf("%s%d", c6, row))
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c1, row),
			fmt.Sprintf("%s — %s", dim.Code, dim.Label))
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", c1, row), fmt.Sprintf("%s%d", c6, row), styleDimHeader)
		_ = f.SetRowHeight(sheet, row, 22)

		// TOP banner
		row = startRow + 1
		topLabel := "TOP 10 TERTINGGI"
		if !dim.HigherIsBetter {
			topLabel = "TOP 10 (Skor Terkecil)"
		}
		_ = f.MergeCell(sheet, fmt.Sprintf("%s%d", c1, row), fmt.Sprintf("%s%d", c6, row))
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c1, row), topLabel)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", c1, row), fmt.Sprintf("%s%d", c6, row), styleSectionTop)

		writeHeader := func(r int) {
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c1, r), "Rank")
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c2, r), "Nama")
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c3, r), "NISN / NIP")
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c4, r), "Kelas")
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c5, r), "Jurusan")
			_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c6, r), "Skor")
			_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", c1, r), fmt.Sprintf("%s%d", c6, r), styleTableHeader)
		}

		writeRow := func(r int, rankNum int, e *rankEntry) {
			rankCell := fmt.Sprintf("%s%d", c1, r)
			scoreCell := fmt.Sprintf("%s%d", c6, r)
			_ = f.SetCellValue(sheet, rankCell, rankNum)
			if e != nil {
				_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c2, r), e.Info.Name)
				_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c3, r), e.Info.NISNNIP)
				_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c4, r), e.Info.Kelas)
				_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c5, r), e.Info.Jurusan)
				_ = f.SetCellValue(sheet, scoreCell, formatScore(e.Score))
			}
			_ = f.SetCellStyle(sheet, rankCell, scoreCell, styleCell)
			_ = f.SetCellStyle(sheet, rankCell, rankCell, styleCellCenter)
			_ = f.SetCellStyle(sheet, scoreCell, scoreCell, styleCellCenter)
		}

		// Top header + data
		row = startRow + 2
		writeHeader(row)
		topEntries := dim.Entries
		if len(topEntries) > 10 {
			topEntries = topEntries[:10]
		}
		for i := 0; i < 10; i++ {
			r := startRow + 3 + i
			var e *rankEntry
			if i < len(topEntries) {
				e = &topEntries[i]
			}
			writeRow(r, i+1, e)
		}

		// BOTTOM banner
		row = startRow + 14
		bottomLabel := "BOTTOM 10 TERENDAH"
		if !dim.HigherIsBetter {
			bottomLabel = "BOTTOM 10 (Skor Terbesar)"
		}
		_ = f.MergeCell(sheet, fmt.Sprintf("%s%d", c1, row), fmt.Sprintf("%s%d", c6, row))
		_ = f.SetCellValue(sheet, fmt.Sprintf("%s%d", c1, row), bottomLabel)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("%s%d", c1, row), fmt.Sprintf("%s%d", c6, row), styleSectionBottom)

		row = startRow + 15
		writeHeader(row)
		bottom := dim.Entries
		if len(bottom) > 10 {
			bottom = bottom[len(bottom)-10:]
		}
		// Bottom: tampilkan dari yang TERPALING bawah ke atas (rank 1 = yang paling terendah/terburuk)
		bottomReversed := make([]rankEntry, len(bottom))
		for i, e := range bottom {
			bottomReversed[len(bottom)-1-i] = e
		}
		for i := 0; i < 10; i++ {
			r := startRow + 16 + i
			var e *rankEntry
			if i < len(bottomReversed) {
				e = &bottomReversed[i]
			}
			writeRow(r, i+1, e)
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// =========================================================================
// Builder per-tes
// =========================================================================

func loadInvitationsForBatch(o orm.Ormer, batchID int) ([]models.TestInvitation, error) {
	var invs []models.TestInvitation
	_, err := o.QueryTable(new(models.TestInvitation)).Filter("BatchId", batchID).All(&invs)
	return invs, err
}

func buildISTRanking(o orm.Ormer, invs []models.TestInvitation) []dimensionRanking {
	entries := []rankEntry{}
	for _, inv := range invs {
		var res models.ISTResult
		if err := o.QueryTable(new(models.ISTResult)).Filter("Invitation__Id", inv.Id).One(&res); err != nil {
			continue
		}
		user, _ := getUserForInvitation(o, &inv)
		entries = append(entries, rankEntry{Info: makeParticipantInfo(user), Score: float64(res.IQ)})
	}
	sortDescByScore(entries)
	return []dimensionRanking{{Code: "IQ", Label: "Inteligensi (IQ)", Entries: entries, HigherIsBetter: true}}
}

var hollandRankingDims = []struct{ Code, Label string }{
	{"R", "Realistic"}, {"I", "Investigative"}, {"A", "Artistic"},
	{"S", "Social"}, {"E", "Enterprising"}, {"C", "Conventional"},
}

func buildHollandRanking(o orm.Ormer, invs []models.TestInvitation) []dimensionRanking {
	dimEntries := map[string][]rankEntry{}
	for _, d := range hollandRankingDims {
		dimEntries[d.Code] = []rankEntry{}
	}
	for _, inv := range invs {
		var res models.HollandResult
		if err := o.QueryTable(new(models.HollandResult)).Filter("Invitation__Id", inv.Id).One(&res); err != nil {
			continue
		}
		user, _ := getUserForInvitation(o, &inv)
		info := makeParticipantInfo(user)
		dimEntries["R"] = append(dimEntries["R"], rankEntry{Info: info, Score: float64(res.ScoreR)})
		dimEntries["I"] = append(dimEntries["I"], rankEntry{Info: info, Score: float64(res.ScoreI)})
		dimEntries["A"] = append(dimEntries["A"], rankEntry{Info: info, Score: float64(res.ScoreA)})
		dimEntries["S"] = append(dimEntries["S"], rankEntry{Info: info, Score: float64(res.ScoreS)})
		dimEntries["E"] = append(dimEntries["E"], rankEntry{Info: info, Score: float64(res.ScoreE)})
		dimEntries["C"] = append(dimEntries["C"], rankEntry{Info: info, Score: float64(res.ScoreC)})
	}
	out := []dimensionRanking{}
	for _, d := range hollandRankingDims {
		es := dimEntries[d.Code]
		sortDescByScore(es)
		out = append(out, dimensionRanking{Code: d.Code, Label: d.Label, Entries: es, HigherIsBetter: true})
	}
	return out
}

var rmibRankingDims = []struct{ Code, Label string }{
	{"OUT", "Outdoor"}, {"MEC", "Mechanical"}, {"COMP", "Computational"},
	{"SCI", "Scientific"}, {"PERS", "Personal Contact"}, {"AEST", "Aesthetic"},
	{"MUS", "Musical"}, {"LIT", "Literary"}, {"SOC", "Social Service"},
	{"CLER", "Clerical"}, {"PRAC", "Practical"}, {"MED", "Medical"},
}

func buildRMIBRanking(o orm.Ormer, invs []models.TestInvitation) []dimensionRanking {
	type entry struct {
		Score int `json:"score"`
	}
	dimEntries := map[string][]rankEntry{}
	for _, d := range rmibRankingDims {
		dimEntries[d.Code] = []rankEntry{}
	}
	for _, inv := range invs {
		var res models.RMIBResult
		if err := o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).One(&res); err != nil {
			continue
		}
		user, _ := getUserForInvitation(o, &inv)
		info := makeParticipantInfo(user)
		parsed := map[string]entry{}
		_ = json.Unmarshal([]byte(res.ResultJSON), &parsed)
		for _, d := range rmibRankingDims {
			s := parsed[d.Code].Score
			dimEntries[d.Code] = append(dimEntries[d.Code], rankEntry{Info: info, Score: float64(s)})
		}
	}
	out := []dimensionRanking{}
	for _, d := range rmibRankingDims {
		es := dimEntries[d.Code]
		sortAscByScore(es)
		out = append(out, dimensionRanking{Code: d.Code, Label: d.Label, Entries: es, HigherIsBetter: false})
	}
	return out
}

var papiRankingDims = []struct{ Code, Label string }{
	{"G", "Pekerja keras"}, {"L", "Kepemimpinan"}, {"I", "Pengambilan keputusan"},
	{"T", "Aktivitas terus-menerus"}, {"V", "Vigorous"}, {"S", "Sosialisasi"},
	{"R", "Ketelitian teoretis"}, {"D", "Perhatian pada detail"}, {"C", "Pengorganisasian"},
	{"E", "Pengendalian emosi"}, {"N", "Berprestasi"}, {"A", "Otonomi"},
	{"P", "Mendominasi"}, {"X", "Diperhatikan"}, {"B", "Dukungan atasan"},
	{"O", "Kedekatan"}, {"Z", "Keragaman"}, {"K", "Agresif"},
	{"F", "Patuh"}, {"W", "Pendekatan tertib"},
}

func buildPAPIRanking(o orm.Ormer, invs []models.TestInvitation) []dimensionRanking {
	type entry struct {
		Score int `json:"score"`
	}
	dimEntries := map[string][]rankEntry{}
	for _, d := range papiRankingDims {
		dimEntries[d.Code] = []rankEntry{}
	}
	for _, inv := range invs {
		var res models.PAPIResult
		if err := o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&res); err != nil {
			continue
		}
		user, _ := getUserForInvitation(o, &inv)
		info := makeParticipantInfo(user)
		parsed := map[string]entry{}
		_ = json.Unmarshal([]byte(res.ResultJSON), &parsed)
		for _, d := range papiRankingDims {
			s := parsed[d.Code].Score
			dimEntries[d.Code] = append(dimEntries[d.Code], rankEntry{Info: info, Score: float64(s)})
		}
	}
	out := []dimensionRanking{}
	for _, d := range papiRankingDims {
		es := dimEntries[d.Code]
		sortDescByScore(es)
		out = append(out, dimensionRanking{Code: d.Code, Label: d.Label, Entries: es, HigherIsBetter: true})
	}
	return out
}

func buildKraepelinRanking(o orm.Ormer, invs []models.TestInvitation) []dimensionRanking {
	entries := []rankEntry{}
	for _, inv := range invs {
		var att models.KraepelinAttempt
		if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).Filter("Status", "finished").One(&att); err != nil {
			continue
		}
		user, _ := getUserForInvitation(o, &inv)
		net := att.TotalCorrect - att.TotalErrors
		entries = append(entries, rankEntry{Info: makeParticipantInfo(user), Score: float64(net)})
	}
	sortDescByScore(entries)
	return []dimensionRanking{{Code: "NET", Label: "Skor Bersih (Benar − Salah)", Entries: entries, HigherIsBetter: true}}
}

var lsRankingDims = []struct{ Code, Label string }{
	{"V", "Visual"}, {"A", "Auditory"}, {"K", "Kinesthetic"},
}

func buildLearningStyleRanking(o orm.Ormer, invs []models.TestInvitation) []dimensionRanking {
	dimEntries := map[string][]rankEntry{
		"V": {}, "A": {}, "K": {},
	}
	for _, inv := range invs {
		var res models.LearningStyleResult
		if err := o.QueryTable(new(models.LearningStyleResult)).Filter("Invitation__Id", inv.Id).One(&res); err != nil {
			continue
		}
		user, _ := getUserForInvitation(o, &inv)
		info := makeParticipantInfo(user)
		dimEntries["V"] = append(dimEntries["V"], rankEntry{Info: info, Score: float64(res.ScoreVisual)})
		dimEntries["A"] = append(dimEntries["A"], rankEntry{Info: info, Score: float64(res.ScoreAuditory)})
		dimEntries["K"] = append(dimEntries["K"], rankEntry{Info: info, Score: float64(res.ScoreKinesthetic)})
	}
	out := []dimensionRanking{}
	for _, d := range lsRankingDims {
		es := dimEntries[d.Code]
		sortDescByScore(es)
		out = append(out, dimensionRanking{Code: d.Code, Label: d.Label, Entries: es, HigherIsBetter: true})
	}
	return out
}

// =========================================================================
// Endpoint
// =========================================================================

// @router /api/admin/test-batches/:id/ranking/:test [get]
// `:test` salah satu: ist, holland, rmib, papi, kraepelin, learning-style
func (c *RankingController) ExportRanking() {
	if !c.verifyAdmin() {
		return
	}

	batchID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "ID batch tidak valid"}
		c.ServeJSON()
		return
	}
	testKey := strings.ToLower(strings.TrimSpace(c.Ctx.Input.Param(":test")))

	o := orm.NewOrm()
	var batch models.TestBatch
	batch.Id = batchID
	if err := o.Read(&batch); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Batch tidak ditemukan"}
		c.ServeJSON()
		return
	}

	invs, err := loadInvitationsForBatch(o, batchID)
	if err != nil {
		logs.Error("Ranking: gagal load invitations: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal memuat undangan"}
		c.ServeJSON()
		return
	}

	var dims []dimensionRanking
	var title string
	switch testKey {
	case "ist":
		if !batch.EnableIST {
			c.respondNotEnabled("IST")
			return
		}
		title = "Tes IST"
		dims = buildISTRanking(o, invs)
	case "holland":
		if !batch.EnableHolland {
			c.respondNotEnabled("Holland")
			return
		}
		title = "Tes Holland (RIASEC)"
		dims = buildHollandRanking(o, invs)
	case "rmib":
		if !batch.EnableRMIB {
			c.respondNotEnabled("RMIB")
			return
		}
		title = "Tes RMIB"
		dims = buildRMIBRanking(o, invs)
	case "papi":
		if !batch.EnablePAPI {
			c.respondNotEnabled("PAPI")
			return
		}
		title = "Tes PAPI"
		dims = buildPAPIRanking(o, invs)
	case "kraepelin":
		if !batch.EnableKraepelin {
			c.respondNotEnabled("Kraepelin")
			return
		}
		title = "Tes Kraepelin"
		dims = buildKraepelinRanking(o, invs)
	case "learning-style", "ls", "vak":
		if !batch.EnableLearningStyle {
			c.respondNotEnabled("Gaya Belajar (VAK)")
			return
		}
		title = "Tes Gaya Belajar (VAK)"
		dims = buildLearningStyleRanking(o, invs)
	default:
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Jenis tes tidak dikenal: " + testKey}
		c.ServeJSON()
		return
	}

	content, err := renderRankingXLSX(&batch, title, dims)
	if err != nil {
		logs.Error("Ranking: gagal render XLSX: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = map[string]interface{}{"success": false, "message": "Gagal membuat file Excel"}
		c.ServeJSON()
		return
	}

	fname := sanitizeFilename(batch.Name)
	if fname == "" {
		fname = fmt.Sprintf("Batch_%d", batchID)
	}
	fileName := fmt.Sprintf("%s_Ranking_%s_%s.xlsx",
		fname, strings.ToUpper(testKey), time.Now().Format("20060102"))

	c.Ctx.Output.Header("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Ctx.Output.Header("Content-Disposition", "attachment; filename=\""+fileName+"\"")
	_, _ = c.Ctx.ResponseWriter.Write(content)
}

func (c *RankingController) respondNotEnabled(name string) {
	c.Ctx.Output.SetStatus(400)
	c.Data["json"] = map[string]interface{}{
		"success": false,
		"message": "Batch ini tidak mengaktifkan tes " + name,
	}
	c.ServeJSON()
}
