package utils

import (
	"errors"
	"time"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
)

// AgeYears menghitung usia (tahun) pada tanggal at berdasarkan tanggal lahir dob.
func AgeYears(dob time.Time, at time.Time) int {
	y := at.Year() - dob.Year()
	// Jika belum lewat ulang tahun pada tahun ini, kurangi 1.
	if at.Month() < dob.Month() || (at.Month() == dob.Month() && at.Day() < dob.Day()) {
		y--
	}
	if y < 0 {
		return 0
	}
	return y
}

func normSubtestTryCodes(code string) []string {
	// Fallback ZR <-> ZA untuk data lama.
	if code == "ZR" {
		return []string{"ZR", "ZA"}
	}
	if code == "ZA" {
		return []string{"ZA", "ZR"}
	}
	return []string{code}
}

// normalizeNormAge maps ages to the norm buckets the user wants to reuse.
// - usia <= 20 tahun: gunakan norma 21-25
// - usia 46-50 tahun: gunakan norma 41-45
// Catatan: kita mengembalikan "representative age" yang berada di range target
// supaya query (AgeMin__lte, AgeMax__gte) tetap match.
func normalizeNormAge(age int) int {
	if age <= 20 {
		return 21
	}
	if age >= 46 && age <= 50 {
		return 45
	}
	return age
}

// totalSWFromSumRW implements the JUMLAH (TOTAL) RW->SW table from the user's screenshot.
// It returns SW based on SUM RW bucket for the given normAge bucket.
func totalSWFromSumRW(normAge int, sumRW int) (int, bool) {
	type bucket struct {
		min int
		sw  int
	}
	// Buckets use "min RW" (lower bound). Example: 161-170 => min=161.
	// Lookup rule: pick the bucket with the largest min <= sumRW.
	table := map[string][]bucket{
		"21-25": {
			{171, 132}, {161, 128}, {151, 124}, {141, 120}, {131, 117}, {121, 113}, {111, 109}, {101, 105},
			{91, 101}, {81, 97}, {71, 93}, {61, 90}, {51, 86}, {41, 82}, {31, 78}, {21, 74}, {11, 70}, {1, 67},
		},
		"26-30": {
			{171, 133}, {161, 129}, {151, 125}, {141, 121}, {131, 117}, {121, 113}, {111, 110}, {101, 106},
			{91, 102}, {81, 98}, {71, 94}, {61, 90}, {51, 87}, {41, 83}, {31, 79}, {21, 75}, {11, 71}, {1, 67},
		},
		"31-35": {
			{171, 133}, {161, 129}, {151, 125}, {141, 121}, {131, 117}, {121, 113}, {111, 110}, {101, 106},
			{91, 102}, {81, 98}, {71, 94}, {61, 90}, {51, 87}, {41, 83}, {31, 79}, {21, 75}, {11, 71}, {1, 67},
		},
		"36-40": {
			{171, 132}, {161, 128}, {151, 125}, {141, 121}, {131, 118}, {121, 114}, {111, 111}, {101, 108},
			{91, 104}, {81, 101}, {71, 97}, {61, 94}, {51, 90}, {41, 87}, {31, 83}, {21, 80}, {11, 77}, {1, 73},
		},
		"41-45": {
			{171, 133}, {161, 130}, {151, 127}, {141, 123}, {131, 120}, {121, 116}, {111, 113}, {101, 108},
			{91, 106}, {81, 102}, {71, 99}, {61, 96}, {51, 92}, {41, 89}, {31, 85}, {21, 82}, {11, 78}, {1, 75},
		},
		"51-60": {
			{171, 136}, {161, 133}, {151, 130}, {141, 126}, {131, 123}, {121, 120}, {111, 116}, {101, 113},
			{91, 110}, {81, 106}, {71, 103}, {61, 100}, {51, 96}, {41, 93}, {31, 90}, {21, 86}, {11, 83}, {1, 80},
		},
	}
	key := ""
	switch {
	case normAge >= 21 && normAge <= 25:
		key = "21-25"
	case normAge >= 26 && normAge <= 30:
		key = "26-30"
	case normAge >= 31 && normAge <= 35:
		key = "31-35"
	case normAge >= 36 && normAge <= 40:
		key = "36-40"
	case normAge >= 41 && normAge <= 45:
		key = "41-45"
	case normAge >= 51 && normAge <= 60:
		key = "51-60"
	default:
		return 0, false
	}
	bs := table[key]
	if sumRW <= 0 {
		// tabel JUMLAH mulai dari 1-10; treat 0 as 0 (tidak ada norma)
		return 0, false
	}
	best := 0
	for _, b := range bs {
		if sumRW >= b.min && b.sw > 0 {
			if b.min > best {
				best = b.min
			}
		}
	}
	if best == 0 {
		return 0, false
	}
	for _, b := range bs {
		if b.min == best {
			return b.sw, true
		}
	}
	return 0, false
}

