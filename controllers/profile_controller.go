package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"psikologi_apps/models"
	"psikologi_apps/seeds"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	"github.com/beego/beego/v2/core/logs"
	beego "github.com/beego/beego/v2/server/web"
	"golang.org/x/image/draw"
)

type ProfileController struct {
	beego.Controller
}

type ProfileResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// TestResultSummary merangkum hasil tes IST & Holland per undangan untuk halaman profil.
type TestResultSummary struct {
	InvitationID int       `json:"invitation_id"`
	BatchName    string    `json:"batch_name"`
	CreatedAt    time.Time `json:"created_at"`
	HasIST       bool      `json:"has_ist"`
	HasHolland   bool      `json:"has_holland"`
	HasLearningStyle bool  `json:"has_learning_style"`
	ISTIQ        int       `json:"ist_iq,omitempty"`
	ISTIQCat     string    `json:"ist_iq_category,omitempty"`
}

// @router /api/profile [get]
func (c *ProfileController) GetProfile() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	user := models.User{Id: userID.(int)}
	err := o.Read(&user)
	if err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "User tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	teacherName := ""
	if teacherIDVal := c.GetSession("teacher_id"); teacherIDVal != nil {
		if tid, ok := teacherIDVal.(int); ok && tid > 0 {
			var t models.SchoolTeacher
			if o.QueryTable(new(models.SchoolTeacher)).Filter("Id", tid).One(&t) == nil {
				teacherName = t.Nama
			}
		}
	}

	if user.Role == models.RoleSekolah {
		user.Kelas = user.Sekolah
		user.Jurusan = "BK"
	}

	isImpersonated := c.GetSession("impersonator_user_id") != nil

	type ProfileData struct {
		models.User
		TeacherName    string `json:"teacher_name,omitempty"`
		IsImpersonated bool   `json:"is_impersonated"`
	}

	c.Data["json"] = ProfileResponse{
		Success: true,
		Data: ProfileData{
			User:           user,
			TeacherName:    teacherName,
			IsImpersonated: isImpersonated,
		},
	}
	c.ServeJSON()
}

// @router /api/profile [put]
func (c *ProfileController) UpdateProfile() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	var updateData struct {
		NamaLengkap  string `json:"nama_lengkap"`
		Email        string `json:"email"`
		NoHandphone  string `json:"no_handphone"`
		AsalInstansi string `json:"asal_instansi"`
		JenisKelamin string `json:"jenis_kelamin"`
		Alamat       string `json:"alamat"`
		Kota         string `json:"kota"`
		Provinsi     string `json:"provinsi"`
		Kodepos      string `json:"kodepos"`
		// Onboarding fields (semua opsional di endpoint generic UpdateProfile)
		NISN         string `json:"nisn"`
		NIP          string `json:"nip"`
		Kelas        string `json:"kelas"`
		Jurusan      string `json:"jurusan"`
		TempatLahir  string `json:"tempat_lahir"`
		TanggalLahir string `json:"tanggal_lahir"` // ISO YYYY-MM-DD
		Kecamatan    string `json:"kecamatan"`

		// Detailed Student Identity Fields
		NoNIK            string `json:"no_nik"`
		NoKK             string `json:"no_kk"`
		NomorAktaLahir   string `json:"nomor_akta_lahir"`
		Agama            string `json:"agama"`
		TempatTinggal    string `json:"tempat_tinggal"`
		ModeTransportasi string `json:"mode_transportasi"`
		AnakKe           int    `json:"anak_ke"`
		JumlahBersaudara int    `json:"jumlah_bersaudara"`
		RiwayatPenyakit  string `json:"riwayat_penyakit"`
	}

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &updateData); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Format data tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	user := models.User{Id: userID.(int)}
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "User tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	// Check if email is already taken by another user
	if updateData.Email != user.Email {
		var existingUser models.User
		err := o.QueryTable("users").Filter("email", updateData.Email).Exclude("id", userID.(int)).One(&existingUser)
		if err == nil {
			c.Ctx.Output.SetStatus(400)
			c.Data["json"] = ProfileResponse{
				Success: false,
				Message: "Email sudah digunakan",
			}
			c.ServeJSON()
			return
		}
	}

	// Update user data
	user.NamaLengkap = updateData.NamaLengkap
	user.Email = updateData.Email
	user.NoHandphone = updateData.NoHandphone
	user.AsalInstansi = updateData.AsalInstansi
	user.JenisKelamin = models.Gender(updateData.JenisKelamin)
	user.Alamat = updateData.Alamat
	user.Kota = updateData.Kota
	user.Provinsi = updateData.Provinsi
	user.Kodepos = updateData.Kodepos
	user.NISN = updateData.NISN
	user.NIP = updateData.NIP
	user.Kelas = updateData.Kelas
	user.Jurusan = updateData.Jurusan
	user.TempatLahir = updateData.TempatLahir
	user.Kecamatan = updateData.Kecamatan
	user.NoNIK = updateData.NoNIK
	user.NoKK = updateData.NoKK
	user.NomorAktaLahir = updateData.NomorAktaLahir
	user.Agama = updateData.Agama
	user.TempatTinggal = updateData.TempatTinggal
	user.ModeTransportasi = updateData.ModeTransportasi
	user.AnakKe = updateData.AnakKe
	user.JumlahBersaudara = updateData.JumlahBersaudara
	user.RiwayatPenyakit = updateData.RiwayatPenyakit
	if strings.TrimSpace(updateData.TanggalLahir) != "" {
		if t, perr := time.Parse("2006-01-02", strings.TrimSpace(updateData.TanggalLahir)); perr == nil {
			user.TanggalLahir = &t
		}
	}

	// Pisahkan tanggal_lahir → raw SQL (kolom DATE, ORM kirim sebagai datetime).
	tanggalLahirToSet := user.TanggalLahir
	if _, err := o.Update(&user,
		"NamaLengkap", "Email", "NoHandphone", "AsalInstansi", "JenisKelamin",
		"Alamat", "Kota", "Provinsi", "Kodepos",
		"NISN", "NIP", "Kelas", "Jurusan", "TempatLahir", "Kecamatan",
		"NoNIK", "NoKK", "NomorAktaLahir", "Agama", "TempatTinggal", "ModeTransportasi",
		"AnakKe", "JumlahBersaudara", "RiwayatPenyakit",
	); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Gagal memperbarui profil",
		}
		c.ServeJSON()
		return
	}

	if tanggalLahirToSet != nil {
		dateStr := tanggalLahirToSet.Format("2006-01-02")
		if _, err := o.Raw("UPDATE users SET tanggal_lahir = ? WHERE id = ?", dateStr, user.Id).Exec(); err != nil {
			logs.Error("UpdateProfile update tanggal_lahir gagal: %v", err)
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = ProfileResponse{Success: false, Message: "Gagal memperbarui tanggal lahir: " + err.Error()}
			c.ServeJSON()
			return
		}
	}

	// Send activity notification
	go func() {
		utils.SendActivityNotification(userID.(int), "Profil Diperbarui", "Data profil Anda telah berhasil diperbarui.")
	}()

	if user.Role == models.RoleSekolah {
		user.Kelas = user.Sekolah
		user.Jurusan = "BK"
	}

	c.Data["json"] = ProfileResponse{
		Success: true,
		Message: "Profil berhasil diperbarui",
		Data:    user,
	}
	c.ServeJSON()
}

