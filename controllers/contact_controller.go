package controllers

import (
	"encoding/json"
	"psikologi_apps/utils"

	beego "github.com/beego/beego/v2/server/web"
)

type ContactController struct {
	beego.Controller
}

type ContactRequest struct {
	Name    string `json:"name"`
	Email   string `json:"email"`
	Phone   string `json:"phone"`
	Message string `json:"message"`
}

// @router /api/contact [post]
func (c *ContactController) SendMessage() {
	var req ContactRequest
	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &req); err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Format data tidak valid",
		}
		c.ServeJSON()
		return
	}

	// Validate required fields
	if req.Name == "" || req.Email == "" || req.Message == "" {
		c.Data["json"] = Response{
			Success: false,
			Message: "Nama, email, dan pesan wajib diisi",
		}
		c.ServeJSON()
		return
	}

	// Get email config
	config := utils.GetEmailConfig()

	// Send email
	err := utils.SendContactEmail(config, req.Name, req.Email, req.Phone, req.Message)
	if err != nil {
		c.Data["json"] = Response{
			Success: false,
			Message: "Gagal mengirim pesan: " + err.Error(),
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = Response{
		Success: true,
		Message: "Pesan berhasil dikirim. Kami akan menghubungi Anda segera.",
	}
	c.ServeJSON()
}
