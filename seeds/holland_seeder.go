package seeds

import (
	"fmt"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
)

// SeedHollandActivities inserts Holland RIASEC activity items and item->code mapping.
// If holland_questions already has data, seeder skips to avoid duplicates.
func SeedHollandActivities() error {
	o := orm.NewOrm()

	cnt, err := o.QueryTable(new(models.HollandQuestion)).Count()
	if err != nil {
		return fmt.Errorf("query holland_questions count: %w", err)
	}
	if cnt > 0 {
		return nil
	}

	type item struct {
		code    string
		number  int
		prompt  string
		answerT string
	}

	// Mapping RIASEC (R,I,A,S,E,C) based on activity theme.
	// Scale: 0..4 (value preference).
	items := []item{
		// Page 1 (1..35)
		{code: "R", number: 1, prompt: "Membangun lemari dapur", answerT: "scale"},
		{code: "R", number: 2, prompt: "Letakkan batu bata atau ubin", answerT: "scale"},
		{code: "R", number: 3, prompt: "Memperbaiki peralatan rumah tangga", answerT: "scale"},
		{code: "I", number: 4, prompt: "Memelihara ikan di tempat pembenihan ikan", answerT: "scale"},
		{code: "R", number: 5, prompt: "Merakit komponen elektronik", answerT: "scale"},
		{code: "R", number: 6, prompt: "Mengendarai truk untuk mengantarkan paket ke kantor dan rumah", answerT: "scale"},
		{code: "I", number: 7, prompt: "Uji kualitas suku cadang sebelum pengiriman", answerT: "scale"},
		{code: "R", number: 8, prompt: "Perbaiki dan pasang kunci", answerT: "scale"},
		{code: "R", number: 9, prompt: "Menyiapkan dan mengoperasikan mesin untuk membuat produk", answerT: "scale"},
		{code: "R", number: 10, prompt: "Padamkan kebakaran hutan", answerT: "scale"},
		{code: "I", number: 11, prompt: "Kembangkan obat baru", answerT: "scale"},
		{code: "I", number: 12, prompt: "Pelajari cara untuk mengurangi polusi air", answerT: "scale"},
		{code: "I", number: 13, prompt: "Melakukan eksperimen kimia", answerT: "scale"},
		{code: "I", number: 14, prompt: "Pelajari pergerakan planet", answerT: "scale"},
		{code: "I", number: 15, prompt: "Periksa sampel darah menggunakan mikroskop", answerT: "scale"},
		{code: "I", number: 16, prompt: "Selidiki penyebab kebakaran", answerT: "scale"},
		{code: "I", number: 17, prompt: "Kembangkan cara untuk memprediksi cuaca dengan lebih baik", answerT: "scale"},
		{code: "I", number: 18, prompt: "Bekerja di laboratorium biologi", answerT: "scale"},
		{code: "I", number: 19, prompt: "Temukan pengganti gula", answerT: "scale"},
		{code: "I", number: 20, prompt: "Lakukan pemeriksaan laboratorium untuk mengidentifikasi penyakit", answerT: "scale"},
		{code: "A", number: 21, prompt: "Menulis buku atau drama", answerT: "scale"},
		{code: "A", number: 22, prompt: "Mainkan alat musik", answerT: "scale"},
		{code: "A", number: 23, prompt: "Menulis atau mengaransemen musik", answerT: "scale"},
		{code: "A", number: 24, prompt: "Menggambar", answerT: "scale"},
		{code: "A", number: 25, prompt: "Buat efek khusus untuk film", answerT: "scale"},
		{code: "A", number: 26, prompt: "Set cat untuk drama", answerT: "scale"},
		{code: "A", number: 27, prompt: "Tulis skrip untuk film atau acara televisi", answerT: "scale"},
		{code: "A", number: 28, prompt: "Lakukan tarian jazz atau tap", answerT: "scale"},
		{code: "A", number: 29, prompt: "Bernyanyi dalam sebuah band", answerT: "scale"},
		{code: "A", number: 30, prompt: "Sunting film", answerT: "scale"},
		{code: "S", number: 31, prompt: "Ajari seseorang rutinitas olahraga", answerT: "scale"},
		{code: "S", number: 32, prompt: "Membantu orang dengan masalah pribadi atau emosional", answerT: "scale"},
		{code: "S", number: 33, prompt: "Memberikan bimbingan karir kepada orang-orang", answerT: "scale"},
		{code: "S", number: 34, prompt: "Lakukan terapi rehabilitasi", answerT: "scale"},
		{code: "S", number: 35, prompt: "Lakukan pekerjaan sukarela di organisasi nirlaba", answerT: "scale"},

		// Page 2 (36..60)
		{code: "S", number: 36, prompt: "Ajari anak cara berolahraga", answerT: "scale"},
		{code: "S", number: 37, prompt: "Ajarkan bahasa isyarat kepada penyandang disabilitas pendengaran", answerT: "scale"},
		{code: "S", number: 38, prompt: "Membantu melakukan sesi terapi kelompok", answerT: "scale"},
		{code: "S", number: 39, prompt: "Jaga anak-anak di pusat penitipan anak", answerT: "scale"},
		{code: "S", number: 40, prompt: "Mengajar kelas sekolah menengah", answerT: "scale"},
		{code: "E", number: 41, prompt: "Membeli dan menjual saham dan obligasi", answerT: "scale"},
		{code: "E", number: 42, prompt: "Kelola toko ritel", answerT: "scale"},
		{code: "E", number: 43, prompt: "Mengoperasikan salon kecantikan atau toko tukang cukur", answerT: "scale"},
		{code: "E", number: 44, prompt: "Kelola departemen dalam perusahaan besar", answerT: "scale"},
		{code: "E", number: 45, prompt: "Mulailah bisnis Anda sendiri", answerT: "scale"},
		{code: "E", number: 46, prompt: "Negosiasikan kontrak bisnis", answerT: "scale"},
		{code: "E", number: 47, prompt: "Mewakili klien dalam tuntutan hukum", answerT: "scale"},
		{code: "E", number: 48, prompt: "Pasarkan lini pakaian baru", answerT: "scale"},
		{code: "E", number: 49, prompt: "Jual barang dagangan di department store", answerT: "scale"},
		{code: "E", number: 50, prompt: "Kelola toko pakaian", answerT: "scale"},
		{code: "C", number: 51, prompt: "Kembangkan spreadsheet menggunakan perangkat lunak komputer", answerT: "scale"},
		{code: "C", number: 52, prompt: "Koreksi catatan atau formulir", answerT: "scale"},
		{code: "C", number: 53, prompt: "Memuat perangkat lunak komputer ke dalam jaringan komputer besar", answerT: "scale"},
		{code: "C", number: 54, prompt: "Operasikan kalkulator", answerT: "scale"},
		{code: "C", number: 55, prompt: "Simpan catatan pengiriman dan penerimaan", answerT: "scale"},
		{code: "C", number: 56, prompt: "Menghitung gaji karyawan", answerT: "scale"},
		{code: "C", number: 57, prompt: "Persediaan persediaan menggunakan komputer genggam", answerT: "scale"},
		{code: "C", number: 58, prompt: "Catat pembayaran sewa", answerT: "scale"},
		{code: "C", number: 59, prompt: "Simpan catatan inventaris", answerT: "scale"},
		{code: "C", number: 60, prompt: "Stempel, sortir, dan distribusikan surat untuk organisasi", answerT: "scale"},
	}

	for _, it := range items {
		q := models.HollandQuestion{
			Code:       it.code,
			Number:     it.number,
			Prompt:     it.prompt,
			AnswerType: it.answerT,
		}
		// Code & Number have UNIQUE together, but seeder runs only once anyway.
		if _, err := o.Insert(&q); err != nil {
			return fmt.Errorf("insert holland question %d (%s): %w", it.number, it.code, err)
		}
	}

	return nil
}

