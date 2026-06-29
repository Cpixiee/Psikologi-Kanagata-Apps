package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

type StudentSurvey struct {
	Id                         int       `orm:"auto;pk" json:"id"`
	User                       *User     `orm:"rel(fk)" json:"user"`
	
	// A. Latar Belakang Personal
	NamaPanggilan              string    `orm:"size(100)" json:"nama_panggilan"`
	JalurMasuk                 string    `orm:"size(100)" json:"jalur_masuk"`
	JalurMasukLain             string    `orm:"size(255);null" json:"jalur_masuk_lain"`
	Disabilitas                string    `orm:"type(text)" json:"disabilitas"` // comma-separated
	NomorHp                    string    `orm:"size(30)" json:"nomor_hp"`
	AsalSekolah                string    `orm:"size(255)" json:"asal_sekolah"`
	TahunLulus                 string    `orm:"size(10)" json:"tahun_lulus"`
	BahasaRumah                string    `orm:"size(100)" json:"bahasa_rumah"`
	BahasaRumahLain            string    `orm:"size(255);null" json:"bahasa_rumah_lain"`
	
	// B. Akademik & Minat/Belajar
	SesuaiMinat                string    `orm:"size(50)" json:"sesuai_minat"`
	AlasanMasuk                string    `orm:"size(100)" json:"alasan_masuk"`
	AlasanMasukLain            string    `orm:"size(255);null" json:"alasan_masuk_lain"`
	PerasaanDiterima           string    `orm:"size(50)" json:"perasaan_diterima"`
	MapelDisenangi             string    `orm:"type(text)" json:"mapel_disenangi"`
	MapelMenantang             string    `orm:"type(text)" json:"mapel_menantang"`
	BakatKeahlian              string    `orm:"type(text)" json:"bakat_keahlian"`
	PengalamanSiswa            string    `orm:"type(text)" json:"pengalaman_siswa"`
	
	EkskulDiminati             string    `orm:"type(text)" json:"ekskul_diminati"` // comma-separated
	KebiasaanBelajar           string    `orm:"type(text)" json:"kebiasaan_belajar"` // comma-separated
	KebiasaanBelajarLain       string    `orm:"size(255);null" json:"kebiasaan_belajar_lain"`
	DurasiBelajar              string    `orm:"size(100)" json:"durasi_belajar"`
	CaraBelajarEfektif         string    `orm:"type(text)" json:"cara_belajar_efektif"` // comma-separated
	CaraBelajarEfektifLain     string    `orm:"size(255);null" json:"cara_belajar_efektif_lain"`
	KesulitanBelajar           string    `orm:"type(text)" json:"kesulitan_belajar"` // comma-separated
	KesulitanBelajarLain       string    `orm:"size(255);null" json:"kesulitan_belajar_lain"`
	ButuhBantuanBelajar        string    `orm:"size(100)" json:"butuh_bantuan_belajar"`
	JenisBantuanBelajar        string    `orm:"type(text)" json:"jenis_bantuan_belajar"` // comma-separated
	JenisBantuanBelajarLain    string    `orm:"size(255);null" json:"jenis_bantuan_belajar_lain"`
	KenyamananBelajar          string    `orm:"size(100)" json:"kenyamanan_belajar"`
	AksesInternetPerangkat     string    `orm:"size(255)" json:"akses_internet_perangkat"`
	AksesInternetPerangkatLain string    `orm:"size(255);null" json:"akses_internet_perangkat_lain"`
	
	// Career / College / Digital
	RencanaSetelahLulus        string    `orm:"size(255)" json:"rencana_setelah_lulus"`
	RencanaSetelahLulusLain    string    `orm:"size(255);null" json:"rencana_setelah_lulus_lain"`
	PekerjaanImpian            string    `orm:"type(text)" json:"pekerjaan_impian"`
	JurusanDiminati            string    `orm:"type(text)" json:"jurusan_diminati"`
	ButuhBimbinganKarier       string    `orm:"size(100)" json:"butuh_bimbingan_karier"`
	DurasiLayar                string    `orm:"size(100)" json:"durasi_layar"`
	AplikasiSeringDigunakan    string    `orm:"type(text)" json:"aplikasi_sering_digunakan"` // comma-separated
	AplikasiSeringDigunakanLain string   `orm:"size(255);null" json:"aplikasi_sering_digunakan_lain"`
	AturanHpRumah              string    `orm:"size(255)" json:"aturan_hp_rumah"`
	
	// C. Keluarga
	TinggalDengan              string    `orm:"size(100)" json:"tinggal_dengan"`
	TinggalDenganLain          string    `orm:"size(255);null" json:"tinggal_dengan_lain"`
	PendampingBelajar          string    `orm:"type(text)" json:"pendamping_belajar"` // comma-separated
	PendampingBelajarLain      string    `orm:"size(255);null" json:"pendamping_belajar_lain"`
	StatusOrangTua             string    `orm:"size(100)" json:"status_orang_tua"`
	StatusTinggalOrangTua      string    `orm:"size(255)" json:"status_tinggal_orang_tua"`
	StatusTinggalOrangTuaLain  string    `orm:"size(255);null" json:"status_tinggal_orang_tua_lain"`
	TanggungJawabRumah         string    `orm:"type(text)" json:"tanggung_jawab_rumah"` // comma-separated
	TanggungJawabRumahLain     string    `orm:"size(255);null" json:"tanggung_jawab_rumah_lain"`
	PemenuhanKebutuhanBelajar  string    `orm:"size(255)" json:"pemenuhan_kebutuhan_belajar"`
	FasilitasBelajar           string    `orm:"type(text)" json:"fasilitas_belajar"` // comma-separated
	FasilitasBelajarLain       string    `orm:"size(255);null" json:"fasilitas_belajar_lain"`

	BantuanPendidikan          string    `orm:"type(text)" json:"bantuan_pendidikan"` // comma-separated
	BantuanPendidikanLain      string    `orm:"size(255);null" json:"bantuan_pendidikan_lain"`
	ButuhInfoBantuan           string    `orm:"size(100)" json:"butuh_info_bantuan"`
	HarapanOrangTua            string    `orm:"size(255)" json:"harapan_orang_tua"`
	HarapanOrangTuaLain        string    `orm:"size(255);null" json:"harapan_orang_tua_lain"`
	DukunganOrangTua           string    `orm:"size(100)" json:"dukungan_orang_tua"`
	
	// D. Kesehatan
	KesehatanKhusus            string    `orm:"size(100)" json:"kesehatan_khusus"`
	KesehatanKhususDetail      string    `orm:"type(text)" json:"kesehatan_khusus_detail"`
	Alergi                     string    `orm:"size(100)" json:"alergi"`
	AlergiDetail               string    `orm:"type(text)" json:"alergi_detail"`
	PengobatanPanjang          string    `orm:"size(100)" json:"pengobatan_panjang"`
	PembatasanFisik            string    `orm:"size(100)" json:"pembatasan_fisik"`
	RawatRs                    string    `orm:"size(100)" json:"rawat_rs"`
	TingkatCemas               string    `orm:"size(100)" json:"tingkat_cemas"`
	TempatBercerita            string    `orm:"type(text)" json:"tempat_bercerita"` // comma-separated
	TempatBerceritaLain        string    `orm:"size(255);null" json:"tempat_bercerita_lain"`
	RekomendasiProfesional     string    `orm:"size(100)" json:"rekomendasi_profesional"`
	
	// E. Karakteristik Personal
	InteraksiSosial            string    `orm:"size(100)" json:"interaksi_sosial"`
	KelebihanDiri              string    `orm:"type(text)" json:"kelebihan_diri"`
	HobiMinat                  string    `orm:"type(text)" json:"hobi_minat"`
	BidangDikembangkan         string    `orm:"type(text)" json:"bidang_dikembangkan"` // comma-separated
	BidangDikembangkanLain     string    `orm:"size(255);null" json:"bidang_dikembangkan_lain"`
	AdaptasiLingkungan         string    `orm:"size(100)" json:"adaptasi_lingkungan"`
	AdaptasiLingkunganLain     string    `orm:"size(255);null" json:"adaptasi_lingkungan_lain"`
	EkspresiEmosi              string    `orm:"type(text)" json:"ekspresi_emosi"` // comma-separated
	EkspresiEmosiLain          string    `orm:"size(255);null" json:"ekspresi_emosi_lain"`
	PertemananMudah            string    `orm:"type(text)" json:"pertemanan_mudah"` // comma-separated
	PertemananMudahLain        string    `orm:"size(255);null" json:"pertemanan_mudah_lain"`
	PertemananTantangan        string    `orm:"type(text)" json:"pertemanan_tantangan"` // comma-separated
	PertemananTantanganLain    string    `orm:"size(255);null" json:"pertemanan_tantangan_lain"`
	TemanDekat                 string    `orm:"size(100)" json:"teman_dekat"`
	NyamanSekolah              string    `orm:"type(text)" json:"nyaman_sekolah"`
	TidakNyamanSekolah         string    `orm:"type(text)" json:"tidak_nyaman_sekolah"`
	KemandirianJadwal          string    `orm:"size(100)" json:"kemandirian_jadwal"`
	KesulitanBelajarTindakan   string    `orm:"type(text)" json:"kesulitan_belajar_tindakan"` // comma-separated
	KesulitanBelajarTindakanLain string  `orm:"size(255);null" json:"kesulitan_belajar_tindakan_lain"`
	PesanUntukSekolah          string    `orm:"type(text)" json:"pesan_untuk_sekolah"`

	CreatedAt                  time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
	UpdatedAt                  time.Time `orm:"auto_now;type(datetime)" json:"updated_at"`
}

