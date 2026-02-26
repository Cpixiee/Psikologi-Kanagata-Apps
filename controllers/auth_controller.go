package controllers

import (
	"encoding/json"
	"psikologi_apps/models"
	"psikologi_apps/utils"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/core/logs"
	"github.com/dchest/captcha"
)

type AuthController struct {
	beego.Controller
}

type RegisterRequest struct {
	NamaLengkap  string `json:"nama_lengkap"`
	Alamat       string `json:"alamat"`
	JenisKelamin string `json:"jenis_kelamin"`
	Email        string `json:"email"`
	NoHandphone  string `json:"no_handphone"`
	Password     string `json:"password"`
	CaptchaId    string `json:"captcha_id"`
	CaptchaValue string `json:"captcha_value"`
	Role         string `json:"role"` // siswa, guru, pekerja; admin biasanya di-set manual
}

type LoginRequest struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	CaptchaId    string `json:"captcha_id"`
	CaptchaValue string `json:"captcha_value"`
}

type Response struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// @router /api/auth/register [post]
func (c *AuthController) Register() {
	var req RegisterRequest
	json.Unmarshal(c.Ctx.Input.RequestBody, &req)

	// Validate CAPTCHA
	if !captcha.VerifyString(req.CaptchaId, req.CaptchaValue) {
		c.Data["json"] = Response{
			Success: false,
			Message: "CAPTCHA tidak valid",
		}
		c.ServeJSON()
		return
	}

	// Validate required fields
	if req.NamaLengkap == "" || req.Email == "" || req.Password == "" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Semua field wajib diisi",
		}
		c.ServeJSON()
		return
	}

	// Validate gender
	if req.JenisKelamin != "laki_laki" && req.JenisKelamin != "perempuan" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Jenis kelamin harus laki_laki atau perempuan",
		}
		c.ServeJSON()
		return
	}

	// Check if email already exists
	o := orm.NewOrm()
	existingUser := models.User{Email: req.Email}
	err := o.Read(&existingUser, "Email")
	if err == nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email sudah terdaftar",
		}
		c.ServeJSON()
		return
	}

	// Determine role (default: siswa)
	role := req.Role
	if role == "" {
		role = string(models.RoleSiswa)
	}
	if role != string(models.RoleSiswa) &&
		role != string(models.RoleGuru) &&
		role != string(models.RolePekerja) &&
		role != string(models.RoleMahasiswa) &&
		role != string(models.RoleUmum) &&
		role != string(models.RoleAdmin) {
		c.Data["json"] = Response{
			Success: false,
			Message: "Role tidak valid. Gunakan: siswa, guru, pekerja, mahasiswa, umum, atau admin",
		}
		c.ServeJSON()
		return
	}

	// Create new user
	user := models.User{
		NamaLengkap:  req.NamaLengkap,
		Alamat:       req.Alamat,
		JenisKelamin: models.Gender(req.JenisKelamin),
		Email:        req.Email,
		NoHandphone:  req.NoHandphone,
		Password:     req.Password,
		Role:         models.Role(role),
	}

	// Hash password
	if err := user.HashPassword(); err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal memproses password",
		}
		c.ServeJSON()
		return
	}

	// Insert user
	_, err = o.Insert(&user)
	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal mendaftarkan user: " + err.Error(),
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = Response{
		Success: true,
		Message: "Registrasi berhasil",
		Data: map[string]interface{}{
			"id":    user.Id,
			"email": user.Email,
		},
	}
	c.ServeJSON()
}

// @router /api/auth/login [post]
func (c *AuthController) Login() {
	var req LoginRequest
	json.Unmarshal(c.Ctx.Input.RequestBody, &req)

	// Validate CAPTCHA
	if !captcha.VerifyString(req.CaptchaId, req.CaptchaValue) {
		c.Data["json"] = Response{
			Success: false,
			Message: "CAPTCHA tidak valid",
		}
		c.ServeJSON()
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email dan password wajib diisi",
		}
		c.ServeJSON()
		return
	}

	// Find user by email
	o := orm.NewOrm()
	user := models.User{Email: req.Email}
	err := o.Read(&user, "Email")
	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email atau password salah",
		}
		c.ServeJSON()
		return
	}

	// Verify password
	if !user.CheckPassword(req.Password) {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email atau password salah",
		}
		c.ServeJSON()
		return
	}

	// Get device info
	userAgent := c.Ctx.Input.UserAgent()
	ipAddress := c.Ctx.Input.IP()
	
	// Check and register device
	isNewDevice := false
	var device *models.UserDevice
	deviceCheck, devicePtr, err := utils.CheckAndRegisterDevice(user.Id, userAgent, ipAddress)
	if err != nil {
		// Log error but don't fail login
		logs.Error("Error checking device: %v", err)
	} else {
		isNewDevice = deviceCheck
		device = devicePtr
	}
	
	// Check if device is blocked
	if device != nil && device.IsBlocked {
		c.Data["json"] = Response{
			Success: false,
			Message: "Perangkat ini telah diblokir. Silakan hubungi administrator.",
		}
		c.ServeJSON()
		return
	}
	
	// Set session
	c.SetSession("user_id", user.Id)
	c.SetSession("user_email", user.Email)
	c.SetSession("user_role", string(user.Role))
	if device != nil {
		c.SetSession("device_id", device.DeviceId)
	}

	// Send notifications
	go func() {
		// Send "New for you" notification for successful login
		utils.SendNewForYouNotification(user.Id, "Login Berhasil", "Anda berhasil masuk ke akun Anda.")
		
		// If new device, send device link notification
		if isNewDevice && device != nil {
			deviceInfo := device.DeviceName
			if device.IpAddress != "" {
				deviceInfo += " (" + device.IpAddress + ")"
			}
			utils.SendDeviceLinkNotification(user.Id, deviceInfo)
		} else if device != nil {
			// Check if this is a different browser on same device
			// For now, we'll send browser login notification for any login
			browserInfo := utils.ParseUserAgent(userAgent)
			utils.SendBrowserLoginNotification(user.Id, browserInfo)
		}
	}()

	c.Data["json"] = Response{
		Success: true,
		Message: "Login berhasil",
		Data: map[string]interface{}{
			"id":           user.Id,
			"nama_lengkap": user.NamaLengkap,
			"email":        user.Email,
			"role":         user.Role,
			"is_new_device": isNewDevice,
		},
	}
	c.ServeJSON()
}

// @router /api/auth/logout [post]
func (c *AuthController) Logout() {
	// Destroy all session data for current user
	c.DestroySession()

	c.Data["json"] = Response{
		Success: true,
		Message: "Berhasil logout",
	}
	c.ServeJSON()
}

// @router /api/auth/captcha [get]
func (c *AuthController) GetCaptcha() {
	captchaId := captcha.New()
	c.Data["json"] = Response{
		Success: true,
		Data: map[string]string{
			"captcha_id": captchaId,
		},
	}
	c.ServeJSON()
}

// @router /api/auth/captcha/:id [get]
func (c *AuthController) CaptchaImage() {
	captchaId := c.Ctx.Input.Param(":id")
	c.Ctx.Output.Header("Content-Type", "image/png")
	captcha.WriteImage(c.Ctx.ResponseWriter, captchaId, 200, 60)
}