// @router /api/profile/onboarding-status [get]
// Mengecek apakah profil user sudah lengkap. Dipakai oleh halaman dashboard
// untuk memutuskan apakah modal onboarding perlu ditampilkan.
func (c *ProfileController) OnboardingStatus() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	user := models.User{Id: userID.(int)}
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{Success: false, Message: "User tidak ditemukan"}
		c.ServeJSON()
		return
	}

	// Admin dan Sekolah tidak diharuskan mengisi onboarding peserta (NISN/Kelas/Jurusan,
	// dsb). Selalu anggap profil mereka "lengkap" supaya modal onboarding tidak
	// muncul untuk akun admin / sekolah.
	isAdmin := user.Role == models.RoleAdmin
	isSekolah := user.Role == models.RoleSekolah
	completed := isAdmin || isSekolah || isOnboardingComplete(&user)
	if user.ProfileCompleted != completed {
		user.ProfileCompleted = completed
		_, _ = o.Update(&user, "ProfileCompleted")
	}

	tglStr := ""
	if user.TanggalLahir != nil {
		tglStr = user.TanggalLahir.Format("2006-01-02")
	}

	c.Data["json"] = ProfileResponse{
		Success: true,
		Data: map[string]interface{}{
			"completed":     completed,
			"role":          user.Role,
			"nisn":          user.NISN,
			"nip":           user.NIP,
			"kelas":         user.Kelas,
			"jurusan":       user.Jurusan,
			"sekolah":       user.Sekolah,
			"tempat_lahir":  user.TempatLahir,
			"tanggal_lahir": tglStr,
			"alamat":        user.Alamat,
			"kecamatan":     user.Kecamatan,
			"kota":          user.Kota,
			"provinsi":      user.Provinsi,
		},
	}
	c.ServeJSON()
}

// isOnboardingComplete menentukan profil "lengkap" jika field-field wajib
// terisi: NISN/NIP (paling tidak salah satu), Kelas, Jurusan, TempatLahir,
// TanggalLahir, Kecamatan, Kota, dan Provinsi.
func isOnboardingComplete(u *models.User) bool {
	if u == nil {
		return false
	}
	if strings.TrimSpace(u.NISN) == "" && strings.TrimSpace(u.NIP) == "" {
		return false
	}
	if strings.TrimSpace(u.Kelas) == "" {
		return false
	}
	if strings.TrimSpace(u.Jurusan) == "" {
		return false
	}
	if strings.TrimSpace(u.TempatLahir) == "" || u.TanggalLahir == nil {
		return false
	}
	if strings.TrimSpace(u.Kecamatan) == "" || strings.TrimSpace(u.Kota) == "" || strings.TrimSpace(u.Provinsi) == "" {
		return false
	}
	return true
}

