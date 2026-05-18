package controllers

import (
	"encoding/json"
	"strconv"
	"strings"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

// AdminSchoolController mengelola akun sekolah (role=sekolah) dan daftar
// guru yang terikat ke masing-masing akun sekolah.
type AdminSchoolController struct {
	beego.Controller
}

type schoolTeacherInput struct {
	Id    int    `json:"id"`
	Nama  string `json:"nama"`
	Kelas string `json:"kelas"`
	Email string `json:"email"`
}

type schoolUpsertRequest struct {
	NamaLengkap  string               `json:"nama_lengkap"`
	Email        string               `json:"email"`
	Sekolah      string               `json:"sekolah"`
	Password     string               `json:"password"`
	JenisKelamin string               `json:"jenis_kelamin"`
	Teachers     []schoolTeacherInput `json:"teachers"`
}

type schoolResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

func (c *AdminSchoolController) requireAdmin() bool {
	roleVal := c.GetSession("user_role")
	roleStr, _ := roleVal.(string)
	if roleStr != string(models.RoleAdmin) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = schoolResponse{Success: false, Message: "Akses ditolak"}
		c.ServeJSON()
		return false
	}
	return true
}

// @router /api/admin/schools [get]
// Daftar semua akun sekolah beserta jumlah guru-nya.
func (c *AdminSchoolController) List() {
	if !c.requireAdmin() {
		return
	}
	o := orm.NewOrm()
	var schools []models.User
	_, err := o.QueryTable(new(models.User)).
		Filter("Role", string(models.RoleSekolah)).
		OrderBy("-id").
		All(&schools)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal memuat sekolah"}
		c.ServeJSON()
		return
	}

	type schoolItem struct {
		Id           int    `json:"id"`
		NamaLengkap  string `json:"nama_lengkap"`
		Email        string `json:"email"`
		Sekolah      string `json:"sekolah"`
		TeacherCount int    `json:"teacher_count"`
	}
	items := make([]schoolItem, 0, len(schools))
	for _, s := range schools {
		cnt, _ := o.QueryTable(new(models.SchoolTeacher)).Filter("SchoolId", s.Id).Count()
		items = append(items, schoolItem{
			Id:           s.Id,
			NamaLengkap:  s.NamaLengkap,
			Email:        s.Email,
			Sekolah:      s.Sekolah,
			TeacherCount: int(cnt),
		})
	}

	c.Data["json"] = schoolResponse{
		Success: true,
		Data: map[string]interface{}{
			"schools":      items,
			"sekolah_list": models.SekolahList,
		},
	}
	c.ServeJSON()
}

// @router /api/admin/schools/:id [get]
// Detail satu akun sekolah + daftar guru.
func (c *AdminSchoolController) Detail() {
	if !c.requireAdmin() {
		return
	}
	id, _ := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if id <= 0 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "ID tidak valid"}
		c.ServeJSON()
		return
	}
	o := orm.NewOrm()
	school := models.User{Id: id}
	if err := o.Read(&school); err != nil || school.Role != models.RoleSekolah {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = schoolResponse{Success: false, Message: "Sekolah tidak ditemukan"}
		c.ServeJSON()
		return
	}
	var teachers []models.SchoolTeacher
	_, _ = o.QueryTable(new(models.SchoolTeacher)).Filter("SchoolId", id).OrderBy("id").All(&teachers)
	c.Data["json"] = schoolResponse{
		Success: true,
		Data: map[string]interface{}{
			"id":           school.Id,
			"nama_lengkap": school.NamaLengkap,
			"email":        school.Email,
			"sekolah":      school.Sekolah,
			"teachers":     teachers,
			"sekolah_list": models.SekolahList,
		},
	}
	c.ServeJSON()
}

