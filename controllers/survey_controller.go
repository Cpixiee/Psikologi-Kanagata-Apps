package controllers

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/jung-kurt/gofpdf"
)

type SurveyController struct {
	beego.Controller
}

type SurveySubmitResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

func (c *SurveyController) IntroPage() {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		c.Redirect("/login", 302)
		return
	}

	o := orm.NewOrm()
	var user models.User
	user.Id = userIDAny.(int)
	if err := o.Read(&user); err != nil {
		c.Redirect("/login", 302)
		return
	}

	// Redirect to review if already filled
	var existing models.StudentSurvey
	err := o.QueryTable(new(models.StudentSurvey)).Filter("User__Id", user.Id).One(&existing)
	if err == nil && existing.Id != 0 {
		c.Redirect("/survey/review", 302)
		return
	}

	c.Data["User"] = user
	c.TplName = "survey_intro.html"
}

func (c *SurveyController) NotePage() {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		c.Redirect("/login", 302)
		return
	}

	o := orm.NewOrm()
	var user models.User
	user.Id = userIDAny.(int)
	if err := o.Read(&user); err != nil {
		c.Redirect("/login", 302)
		return
	}

	// Redirect to review if already filled
	var existing models.StudentSurvey
	err := o.QueryTable(new(models.StudentSurvey)).Filter("User__Id", user.Id).One(&existing)
	if err == nil && existing.Id != 0 {
		c.Redirect("/survey/review", 302)
		return
	}

	c.Data["User"] = user
	c.TplName = "survey_note.html"
}

func (c *SurveyController) FormPage() {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		c.Redirect("/login", 302)
		return
	}

	o := orm.NewOrm()
	var user models.User
	user.Id = userIDAny.(int)
	if err := o.Read(&user); err != nil {
		c.Redirect("/login", 302)
		return
	}

	// Redirect to review if already filled
	var existing models.StudentSurvey
	err := o.QueryTable(new(models.StudentSurvey)).Filter("User__Id", user.Id).One(&existing)
	if err == nil && existing.Id != 0 {
		c.Redirect("/survey/review", 302)
		return
	}

	c.Data["User"] = user
	c.TplName = "survey_form.html"
}

func (c *SurveyController) ReviewPage() {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		c.Redirect("/login", 302)
		return
	}

	sessionRole := c.GetSession("user_role")
	roleStr, _ := sessionRole.(string)

	targetUserID := userIDAny.(int)
	studentIDStr := strings.TrimSpace(c.GetString("student_id"))

	if studentIDStr != "" {
		if roleStr != "sekolah" && roleStr != "admin" {
			c.Ctx.Output.SetStatus(403)
			c.Ctx.WriteString("Akses ditolak. Hanya sekolah/admin yang dapat melihat survei siswa lain.")
			return
		}
		uid, err := strconv.Atoi(studentIDStr)
		if err != nil || uid <= 0 {
			c.Ctx.Output.SetStatus(400)
			c.Ctx.WriteString("ID Siswa tidak valid.")
			return
		}
		targetUserID = uid
		c.Data["StudentId"] = targetUserID
	}

	o := orm.NewOrm()
	var user models.User
	user.Id = targetUserID
	if err := o.Read(&user); err != nil {
		c.Redirect("/login", 302)
		return
	}

	var survey models.StudentSurvey
	err := o.QueryTable(new(models.StudentSurvey)).Filter("User__Id", user.Id).One(&survey)
	if err != nil || survey.Id == 0 {
		if studentIDStr != "" {
			c.Ctx.Output.SetStatus(404)
			c.Ctx.WriteString("Siswa belum mengisi survei.")
			return
		}
		c.Redirect("/survey", 302)
		return
	}

	c.Data["User"] = user
	c.Data["Survey"] = survey
	c.Data["CanDownloadPDF"] = (roleStr == "sekolah" || roleStr == "admin")
	c.TplName = "survey_review.html"
}