// @router /api/profile/onboarding [post]
// Menyimpan data onboarding user (NISN/NIP, kelas, jurusan, ttl, alamat
// regional). Setelah berhasil, ProfileCompleted di-set true.
func (c *ProfileController) SaveOnboarding() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return
	}

	var p struct {
		NISN         string `json:"nisn"`
		NIP          string `json:"nip"`
		Kelas        string `json:"kelas"`
		Jurusan      string `json:"jurusan"`
		Sekolah      string `json:"sekolah"`
		TempatLahir  string `json:"tempat_lahir"`
		TanggalLahir string `json:"tanggal_lahir"` // YYYY-MM-DD
		Alamat       string `json:"alamat"`
		Kecamatan    string `json:"kecamatan"`
		Kota         string `json:"kota"`
		Provinsi     string `json:"provinsi"`
	}
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &p); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{Success: false, Message: "Format data tidak valid"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	user := models.User{Id: userID.(int)}
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{Success: false, Message: "User tidak ditemukan"}
		c.ServeJSON()
		return
	}

	user.NISN = strings.TrimSpace(p.NISN)
	user.NIP = strings.TrimSpace(p.NIP)
	user.Kelas = strings.TrimSpace(p.Kelas)
	user.Jurusan = strings.TrimSpace(p.Jurusan)
	user.Sekolah = strings.TrimSpace(p.Sekolah)
	user.TempatLahir = strings.TrimSpace(p.TempatLahir)
	user.Alamat = strings.TrimSpace(p.Alamat)
	user.Kecamatan = strings.TrimSpace(p.Kecamatan)
	user.Kota = strings.TrimSpace(p.Kota)
	user.Provinsi = strings.TrimSpace(p.Provinsi)
	if strings.TrimSpace(p.TanggalLahir) != "" {
		if t, perr := time.Parse("2006-01-02", strings.TrimSpace(p.TanggalLahir)); perr == nil {
			user.TanggalLahir = &t
		}
	}

	// Validasi minimal: harus ada NISN atau NIP, kelas, jurusan, ttl, kec/kota/prov.
	if user.NISN == "" && user.NIP == "" {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{Success: false, Message: "NISN atau NIP wajib diisi"}
		c.ServeJSON()
		return
	}
	if user.Kelas == "" || user.Jurusan == "" || user.TempatLahir == "" || user.TanggalLahir == nil ||
		user.Kecamatan == "" || user.Kota == "" || user.Provinsi == "" {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{Success: false, Message: "Semua field wajib diisi"}
		c.ServeJSON()
		return
	}
	if user.Sekolah == "" || !models.IsValidSekolah(user.Sekolah) {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{Success: false, Message: "Sekolah wajib dipilih dari daftar yang tersedia"}
		c.ServeJSON()
		return
	}
	user.ProfileCompleted = true

	// Update field non-date via ORM.
	if _, err := o.Update(&user,
		"NISN", "NIP", "Kelas", "Jurusan", "Sekolah", "TempatLahir",
		"Alamat", "Kecamatan", "Kota", "Provinsi", "ProfileCompleted",
	); err != nil {
		logs.Error("SaveOnboarding update gagal: %v", err)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = ProfileResponse{Success: false, Message: "Gagal menyimpan data profil: " + err.Error()}
		c.ServeJSON()
		return
	}

	// Beego ORM mengirim *time.Time sebagai timestamp (mis. "2007-02-26 00:00:00Z")
	// ke kolom DATE PostgreSQL → ditolak. Karena itu update tanggal_lahir
	// terpisah pakai raw SQL dengan format DATE.
	if user.TanggalLahir != nil {
		dateStr := user.TanggalLahir.Format("2006-01-02")
		if _, err := o.Raw("UPDATE users SET tanggal_lahir = ? WHERE id = ?", dateStr, user.Id).Exec(); err != nil {
			logs.Error("SaveOnboarding update tanggal_lahir gagal: %v", err)
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = ProfileResponse{Success: false, Message: "Gagal menyimpan tanggal lahir: " + err.Error()}
			c.ServeJSON()
			return
		}
	}

	c.Data["json"] = ProfileResponse{Success: true, Message: "Profil berhasil disimpan"}
	c.ServeJSON()
}