func lookupStandardScore(o orm.Ormer, subtestCode string, age int, raw int) (int, error) {
	age = normalizeNormAge(age)
	// Untuk "TOTAL", gunakan range lookup karena norma menggunakan range (misalnya 171-180, 161-170)
	// Data biasanya disimpan dengan raw_score = batas bawah atau tengah range
	if subtestCode == "TOTAL" {
		// Prefer hardcoded JUMLAH table (lebih akurat & tidak tergantung kelengkapan CSV).
		if sw, ok := totalSWFromSumRW(age, raw); ok {
			return sw, nil
		}
		// Cari norma dengan raw_score <= totalRaw, ambil yang terbesar (paling dekat)
		var norms []models.ISTNorm
		for _, c := range normSubtestTryCodes(subtestCode) {
			_, err := o.QueryTable(new(models.ISTNorm)).
				Filter("SubtestCode", c).
				Filter("AgeMin__lte", age).
				Filter("AgeMax__gte", age).
				Filter("RawScore__lte", raw).
				OrderBy("-RawScore").
				Limit(1).
				All(&norms)
			if err == nil && len(norms) > 0 {
				return norms[0].StandardScore, nil
			}
		}
		// Fallback: cari norma usia terdekat jika exact match tidak ditemukan
		return lookupStandardScoreFallback(o, subtestCode, age, raw)
	}
	
	// Untuk subtest biasa, gunakan "closest match" (RawScore <= raw, ambil yang terbesar).
	// Ini lebih robust karena sebagian tabel norma sering disimpan per-bucket/range.
	{
		var norms []models.ISTNorm
		for _, c := range normSubtestTryCodes(subtestCode) {
			_, err := o.QueryTable(new(models.ISTNorm)).
				Filter("SubtestCode", c).
				Filter("AgeMin__lte", age).
				Filter("AgeMax__gte", age).
				Filter("RawScore__lte", raw).
				OrderBy("-RawScore").
				Limit(1).
				All(&norms)
			if err == nil && len(norms) > 0 {
				return norms[0].StandardScore, nil
			}
		}
	}
	// Fallback: cari norma usia terdekat jika tetap tidak ditemukan
	return lookupStandardScoreFallback(o, subtestCode, age, raw)
}

// lookupStandardScoreFallback mencari norma dengan usia terdekat jika exact match tidak ditemukan
func lookupStandardScoreFallback(o orm.Ormer, subtestCode string, age int, raw int) (int, error) {
	age = normalizeNormAge(age)
	// Cari semua norma untuk subtest dan raw score ini, tanpa filter usia
	var allNorms []models.ISTNorm
	for _, c := range normSubtestTryCodes(subtestCode) {
		_, err := o.QueryTable(new(models.ISTNorm)).
			Filter("SubtestCode", c).
			Filter("RawScore", raw).
			OrderBy("AgeMin", "AgeMax").
			All(&allNorms)
		if err == nil && len(allNorms) > 0 {
			break
		}
	}
	
	if len(allNorms) == 0 {
		return 0, errors.New("norm not found")
	}
	
	// Cari norma dengan usia terdekat (prioritas: usia yang mencakup age, lalu yang paling dekat)
	var bestNorm *models.ISTNorm
	var bestDistance int = 999
	
	for i := range allNorms {
		norm := &allNorms[i]
		distance := 0
		
		// Jika usia masuk dalam range norma, prioritas tertinggi (distance = 0)
		if age >= norm.AgeMin && age <= norm.AgeMax {
			bestNorm = norm
			bestDistance = 0
			break
		}
		
		// Hitung jarak ke range usia norma
		if age < norm.AgeMin {
			distance = norm.AgeMin - age
		} else if age > norm.AgeMax {
			distance = age - norm.AgeMax
		}
		
		if distance < bestDistance {
			bestDistance = distance
			bestNorm = norm
		}
	}
	
	if bestNorm != nil {
		logs.Info("Using fallback norm for %s: age=%d (requested), norm_age_range=%d-%d, raw=%d, std=%d", 
			subtestCode, age, bestNorm.AgeMin, bestNorm.AgeMax, raw, bestNorm.StandardScore)
		return bestNorm.StandardScore, nil
	}
	
	return 0, errors.New("norm not found")
}

