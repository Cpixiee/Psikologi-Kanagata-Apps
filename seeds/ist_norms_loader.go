package seeds

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
)

// EnsureISTNorms memastikan tabel ist_norms & ist_iq_norms terisi.
// Prioritas file:
// - data/ist_norms.csv dan data/ist_iq_norms.csv (real data)
// - fallback: data/ist_norms.sample.csv dan data/ist_iq_norms.sample.csv
func EnsureISTNorms() error {
	o := orm.NewOrm()

	normPath := pickFirstExisting(
		filepath.Join("data", "ist_norms.csv"),
		filepath.Join("data", "ist_norms.sample.csv"),
	)
	if normPath == "" {
		return fmt.Errorf("IST norms file not found (expected data/ist_norms.csv)")
	}

	if err := loadISTNormsCSV(o, normPath); err != nil {
		return err
	}

	// IQ norms
	// Selalu sinkronkan tabel ist_iq_norms dengan CSV di setiap startup,
	// supaya perubahan file norma langsung ter-update tanpa perlu TRUNCATE manual.
	iqPath := pickFirstExisting(
		filepath.Join("data", "ist_iq_norms.csv"),
		filepath.Join("data", "ist_iq_norms.sample.csv"),
	)
	if iqPath == "" {
		return fmt.Errorf("IST IQ norms file not found (expected data/ist_iq_norms.csv)")
	}
	if err := loadISTIQNormsCSV(o, iqPath); err != nil {
		return err
	}

	return nil
}