// @router /api/profile/upload [post]
func (c *ProfileController) UploadFoto() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	file, header, err := c.GetFile("foto_profil")
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "File tidak ditemukan",
		}
		c.ServeJSON()
		return
	}
	defer file.Close()

	// Validate file size (5MB max before crop)
	if header.Size > 5*1024*1024 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Ukuran file maksimal 5MB",
		}
		c.ServeJSON()
		return
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Format file tidak valid. Hanya JPG, JPEG, atau PNG yang diizinkan",
		}
		c.ServeJSON()
		return
	}

	// Read file into memory for decoding
	fileBytes := make([]byte, header.Size)
	_, err = file.Read(fileBytes)
	if err != nil && err != io.EOF {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Gagal membaca file",
		}
		c.ServeJSON()
		return
	}

	// Decode image from bytes
	var img image.Image
	var decodeErr error
	reader := bytes.NewReader(fileBytes)
	if ext == ".png" {
		img, decodeErr = png.Decode(reader)
	} else {
		img, decodeErr = jpeg.Decode(reader)
	}
	if decodeErr != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Gagal memproses gambar",
		}
		c.ServeJSON()
		return
	}

	// Resize image to max 400x400 (maintain aspect ratio)
	const maxSize = 400
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	var newWidth, newHeight int
	if width > height {
		if width > maxSize {
			newWidth = maxSize
			newHeight = (height * maxSize) / width
		} else {
			newWidth = width
			newHeight = height
		}
	} else {
		if height > maxSize {
			newHeight = maxSize
			newWidth = (width * maxSize) / height
		} else {
			newWidth = width
			newHeight = height
		}
	}

	// Create resized image
	resized := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.BiLinear.Scale(resized, resized.Bounds(), img, bounds, draw.Over, nil)

	// Generate unique filename (always use .jpg for consistency)
	filename := fmt.Sprintf("%d_%d.jpg", userID.(int), time.Now().Unix())
	uploadPath := filepath.Join("static", "uploads", "profiles", filename)

	// Create uploads/profiles directory if it doesn't exist
	uploadDir := filepath.Dir(uploadPath)
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Gagal membuat direktori upload",
		}
		c.ServeJSON()
		return
	}

	// Save resized image as JPEG
	dst, err := os.Create(uploadPath)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Gagal menyimpan file",
		}
		c.ServeJSON()
		return
	}
	defer dst.Close()

	// Encode as JPEG with quality 90
	if err := jpeg.Encode(dst, resized, &jpeg.Options{Quality: 90}); err != nil {
		os.Remove(uploadPath)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Gagal menyimpan gambar",
		}
		c.ServeJSON()
		return
	}

	// Update user's foto_profil in database
	o := orm.NewOrm()
	user := models.User{Id: userID.(int)}
	if err := o.Read(&user); err != nil {
		// Delete uploaded file if database update fails
		os.Remove(uploadPath)
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "User tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	// Delete old photo if exists
	if user.FotoProfil != "" {
		oldPath := filepath.Join("static", "uploads", "profiles", user.FotoProfil)
		os.Remove(oldPath)
	}

	user.FotoProfil = filename
	if _, err := o.Update(&user, "FotoProfil"); err != nil {
		// Delete uploaded file if database update fails
		os.Remove(uploadPath)
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Gagal memperbarui foto profil di database",
		}
		c.ServeJSON()
		return
	}

	// Send activity notification
	go func() {
		utils.SendActivityNotification(userID.(int), "Foto Profil Diperbarui", "Foto profil Anda telah berhasil diubah.")
	}()

	c.Data["json"] = ProfileResponse{
		Success: true,
		Message: "Foto profil berhasil diunggah",
		Data: map[string]string{
			"foto_profil": filename,
		},
	}
	c.ServeJSON()
}

// @router /api/profile/tests [get]
// Mengembalikan daftar undangan tes + ringkasan hasil IST/Holland untuk user yang login.
func (c *ProfileController) GetTestResults() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	// Ambil user untuk mendapatkan email
	var user models.User
	user.Id = userID.(int)
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "User tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	// Ambil semua undangan yang terhubung ke user (baik via UserId maupun Email), urut terbaru
	var invitations []models.TestInvitation
	// Cari berdasarkan UserId ATAU Email (karena invitation bisa dibuat dengan email sebelum user_id di-set)
	cond := orm.NewCondition()
	cond = cond.Or("UserId", userID.(int)).Or("Email", user.Email)
	_, _ = o.QueryTable(new(models.TestInvitation)).
		SetCond(cond).
		OrderBy("-CreatedAt").
		All(&invitations)

	if len(invitations) == 0 {
		c.Data["json"] = ProfileResponse{
			Success: true,
			Data:    []TestResultSummary{},
		}
		c.ServeJSON()
		return
	}

	// Load batches untuk nama
	batchMap := make(map[int]string)
	for _, inv := range invitations {
		if inv.BatchId != nil {
			if _, ok := batchMap[*inv.BatchId]; !ok {
				var b models.TestBatch
				b.Id = *inv.BatchId
				if err := o.Read(&b); err == nil {
					batchMap[*inv.BatchId] = b.Name
				}
			}
		}
	}

	var result []TestResultSummary
	for _, inv := range invitations {
		batchName := "Batch tidak ditemukan"
		if inv.BatchId != nil {
			if name, ok := batchMap[*inv.BatchId]; ok {
				batchName = name
			}
		}
		s := TestResultSummary{
			InvitationID: inv.Id,
			BatchName:    batchName,
			CreatedAt:    inv.CreatedAt,
		}

		// IST
		var ist models.ISTResult
		if err := o.QueryTable(new(models.ISTResult)).
			Filter("Invitation__Id", inv.Id).
			One(&ist); err == nil && ist.Id != 0 {
			s.HasIST = true
			s.ISTIQ = ist.IQ
			s.ISTIQCat = ist.IQCategory
		}

		// Holland
		var hol models.HollandResult
		if err := o.QueryTable(new(models.HollandResult)).
			Filter("Invitation__Id", inv.Id).
			One(&hol); err == nil && hol.Id != 0 {
			s.HasHolland = true
		}

		// Learning style (VAK)
		var vak models.LearningStyleResult
		if err := o.QueryTable(new(models.LearningStyleResult)).
			Filter("Invitation__Id", inv.Id).
			One(&vak); err == nil && vak.Id != 0 {
			s.HasLearningStyle = true
		}

		result = append(result, s)
	}

	c.Data["json"] = ProfileResponse{
		Success: true,
		Data:    result,
	}
	c.ServeJSON()
}

