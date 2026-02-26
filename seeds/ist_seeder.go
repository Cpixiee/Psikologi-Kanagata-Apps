package seeds

import (
	"fmt"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
)

// SeedIST membuat data trial untuk IST: 9 subtest + soal contoh + norma.
// Data lengkap bisa diimpor dari CSV nanti.
func SeedIST() error {
	o := orm.NewOrm()

	subtests := []struct {
		Code       string
		Name       string
		OrderIndex int
	}{
		{"SE", "Sentence Completion", 1},
		{"WA", "Word Analogies", 2},
		{"AN", "Arithmetic", 3},
		{"ME", "General Knowledge", 4},
		{"RA", "Number Series", 5},
		{"ZA", "Figure Analogies", 6},
		{"FA", "Shape Assembly", 7},
		{"WU", "Cube Rotation", 8},
		{"GE", "General Comprehension", 9},
	}

	for _, s := range subtests {
		var sub models.ISTSubtest
		err := o.QueryTable(new(models.ISTSubtest)).Filter("Code", s.Code).One(&sub)
		if err == nil {
			continue
		}
		sub = models.ISTSubtest{Code: s.Code, Name: s.Name, OrderIndex: s.OrderIndex}
		if _, err := o.Insert(&sub); err != nil {
			return fmt.Errorf("insert subtest %s: %w", s.Code, err)
		}
	}

	// Ambil subtest untuk membuat soal contoh (minimal 2 soal per subtest untuk trial)
	subList := []models.ISTSubtest{}
	_, err := o.QueryTable(new(models.ISTSubtest)).OrderBy("OrderIndex").All(&subList)
	if err != nil {
		return fmt.Errorf("query subtests: %w", err)
	}

	for _, sub := range subList {
		count, _ := o.QueryTable(new(models.ISTQuestion)).Filter("Subtest__Id", sub.Id).Count()
		if count > 0 {
			continue
		}
		// Contoh 2 soal per subtest untuk trial
		questions := []struct {
			prompt  string
			opts    []string
			correct string
		}{
			{
				"Soal contoh 1 untuk " + sub.Code + ". Pilih jawaban yang benar.",
				[]string{"A", "B", "C", "D", "E"},
				"A",
			},
			{
				"Soal contoh 2 untuk " + sub.Code + ". Pilih jawaban yang benar.",
				[]string{"A", "B", "C", "D", "E"},
				"B",
			},
		}
		for i, q := range questions {
			opts := q.opts
			if len(opts) < 5 {
				opts = append(opts, "D", "E")
			}
			iq := models.ISTQuestion{
				Subtest:  &sub,
				Number:   i + 1,
				Prompt:   q.prompt,
				OptionA:  opts[0],
				OptionB:  opts[1],
				OptionC:  opts[2],
				OptionD:  opts[3],
				OptionE:  opts[4],
				Correct:  q.correct,
			}
			if _, err := o.Insert(&iq); err != nil {
				return fmt.Errorf("insert question %s #%d: %w", sub.Code, i+1, err)
			}
		}
	}

	// Norma contoh: raw 0-20 -> std 1-7 untuk usia 15-25
	if cnt, _ := o.QueryTable(new(models.ISTNorm)).Count(); cnt > 0 {
		fmt.Println("IST norms already exist, skip")
		return nil
	}

	for _, sub := range subList {
		for raw := 0; raw <= 20; raw++ {
			std := 1 + (raw * 6 / 20)
			if std > 7 {
				std = 7
			}
			n := models.ISTNorm{
				SubtestCode:   sub.Code,
				AgeMin:        15,
				AgeMax:        99,
				RawScore:      raw,
				StandardScore: std,
			}
			if _, err := o.Insert(&n); err != nil {
				continue
			}
		}
	}

	// IQ norms: total SS (9-63) -> IQ
	if cnt, _ := o.QueryTable(new(models.ISTIQNorm)).Count(); cnt > 0 {
		fmt.Println("IST IQ norms already exist, skip")
		return nil
	}

	for total := 9; total <= 63; total++ {
		iq := 50 + (total-9)*2
		if iq < 55 {
			iq = 55
		}
		if iq > 145 {
			iq = 145
		}
		cat := "Sangat rendah"
		if iq >= 70 {
			cat = "Borderline"
		}
		if iq >= 90 {
			cat = "Rata-rata bawah"
		}
		if iq >= 100 {
			cat = "Rata-rata"
		}
		if iq >= 110 {
			cat = "Rata-rata atas"
		}
		if iq >= 120 {
			cat = "Superior"
		}
		if iq >= 130 {
			cat = "Sangat superior"
		}
		n := models.ISTIQNorm{
			TotalStandardScore: total,
			IQ:                 iq,
			Category:           cat,
		}
		o.Insert(&n)
	}

	fmt.Println("IST seeder done: 9 subtests, sample questions, norms")
	return nil
}
