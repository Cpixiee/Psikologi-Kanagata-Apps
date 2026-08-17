package controllers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	"github.com/xuri/excelize/v2"
)

// File ini berisi builder XLSX standalone untuk RMIB, PAPI, dan Kraepelin yang
// dipakai saat admin men-download ZIP massal lewat ExportBatchAnswers. Builder
// per-peserta yang sudah ada di masing-masing test_controller (ExportResultExcel)
// tetap untuk endpoint user-facing dan tidak diubah.
//
// Setiap builder mengembalikan ([]byte, error) — bytes adalah konten XLSX,
// atau error bila peserta belum punya hasil (sehingga ZIP melewati peserta tsb).

// commonBorder + commonStyles dipakai bersama agar konsisten antar tes.
func newCommonBorder() []excelize.Border {
	return []excelize.Border{
		{Type: "left", Color: "000000", Style: 1},
		{Type: "right", Color: "000000", Style: 1},
		{Type: "top", Color: "000000", Style: 1},
		{Type: "bottom", Color: "000000", Style: 1},
	}
}

// writeBiodataBlock menulis blok identitas peserta yang seragam:
//   Nama, NISN/NIP, Kelas, Jurusan, Email, Batch, Institusi
// Ditulis mulai dari `startRow` di kolom A:C. Mengembalikan baris setelah blok.
func writeBiodataBlock(f *excelize.File, sheet string, startRow int, batch *models.TestBatch, inv *models.TestInvitation, user *models.User) int {
	if user == nil {
		user = &models.User{}
	}
	id := strings.TrimSpace(user.NISN)
	idLabel := "NISN"
	if id == "" && strings.TrimSpace(user.NIP) != "" {
		id = user.NIP
		idLabel = "NIP"
	}
	if id == "" {
		idLabel = "NISN/NIP"
	}
	nama := strings.TrimSpace(user.NamaLengkap)
	if nama == "" && inv != nil {
		nama = strings.TrimSpace(inv.Email)
	}
	email := ""
	if user.Email != "" {
		email = user.Email
	} else if inv != nil {
		email = inv.Email
	}

	rows := [][2]string{
		{"Nama Lengkap", nama},
		{idLabel, id},
		{"Kelas", user.Kelas},
		{"Jurusan", user.Jurusan},
		{"Email", email},
	}
	if batch != nil {
		rows = append(rows,
			[2]string{"Batch", batch.Name},
			[2]string{"Institusi", batch.Institution},
		)
	}

	r := startRow
	for _, kv := range rows {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", r), kv[0])
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", r), ":")
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", r), kv[1])
		r++
	}
	return r
}

