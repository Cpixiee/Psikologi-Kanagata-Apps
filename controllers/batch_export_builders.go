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

	var counts []int
	if att.CorrectCountsJSON != "" {
		_ = json.Unmarshal([]byte(att.CorrectCountsJSON), &counts)
	}
	if len(counts) != 40 {
		counts = make([]int, 40)
	}

	// Faktor-faktor Kraepelin
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
	panker := (2.0*float64(aboveCount) - float64(onLineCount)) / 40.0
	janker := maxY - minY
	y0 := 0
	yLast := 0
	if len(counts) > 0 {
		y0 = counts[0]
		yLast = counts[len(counts)-1]
	}
	hankerDiff := yLast - y0
	tianker := att.TotalErrors + att.TotalSkipped

	f := excelize.NewFile()
	sheet := "Kraepelin"
	f.SetSheetName(f.GetSheetName(0), sheet)
	showGridLines := false
	_ = f.SetSheetView(sheet, 0, &excelize.ViewOptions{ShowGridLines: &showGridLines})

	_ = f.SetColWidth(sheet, "A", "A", 10)
	_ = f.SetColWidth(sheet, "B", "B", 12)
	_ = f.SetColWidth(sheet, "D", "E", 20)
	_ = f.SetColWidth(sheet, "G", "N", 12)
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

	_ = f.SetCellValue(sheet, "A1", "TES KRAEPELIN")
	_ = f.MergeCell(sheet, "A1", "E1")
	_ = f.SetCellStyle(sheet, "A1", "E1", titleStyle)

	nama := user.NamaLengkap
	if strings.TrimSpace(nama) == "" {
		nama = att.TestName
	}
	nisn := user.NISN
	if strings.TrimSpace(nisn) == "" {
		nisn = user.NIP
	}

	_ = f.SetCellValue(sheet, "A3", "Nama")
	_ = f.SetCellValue(sheet, "B3", nama)
	_ = f.SetCellValue(sheet, "A4", "NISN/NIP")
	_ = f.SetCellValue(sheet, "B4", nisn)
	_ = f.SetCellValue(sheet, "A5", "Kelas")
	_ = f.SetCellValue(sheet, "B5", user.Kelas)
	_ = f.SetCellValue(sheet, "A6", "Jurusan")
	_ = f.SetCellValue(sheet, "B6", user.Jurusan)
	_ = f.SetCellValue(sheet, "A7", "Tanggal tes")
	_ = f.SetCellValue(sheet, "B7", att.TestDate.Format("2006-01-02 15:04"))
	_ = f.SetCellStyle(sheet, "A3", "A7", labelStyle)
	_ = f.SetCellStyle(sheet, "B3", "B7", valueStyle)

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

	_ = f.SetCellValue(sheet, "H3", "Sum of Error")
	_ = f.SetCellValue(sheet, "I3", att.TotalErrors)
	_ = f.SetCellValue(sheet, "H4", "Sum of Skippeds")
	_ = f.SetCellValue(sheet, "I4", att.TotalSkipped)
	_ = f.SetCellStyle(sheet, "H3", "H4", labelStyle)
	_ = f.SetCellStyle(sheet, "I3", "I4", yColStyle)

	analysisHeaderStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	_ = f.SetCellValue(sheet, "H10", "Pembulatan")
	_ = f.SetCellValue(sheet, "I10", "Analisis")
	_ = f.SetCellStyle(sheet, "H10", "I10", analysisHeaderStyle)
	_ = f.SetCellValue(sheet, "G11", "Panker (Titik Setimbang)")
	_ = f.SetCellValue(sheet, "G12", "Janker (Faktor Ritme)")
	_ = f.SetCellValue(sheet, "G13", "Hanker (Daya Tahan y40-y0)")
	_ = f.SetCellValue(sheet, "G14", "Tianker (Ketelitian & Skip)")
	_ = f.SetCellValue(sheet, "H11", fmt.Sprintf("%.2f", panker))
	_ = f.SetCellValue(sheet, "H12", fmt.Sprintf("%.2f", float64(janker)))
	_ = f.SetCellValue(sheet, "H13", fmt.Sprintf("%.2f", float64(hankerDiff)))
	_ = f.SetCellValue(sheet, "H14", fmt.Sprintf("%d", tianker))
	_ = f.SetCellValue(sheet, "I11", "(speed factor)")
	_ = f.SetCellValue(sheet, "I12", "(rhitme factor)")
	_ = f.SetCellValue(sheet, "I13", "(ausdeur factor)")
	_ = f.SetCellValue(sheet, "I14", "(tianker)")

	redValueStyle, _ := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Size: 10, Color: "#FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#C00000"}},
		Alignment: &excelize.Alignment{Horizontal: "center"},
	})
	_ = f.SetCellStyle(sheet, "H11", "H14", redValueStyle)
	_ = f.SetCellStyle(sheet, "G11", "G14", labelStyle)
	_ = f.SetCellStyle(sheet, "I11", "I14", valueStyle)

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
		Legend: excelize.ChartLegend{Position: "bottom"},
	})

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
