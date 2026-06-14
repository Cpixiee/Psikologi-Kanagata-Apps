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
	Id           int    `json:"id"`
	Nama         string `json:"nama"`
	Kelas        string `json:"kelas"`
	Email        string `json:"email"`
	JenisKelamin string `json:"jenis_kelamin"`
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
			Nama:         nama,
			Kelas:        kelas,
			Email:        email,
			JenisKelamin: strings.TrimSpace(t.JenisKelamin),
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
		// JenisKelamin diisi default agar tidak melanggar constraint DB.
		// Akun sekolah tidak menggunakan gender; field ini tidak ditampilkan di UI.
		JenisKelamin: models.GenderLakiLaki,
	}
	if err := school.HashPassword(); err != nil {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal mengamankan password"}
		c.ServeJSON()
		return
	}
	createdId, err := tx.Insert(&school)
	if err != nil {
		_ = tx.Rollback()
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal membuat akun sekolah: " + err.Error()}
		c.ServeJSON()
		return
	}
	school.Id = int(createdId)

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

// @router /api/schools/my-teachers [get]
// Daftar guru milik akun sekolah yang sedang login (untuk akun sekolah sendiri).
func (c *AdminSchoolController) MyTeachers() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = schoolResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return
	}
	roleVal := c.GetSession("user_role")
	roleStr, _ := roleVal.(string)
	if roleStr != string(models.RoleSekolah) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = schoolResponse{Success: false, Message: "Hanya akun sekolah yang dapat mengakses endpoint ini"}
		c.ServeJSON()
		return
	}
	o := orm.NewOrm()
	
	// Convert session user_id safely to prevent any type-assertion panic
	var schoolID int
	if idInt, ok := userID.(int); ok {
		schoolID = idInt
	} else if idInt64, ok := userID.(int64); ok {
		schoolID = int(idInt64)
	} else if idFloat, ok := userID.(float64); ok {
		schoolID = int(idFloat)
	}
	
	var teachers []models.SchoolTeacher
	_, _ = o.QueryTable(new(models.SchoolTeacher)).Filter("SchoolId", schoolID).OrderBy("id").All(&teachers)
	c.Data["json"] = schoolResponse{Success: true, Data: teachers}
	c.ServeJSON()
}

// @router /api/schools/students [get]
func (c *AdminSchoolController) ListStudents() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = schoolResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return
	}
	roleVal := c.GetSession("user_role")
	roleStr, _ := roleVal.(string)
	if roleStr != string(models.RoleSekolah) && roleStr != string(models.RoleAdmin) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = schoolResponse{Success: false, Message: "Akses ditolak"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	sekolahName := ""

	if roleStr == string(models.RoleSekolah) {
		var schoolID int
		if idInt, ok := userID.(int); ok {
			schoolID = idInt
		} else if idInt64, ok := userID.(int64); ok {
			schoolID = int(idInt64)
		} else if idFloat, ok := userID.(float64); ok {
			schoolID = int(idFloat)
		}
		var schoolUser models.User
		schoolUser.Id = schoolID
		if err := o.Read(&schoolUser); err == nil {
			sekolahName = schoolUser.Sekolah
		}
	} else {
		// Admin can pass a school name filter
		sekolahName = c.GetString("sekolah")
	}

	if sekolahName == "" && roleStr == string(models.RoleSekolah) {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "Sekolah tidak terkonfigurasi untuk akun Anda"}
		c.ServeJSON()
		return
	}

	// Query students for the school
	var students []models.User
	qs := o.QueryTable(new(models.User)).Filter("Role", string(models.RoleSiswa))
	if sekolahName != "" {
		qs = qs.Filter("Sekolah", sekolahName)
	}
	_, err := qs.OrderBy("NamaLengkap").All(&students)
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = schoolResponse{Success: false, Message: "Gagal mengambil data siswa"}
		c.ServeJSON()
		return
	}

	// Count unique non-empty Jurusan and Kelas
	classesMap := make(map[string]bool)
	majorsMap := make(map[string]bool)
	for _, s := range students {
		cls := strings.TrimSpace(s.Kelas)
		if cls != "" {
			classesMap[cls] = true
		}
		mjr := strings.TrimSpace(s.Jurusan)
		if mjr != "" {
			majorsMap[mjr] = true
		}
	}

	c.Data["json"] = schoolResponse{
		Success: true,
		Data: map[string]interface{}{
			"students":      students,
			"total_students": len(students),
			"total_classes":  len(classesMap),
			"total_majors":   len(majorsMap),
			"sekolah":       sekolahName,
		},
	}
	c.ServeJSON()
}

// @router /api/schools/access-student/:id [post]
func (c *AdminSchoolController) AccessStudent() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = schoolResponse{Success: false, Message: "Silakan login terlebih dahulu"}
		c.ServeJSON()
		return
	}
	roleVal := c.GetSession("user_role")
	roleStr, _ := roleVal.(string)
	if roleStr != string(models.RoleSekolah) && roleStr != string(models.RoleAdmin) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = schoolResponse{Success: false, Message: "Hanya akun sekolah atau admin yang dapat menggunakan fitur ini"}
		c.ServeJSON()
		return
	}

	studentID, err := strconv.Atoi(c.Ctx.Input.Param(":id"))
	if err != nil || studentID <= 0 {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = schoolResponse{Success: false, Message: "ID siswa tidak valid"}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	var student models.User
	student.Id = studentID
	if err := o.Read(&student); err != nil || student.Role != models.RoleSiswa {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = schoolResponse{Success: false, Message: "Siswa tidak ditemukan"}
		c.ServeJSON()
		return
	}

	// Verify school matching
	if roleStr == string(models.RoleSekolah) {
		var schoolID int
		if idInt, ok := userID.(int); ok {
			schoolID = idInt
		} else if idInt64, ok := userID.(int64); ok {
			schoolID = int(idInt64)
		} else if idFloat, ok := userID.(float64); ok {
			schoolID = int(idFloat)
		}
		var schoolUser models.User
		schoolUser.Id = schoolID
		if err := o.Read(&schoolUser); err != nil || !strings.EqualFold(student.Sekolah, schoolUser.Sekolah) {
			c.Ctx.Output.SetStatus(403)
			c.Data["json"] = schoolResponse{Success: false, Message: "Anda tidak diizinkan mengakses siswa dari sekolah lain"}
			c.ServeJSON()
			return
		}
	}

	// Store original session details as backup for reverting
	c.SetSession("impersonator_user_id", userID)
	c.SetSession("impersonator_role", roleStr)
	c.SetSession("impersonator_email", c.GetSession("user_email"))

	// Switch session details to student
	c.SetSession("user_id", student.Id)
	c.SetSession("user_role", string(student.Role))
	c.SetSession("user_email", student.Email)

	c.Data["json"] = schoolResponse{
		Success: true,
		Message: "Akses akun siswa berhasil",
	}
	c.ServeJSON()
}