// =========================================================================
// RMIB
// =========================================================================
func buildRMIBResultXLSX(o orm.Ormer, batch *models.TestBatch, inv *models.TestInvitation, user *models.User) ([]byte, error) {
	if inv == nil {
		return nil, fmt.Errorf("nil invitation")
	}
	var res models.RMIBResult
	if err := o.QueryTable(new(models.RMIBResult)).Filter("Invitation__Id", inv.Id).One(&res); err != nil || res.Id == 0 {
		return nil, fmt.Errorf("no rmib result")
	}

	type entry struct {
		Label string `json:"label"`
		Score int    `json:"score"`
		Rank  int    `json:"rank"`
	}
	parsed := map[string]entry{}
	_ = json.Unmarshal([]byte(res.ResultJSON), &parsed)

	f := excelize.NewFile()
	sheet := "RMIB"
	f.SetSheetName(f.GetSheetName(0), sheet)
	showGrid := false
	_ = f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGrid})

	border := newCommonBorder()
	styleTitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styleHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#CFE2F3"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	styleBody, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    border,
	})
	styleTop3, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "#1B5E20"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#C6EFCE"}, Pattern: 1},
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    border,
	})
	styleLabel, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})

	_ = f.SetColWidth(sheet, "A", "A", 16)
	_ = f.SetColWidth(sheet, "B", "B", 4)
	_ = f.SetColWidth(sheet, "C", "C", 32)
	_ = f.SetColWidth(sheet, "D", "D", 14)
	_ = f.SetColWidth(sheet, "E", "E", 18)

	_ = f.MergeCell(sheet, "A1", "E1")
	_ = f.SetCellValue(sheet, "A1", "HASIL TES RMIB ("+strings.ToUpper(res.GenderVersion)+")")
	_ = f.SetCellStyle(sheet, "A1", "E1", styleTitle)
	_ = f.SetRowHeight(sheet, 1, 26)

	// Biodata peserta (A:C)
	nextRow := writeBiodataBlock(f, sheet, 3, batch, inv, user)
	// Bold the labels (col A)
	_ = f.SetCellStyle(sheet, "A3", fmt.Sprintf("A%d", nextRow-1), styleLabel)
	// Tanggal tes
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Tanggal Tes")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", nextRow), ":")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", nextRow), res.CompletedAt.Format("02 Jan 2006 15:04"))
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("A%d", nextRow), styleLabel)
	nextRow++

	// Tabel ranking
	headerRow := nextRow + 1
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", headerRow), "No")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", headerRow), "")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", headerRow), "Kategori")
	_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", headerRow), "Total Skor")
	_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", headerRow), "Peringkat")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("E%d", headerRow), styleHeader)

	type catRow struct {
		Code  string
		Label string
		Score int
		Rank  int
	}
	allRows := make([]catRow, 0, len(rmibCategoryOrder))
	for _, code := range rmibCategoryOrder {
		e, ok := parsed[code]
		label := rmibCategoryLabel[code]
		if !ok {
			e = entry{Label: label}
		}
		allRows = append(allRows, catRow{Code: code, Label: label, Score: e.Score, Rank: e.Rank})
	}
	sort.SliceStable(allRows, func(i, j int) bool { return allRows[i].Rank < allRows[j].Rank })

	row := headerRow + 1
	for i, r := range allRows {
		isTop3 := r.Code == res.Top1 || r.Code == res.Top2 || r.Code == res.Top3
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), r.Label+" ("+r.Code+")")
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), r.Score)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), r.Rank)
		st := styleBody
		if isTop3 {
			st = styleTop3
		}
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), st)
		row++
	}

	row++
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row),
		"3 Minat Dominan: 1) "+rmibCategoryLabel[res.Top1]+
			"  2) "+rmibCategoryLabel[res.Top2]+
			"  3) "+rmibCategoryLabel[res.Top3])
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("E%d", row), styleHeader)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// =========================================================================
// PAPI
// =========================================================================
func buildPAPIResultXLSX(o orm.Ormer, batch *models.TestBatch, inv *models.TestInvitation, user *models.User) ([]byte, error) {
	if inv == nil {
		return nil, fmt.Errorf("nil invitation")
	}
	var res models.PAPIResult
	if err := o.QueryTable(new(models.PAPIResult)).Filter("Invitation__Id", inv.Id).One(&res); err != nil || res.Id == 0 {
		return nil, fmt.Errorf("no papi result")
	}

	type entry struct {
		Label string `json:"label"`
		Score int    `json:"score"`
		Rank  int    `json:"rank"`
	}
	parsed := map[string]entry{}
	_ = json.Unmarshal([]byte(res.ResultJSON), &parsed)

	f := excelize.NewFile()
	sheet := "PAPI"
	f.SetSheetName(f.GetSheetName(0), sheet)
	showGrid := false
	_ = f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGrid})

	border := newCommonBorder()
	styleTitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styleHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2E75B6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    border,
	})
	styleBody, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    border,
	})
	styleBodyCenter, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	styleSubtitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 12, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#5B9BD5"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styleLabel, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})

	_ = f.SetColWidth(sheet, "A", "A", 18)
	_ = f.SetColWidth(sheet, "B", "B", 4)
	_ = f.SetColWidth(sheet, "C", "C", 30)
	_ = f.SetColWidth(sheet, "D", "D", 12)
	_ = f.SetColWidth(sheet, "E", "E", 18)
	_ = f.SetColWidth(sheet, "F", "F", 14)

	_ = f.MergeCell(sheet, "A1", "F1")
	_ = f.SetCellValue(sheet, "A1", "HASIL TES PAPI KOSTICK")
	_ = f.SetCellStyle(sheet, "A1", "F1", styleTitle)
	_ = f.SetRowHeight(sheet, 1, 26)

	nextRow := writeBiodataBlock(f, sheet, 3, batch, inv, user)
	_ = f.SetCellStyle(sheet, "A3", fmt.Sprintf("A%d", nextRow-1), styleLabel)
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Tanggal Tes")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", nextRow), ":")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", nextRow), res.CompletedAt.Format("02 Jan 2006 15:04"))
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("A%d", nextRow), styleLabel)
	nextRow++
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Waktu Pengerjaan")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", nextRow), ":")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", nextRow), fmt.Sprintf("%d menit", res.TimeTakenMinutes))
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("A%d", nextRow), styleLabel)
	nextRow += 2

	// Header tabel
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Skor Per Kategori (10 Peran + 10 Kebutuhan)")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("F%d", nextRow), styleSubtitle)
	nextRow++

	headerRow := nextRow
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", headerRow), "Kelompok")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", headerRow), "Kode")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", headerRow), "Aspek")
	_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", headerRow), "Skor")
	_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", headerRow), "Peringkat")
	_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", headerRow), "Tingkat")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("F%d", headerRow), styleHeader)

	row := headerRow + 1
	for _, code := range papiCategoryOrder {
		e := parsed[code]
		level, _ := papiCategoryLabelFromRank(e.Rank)
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), papiCategoryGroup[code])
		_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", row), code)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), papiCategoryLabel[code])
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), e.Score)
		_ = f.SetCellValue(sheet, fmt.Sprintf("E%d", row), e.Rank)
		_ = f.SetCellValue(sheet, fmt.Sprintf("F%d", row), level)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("C%d", row), styleBody)
		_ = f.SetCellStyle(sheet, fmt.Sprintf("D%d", row), fmt.Sprintf("F%d", row), styleBodyCenter)
		row++
	}

	row++
	_ = f.MergeCell(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("F%d", row))
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row),
		"Kategori Dominan: "+res.DominantCategory+" — "+papiCategoryLabel[res.DominantCategory])
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("F%d", row), styleSubtitle)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// =========================================================================
// Kraepelin
// =========================================================================
func buildKraepelinResultXLSX(o orm.Ormer, batch *models.TestBatch, inv *models.TestInvitation, user *models.User) ([]byte, error) {
	if inv == nil {
		return nil, fmt.Errorf("nil invitation")
	}
	var att models.KraepelinAttempt
	if err := o.QueryTable(new(models.KraepelinAttempt)).Filter("Invitation__Id", inv.Id).Filter("Status", "finished").One(&att); err != nil || att.Id == 0 {
		return nil, fmt.Errorf("no kraepelin attempt")
	}

	f := excelize.NewFile()
	sheet := "Kraepelin"
	f.SetSheetName(f.GetSheetName(0), sheet)
	showGrid := false
	_ = f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGrid})

	border := newCommonBorder()
	styleTitle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
	})
	styleHeader, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#2E75B6"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	styleBody, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})
	styleLabel, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	styleScoreBig, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 14, Color: "1B5E20"},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"#C6EFCE"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    border,
	})

	_ = f.SetColWidth(sheet, "A", "A", 22)
	_ = f.SetColWidth(sheet, "B", "B", 4)
	_ = f.SetColWidth(sheet, "C", "C", 28)
	_ = f.SetColWidth(sheet, "D", "D", 14)

	_ = f.MergeCell(sheet, "A1", "D1")
	_ = f.SetCellValue(sheet, "A1", "HASIL TES KRAEPELIN")
	_ = f.SetCellStyle(sheet, "A1", "D1", styleTitle)
	_ = f.SetRowHeight(sheet, 1, 26)

	nextRow := writeBiodataBlock(f, sheet, 3, batch, inv, user)
	_ = f.SetCellStyle(sheet, "A3", fmt.Sprintf("A%d", nextRow-1), styleLabel)

	// Tambahan info attempt
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Mulai")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", nextRow), ":")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", nextRow), att.StartedAt.Format("02 Jan 2006 15:04"))
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("A%d", nextRow), styleLabel)
	nextRow++
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Selesai")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", nextRow), ":")
	if !att.FinishedAt.IsZero() {
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", nextRow), att.FinishedAt.Format("02 Jan 2006 15:04"))
	}
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("A%d", nextRow), styleLabel)
	nextRow++
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", nextRow), "Total Kolom")
	_ = f.SetCellValue(sheet, fmt.Sprintf("B%d", nextRow), ":")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", nextRow), att.ColumnCount)
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", nextRow), fmt.Sprintf("A%d", nextRow), styleLabel)
	nextRow += 2

	// Tabel skor
	headerRow := nextRow
	_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", headerRow), "No")
	_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", headerRow), "Aspek")
	_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", headerRow), "Nilai")
	_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", headerRow), fmt.Sprintf("D%d", headerRow), styleHeader)

	netto := att.TotalCorrect - att.TotalErrors
	scoreRows := []struct {
		Aspek string
		Nilai int
	}{
		{"Total Benar", att.TotalCorrect},
		{"Total Salah", att.TotalErrors},
		{"Total Dilewati (Skip)", att.TotalSkipped},
		{"Skor Bersih (Benar − Salah)", netto},
	}
	row := headerRow + 1
	for i, sr := range scoreRows {
		_ = f.SetCellValue(sheet, fmt.Sprintf("A%d", row), i+1)
		_ = f.SetCellValue(sheet, fmt.Sprintf("C%d", row), sr.Aspek)
		_ = f.SetCellValue(sheet, fmt.Sprintf("D%d", row), sr.Nilai)
		st := styleBody
		if i == len(scoreRows)-1 {
			st = styleScoreBig
		}
		_ = f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("D%d", row), st)
		row++
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