// @router /api/admin/schools [post]
// Buat akun sekolah baru beserta daftar guru.
func (c *AdminSchoolController) Create() {
	if !c.requireAdmin() {
		return
	}
	var req schoolUpsertRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "Payload tidak valid"}
		c.ServeJSON()
		return
	}
	req.NamaLengkap = strings.TrimSpace(req.NamaLengkap)
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	req.Sekolah = strings.TrimSpace(req.Sekolah)
	req.Password = strings.TrimSpace(req.Password)
	req.JenisKelamin = strings.TrimSpace(req.JenisKelamin)

	if req.NamaLengkap == "" || req.Email == "" || req.Sekolah == "" || req.Password == "" {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "Nama, email, sekolah, dan password wajib diisi"}
		c.ServeJSON()
		return
	}
	if req.JenisKelamin != string(models.GenderLakiLaki) && req.JenisKelamin != string(models.GenderPerempuan) {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "Jenis kelamin wajib dipilih (laki-laki / perempuan)"}
		c.ServeJSON()
		return
	}
	if !models.IsValidSekolah(req.Sekolah) {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "Sekolah tidak valid"}
		c.ServeJSON()
		return
	}
	if len(req.Password) < 6 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "Password minimal 6 karakter"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	// Pastikan email sekolah belum dipakai user lain atau guru lain.
	if cnt, _ := o.QueryTable(new(models.User)).Filter("Email", req.Email).Count(); cnt > 0 {
		c.Ctx.Output.SetStatus(409)
		c.Data["json"] = schoolResponse{Success: false, Message: "Email sudah dipakai user lain"}
		c.ServeJSON()
		return
	}
	if cnt, _ := o.QueryTable(new(models.SchoolTeacher)).Filter("Email", req.Email).Count(); cnt > 0 {
		c.Ctx.Output.SetStatus(409)
		c.Data["json"] = schoolResponse{Success: false, Message: "Email sudah dipakai guru lain"}
		c.ServeJSON()
		return
	}

	// Validasi daftar guru: email unik & belum dipakai.
	cleanTeachers := make([]models.SchoolTeacher, 0, len(req.Teachers))
	seenEmail := map[string]bool{req.Email: true}
	for _, t := range req.Teachers {
		nama := strings.TrimSpace(t.Nama)
		kelas := strings.TrimSpace(t.Kelas)
		email := strings.ToLower(strings.TrimSpace(t.Email))
		if nama == "" && kelas == "" && email == "" {
			continue // baris kosong, diabaikan
		}
		if nama == "" || email == "" {
			c.Ctx.Output.SetStatus(400)
			c.Data["json"] = schoolResponse{Success: false, Message: "Nama dan email guru wajib diisi"}
			c.ServeJSON()
			return
		}
		if seenEmail[email] {
			c.Ctx.Output.SetStatus(409)
			c.Data["json"] = schoolResponse{Success: false, Message: "Email guru duplikat: " + email}
			c.ServeJSON()
			return
		}
		seenEmail[email] = true
		if cnt, _ := o.QueryTable(new(models.User)).Filter("Email", email).Count(); cnt > 0 {
			c.Ctx.Output.SetStatus(409)
			c.Data["json"] = schoolResponse{Success: false, Message: "Email guru sudah dipakai user lain: " + email}
			c.ServeJSON()
			return
		}
		if cnt, _ := o.QueryTable(new(models.SchoolTeacher)).Filter("Email", email).Count(); cnt > 0 {
			c.Ctx.Output.SetStatus(409)
			c.Data["json"] = schoolResponse{Success: false, Message: "Email guru sudah dipakai guru lain: " + email}
			c.ServeJSON()
			return
		}
		cleanTeachers = append(cleanTeachers, models.SchoolTeacher{
			Nama:  nama,
			Kelas: kelas,
			Email: email,
		})
	}

	tx, err := o.Begin()
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal memulai transaksi"}
		c.ServeJSON()
		return
	}

	school := models.User{
		NamaLengkap:      req.NamaLengkap,
		Email:            req.Email,
		Sekolah:          req.Sekolah,
		Password:         req.Password,
		Role:             models.RoleSekolah,
		ProfileCompleted: true,
		JenisKelamin:     models.Gender(req.JenisKelamin),
	}
	if err := school.HashPassword(); err != nil {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal mengamankan password"}
		c.ServeJSON()
		return
	}
	if _, err := tx.Insert(&school); err != nil {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal membuat akun sekolah: " + err.Error()}
		c.ServeJSON()
		return
	}

	for i := range cleanTeachers {
		cleanTeachers[i].SchoolId = school.Id
		if _, err := tx.Insert(&cleanTeachers[i]); err != nil {
			_ = tx.Rollback()
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = schoolResponse{Success: false, Message: "Gagal menyimpan guru: " + err.Error()}
			c.ServeJSON()
			return
		}
	}

	if err := tx.Commit(); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal commit transaksi"}
		c.ServeJSON()
		return
	}

	c.Data["json"] = schoolResponse{
		Success: true,
		Message: "Akun sekolah berhasil dibuat",
		Data:    map[string]interface{}{"id": school.Id},
	}
	c.ServeJSON()
}

// @router /api/admin/schools/:id [delete]
// Hapus akun sekolah (cascade akan menghapus guru-nya juga).
func (c *AdminSchoolController) Delete() {
	if !c.requireAdmin() {
		return
	}
	id, _ := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if id <= 0 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "ID tidak valid"}
		c.ServeJSON()
		return
	}
	o := orm.NewOrm()
	school := models.User{Id: id}
	if err := o.Read(&school); err != nil || school.Role != models.RoleSekolah {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = schoolResponse{Success: false, Message: "Sekolah tidak ditemukan"}
		c.ServeJSON()
		return
	}
	if _, err := o.Delete(&school); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal menghapus akun sekolah"}
		c.ServeJSON()
		return
	}
	c.Data["json"] = schoolResponse{Success: true, Message: "Akun sekolah dihapus"}
	c.ServeJSON()
}