func (c *SurveyController) SubmitAPI() {
	userIDAny := c.GetSession("user_id")
	if userIDAny == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = SurveySubmitResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return
	}

	var req struct {
		NamaPanggilan              string   `json:"nama_panggilan"`
		JalurMasuk                 string   `json:"jalur_masuk"`
		JalurMasukLain             string   `json:"jalur_masuk_lain"`
		Disabilitas                []string `json:"disabilitas"`
		NomorHp                    string   `json:"nomor_hp"`
		AsalSekolah                string   `json:"asal_sekolah"`
		TahunLulus                 string   `json:"tahun_lulus"`
		BahasaRumah                string   `json:"bahasa_rumah"`
		BahasaRumahLain            string   `json:"bahasa_rumah_lain"`
		SesuaiMinat                string   `json:"sesuai_minat"`
		AlasanMasuk                string   `json:"alasan_masuk"`
		AlasanMasukLain            string   `json:"alasan_masuk_lain"`
		PerasaanDiterima           string   `json:"perasaan_diterima"`
		MapelDisenangi             string   `json:"mapel_disenangi"`
		MapelMenantang             string   `json:"mapel_menantang"`
		BakatKeahlian              string   `json:"bakat_keahlian"`
		PengalamanSiswa            string   `json:"pengalaman_siswa"`
		
		EkskulDiminati             []string `json:"ekskul_diminati"`
		KebiasaanBelajar           []string `json:"kebiasaan_belajar"`
		KebiasaanBelajarLain       string   `json:"kebiasaan_belajar_lain"`
		DurasiBelajar              string   `json:"durasi_belajar"`
		CaraBelajarEfektif         []string `json:"cara_belajar_efektif"`
		CaraBelajarEfektifLain     string   `json:"cara_belajar_efektif_lain"`
		KesulitanBelajar           []string `json:"kesulitan_belajar"`
		KesulitanBelajarLain       string   `json:"kesulitan_belajar_lain"`
		ButuhBantuanBelajar        string   `json:"butuh_bantuan_belajar"`
		JenisBantuanBelajar        []string `json:"jenis_bantuan_belajar"`
		JenisBantuanBelajarLain    string   `json:"jenis_bantuan_belajar_lain"`
		KenyamananBelajar          string   `json:"kenyamanan_belajar"`
		AksesInternetPerangkat     string   `json:"akses_internet_perangkat"`
		AksesInternetPerangkatLain string   `json:"akses_internet_perangkat_lain"`
		RencanaSetelahLulus        string   `json:"rencana_setelah_lulus"`
		RencanaSetelahLulusLain    string   `json:"rencana_setelah_lulus_lain"`
		PekerjaanImpian            string   `json:"pekerjaan_impian"`
		JurusanDiminati            string   `json:"jurusan_diminati"`
		ButuhBimbinganKarier       string   `json:"butuh_bimbingan_karier"`
		DurasiLayar                string   `json:"durasi_layar"`
		AplikasiSeringDigunakan    []string `json:"aplikasi_sering_digunakan"`
		AplikasiSeringDigunakanLain string   `json:"aplikasi_sering_digunakan_lain"`
		AturanHpRumah              string   `json:"aturan_hp_rumah"`
		TinggalDengan              string   `json:"tinggal_dengan"`
		TinggalDenganLain          string   `json:"tinggal_dengan_lain"`
		PendampingBelajar          []string `json:"pendamping_belajar"`
		PendampingBelajarLain      string   `json:"pendamping_belajar_lain"`
		StatusOrangTua             string   `json:"status_orang_tua"`
		StatusTinggalOrangTua      string   `json:"status_tinggal_orang_tua"`
		StatusTinggalOrangTuaLain  string   `json:"status_tinggal_orang_tua_lain"`
		TanggungJawabRumah         []string `json:"tanggung_jawab_rumah"`
		TanggungJawabRumahLain     string   `json:"tanggung_jawab_rumah_lain"`
		PemenuhanKebutuhanBelajar  string   `json:"pemenuhan_kebutuhan_belajar"`
		FasilitasBelajar           []string `json:"fasilitas_belajar"`
		FasilitasBelajarLain       string   `json:"fasilitas_belajar_lain"`

		BantuanPendidikan          []string `json:"bantuan_pendidikan"`
		BantuanPendidikanLain      string   `json:"bantuan_pendidikan_lain"`
		ButuhInfoBantuan           string   `json:"butuh_info_bantuan"`
		HarapanOrangTua            string   `json:"harapan_orang_tua"`
		HarapanOrangTuaLain        string   `json:"harapan_orang_tua_lain"`
		DukunganOrangTua           string   `json:"dukungan_orang_tua"`
		KesehatanKhusus            string   `json:"kesehatan_khusus"`
		KesehatanKhususDetail      string   `json:"kesehatan_khusus_detail"`
		Alergi                     string   `json:"alergi"`
		AlergiDetail               string   `json:"alergi_detail"`
		PengobatanPanjang          string   `orm:"size(100)" json:"pengobatan_panjang"`
		PembatasanFisik            string   `orm:"size(100)" json:"pembatasan_fisik"`
		RawatRs                    string   `orm:"size(100)" json:"rawat_rs"`
		TingkatCemas               string   `orm:"size(100)" json:"tingkat_cemas"`
		TempatBercerita            []string `json:"tempat_bercerita"`
		TempatBerceritaLain        string   `json:"tempat_bercerita_lain"`
		RekomendasiProfesional     string   `json:"rekomendasi_profesional"`
		InteraksiSosial            string   `json:"interaksi_sosial"`
		KelebihanDiri              string   `json:"kelebihan_diri"`
		HobiMinat                  string   `json:"hobi_minat"`
		BidangDikembangkan         []string `json:"bidang_dikembangkan"`
		BidangDikembangkanLain     string   `json:"bidang_dikembangkan_lain"`
		AdaptasiLingkungan         string   `json:"adaptasi_lingkungan"`
		AdaptasiLingkunganLain     string   `json:"adaptasi_lingkungan_lain"`
		EkspresiEmosi              []string `json:"ekspresi_emosi"`
		EkspresiEmosiLain          string   `json:"ekspresi_emosi_lain"`
		PertemananMudah            []string `json:"pertemanan_mudah"`
		PertemananMudahLain        string   `json:"pertemanan_mudah_lain"`
		PertemananTantangan        []string `json:"pertemanan_tantangan"`
		PertemananTantanganLain    string   `json:"pertemanan_tantangan_lain"`
		TemanDekat                 string   `json:"teman_dekat"`
		NyamanSekolah              string   `json:"nyaman_sekolah"`
		TidakNyamanSekolah         string   `json:"tidak_nyaman_sekolah"`
		KemandirianJadwal          string   `json:"kemandirian_jadwal"`
		KesulitanBelajarTindakan   []string `json:"kesulitan_belajar_tindakan"`
		KesulitanBelajarTindakanLain string  `json:"kesulitan_belajar_tindakan_lain"`
		PesanUntukSekolah          string   `json:"pesan_untuk_sekolah"`
	}

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = SurveySubmitResponse{Success: false, Message: "Format data tidak valid"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	
	// Check if already filled
	var existing models.StudentSurvey
	err := o.QueryTable(new(models.StudentSurvey)).Filter("User__Id", userIDAny.(int)).One(&existing)
	
	survey := models.StudentSurvey{
		User:                       &models.User{Id: userIDAny.(int)},
		NamaPanggilan:              strings.TrimSpace(req.NamaPanggilan),
		JalurMasuk:                 strings.TrimSpace(req.JalurMasuk),
		JalurMasukLain:             strings.TrimSpace(req.JalurMasukLain),
		Disabilitas:                strings.Join(req.Disabilitas, ","),
		NomorHp:                    strings.TrimSpace(req.NomorHp),
		AsalSekolah:                strings.TrimSpace(req.AsalSekolah),
		TahunLulus:                 strings.TrimSpace(req.TahunLulus),
		BahasaRumah:                strings.TrimSpace(req.BahasaRumah),
		BahasaRumahLain:            strings.TrimSpace(req.BahasaRumahLain),
		SesuaiMinat:                strings.TrimSpace(req.SesuaiMinat),
		AlasanMasuk:                strings.TrimSpace(req.AlasanMasuk),
		AlasanMasukLain:            strings.TrimSpace(req.AlasanMasukLain),
		PerasaanDiterima:           strings.TrimSpace(req.PerasaanDiterima),
		MapelDisenangi:             strings.TrimSpace(req.MapelDisenangi),
		MapelMenantang:             strings.TrimSpace(req.MapelMenantang),
		BakatKeahlian:              strings.TrimSpace(req.BakatKeahlian),
		PengalamanSiswa:            strings.TrimSpace(req.PengalamanSiswa),
		
		EkskulDiminati:             strings.Join(req.EkskulDiminati, ","),
		KebiasaanBelajar:           strings.Join(req.KebiasaanBelajar, ","),
		KebiasaanBelajarLain:       strings.TrimSpace(req.KebiasaanBelajarLain),
		DurasiBelajar:              strings.TrimSpace(req.DurasiBelajar),
		CaraBelajarEfektif:         strings.Join(req.CaraBelajarEfektif, ","),
		CaraBelajarEfektifLain:     strings.TrimSpace(req.CaraBelajarEfektifLain),
		KesulitanBelajar:           strings.Join(req.KesulitanBelajar, ","),
		KesulitanBelajarLain:       strings.TrimSpace(req.KesulitanBelajarLain),
		ButuhBantuanBelajar:        strings.TrimSpace(req.ButuhBantuanBelajar),
		JenisBantuanBelajar:        strings.Join(req.JenisBantuanBelajar, ","),
		JenisBantuanBelajarLain:    strings.TrimSpace(req.JenisBantuanBelajarLain),
		KenyamananBelajar:          strings.TrimSpace(req.KenyamananBelajar),
		AksesInternetPerangkat:     strings.TrimSpace(req.AksesInternetPerangkat),
		AksesInternetPerangkatLain: strings.TrimSpace(req.AksesInternetPerangkatLain),
		RencanaSetelahLulus:        strings.TrimSpace(req.RencanaSetelahLulus),
		RencanaSetelahLulusLain:    strings.TrimSpace(req.RencanaSetelahLulusLain),
		PekerjaanImpian:            strings.TrimSpace(req.PekerjaanImpian),
		JurusanDiminati:            strings.TrimSpace(req.JurusanDiminati),
		ButuhBimbinganKarier:       strings.TrimSpace(req.ButuhBimbinganKarier),
		DurasiLayar:                strings.TrimSpace(req.DurasiLayar),
		AplikasiSeringDigunakan:    strings.Join(req.AplikasiSeringDigunakan, ","),
		AplikasiSeringDigunakanLain: strings.TrimSpace(req.AplikasiSeringDigunakanLain),
		AturanHpRumah:              strings.TrimSpace(req.AturanHpRumah),
		TinggalDengan:              strings.TrimSpace(req.TinggalDengan),
		TinggalDenganLain:          strings.TrimSpace(req.TinggalDenganLain),
		PendampingBelajar:          strings.Join(req.PendampingBelajar, ","),
		PendampingBelajarLain:      strings.TrimSpace(req.PendampingBelajarLain),
		StatusOrangTua:             strings.TrimSpace(req.StatusOrangTua),
		StatusTinggalOrangTua:      strings.TrimSpace(req.StatusTinggalOrangTua),
		StatusTinggalOrangTuaLain:  strings.TrimSpace(req.StatusTinggalOrangTuaLain),
		TanggungJawabRumah:         strings.Join(req.TanggungJawabRumah, ","),
		TanggungJawabRumahLain:     strings.TrimSpace(req.TanggungJawabRumahLain),
		PemenuhanKebutuhanBelajar:  strings.TrimSpace(req.PemenuhanKebutuhanBelajar),
		FasilitasBelajar:           strings.Join(req.FasilitasBelajar, ","),
		FasilitasBelajarLain:       strings.TrimSpace(req.FasilitasBelajarLain),

		BantuanPendidikan:          strings.Join(req.BantuanPendidikan, ","),
		BantuanPendidikanLain:      strings.TrimSpace(req.BantuanPendidikanLain),
		ButuhInfoBantuan:           strings.TrimSpace(req.ButuhInfoBantuan),
		HarapanOrangTua:            strings.TrimSpace(req.HarapanOrangTua),
		HarapanOrangTuaLain:        strings.TrimSpace(req.HarapanOrangTuaLain),
		DukunganOrangTua:           strings.TrimSpace(req.DukunganOrangTua),
		KesehatanKhusus:            strings.TrimSpace(req.KesehatanKhusus),
		KesehatanKhususDetail:      strings.TrimSpace(req.KesehatanKhususDetail),
		Alergi:                     strings.TrimSpace(req.Alergi),
		AlergiDetail:               strings.TrimSpace(req.AlergiDetail),
		PengobatanPanjang:          strings.TrimSpace(req.PengobatanPanjang),
		PembatasanFisik:            strings.TrimSpace(req.PembatasanFisik),
		RawatRs:                    strings.TrimSpace(req.RawatRs),
		TingkatCemas:               strings.TrimSpace(req.TingkatCemas),
		TempatBercerita:            strings.Join(req.TempatBercerita, ","),
		TempatBerceritaLain:        strings.TrimSpace(req.TempatBerceritaLain),
		RekomendasiProfesional:     strings.TrimSpace(req.RekomendasiProfesional),
		InteraksiSosial:            strings.TrimSpace(req.InteraksiSosial),
		KelebihanDiri:              strings.TrimSpace(req.KelebihanDiri),
		HobiMinat:                  strings.TrimSpace(req.HobiMinat),
		BidangDikembangkan:         strings.Join(req.BidangDikembangkan, ","),
		BidangDikembangkanLain:     strings.TrimSpace(req.BidangDikembangkanLain),
		AdaptasiLingkungan:         strings.TrimSpace(req.AdaptasiLingkungan),
		AdaptasiLingkunganLain:     strings.TrimSpace(req.AdaptasiLingkunganLain),
		EkspresiEmosi:              strings.Join(req.EkspresiEmosi, ","),
		EkspresiEmosiLain:          strings.TrimSpace(req.EkspresiEmosiLain),
		PertemananMudah:            strings.Join(req.PertemananMudah, ","),
		PertemananMudahLain:        strings.TrimSpace(req.PertemananMudahLain),
		PertemananTantangan:        strings.Join(req.PertemananTantangan, ","),
		PertemananTantanganLain:    strings.TrimSpace(req.PertemananTantanganLain),
		TemanDekat:                 strings.TrimSpace(req.TemanDekat),
		NyamanSekolah:              strings.TrimSpace(req.NyamanSekolah),
		TidakNyamanSekolah:         strings.TrimSpace(req.TidakNyamanSekolah),
		KemandirianJadwal:          strings.TrimSpace(req.KemandirianJadwal),
		KesulitanBelajarTindakan:   strings.Join(req.KesulitanBelajarTindakan, ","),
		KesulitanBelajarTindakanLain: strings.TrimSpace(req.KesulitanBelajarTindakanLain),
		PesanUntukSekolah:          strings.TrimSpace(req.PesanUntukSekolah),
	}

	if err == nil && existing.Id != 0 {
		// Update
		survey.Id = existing.Id
		survey.CreatedAt = existing.CreatedAt
		survey.UpdatedAt = time.Now()
		if _, err := o.Update(&survey); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = SurveySubmitResponse{Success: false, Message: "Gagal menyimpan survei: " + err.Error()}
			c.ServeJSON()
			return
		}
	} else {
		// Insert
		survey.CreatedAt = time.Now()
		survey.UpdatedAt = time.Now()
		if _, err := o.Insert(&survey); err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = SurveySubmitResponse{Success: false, Message: "Gagal menyimpan survei: " + err.Error()}
			c.ServeJSON()
			return
		}
	}

	c.Data["json"] = SurveySubmitResponse{Success: true, Message: "Survei berhasil disimpan"}
	c.ServeJSON()
}