func (s *StudentSurvey) TableName() string {
	return "student_surveys"
}

// EnsureSurveyTables is a helper to verify or build the table schema.
func EnsureSurveyTables() error {
	o := orm.NewOrm()
	_, err := o.Raw(`
		CREATE TABLE IF NOT EXISTS student_surveys (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			nama_panggilan VARCHAR(100) NOT NULL DEFAULT '',
			jalur_masuk VARCHAR(100) NOT NULL DEFAULT '',
			jalur_masuk_lain VARCHAR(255) DEFAULT '',
			disabilitas TEXT NOT NULL DEFAULT '',
			nomor_hp VARCHAR(30) NOT NULL DEFAULT '',
			asal_sekolah VARCHAR(255) NOT NULL DEFAULT '',
			tahun_lulus VARCHAR(10) NOT NULL DEFAULT '',
			bahasa_rumah VARCHAR(100) NOT NULL DEFAULT '',
			bahasa_rumah_lain VARCHAR(255) DEFAULT '',
			sesuai_minat VARCHAR(50) NOT NULL DEFAULT '',
			alasan_masuk VARCHAR(100) NOT NULL DEFAULT '',
			alasan_masuk_lain VARCHAR(255) DEFAULT '',
			perasaan_diterima VARCHAR(50) NOT NULL DEFAULT '',
			mapel_disenangi TEXT NOT NULL DEFAULT '',
			mapel_menantang TEXT NOT NULL DEFAULT '',
			bakat_keahlian TEXT NOT NULL DEFAULT '',
			pengalaman_siswa TEXT NOT NULL DEFAULT '',
			
			ekskul_diminati TEXT NOT NULL DEFAULT '',
			kebiasaan_belajar TEXT NOT NULL DEFAULT '',
			kebiasaan_belajar_lain VARCHAR(255) DEFAULT '',
			durasi_belajar VARCHAR(100) NOT NULL DEFAULT '',
			cara_belajar_efektif TEXT NOT NULL DEFAULT '',
			cara_belajar_efektif_lain VARCHAR(255) DEFAULT '',
			kesulitan_belajar TEXT NOT NULL DEFAULT '',
			kesulitan_belajar_lain VARCHAR(255) DEFAULT '',
			butuh_bantuan_belajar VARCHAR(100) NOT NULL DEFAULT '',
			jenis_bantuan_belajar TEXT NOT NULL DEFAULT '',
			jenis_bantuan_belajar_lain VARCHAR(255) DEFAULT '',
			kenyamanan_belajar VARCHAR(100) NOT NULL DEFAULT '',
			akses_internet_perangkat VARCHAR(255) NOT NULL DEFAULT '',
			akses_internet_perangkat_lain VARCHAR(255) DEFAULT '',
			rencana_setelah_lulus VARCHAR(255) NOT NULL DEFAULT '',
			rencana_setelah_lulus_lain VARCHAR(255) DEFAULT '',
			pekerjaan_impian TEXT NOT NULL DEFAULT '',
			jurusan_diminati TEXT NOT NULL DEFAULT '',
			butuh_bimbingan_karier VARCHAR(100) NOT NULL DEFAULT '',
			durasi_layar VARCHAR(100) NOT NULL DEFAULT '',
			aplikasi_sering_digunakan TEXT NOT NULL DEFAULT '',
			aplikasi_sering_digunakan_lain VARCHAR(255) DEFAULT '',
			aturan_hp_rumah VARCHAR(255) NOT NULL DEFAULT '',
			tinggal_dengan VARCHAR(100) NOT NULL DEFAULT '',
			tinggal_dengan_lain VARCHAR(255) DEFAULT '',
			pendamping_belajar TEXT NOT NULL DEFAULT '',
			pendamping_belajar_lain VARCHAR(255) DEFAULT '',
			status_orang_tua VARCHAR(100) NOT NULL DEFAULT '',
			status_tinggal_orang_tua VARCHAR(255) NOT NULL DEFAULT '',
			status_tinggal_orang_tua_lain VARCHAR(255) DEFAULT '',
			tanggung_jawab_rumah TEXT NOT NULL DEFAULT '',
			tanggung_jawab_rumah_lain VARCHAR(255) DEFAULT '',
			pemenuhan_kebutuhan_belajar VARCHAR(255) NOT NULL DEFAULT '',
			fasilitas_belajar TEXT NOT NULL DEFAULT '',
			fasilitas_belajar_lain VARCHAR(255) DEFAULT '',

			bantuan_pendidikan TEXT NOT NULL DEFAULT '',
			bantuan_pendidikan_lain VARCHAR(255) DEFAULT '',
			butuh_info_bantuan VARCHAR(100) NOT NULL DEFAULT '',
			harapan_orang_tua VARCHAR(255) DEFAULT '',
			harapan_orang_tua_lain VARCHAR(255) DEFAULT '',
			dukungan_orang_tua VARCHAR(100) DEFAULT '',
			kesehatan_khusus VARCHAR(100) DEFAULT '',
			kesehatan_khusus_detail TEXT DEFAULT '',
			alergi VARCHAR(100) DEFAULT '',
			alergi_detail TEXT DEFAULT '',
			pengobatan_panjang VARCHAR(100) DEFAULT '',
			pembatasan_fisik VARCHAR(100) DEFAULT '',
			rawat_rs VARCHAR(100) DEFAULT '',
			tingkat_cemas VARCHAR(100) DEFAULT '',
			tempat_bercerita TEXT DEFAULT '',
			tempat_bercerita_lain VARCHAR(255) DEFAULT '',
			rekomendasi_profesional VARCHAR(100) DEFAULT '',
			interaksi_sosial VARCHAR(100) DEFAULT '',
			kelebihan_diri TEXT DEFAULT '',
			hobi_minat TEXT DEFAULT '',
			bidang_dikembangkan TEXT DEFAULT '',
			bidang_dikembangkan_lain VARCHAR(255) DEFAULT '',
			adaptasi_lingkungan VARCHAR(100) DEFAULT '',
			adaptasi_lingkungan_lain VARCHAR(255) DEFAULT '',
			ekspresi_emosi TEXT DEFAULT '',
			ekspresi_emosi_lain VARCHAR(255) DEFAULT '',
			pertemanan_mudah TEXT DEFAULT '',
			pertemanan_mudah_lain VARCHAR(255) DEFAULT '',
			pertemanan_tantangan TEXT DEFAULT '',
			pertemanan_tantangan_lain VARCHAR(255) DEFAULT '',
			teman_dekat VARCHAR(100) DEFAULT '',
			nyaman_sekolah TEXT DEFAULT '',
			tidak_nyaman_sekolah TEXT DEFAULT '',
			kemandirian_jadwal VARCHAR(100) DEFAULT '',
			kesulitan_belajar_tindakan TEXT DEFAULT '',
			kesulitan_belajar_tindakan_lain VARCHAR(255) DEFAULT '',
			pesan_untuk_sekolah TEXT DEFAULT '',
			
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		);
	`).Exec()
	return err
}

func init() {
	orm.RegisterModel(new(StudentSurvey))
}
