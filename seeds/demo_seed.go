package seeds

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
)

// EnsureDemoThreeTests membuat Batch Demo dengan 3-4 tes (IST, Holland, VAK, RMIB)
// yang sudah terisi hasil lengkap untuk siswa demo, sehingga user bisa langsung melihat
// interpretasi AI integratif, rekomendasi jurusan, dan karir tanpa harus tes manual.
func EnsureDemoThreeTests() error {
	o := orm.NewOrm()

	// 1. Cek atau buat TestBatch demo
	var batch models.TestBatch
	err := o.QueryTable("test_batches").Filter("name", "Batch Demo 3 Tes Psikotes (IST + Holland + VAK)").One(&batch)
	if err == orm.ErrNoRows {
		batch = models.TestBatch{
			Name:                "Batch Demo 3 Tes Psikotes (IST + Holland + VAK)",
			Institution:         "Kanagata Psychee",
			TahunAjaran:         "2025/2026",
			Sekolah:             "", // Kosongkan agar dapat diakses oleh semua Akun Sekolah
			Kelas:               "XII MIPA 1",
			Jurusan:             "MIPA",
			EnableIST:           true,
			EnableHolland:       true,
			EnableLearningStyle: true,
			EnableRMIB:          true,
			EnablePAPI:          false,
			EnableKraepelin:     false,
			TestOrder:           "ist,holland,learning_style,rmib",
			PurposeCategory:     models.PurposeCategoryEducation,
			PurposeDetail:       models.PurposeDetailMenentukanJurusan,
			Status:              models.StatusBatchActive,
			CreatedBy:           1,
			CreatedAt:           time.Now(),
		}
		id, ierr := o.Insert(&batch)
		if ierr != nil {
			return fmt.Errorf("gagal membuat batch demo: %v", ierr)
		}
		batch.Id = int(id)
		log.Printf("[SEED] TestBatch Demo berhasil dibuat dengan ID: %d", batch.Id)
	} else if batch.Sekolah != "" {
		batch.Sekolah = ""
		_, _ = o.Update(&batch, "Sekolah")
	}

	// 1.5. Cek atau buat Akun Sekolah Demo
	var schoolUser models.User
	sErr := o.QueryTable("users").Filter("email", "sekolah@psikologi.local").One(&schoolUser)
	if sErr == orm.ErrNoRows {
		schoolUser = models.User{
			Email:        "sekolah@psikologi.local",
			Password:     "sekolah123",
			Role:         models.RoleSekolah,
			NamaLengkap:  "SMA Negeri 1 Kanagata (Akun Sekolah)",
			JenisKelamin: models.GenderLakiLaki,
			Sekolah:      "SMA Negeri 1 Kanagata",
			CreatedAt:    time.Now(),
		}
		_ = schoolUser.HashPassword()
		if _, sInsertErr := o.Insert(&schoolUser); sInsertErr == nil {
			log.Printf("[SEED] Akun Sekolah Demo berhasil dibuat: sekolah@psikologi.local / sekolah123")
		}
	} else {
		schoolUser.Password = "sekolah123"
		_ = schoolUser.HashPassword()
		schoolUser.Role = models.RoleSekolah
		if schoolUser.JenisKelamin == "" {
			schoolUser.JenisKelamin = models.GenderLakiLaki
		}
		_, _ = o.Update(&schoolUser, "Password", "Role", "JenisKelamin")
		log.Printf("[SEED] Akun Sekolah Demo password berhasil di-update: sekolah@psikologi.local / sekolah123")
	}

	// 2. Cek atau buat User Demo (Budi Pratama)
	var user models.User
	uErr := o.QueryTable("users").Filter("email", "budi.demo@psikologi.local").One(&user)
	if uErr == orm.ErrNoRows {
		user = models.User{
			Email:        "budi.demo@psikologi.local",
			Password:     "$2a$10$wN31XvP60nF70uB.NqjZ7e8a9Z0b1c2d3e4f5g6h7i8j9k0l1m2n", // dummy hash
			Role:         models.RoleSiswa,
			NamaLengkap:  "Budi Pratama (Demo 3 Tes)",
			JenisKelamin: models.GenderLakiLaki,
			Sekolah:      "",
			Kelas:        "XII MIPA 1",
			NoHandphone:  "081234567890",
			CreatedAt:   time.Now(),
		}
		uid, uinsertErr := o.Insert(&user)
		if uinsertErr != nil {
			log.Printf("[SEED WARNING] Gagal membuat user demo: %v", uinsertErr)
		} else {
			user.Id = int(uid)
		}
	} else if user.Sekolah != "" {
		user.Sekolah = ""
		_, _ = o.Update(&user, "Sekolah")
	}

	// 3. Cek atau buat TestInvitation
	var inv models.TestInvitation
	var bIdPtr *int
	if batch.Id > 0 {
		bIdPtr = &batch.Id
	}
	var uIdPtr *int
	if user.Id > 0 {
		uIdPtr = &user.Id
	}

	invErr := o.QueryTable("test_invitations").Filter("token", "demo-3-test-token").One(&inv)
	if invErr == orm.ErrNoRows {
		inv = models.TestInvitation{
			BatchId:   bIdPtr,
			UserId:    uIdPtr,
			Email:     "budi.demo@psikologi.local",
			Phone:     "081234567890",
			Token:     "demo-3-test-token",
			Status:    models.StatusInvitationUsed,
			ExpiresAt: time.Now().AddDate(1, 0, 0),
			UsedAt:    time.Now(),
			CreatedAt: time.Now(),
		}
		iid, iinsertErr := o.Insert(&inv)
		if iinsertErr != nil {
			return fmt.Errorf("gagal membuat invitation demo: %v", iinsertErr)
		}
		inv.Id = int(iid)
		log.Printf("[SEED] TestInvitation Demo berhasil dibuat dengan Token: demo-3-test-token")
	}

	var uPtr *models.User
	if user.Id > 0 {
		uPtr = &user
	}

	// 4. Seed Hasil IST (IQ 118, High Average)
	var istRes models.ISTResult
	if err := o.QueryTable("ist_results").Filter("invitation_id", inv.Id).One(&istRes); err == orm.ErrNoRows {
		istRes = models.ISTResult{
			Invitation:         &inv,
			User:               uPtr,
			RawSE:              15, StdSE: 115,
			RawWA:              14, StdWA: 112,
			RawAN:              16, StdAN: 118,
			RawME:              13, StdME: 108,
			RawRA:              17, StdRA: 122,
			RawZA:              15, StdZA: 116,
			RawFA:              14, StdFA: 110,
			RawWU:              16, StdWU: 120,
			RawGE:              13, StdGE: 106,
			TotalStandardScore: 1027,
			IQ:                 118,
			IQCategory:         "Diatas Rata-Rata (High Average)",
			Strengths:          "Analisis Numerik, Penalaran Abstrak, Daya Bayang Ruang & Visualisasi 3D",
			Weaknesses:         "Kecepatan Analogik Kata Kompleks",
			Summary:            "Siswa memiliki kapasitas intelektual yang sangat baik di atas rata-rata dengan keunggulan utama dalam penalaran logis-matematis (RA) dan kecerdasan spasial (WU). Sangat potensial untuk bidang sains, teknologi, dan rekayasa.",
			CreatedAt:          time.Now(),
		}
		if _, ierr := o.Insert(&istRes); ierr != nil {
			log.Printf("[SEED WARNING] Gagal membuat ISTResult demo: %v", ierr)
		} else {
			log.Printf("[SEED] ISTResult demo berhasil dibuat (IQ 118)")
		}
	}

	// 5. Seed Hasil Holland (Kode: ISA - Investigatif, Sosial, Artistik)
	var holRes models.HollandResult
	if err := o.QueryTable("holland_results").Filter("invitation_id", inv.Id).One(&holRes); err == orm.ErrNoRows {
		holRes = models.HollandResult{
			Invitation:      &inv,
			User:            uPtr,
			ScoreR:          18,
			ScoreI:          28,
			ScoreA:          22,
			ScoreS:          25,
			ScoreE:          20,
			ScoreC:          16,
			Top1:            "I",
			Top2:            "S",
			Top3:            "A",
			Code:            "ISA",
			Interpretation:  "Profil minat Holland ISA menunjukkan kombinasi Investigatif (senang memecahkan masalah & analisis), Sosial (senang membantu & membimbing orang lain), serta Artistik (kreatif & ekspresif). Sangat ideal untuk karir di bidang Data Science, AI Engineering, Edukator Teknologi, atau UI/UX Design.",
			DreamJob1:       "AI & Data Scientist",
			DreamJob2:       "Software Architect / UI-UX Designer",
			DreamJob3:       "Dosen / Edukator Sains & Teknologi",
			FavoriteSubject: "Informatika, Matematika Lanjut, & Fisika",
			DislikedSubject: "Hafalan Teori Murni",
			CreatedAt:       time.Now(),
		}
		if _, ierr := o.Insert(&holRes); ierr != nil {
			log.Printf("[SEED WARNING] Gagal membuat HollandResult demo: %v", ierr)
		} else {
			log.Printf("[SEED] HollandResult demo berhasil dibuat (Kode ISA)")
		}
	}

	// 6. Seed Hasil Gaya Belajar VAK (Dominan Visual)
	var vakRes models.LearningStyleResult
	if err := o.QueryTable("learning_style_results").Filter("invitation_id", inv.Id).One(&vakRes); err == orm.ErrNoRows {
		vakRes = models.LearningStyleResult{
			Invitation:                &inv,
			User:                      uPtr,
			TestName:                  "Tes Gaya Belajar VAK",
			TestAge:                   17,
			TestInstitution:           "SMA Negeri 1 Kanagata",
			TestGender:                "Laki-laki",
			TestDate:                  time.Now(),
			ScoreVisual:               18,
			ScoreAuditory:             14,
			ScoreKinesthetic:          10,
			DominantType:              "Visual",
			InterpretationVisual:      "Sangat kuat dalam gaya belajar Visual. Lebih mudah memahami materi berupa peta konsep, diagram logika, diagram arsitektur, dan instruksi tertulis/skema.",
			InterpretationAuditory:    "Kapasitas Auditori cukup baik melalui diskusi terstruktur.",
			InterpretationKinesthetic: "Kapasitas Kinestetik berfungsi baik saat praktik coding / eksperimen langsung.",
			CreatedAt:                 time.Now(),
		}
		if _, ierr := o.Insert(&vakRes); ierr != nil {
			log.Printf("[SEED WARNING] Gagal membuat LearningStyleResult demo: %v", ierr)
		} else {
			log.Printf("[SEED] LearningStyleResult demo berhasil dibuat (Dominan Visual)")
		}
	}

	// 7. Seed Hasil RMIB (Top: SCI, PERS, AEST)
	var rmibRes models.RMIBResult
	if err := o.QueryTable("rmib_results").Filter("invitation_id", inv.Id).One(&rmibRes); err == orm.ErrNoRows {
		rmibScores := map[string]int{
			"SCI": 88, "PERS": 82, "AEST": 76, "COMP": 70,
			"OUT": 62, "MED": 58, "PRAC": 52, "CLER": 45,
		}
		rmibJSON, _ := json.Marshal(rmibScores)

		rmibRes = models.RMIBResult{
			Invitation:       &inv,
			User:             uPtr,
			GenderVersion:    "pria",
			ResultJSON:       string(rmibJSON),
			DominantCategory: "SCI",
			Top1:             "SCI",
			Top2:             "PERS",
			Top3:             "AEST",
			Interpretation:   "Minat pekerjaan teratas berada pada kategori Scientific (Sains, Data, & Teknologi), Persuasive (Komunikasi & Manajemen), dan Aesthetic (Kreativitas Visual).",
			CompletedAt:      time.Now(),
		}
		if _, ierr := o.Insert(&rmibRes); ierr != nil {
			log.Printf("[SEED WARNING] Gagal membuat RMIBResult demo: %v", ierr)
		} else {
			log.Printf("[SEED] RMIBResult demo berhasil dibuat (Top SCI/PERS/AEST)")
		}
	}

	return nil
}