func GenerateSurveyPDF(user *models.User, survey *models.StudentSurvey) (*gofpdf.Fpdf, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Hasil Survei Karakteristik Murid", false)
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 14)
	pdf.CellFormat(0, 8, "HASIL SURVEI PENGENALAN KARAKTERISTIK DAN KEBUTUHAN MURID", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "I", 9)
	pdf.CellFormat(0, 5, fmt.Sprintf("Dicetak pada: %s", time.Now().Format("02-01-2006 15:04")), "", 1, "C", false, 0, "")
	pdf.Ln(6)

	// Section Helper
	drawSectionHeader := func(title string) {
		pdf.SetFont("Arial", "B", 11)
		pdf.SetFillColor(245, 243, 255)
		pdf.CellFormat(0, 7, "  "+title, "1", 1, "L", true, 0, "")
		pdf.Ln(2)
	}

	drawRow := func(label, value string) {
		pdf.SetFont("Arial", "B", 9)
		pdf.CellFormat(72, 5, label, "", 0, "L", false, 0, "")
		pdf.CellFormat(3, 5, ":", "", 0, "L", false, 0, "")
		
		pdf.SetFont("Arial", "", 9)
		pdf.MultiCell(0, 5, value, "", "L", false)
		pdf.Ln(1)
	}

	// A. Latar Belakang Personal
	drawSectionHeader("A. LATAR BELAKANG PERSONAL")
	drawRow("Nama Lengkap", user.NamaLengkap)
	drawRow("Nama Panggilan", survey.NamaPanggilan)
	drawRow("NISN / NIP", user.NISN)
	drawRow("Kelas", user.Kelas)
	jalurStr := survey.JalurMasuk
	if survey.JalurMasukLain != "" {
		jalurStr += " (" + survey.JalurMasukLain + ")"
	}
	drawRow("Jalur Masuk", jalurStr)
	drawRow("Disabilitas/Kebutuhan Khusus", survey.Disabilitas)
	drawRow("Nomor HP", survey.NomorHp)
	drawRow("Asal Sekolah SMP", survey.AsalSekolah)
	drawRow("Tahun Lulus SMP", survey.TahunLulus)
	bahasaStr := survey.BahasaRumah
	if survey.BahasaRumahLain != "" {
		bahasaStr += " (" + survey.BahasaRumahLain + ")"
	}
	drawRow("Bahasa Komunikasi di Rumah", bahasaStr)
	pdf.Ln(4)

	// B. Akademik & Kebiasaan Belajar
	drawSectionHeader("B. AKADEMIK & KEBIASAAN BELAJAR")
	drawRow("Sesuai Minat ke SMA Ini", survey.SesuaiMinat)
	alasanStr := survey.AlasanMasuk
	if survey.AlasanMasukLain != "" {
		alasanStr += " (" + survey.AlasanMasukLain + ")"
	}
	drawRow("Alasan Masuk SMA", alasanStr)
	drawRow("Perasaan Diterima di Sekolah", survey.PerasaanDiterima)
	drawRow("Mata Pelajaran Paling Disenangi", survey.MapelDisenangi)
	drawRow("Mata Pelajaran Paling Menantang", survey.MapelMenantang)
	drawRow("Bakat/Keahlian", survey.BakatKeahlian)
	drawRow("Pengalaman Kegiatan/Organisasi", survey.PengalamanSiswa)
	drawRow("Ekstrakurikuler yang Diminati", survey.EkskulDiminati)
	kebiasaanStr := survey.KebiasaanBelajar
	if survey.KebiasaanBelajarLain != "" {
		kebiasaanStr += " (" + survey.KebiasaanBelajarLain + ")"
	}
	drawRow("Kebiasaan Belajar Luar Sekolah", kebiasaanStr)
	drawRow("Durasi Belajar Rata-rata", survey.DurasiBelajar)
	caraStr := survey.CaraBelajarEfektif
	if survey.CaraBelajarEfektifLain != "" {
		caraStr += " (" + survey.CaraBelajarEfektifLain + ")"
	}
	drawRow("Cara Belajar Paling Membantu", caraStr)
	kesulitanStr := survey.KesulitanBelajar
	if survey.KesulitanBelajarLain != "" {
		kesulitanStr += " (" + survey.KesulitanBelajarLain + ")"
	}
	drawRow("Hal Paling Menyulitkan Belajar", kesulitanStr)
	drawRow("Butuh Bantuan Memahami Materi", survey.ButuhBantuanBelajar)
	jenisBantuanStr := survey.JenisBantuanBelajar
	if survey.JenisBantuanBelajarLain != "" {
		jenisBantuanStr += " (" + survey.JenisBantuanBelajarLain + ")"
	}
	drawRow("Jenis Bantuan Belajar Diperlukan", jenisBantuanStr)
	drawRow("Kenyamanan Belajar di Rumah", survey.KenyamananBelajar)
	aksesInternetStr := survey.AksesInternetPerangkat
	if survey.AksesInternetPerangkatLain != "" {
		aksesInternetStr += " (" + survey.AksesInternetPerangkatLain + ")"
	}
	drawRow("Akses Internet & Perangkat", aksesInternetStr)
	rencanaStr := survey.RencanaSetelahLulus
	if survey.RencanaSetelahLulusLain != "" {
		rencanaStr += " (" + survey.RencanaSetelahLulusLain + ")"
	}
	drawRow("Rencana Setelah Lulus SMA", rencanaStr)
	drawRow("Pekerjaan Impian Masa Depan", survey.PekerjaanImpian)
	drawRow("Jurusan Kuliah Diminati", survey.JurusanDiminati)
	drawRow("Butuh Bimbingan Pilihan Karier", survey.ButuhBimbinganKarier)
	drawRow("Durasi Depan Layar Gadget", survey.DurasiLayar)
	appStr := survey.AplikasiSeringDigunakan
	if survey.AplikasiSeringDigunakanLain != "" {
		appStr += " (" + survey.AplikasiSeringDigunakanLain + ")"
	}
	drawRow("Aplikasi HP Sering Digunakan", appStr)
	drawRow("Aturan HP di Rumah", survey.AturanHpRumah)
	pdf.Ln(4)

	// C. Latar Belakang Keluarga
	drawSectionHeader("C. LATAR BELAKANG KELUARGA")
	tinggalStr := survey.TinggalDengan
	if survey.TinggalDenganLain != "" {
		tinggalStr += " (" + survey.TinggalDenganLain + ")"
	}
	drawRow("Tinggal Dengan Siapa", tinggalStr)
	pendampingStr := survey.PendampingBelajar
	if survey.PendampingBelajarLain != "" {
		pendampingStr += " (" + survey.PendampingBelajarLain + ")"
	}
	drawRow("Pendamping Utama Sekolah", pendampingStr)
	drawRow("Status Orang Tua Kandung", survey.StatusOrangTua)
	statusTinggalStr := survey.StatusTinggalOrangTua
	if survey.StatusTinggalOrangTuaLain != "" {
		statusTinggalStr += " (" + survey.StatusTinggalOrangTuaLain + ")"
	}
	drawRow("Status Tinggal Orang Tua", statusTinggalStr)
	tanggungJawabStr := survey.TanggungJawabRumah
	if survey.TanggungJawabRumahLain != "" {
		tanggungJawabStr += " (" + survey.TanggungJawabRumahLain + ")"
	}
	drawRow("Tanggung Jawab di Rumah", tanggungJawabStr)
	drawRow("Pemenuhan Kebutuhan Belajar", survey.PemenuhanKebutuhanBelajar)
	fasilitasStr := survey.FasilitasBelajar
	if survey.FasilitasBelajarLain != "" {
		fasilitasStr += " (" + survey.FasilitasBelajarLain + ")"
	}
	drawRow("Fasilitas Belajar Tersedia", fasilitasStr)
	bantuanPendidikanStr := survey.BantuanPendidikan
	if survey.BantuanPendidikanLain != "" {
		bantuanPendidikanStr += " (" + survey.BantuanPendidikanLain + ")"
	}
	drawRow("Pernah Bantuan Pendidikan SMP", bantuanPendidikanStr)
	drawRow("Butuh Info Beasiswa/Bantuan", survey.ButuhInfoBantuan)
	harapanOrangTuaStr := survey.HarapanOrangTua
	if survey.HarapanOrangTuaLain != "" {
		harapanOrangTuaStr += " (" + survey.HarapanOrangTuaLain + ")"
	}
	drawRow("Harapan Orang Tua Setelah SMA", harapanOrangTuaStr)
	drawRow("Dukungan Orang Tua Rencana Masa Depan", survey.DukunganOrangTua)
	pdf.Ln(4)

	// D. Kesehatan
	drawSectionHeader("D. KESEHATAN")
	drawRow("Kondisi Kesehatan Khusus", survey.KesehatanKhusus)
	if survey.KesehatanKhusus == "Ya" {
		drawRow("Detail Kesehatan", survey.KesehatanKhususDetail)
	}
	drawRow("Memiliki Alergi", survey.Alergi)
	if survey.Alergi == "Ya" {
		drawRow("Detail Alergi", survey.AlergiDetail)
	}
	drawRow("Pengobatan Jangka Panjang", survey.PengobatanPanjang)
	drawRow("Pembatasan Aktivitas Fisik", survey.PembatasanFisik)
	drawRow("Rawat RS 1 Tahun Terakhir", survey.RawatRs)
	drawRow("Tingkat Cemas/Tertekan (2 Minggu)", survey.TingkatCemas)
	tempatBerceritaStr := survey.TempatBercerita
	if survey.TempatBerceritaLain != "" {
		tempatBerceritaStr += " (" + survey.TempatBerceritaLain + ")"
	}
	drawRow("Tempat Bercerita Masalah", tempatBerceritaStr)
	drawRow("Pernah Pendampingan Profesional", survey.RekomendasiProfesional)
	pdf.Ln(4)

	// E. Karakteristik Personal
	drawSectionHeader("E. KARAKTERISTIK PERSONAL")
	drawRow("Kecenderungan Interaksi Sosial", survey.InteraksiSosial)
	drawRow("Kelebihan / Kekuatan Diri", survey.KelebihanDiri)
	drawRow("Hobi / Minat", survey.HobiMinat)
	bidangStr := survey.BidangDikembangkan
	if survey.BidangDikembangkanLain != "" {
		bidangStr += " (" + survey.BidangDikembangkanLain + ")"
	}
	drawRow("Bidang Ingin Dikembangkan", bidangStr)
	adaptasiStr := survey.AdaptasiLingkungan
	if survey.AdaptasiLingkunganLain != "" {
		adaptasiStr += " (" + survey.AdaptasiLingkunganLain + ")"
	}
	drawRow("Adaptasi Lingkungan Baru", adaptasiStr)
	ekspresiStr := survey.EkspresiEmosi
	if survey.EkspresiEmosiLain != "" {
		ekspresiStr += " (" + survey.EkspresiEmosiLain + ")"
	}
	drawRow("Cara Mengekspresikan Emosi", ekspresiStr)
	mudahStr := survey.PertemananMudah
	if survey.PertemananMudahLain != "" {
		mudahStr += " (" + survey.PertemananMudahLain + ")"
	}
	drawRow("Hal Mudah dalam Pertemanan", mudahStr)
	tantanganStr := survey.PertemananTantangan
	if survey.PertemananTantanganLain != "" {
		tantanganStr += " (" + survey.PertemananTantanganLain + ")"
	}
	drawRow("Hal Menantang dalam Pertemanan", tantanganStr)
	drawRow("Memiliki Teman Dekat di Sekolah", survey.TemanDekat)
	drawRow("Hal Membuat Nyaman/Semangat Belajar", survey.NyamanSekolah)
	drawRow("Hal Membuat Tidak Nyaman/Sulit Fokus", survey.TidakNyamanSekolah)
	drawRow("Kemandirian Mengatur Jadwal/Tugas", survey.KemandirianJadwal)
	tindakanStr := survey.KesulitanBelajarTindakan
	if survey.KesulitanBelajarTindakanLain != "" {
		tindakanStr += " (" + survey.KesulitanBelajarTindakanLain + ")"
	}
	drawRow("Tindakan Saat Kesulitan Belajar", tindakanStr)
	drawRow("Pesan Penting untuk Sekolah/BK", survey.PesanUntukSekolah)
	pdf.Ln(8)

	// Ensure the entire signature block fits on one page (approx 50mm height needed)
	if pdf.GetY() > 230 {
		pdf.AddPage()
	}

	// Signature Statement
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(0, 5, "Saya menyatakan bahwa data yang diisi dalam formulir ini benar dan dapat dipertanggungjawabkan.", "", "L", false)
	pdf.Ln(4)

	dateStr := time.Now().Format("02 January 2006")
	months := map[string]string{
		"January": "Januari", "February": "Februari", "March": "Maret", "April": "April",
		"May": "Mei", "June": "Juni", "July": "Juli", "August": "Agustus",
		"September": "September", "October": "Oktober", "November": "November", "December": "Desember",
	}
	for en, id := range months {
		dateStr = strings.Replace(dateStr, en, id, 1)
	}

	location := survey.AsalSekolah
	if location == "" {
		location = "......................"
	}

	pdf.CellFormat(120, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, location+", "+dateStr, "", 1, "R", false, 0, "")
	pdf.CellFormat(120, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, "TTD Murid", "", 1, "R", false, 0, "")
	pdf.Ln(18)

	pdf.CellFormat(120, 5, "", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 5, "( "+user.NamaLengkap+" )", "", 1, "R", false, 0, "")

	return pdf, nil
}