// EnsureHollandDescriptions inserts default interpretation texts if empty.
func EnsureHollandDescriptions() error {
	o := orm.NewOrm()

	cnt, err := o.QueryTable(new(models.HollandDescription)).Count()
	if err != nil {
		return fmt.Errorf("query holland_descriptions count: %w", err)
	}
	if cnt > 0 {
		return nil
	}

	descs := []models.HollandDescription{
		{Code: "R", Title: "Realistic (R)", Description: "Anda cenderung menyukai pekerjaan yang praktis, nyata, dan melibatkan keterampilan teknis atau mekanis.", RecommendedMajors: "", RecommendedJobs: "Teknik, operator mesin, teknisi, perbaikan perangkat, pemadam kebakaran"},
		{Code: "I", Title: "Investigative (I)", Description: "Anda cenderung menyukai aktivitas penelitian, analisis, dan pemecahan masalah berbasis sains atau pengamatan.", RecommendedMajors: "", RecommendedJobs: "Laboratorium, analis, peneliti, analis kualitas, meteorologi, biologi"},
		{Code: "A", Title: "Artistic (A)", Description: "Anda cenderung menyukai bidang kreatif seperti menulis, musik, desain, dan ekspresi artistik.", RecommendedMajors: "", RecommendedJobs: "Penulis, musisi, desainer, sutradara, editor film, aktor"},
		{Code: "S", Title: "Social (S)", Description: "Anda cenderung menyukai pekerjaan yang melibatkan membantu, membimbing, atau mendidik orang lain.", RecommendedMajors: "", RecommendedJobs: "Guru, konselor, terapis, pekerja sosial, pelatih olahraga, pendamping"},
		{Code: "E", Title: "Enterprising (E)", Description: "Anda cenderung menyukai aktivitas bisnis, memimpin, bernegosiasi, dan memengaruhi keputusan.", RecommendedMajors: "", RecommendedJobs: "Wirausaha, manajer, negosiator, tenaga penjualan, praktisi hukum"},
		{Code: "C", Title: "Conventional (C)", Description: "Anda cenderung menyukai pekerjaan yang terstruktur, rapi, dan berorientasi administrasi atau pengolahan data.", RecommendedMajors: "", RecommendedJobs: "Administrasi, akuntansi, analis data, back office, staf operasional"},
	}

	for _, d := range descs {
		dd := d
		if _, err := o.Insert(&dd); err != nil {
			return fmt.Errorf("insert holland description %s: %w", d.Code, err)
		}
	}

	return nil
}