// ProfileTestSummary untuk halaman profil dengan data lengkap
type ProfileTestSummary struct {
	LastISTResult     *ISTDetailResult     `json:"last_ist_result,omitempty"`
	LastHollandResult *HollandDetailResult  `json:"last_holland_result,omitempty"`
	LastLearningStyleResult *LearningStyleDetailResult `json:"last_learning_style_result,omitempty"`
	LastKraepelinAttempt *KraepelinDetailResult `json:"last_kraepelin_attempt,omitempty"`
	AverageIQ         float64               `json:"average_iq"`
	TotalISTTests     int                   `json:"total_ist_tests"`
	TotalHollandTests int                   `json:"total_holland_tests"`
	TotalLearningStyleTests int             `json:"total_learning_style_tests"`
	TotalKraepelinTests int                 `json:"total_kraepelin_tests"`
	LastInstitution   string                `json:"last_institution,omitempty"`
}

// ISTDetailResult detail hasil IST untuk profil
type ISTDetailResult struct {
	InvitationID      int       `json:"invitation_id"`
	BatchName         string    `json:"batch_name"`
	Institution       string    `json:"institution"`
	TestDate          time.Time `json:"test_date"`
	IQ                int       `json:"iq"`
	IQCategory        string    `json:"iq_category"`
	TotalStandardScore int      `json:"total_standard_score"`
	StdScores         []int     `json:"std_scores"` // [SE, WA, AN, GE, RA, ZA, FA, WU, ME]
	RawScores         []int     `json:"raw_scores"` // [SE, WA, AN, GE, RA, ZA, FA, WU, ME]
}

// HollandDetailResult detail hasil Holland untuk profil
type HollandDetailResult struct {
	InvitationID int       `json:"invitation_id"`
	BatchName    string    `json:"batch_name"`
	Institution  string    `json:"institution"`
	TestDate     time.Time `json:"test_date"`
	ScoreR       int       `json:"score_r"`
	ScoreI       int       `json:"score_i"`
	ScoreA       int       `json:"score_a"`
	ScoreS       int       `json:"score_s"`
	ScoreE       int       `json:"score_e"`
	ScoreC       int       `json:"score_c"`
	Top1         string    `json:"top1"`
	Top2         string    `json:"top2"`
	Top3         string    `json:"top3"`
	Code         string    `json:"code"`
}

type LearningStyleDetailResult struct {
	InvitationID int       `json:"invitation_id"`
	BatchName    string    `json:"batch_name"`
	Institution  string    `json:"institution"`
	TestDate     time.Time `json:"test_date"`
	ScoreVisual  int       `json:"score_visual"`
	ScoreAuditory int      `json:"score_auditory"`
	ScoreKinesthetic int   `json:"score_kinesthetic"`
	DominantType string    `json:"dominant_type"`
	InterpretationVisual string `json:"interpretation_visual"`
	InterpretationAuditory string `json:"interpretation_auditory"`
	InterpretationKinesthetic string `json:"interpretation_kinesthetic"`
}

type KraepelinDetailResult struct {
	InvitationID int       `json:"invitation_id"`
	BatchName    string    `json:"batch_name"`
	Institution  string    `json:"institution"`
	TestDate     time.Time `json:"test_date"`
	TestName     string    `json:"test_name"`
	TotalCorrect int       `json:"total_correct"`
	TotalErrors  int       `json:"total_errors"`
	TotalSkipped int       `json:"total_skipped"`
	CorrectCounts []int    `json:"correct_counts"`
}

// RMIBProfileEntry digunakan untuk endpoint /api/profile/rmib.
type RMIBProfileEntry struct {
	Invitation map[string]interface{} `json:"invitation"`
	RMIBResult map[string]interface{} `json:"rmib_result"`
}

