package controllers

import (
	"encoding/json"
	"psikologi_apps/models"
	"psikologi_apps/utils"
	"time"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

type PasswordResetController struct {
	beego.Controller
}

type RequestOTPRequest struct {
	Email string `json:"email"`
}

type VerifyOTPRequest struct {
	Email    string `json:"email"`
	OtpCode  string `json:"otp_code"`
	Password string `json:"password"`
}

// @router /api/auth/request-reset [post]
func (c *PasswordResetController) RequestOTP() {
	var req RequestOTPRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Format data tidak valid",
		}
		c.ServeJSON()
		return
	}

	// Validate email
	if req.Email == "" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email wajib diisi",
		}
		c.ServeJSON()
		return
	}

	// Check if user exists
	o := orm.NewOrm()
	user := models.User{Email: req.Email}
	err := o.Read(&user, "Email")
	if err != nil {
		// Don't reveal if email exists or not for security
		c.Data["json"] = Response{
			Success: true,
			Message: "Jika email terdaftar, kode OTP telah dikirim ke email Anda",
		}
		c.ServeJSON()
		return
	}

	// Invalidate previous OTPs for this email
	_, err = o.QueryTable(new(models.PasswordReset)).
		Filter("email", req.Email).
		Filter("used", false).
		Update(orm.Params{"used": true})
	if err != nil {
		// Log error but continue
	}

	// Generate OTP
	passwordReset := models.PasswordReset{
		Email:     req.Email,
		ExpiresAt: time.Now().Add(15 * time.Minute), // OTP expires in 15 minutes
		Used:      false,
	}

	if err := passwordReset.GenerateOTP(); err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal menghasilkan kode OTP",
		}
		c.ServeJSON()
		return
	}

	if err := passwordReset.GenerateToken(); err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal menghasilkan token",
		}
		c.ServeJSON()
		return
	}

	// Save OTP to database
	_, err = o.Insert(&passwordReset)
	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal menyimpan kode OTP",
		}
		c.ServeJSON()
		return
	}

	// Send OTP via email
	config := utils.GetEmailConfig()
	err = utils.SendOTPEmail(config, req.Email, passwordReset.OtpCode)
	if err != nil {
		// Delete the OTP if email fails
		o.Delete(&passwordReset)
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal mengirim email. Pastikan konfigurasi SMTP sudah benar.",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = Response{
		Success: true,
		Message: "Kode OTP telah dikirim ke email Anda. Silakan cek inbox atau spam folder.",
		Data: map[string]interface{}{
			"expires_at": passwordReset.ExpiresAt.Format(time.RFC3339),
		},
	}
	c.ServeJSON()
}

// @router /api/auth/verify-reset [post]
func (c *PasswordResetController) VerifyOTP() {
	var req VerifyOTPRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Format data tidak valid",
		}
		c.ServeJSON()
		return
	}

	// Validate required fields
	if req.Email == "" || req.OtpCode == "" || req.Password == "" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Email, kode OTP, dan password baru wajib diisi",
		}
		c.ServeJSON()
		return
	}

	// Validate password length
	if len(req.Password) < 6 {
		c.Data["json"] = Response{
			Success: false,
			Message: "Password minimal 6 karakter",
		}
		c.ServeJSON()
		return
	}

	// Find OTP record
	o := orm.NewOrm()
	passwordReset := models.PasswordReset{}
	err := o.QueryTable(new(models.PasswordReset)).
		Filter("email", req.Email).
		Filter("otp_code", req.OtpCode).
		Filter("used", false).
		OrderBy("-created_at").
		One(&passwordReset)

	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Kode OTP tidak valid atau sudah digunakan",
		}
		c.ServeJSON()
		return
	}

	// Check if OTP is expired
	if passwordReset.IsExpired() {
		c.Data["json"] = Response{
			Success: false,
			Message: "Kode OTP sudah kadaluarsa. Silakan request kode baru",
		}
		c.ServeJSON()
		return
	}

	// Find user
	user := models.User{Email: req.Email}
	err = o.Read(&user, "Email")
	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "User tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	// Update password
	user.Password = req.Password
	if err := user.HashPassword(); err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal memproses password baru",
		}
		c.ServeJSON()
		return
	}

	_, err = o.Update(&user, "Password")
	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal mengupdate password",
		}
		c.ServeJSON()
		return
	}

	// Mark OTP as used
	passwordReset.Used = true
	o.Update(&passwordReset, "Used")

	c.Data["json"] = Response{
		Success: true,
		Message: "Password berhasil direset. Silakan login dengan password baru Anda.",
	}
	c.ServeJSON()
}