// EnsureISTStandardAndIQScores mengisi Std* + TotalStandardScore + IQ + IQCategory pada ISTResult.
// Catatan: subtest VI di flow IST 176 memakai "ZR", tapi kolom DB/result memakai field ZA.
// Age harus dalam range yang valid (biasanya 5-65 tahun untuk IST) untuk perhitungan yang akurat.
func EnsureISTStandardAndIQScores(o orm.Ormer, res *models.ISTResult, age int) (*models.ISTResult, error) {
	if res == nil {
		return nil, errors.New("nil result")
	}
	
	// Validasi age: harus > 0 dan dalam range yang wajar untuk tes IST (5-65 tahun)
	if age <= 0 {
		// Tidak bisa hitung tanpa usia valid.
		return res, errors.New("age must be greater than 0")
	}
	if age < 5 || age > 65 {
		// Age di luar range normal untuk tes IST, tapi tetap coba hitung
		// (beberapa norma mungkin masih bisa digunakan)
	}

	// Subtest SW (skor standar/konversi) dari RW per usia
	// Setiap subtest menggunakan age untuk mencari norma yang tepat
	stdSE, errSE := lookupStandardScore(o, "SE", age, res.RawSE)
	stdWA, errWA := lookupStandardScore(o, "WA", age, res.RawWA)
	stdAN, errAN := lookupStandardScore(o, "AN", age, res.RawAN)
	stdGE, errGE := lookupStandardScore(o, "GE", age, res.RawGE)
	stdRA, errRA := lookupStandardScore(o, "RA", age, res.RawRA)
	// subtest VI: ZR -> field ZA
	stdZA, errZA := lookupStandardScore(o, "ZR", age, res.RawZA)
	stdFA, errFA := lookupStandardScore(o, "FA", age, res.RawFA)
	stdWU, errWU := lookupStandardScore(o, "WU", age, res.RawWU)
	stdME, errME := lookupStandardScore(o, "ME", age, res.RawME)
	
	// Log jika ada subtest yang tidak ketemu normanya
	if errSE != nil {
		logs.Warning("Lookup standard score failed for SE: age=%d, raw=%d, error=%v", age, res.RawSE, errSE)
	}
	if errWA != nil {
		logs.Warning("Lookup standard score failed for WA: age=%d, raw=%d, error=%v", age, res.RawWA, errWA)
	}
	if errAN != nil {
		logs.Warning("Lookup standard score failed for AN: age=%d, raw=%d, error=%v", age, res.RawAN, errAN)
	}
	if errGE != nil {
		logs.Warning("Lookup standard score failed for GE: age=%d, raw=%d, error=%v", age, res.RawGE, errGE)
	}
	if errRA != nil {
		logs.Warning("Lookup standard score failed for RA: age=%d, raw=%d, error=%v", age, res.RawRA, errRA)
	}
	if errZA != nil {
		logs.Warning("Lookup standard score failed for ZR: age=%d, raw=%d, error=%v", age, res.RawZA, errZA)
	}
	if errFA != nil {
		logs.Warning("Lookup standard score failed for FA: age=%d, raw=%d, error=%v", age, res.RawFA, errFA)
	}
	if errWU != nil {
		logs.Warning("Lookup standard score failed for WU: age=%d, raw=%d, error=%v", age, res.RawWU, errWU)
	}
	if errME != nil {
		logs.Warning("Lookup standard score failed for ME: age=%d, raw=%d, error=%v", age, res.RawME, errME)
	}

	res.StdSE = stdSE
	res.StdWA = stdWA
	res.StdAN = stdAN
	res.StdGE = stdGE
	res.StdRA = stdRA
	res.StdZA = stdZA
	res.StdFA = stdFA
	res.StdWU = stdWU
	res.StdME = stdME

	// TotalStandardScore harus mengikuti norma "JUMLAH" (berdasarkan SUM RW 9 subtest),
	// bukan rata-rata SW. SUM RW (max 180) dipetakan ke SW TOTAL (mis. 171-180 => 132),
	// lalu SW TOTAL dipetakan ke IQ.
	totalRaw := res.RawSE + res.RawWA + res.RawAN + res.RawGE + res.RawRA + res.RawZA + res.RawFA + res.RawWU + res.RawME
	totalSW, errTotal := lookupStandardScore(o, "TOTAL", age, totalRaw)
	if errTotal != nil {
		logs.Warning("Lookup standard score failed for TOTAL: age=%d, totalRaw=%d, error=%v", age, totalRaw, errTotal)
	}
	res.TotalStandardScore = totalSW
	logs.Info("Calculated TotalStandardScore (TOTAL SW) from SUM RW: age=%d, totalRaw=%d, totalSW=%d", age, totalRaw, totalSW)

	// IQ ditentukan dari TotalStandardScore berdasarkan usia
	// Setiap usia memiliki tabel norma IQ yang berbeda, jadi lookup harus menggunakan age
	normAge := normalizeNormAge(age)
	var iqNorm models.ISTIQNorm
	err := o.QueryTable(new(models.ISTIQNorm)).
		Filter("TotalStandardScore", totalSW).
		Filter("AgeMin__lte", normAge).
		Filter("AgeMax__gte", normAge).
		One(&iqNorm)
	if err == nil && iqNorm.Id != 0 {
		res.IQ = iqNorm.IQ
		res.IQCategory = iqNorm.Category
		logs.Info("Found IQ norm: totalSW=%d, age=%d(normAge=%d), IQ=%d, category=%s", totalSW, age, normAge, iqNorm.IQ, iqNorm.Category)
	} else if totalSW > 0 {
		logs.Warning("IQ norm lookup failed for exact match: totalSW=%d, age=%d(normAge=%d), error=%v. Trying closest match...", totalSW, age, normAge, err)
		// Jika tidak ditemukan exact match, cari yang terdekat untuk usia tersebut
		// Cari IQ norm terdekat (lebih kecil atau sama dengan TotalStandardScore) untuk usia yang sesuai
		var closestNorm models.ISTIQNorm
		err2 := o.QueryTable(new(models.ISTIQNorm)).
			Filter("TotalStandardScore__lte", totalSW).
			Filter("AgeMin__lte", normAge).
			Filter("AgeMax__gte", normAge).
			OrderBy("-TotalStandardScore").
			One(&closestNorm)
		if err2 == nil && closestNorm.Id != 0 {
			res.IQ = closestNorm.IQ
			res.IQCategory = closestNorm.Category
		} else {
			// Jika tidak ada nilai <= totalSW (mis. totalSW lebih kecil dari WS minimum di CSV),
			// cari nilai >= totalSW yang paling kecil (clamp ke WS minimum).
			var upperNorm models.ISTIQNorm
			errUpper := o.QueryTable(new(models.ISTIQNorm)).
				Filter("TotalStandardScore__gte", totalSW).
				Filter("AgeMin__lte", normAge).
				Filter("AgeMax__gte", normAge).
				OrderBy("TotalStandardScore").
				One(&upperNorm)
			if errUpper == nil && upperNorm.Id != 0 {
				res.IQ = upperNorm.IQ
				res.IQCategory = upperNorm.Category
				return res, nil
			}
			// Jika masih tidak ditemukan untuk usia tersebut, coba tanpa filter usia (fallback untuk backward compatibility)
			var fallbackNorm models.ISTIQNorm
			err3 := o.QueryTable(new(models.ISTIQNorm)).
				Filter("TotalStandardScore__lte", totalSW).
				OrderBy("-TotalStandardScore").
				One(&fallbackNorm)
			if err3 == nil && fallbackNorm.Id != 0 {
				res.IQ = fallbackNorm.IQ
				res.IQCategory = fallbackNorm.Category
			} else {
				// Biarkan IQ tetap 0 sebagai indikator bahwa perhitungan tidak lengkap
			}
		}
	}

	return res, nil
}