// @router /api/profile/rmib [get]
// Mengembalikan daftar hasil RMIB milik user yang login (paling baru di depan).
func (c *ProfileController) GetRMIBResults() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	var user models.User
	user.Id = userID.(int)
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{Success: false, Message: "User tidak ditemukan"}
		c.ServeJSON()
		return
	}

	var results []models.RMIBResult
	_, _ = o.QueryTable(new(models.RMIBResult)).
		Filter("User__Id", userID.(int)).
		OrderBy("-CompletedAt").
		All(&results)

	entries := []RMIBProfileEntry{}
	for _, r := range results {
		var inv models.TestInvitation
		if r.Invitation != nil {
			inv.Id = r.Invitation.Id
			_ = o.Read(&inv)
		}
		entries = append(entries, RMIBProfileEntry{
			Invitation: map[string]interface{}{
				"id":         inv.Id,
				"email":      inv.Email,
				"batch_id":   inv.BatchId,
				"created_at": inv.CreatedAt,
			},
			RMIBResult: map[string]interface{}{
				"id":                r.Id,
				"gender_version":    r.GenderVersion,
				"result_json":       r.ResultJSON,
				"dominant_category": r.DominantCategory,
				"top1":              r.Top1,
				"top2":              r.Top2,
				"top3":              r.Top3,
				"interpretation":    r.Interpretation,
				"completed_at":      r.CompletedAt,
			},
		})
	}

	c.Data["json"] = ProfileResponse{
		Success: true,
		Data:    entries,
	}
	c.ServeJSON()
}

// PAPIProfileEntry untuk API profile PAPI
type PAPIProfileEntry struct {
	Invitation  map[string]interface{} `json:"invitation"`
	PAPIResult  map[string]interface{} `json:"papi_result"`
}

// @router /api/profile/papi-results [get]
// Mengembalikan daftar hasil PAPI milik user yang login (paling baru di depan).
func (c *ProfileController) GetPAPIResults() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	var user models.User
	user.Id = userID.(int)
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{Success: false, Message: "User tidak ditemukan"}
		c.ServeJSON()
		return
	}

	var results []models.PAPIResult
	_, _ = o.QueryTable(new(models.PAPIResult)).
		Filter("User__Id", userID.(int)).
		OrderBy("-CompletedAt").
		All(&results)

	entries := []PAPIProfileEntry{}
	for _, r := range results {
		var inv models.TestInvitation
		if r.Invitation != nil {
			inv.Id = r.Invitation.Id
			_ = o.Read(&inv)
		}
		entries = append(entries, PAPIProfileEntry{
			Invitation: map[string]interface{}{
				"id":         inv.Id,
				"email":      inv.Email,
				"batch_id":   inv.BatchId,
				"created_at": inv.CreatedAt,
			},
			PAPIResult: map[string]interface{}{
				"id":                 r.Id,
				"result_json":        r.ResultJSON,
				"dominant_category":  r.DominantCategory,
				"top_categories":     r.TopCategories,
				"interpretation":     r.Interpretation,
				"completed_at":       r.CompletedAt,
				"time_taken_minutes": r.TimeTakenMinutes,
			},
		})
	}

	c.Data["json"] = ProfileResponse{
		Success: true,
		Data:    entries,
	}
	c.ServeJSON()
}

