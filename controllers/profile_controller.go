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
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
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

	c.Data["json"] = ProfileResponse{
		Success: true,
		Data:    user,
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
		JenisKelamin string `json:"jenis_kelamin"`
		Alamat       string `json:"alamat"`
		Kota         string `json:"kota"`
		Provinsi     string `json:"provinsi"`
		Kodepos      string `json:"kodepos"`
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
	user.JenisKelamin = models.Gender(updateData.JenisKelamin)
	user.Alamat = updateData.Alamat
	user.Kota = updateData.Kota
	user.Provinsi = updateData.Provinsi
	user.Kodepos = updateData.Kodepos

	if _, err := o.Update(&user, "NamaLengkap", "Email", "NoHandphone", "JenisKelamin", "Alamat", "Kota", "Provinsi", "Kodepos"); err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = ProfileResponse{
			Success: false,
			Message: "Gagal memperbarui profil",
		}
		c.ServeJSON()
		return
	}

	// Send activity notification
	go func() {
		utils.SendActivityNotification(userID.(int), "Profil Diperbarui", "Data profil Anda telah berhasil diperbarui.")
	}()

	c.Data["json"] = ProfileResponse{
		Success: true,
		Message: "Profil berhasil diperbarui",
		Data:    user,
	}
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