func pickFirstExisting(paths ...string) string {
	for _, p := range paths {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func loadISTNormsCSV(o orm.Ormer, path string) error {
	// Bersihkan tabel terlebih dahulu agar isi selalu persis mengikuti CSV
	if _, err := o.Raw("TRUNCATE TABLE ist_norms RESTART IDENTITY CASCADE").Exec(); err != nil {
		return fmt.Errorf("truncate norms: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open norms csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	// Izinkan jumlah kolom yang bervariasi per baris (supaya baris komentar / penjelasan
	// seperti "Example only..." tidak menyebabkan error "wrong number of fields").
	// Validasi jumlah kolom yang "benar" tetap dilakukan di bawah (len(rec) < 5 -> skip).
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("read norms csv: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("norms csv empty: %s", path)
	}

	inserted := 0
	for i, rec := range records {
		// header / comment
		if i == 0 {
			continue
		}
		if len(rec) == 0 {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(rec[0]), "#") {
			continue
		}
		if len(rec) < 5 {
			continue
		}
		sub := strings.ToUpper(strings.TrimSpace(rec[0]))
		ageMin, _ := strconv.Atoi(strings.TrimSpace(rec[1]))
		ageMax, _ := strconv.Atoi(strings.TrimSpace(rec[2]))
		raw, _ := strconv.Atoi(strings.TrimSpace(rec[3]))
		std, _ := strconv.Atoi(strings.TrimSpace(rec[4]))
		if sub == "" || ageMin <= 0 || ageMax <= 0 {
			continue
		}
		n := models.ISTNorm{
			SubtestCode:   sub,
			AgeMin:        ageMin,
			AgeMax:        ageMax,
			RawScore:      raw,
			StandardScore: std,
		}
		if _, ierr := o.Insert(&n); ierr == nil {
			inserted++
		}
	}
	if inserted == 0 {
		return fmt.Errorf("no IST norms inserted from %s", path)
	}
	return nil
}

func loadISTIQNormsCSV(o orm.Ormer, path string) error {
	// Pastikan skema tabel ist_iq_norms sudah sesuai dengan model:
	// - Punya kolom age_min dan age_max
	// - Unique constraint menggunakan (total_standard_score, age_min, age_max)
	if _, err := o.Raw(`
		ALTER TABLE ist_iq_norms
			ADD COLUMN IF NOT EXISTS age_min INT NOT NULL DEFAULT 0,
			ADD COLUMN IF NOT EXISTS age_max INT NOT NULL DEFAULT 99;
	`).Exec(); err != nil {
		return fmt.Errorf("ensure iq norms age columns: %w", err)
	}
	// Hapus unique constraint lama (jika ada) yang hanya pada total_standard_score
	if _, err := o.Raw(`
		ALTER TABLE ist_iq_norms
		DROP CONSTRAINT IF EXISTS ist_iq_norms_total_standard_score_key;
	`).Exec(); err != nil {
		return fmt.Errorf("drop old iq norms unique constraint: %w", err)
	}
	// Tambahkan unique constraint baru jika belum ada
	if _, err := o.Raw(`
		DO $$
		BEGIN
			IF NOT EXISTS (
				SELECT 1 FROM pg_constraint
				WHERE conname = 'ist_iq_norms_ws_age_unique'
			) THEN
				ALTER TABLE ist_iq_norms
				ADD CONSTRAINT ist_iq_norms_ws_age_unique
				UNIQUE (total_standard_score, age_min, age_max);
			END IF;
		END $$;
	`).Exec(); err != nil {
		return fmt.Errorf("ensure iq norms unique constraint: %w", err)
	}

	// Bersihkan tabel terlebih dahulu agar isi selalu persis mengikuti CSV
	if _, err := o.Raw("TRUNCATE TABLE ist_iq_norms RESTART IDENTITY CASCADE").Exec(); err != nil {
		return fmt.Errorf("truncate iq norms: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open iq norms csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	// Sama seperti ISTNorms: izinkan jumlah kolom bervariasi agar baris komentar
	// atau penjelasan tidak menyebabkan error saat dibaca semua dengan ReadAll().
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	records, err := r.ReadAll()
	if err != nil {
		return fmt.Errorf("read iq norms csv: %w", err)
	}
	if len(records) == 0 {
		return fmt.Errorf("iq norms csv empty: %s", path)
	}

	inserted := 0
	// Check header to determine format: 
	// Format 1 (old): ws,iq,category
	// Format 2 (new): ws,age_min,age_max,iq,category
	hasAgeColumns := false
	if len(records) > 0 {
		header := strings.ToLower(strings.TrimSpace(strings.Join(records[0], ",")))
		hasAgeColumns = strings.Contains(header, "age_min") || strings.Contains(header, "age")
	}
	
	for i, rec := range records {
		if i == 0 {
			continue // skip header
		}
		if len(rec) == 0 {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(rec[0]), "#") {
			continue
		}
		
		var ws, ageMin, ageMax int
		var iq float64
		var cat string
		
		if hasAgeColumns && len(rec) >= 5 {
			// Format baru: ws,age_min,age_max,iq,category
			ws, _ = strconv.Atoi(strings.TrimSpace(rec[0]))
			ageMin, _ = strconv.Atoi(strings.TrimSpace(rec[1]))
			ageMax, _ = strconv.Atoi(strings.TrimSpace(rec[2]))
			iq, _ = strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(rec[3]), ",", "."), 64)
			cat = strings.TrimSpace(rec[4])
		} else if len(rec) >= 3 {
			// Format lama: ws,iq,category (backward compatibility)
			ws, _ = strconv.Atoi(strings.TrimSpace(rec[0]))
			iq, _ = strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(rec[1]), ",", "."), 64)
			cat = strings.TrimSpace(rec[2])
			ageMin = 0  // default: semua usia
			ageMax = 99
		} else {
			continue // skip invalid rows
		}
		
		n := models.ISTIQNorm{
			TotalStandardScore: ws,
			AgeMin:             ageMin,
			AgeMax:             ageMax,
			IQ:                 int(iq + 0.5), // backward-compatible: store rounded IQ as int
			Category:           cat,
		}
		if _, ierr := o.Insert(&n); ierr == nil {
			inserted++
		}
	}
	if inserted == 0 {
		return fmt.Errorf("no IST IQ norms inserted from %s", path)
	}
	return nil
}