// @router /api/profile/test-summary [get]
// Mengembalikan ringkasan hasil tes terakhir untuk halaman profil
func (c *ProfileController) GetTestSummary() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()

	// Ambil user untuk mendapatkan email
	var user models.User
	user.Id = userID.(int)
	if err := o.Read(&user); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "User tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	// Ambil semua undangan yang terhubung ke user (baik via UserId maupun Email), urut terbaru
	var invitations []models.TestInvitation
	// Cari berdasarkan UserId ATAU Email (karena invitation bisa dibuat dengan email sebelum user_id di-set)
	cond := orm.NewCondition()
	cond = cond.Or("UserId", userID.(int)).Or("Email", user.Email)
	_, _ = o.QueryTable(new(models.TestInvitation)).
		SetCond(cond).
		OrderBy("-CreatedAt").
		All(&invitations)

	summary := ProfileTestSummary{
		AverageIQ:         0,
		TotalISTTests:     0,
		TotalHollandTests: 0,
		TotalLearningStyleTests: 0,
		TotalKraepelinTests: 0,
	}

	if len(invitations) == 0 {
		c.Data["json"] = ProfileResponse{
			Success: true,
			Data:    summary,
		}
		c.ServeJSON()
		return
	}

	// Load batches untuk nama dan institution
	batchMap := make(map[int]models.TestBatch)
	for _, inv := range invitations {
		if inv.BatchId != nil {
			if _, ok := batchMap[*inv.BatchId]; !ok {
				var b models.TestBatch
				b.Id = *inv.BatchId
				if err := o.Read(&b); err == nil {
					batchMap[*inv.BatchId] = b
				}
			}
		}
	}

	// Cari hasil IST terakhir dan hitung rata-rata
	// Urutan berdasarkan CreatedAt DESC, jadi yang pertama adalah test terakhir
	var lastIST *models.ISTResult
	var lastISTInvID int
	var allIQ []int
	
	for _, inv := range invitations {
		var ist models.ISTResult
		if err := o.QueryTable(new(models.ISTResult)).
			Filter("Invitation__Id", inv.Id).
			One(&ist); err == nil && ist.Id != 0 {
			summary.TotalISTTests++
			
			// Recalculate jika: IQ / TotalStandardScore masih 0 (data legacy belum dihitung).
			// TotalStandardScore sekarang mengikuti norma TOTAL (SUM RW -> SW TOTAL), bukan rata-rata SW.
			needsRecalc := (ist.IQ == 0 || ist.TotalStandardScore == 0)
			if needsRecalc {
				// Ambil user untuk mendapatkan tanggal lahir
				var userForAge models.User
				userForAge.Id = userID.(int)
				if o.Read(&userForAge) == nil {
					age := 0
					if userForAge.TanggalLahir != nil {
						// Hitung age pada saat test dikerjakan (CreatedAt dari result) atau sekarang
						testDate := ist.CreatedAt
						if testDate.IsZero() {
							testDate = time.Now()
						}
						age = utils.AgeYears(*userForAge.TanggalLahir, testDate)
					}
					if age > 0 {
						// Recalculate IQ dengan age yang tepat
						if updatedRes, err := utils.EnsureISTStandardAndIQScores(o, &ist, age); err == nil && updatedRes != nil {
							ist = *updatedRes
							_, _ = o.Update(&ist,
								"StdSE", "StdWA", "StdAN", "StdGE", "StdRA", "StdZA", "StdFA", "StdWU", "StdME",
								"TotalStandardScore", "IQ", "IQCategory",
							)
							// Reload dari DB untuk mendapatkan nilai terbaru
							o.QueryTable(new(models.ISTResult)).
								Filter("Invitation__Id", inv.Id).
								One(&ist)
						}
					}
				}
			}
			
			if ist.IQ > 0 {
				allIQ = append(allIQ, ist.IQ)
			}
			if lastIST == nil {
				lastIST = &ist
				lastISTInvID = inv.Id
				// Load invitation untuk mendapatkan batch info
				if inv.BatchId != nil {
					if batch, ok := batchMap[*inv.BatchId]; ok {
						summary.LastInstitution = batch.Institution
					}
				}
			}
		}
	}

	// Hitung rata-rata IQ
	if len(allIQ) > 0 {
		sum := 0
		for _, iq := range allIQ {
			sum += iq
		}
		summary.AverageIQ = float64(sum) / float64(len(allIQ))
	}

	// Format hasil IST terakhir - tampilkan jika ada hasil IST (meskipun IQ masih 0)
	if lastIST != nil && lastISTInvID > 0 {
		// Find the invitation from our list
		var invDetail models.TestInvitation
		for _, inv := range invitations {
			if inv.Id == lastISTInvID {
				invDetail = inv
				break
			}
		}
		
		// Handle jika batch sudah dihapus (batch tidak ada di batchMap atau BatchId NULL)
		batchName := "Batch tidak ditemukan"
		institution := "-"
		if invDetail.BatchId != nil {
			if batch, ok := batchMap[*invDetail.BatchId]; ok {
				batchName = batch.Name
				institution = batch.Institution
			}
		}
		
		istDetail := ISTDetailResult{
			InvitationID:       invDetail.Id,
			BatchName:         batchName,
			Institution:       institution,
			TestDate:          invDetail.CreatedAt,
			IQ:                lastIST.IQ,
			IQCategory:        lastIST.IQCategory,
			TotalStandardScore: lastIST.TotalStandardScore,
			StdScores: []int{
				lastIST.StdSE, lastIST.StdWA, lastIST.StdAN,
				lastIST.StdGE, lastIST.StdRA, lastIST.StdZA,
				lastIST.StdFA, lastIST.StdWU, lastIST.StdME,
			},
			RawScores: []int{
				lastIST.RawSE, lastIST.RawWA, lastIST.RawAN,
				lastIST.RawGE, lastIST.RawRA, lastIST.RawZA,
				lastIST.RawFA, lastIST.RawWU, lastIST.RawME,
			},
		}
		summary.LastISTResult = &istDetail
	}

	// Cari hasil Holland terakhir
	var lastHolland *models.HollandResult
	var lastHollandInvID int
	for _, inv := range invitations {
		var hol models.HollandResult
		if err := o.QueryTable(new(models.HollandResult)).
			Filter("Invitation__Id", inv.Id).
			One(&hol); err == nil && hol.Id != 0 {
			summary.TotalHollandTests++
			if lastHolland == nil {
				lastHolland = &hol
				lastHollandInvID = inv.Id
			}
		}
	}

	// Format hasil Holland terakhir
	if lastHolland != nil && lastHollandInvID > 0 {
		// Find the invitation from our list
		var invDetail models.TestInvitation
		for _, inv := range invitations {
			if inv.Id == lastHollandInvID {
				invDetail = inv
				break
			}
		}
		
		// Handle jika batch sudah dihapus (batch tidak ada di batchMap atau BatchId NULL)
		batchName := "Batch tidak ditemukan"
		institution := "-"
		if invDetail.BatchId != nil {
			if batch, ok := batchMap[*invDetail.BatchId]; ok {
				batchName = batch.Name
				institution = batch.Institution
			}
		}
		
		holDetail := HollandDetailResult{
			InvitationID: invDetail.Id,
			BatchName:    batchName,
			Institution:  institution,
			TestDate:     invDetail.CreatedAt,
			ScoreR:       lastHolland.ScoreR,
			ScoreI:       lastHolland.ScoreI,
			ScoreA:       lastHolland.ScoreA,
			ScoreS:       lastHolland.ScoreS,
			ScoreE:       lastHolland.ScoreE,
			ScoreC:       lastHolland.ScoreC,
			Top1:         lastHolland.Top1,
			Top2:         lastHolland.Top2,
			Top3:         lastHolland.Top3,
			Code:         lastHolland.Code,
		}
		summary.LastHollandResult = &holDetail
	}

	// Cari hasil Learning Style (VAK) terakhir
	var lastVAK *models.LearningStyleResult
	var lastVAKInvID int
	for _, inv := range invitations {
		var vak models.LearningStyleResult
		if err := o.QueryTable(new(models.LearningStyleResult)).
			Filter("Invitation__Id", inv.Id).
			One(&vak); err == nil && vak.Id != 0 {
			summary.TotalLearningStyleTests++
			if lastVAK == nil {
				lastVAK = &vak
				lastVAKInvID = inv.Id
			}
		}
	}

	if lastVAK != nil && lastVAKInvID > 0 {
		var invDetail models.TestInvitation
		for _, inv := range invitations {
			if inv.Id == lastVAKInvID {
				invDetail = inv
				break
			}
		}
		batchName := "Batch tidak ditemukan"
		institution := "-"
		if invDetail.BatchId != nil {
			if batch, ok := batchMap[*invDetail.BatchId]; ok {
				batchName = batch.Name
				institution = batch.Institution
			}
		}
		vakDetail := LearningStyleDetailResult{
			InvitationID: invDetail.Id,
			BatchName:    batchName,
			Institution:  institution,
			TestDate:     lastVAK.TestDate,
			ScoreVisual:  lastVAK.ScoreVisual,
			ScoreAuditory: lastVAK.ScoreAuditory,
			ScoreKinesthetic: lastVAK.ScoreKinesthetic,
			DominantType: lastVAK.DominantType,
			InterpretationVisual: seeds.LearningStyleInterpretationVisual(),
			InterpretationAuditory: seeds.LearningStyleInterpretationAuditory(),
			InterpretationKinesthetic: seeds.LearningStyleInterpretationKinesthetic(),
		}
		summary.LastLearningStyleResult = &vakDetail
	}

	// Cari hasil Kraepelin terakhir
	var lastKraepelin *models.KraepelinAttempt
	var lastKraepelinInvID int
	for _, inv := range invitations {
		var att models.KraepelinAttempt
		if err := o.QueryTable(new(models.KraepelinAttempt)).
			Filter("Invitation__Id", inv.Id).
			One(&att); err == nil && att.Id != 0 {
			summary.TotalKraepelinTests++
			if lastKraepelin == nil {
				lastKraepelin = &att
				lastKraepelinInvID = inv.Id
			}
		}
	}

	if lastKraepelin != nil && lastKraepelinInvID > 0 {
		var invDetail models.TestInvitation
		for _, inv := range invitations {
			if inv.Id == lastKraepelinInvID {
				invDetail = inv
				break
			}
		}
		batchName := "Batch tidak ditemukan"
		institution := "-"
		if invDetail.BatchId != nil {
			if batch, ok := batchMap[*invDetail.BatchId]; ok {
				batchName = batch.Name
				institution = batch.Institution
			}
		}

		// Kraepelin di app ini menggunakan 40 raw data (kolom), jadi chart juga 40 titik.
		counts := make([]int, 40)
		if strings.TrimSpace(lastKraepelin.CorrectCountsJSON) != "" {
			_ = json.Unmarshal([]byte(lastKraepelin.CorrectCountsJSON), &counts)
		}
		if len(counts) != 40 {
			counts = make([]int, 40)
		}

		k := KraepelinDetailResult{
			InvitationID: invDetail.Id,
			BatchName:    batchName,
			Institution:  institution,
			TestDate:     lastKraepelin.TestDate,
			TestName:     lastKraepelin.TestName,
			TotalCorrect: lastKraepelin.TotalCorrect,
			TotalErrors:  lastKraepelin.TotalErrors,
			TotalSkipped: lastKraepelin.TotalSkipped,
			CorrectCounts: counts,
		}
		summary.LastKraepelinAttempt = &k
	}

	c.Data["json"] = ProfileResponse{
		Success: true,
		Data:    summary,
	}
	c.ServeJSON()
}