func (c *SurveyController) ExportPDF() {
	o := orm.NewOrm()

	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login", 302)
		return
	}
	loggedInUserID := sessionUser.(int)

	sessionRole := c.GetSession("user_role")
	roleStr, _ := sessionRole.(string)

	if roleStr != "sekolah" && roleStr != "admin" {
		c.Ctx.Output.SetStatus(403)
		c.Ctx.WriteString("Akses ditolak. Unduh PDF hanya diperbolehkan untuk akun Sekolah / Guru BK.")
		return
	}

	targetUserID := loggedInUserID
	studentIDStr := strings.TrimSpace(c.GetString("student_id"))

	if studentIDStr == "" {
		c.Ctx.Output.SetStatus(400)
		c.Ctx.WriteString("ID Siswa wajib disertakan.")
		return
	}

	uid, err := strconv.Atoi(studentIDStr)
	if err != nil || uid <= 0 {
		c.Ctx.Output.SetStatus(400)
		c.Ctx.WriteString("ID Siswa tidak valid.")
		return
	}
	targetUserID = uid

	var user models.User
	user.Id = targetUserID
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Ctx.WriteString("Data siswa tidak ditemukan.")
		return
	}

	var survey models.StudentSurvey
	err = o.QueryTable(new(models.StudentSurvey)).Filter("User__Id", targetUserID).One(&survey)
	if err != nil || survey.Id == 0 {
		c.Ctx.Output.SetStatus(404)
		c.Ctx.WriteString("Siswa belum mengisi survei ini.")
		return
	}

	pdf, err := GenerateSurveyPDF(&user, &survey)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Ctx.WriteString("Gagal menghasilkan PDF: " + err.Error())
		return
	}

	// Set download headers
	fileName := fmt.Sprintf("Survei_Karakteristik_%s.pdf", strings.ReplaceAll(user.NamaLengkap, " ", "_"))
	c.Ctx.Output.Header("Content-Type", "application/pdf")
	c.Ctx.Output.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", url.QueryEscape(fileName)))

	err = pdf.Output(c.Ctx.ResponseWriter)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Ctx.WriteString("Gagal mengalirkan PDF: " + err.Error())
	}
}

func (c *SurveyController) ExportAllZIP() {
	o := orm.NewOrm()

	sessionUser := c.GetSession("user_id")
	if sessionUser == nil {
		c.Redirect("/login", 302)
		return
	}
	loggedInUserID := sessionUser.(int)

	sessionRole := c.GetSession("user_role")
	roleStr, _ := sessionRole.(string)

	if roleStr != "sekolah" && roleStr != "admin" {
		c.Ctx.Output.SetStatus(403)
		c.Ctx.WriteString("Akses ditolak.")
		return
	}

	sekolahName := ""
	if roleStr == "sekolah" {
		var schoolUser models.User
		schoolUser.Id = loggedInUserID
		if err := o.Read(&schoolUser); err == nil {
			sekolahName = schoolUser.Sekolah
		}
	} else {
		// Admin can filter by school query parameter
		sekolahName = c.GetString("sekolah")
	}

	if sekolahName == "" && roleStr == "sekolah" {
		c.Ctx.Output.SetStatus(400)
		c.Ctx.WriteString("Sekolah tidak terkonfigurasi untuk akun Anda.")
		return
	}

	var surveys []models.StudentSurvey
	qs := o.QueryTable(new(models.StudentSurvey)).RelatedSel("User")
	if sekolahName != "" {
		qs = qs.Filter("User__Sekolah", sekolahName)
	}
	_, err := qs.OrderBy("User__NamaLengkap").All(&surveys)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Ctx.WriteString("Gagal mengambil data survei: " + err.Error())
		return
	}

	if len(surveys) == 0 {
		c.Ctx.Output.SetStatus(404)
		c.Ctx.WriteString("Belum ada data survei yang diisi oleh siswa.")
		return
	}

	// Create zip archive in memory
	var zipBuf bytes.Buffer
	zipWriter := zip.NewWriter(&zipBuf)

	for _, s := range surveys {
		if s.User == nil {
			continue
		}
		
		pdf, err := GenerateSurveyPDF(s.User, &s)
		if err != nil {
			continue
		}

		// Add PDF file inside ZIP
		fileName := fmt.Sprintf("%s_hasil-survei.pdf", strings.ReplaceAll(s.User.NamaLengkap, " ", "_"))
		f, err := zipWriter.Create(fileName)
		if err != nil {
			continue
		}
		err = pdf.Output(f)
		if err != nil {
			continue
		}
	}

	err = zipWriter.Close()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Ctx.WriteString("Gagal membuat file ZIP: " + err.Error())
		return
	}

	// Set ZIP download headers
	zipName := "Semua_Hasil_Survei_Karakteristik_Siswa.zip"
	if sekolahName != "" {
		zipName = fmt.Sprintf("Hasil_Survei_Karakteristik_%s.zip", strings.ReplaceAll(sekolahName, " ", "_"))
	}
	c.Ctx.Output.Header("Content-Type", "application/zip")
	c.Ctx.Output.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", url.QueryEscape(zipName)))
	c.Ctx.ResponseWriter.Write(zipBuf.Bytes())
}
