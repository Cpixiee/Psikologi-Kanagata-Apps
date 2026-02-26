package controllers

import (
	"psikologi_apps/models"
	"time"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

type DeviceVerificationController struct {
	beego.Controller
}

type DeviceVerificationResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// @router /verify-device [get]
func (c *DeviceVerificationController) VerifyDevice() {
	token := c.GetString("token")
	if token == "" {
		c.TplName = "device_verification.html"
		c.Data["Error"] = "Token tidak valid"
		c.Data["IsReject"] = false
		return
	}

	o := orm.NewOrm()
	verification := models.DeviceVerification{Token: token}
	err := o.Read(&verification, "Token")
	if err != nil {
		c.TplName = "device_verification.html"
		c.Data["Error"] = "Token tidak ditemukan atau sudah tidak valid"
		c.Data["IsReject"] = false
		return
	}

	// Check if expired
	if time.Now().After(verification.ExpiresAt) {
		c.TplName = "device_verification.html"
		c.Data["Error"] = "Token sudah kadaluarsa"
		c.Data["IsReject"] = false
		return
	}

	// Mark as verified
	verification.IsVerified = true
	o.Update(&verification, "IsVerified")

	// Update device as verified
	device := models.UserDevice{DeviceId: verification.DeviceId}
	if o.Read(&device, "DeviceId") == nil {
		device.IsVerified = true
		o.Update(&device, "IsVerified")
	}

	c.TplName = "device_verification.html"
	c.Data["Success"] = true
	c.Data["Message"] = "Perangkat berhasil diverifikasi"
	c.Data["IsReject"] = false
}

// @router /reject-device [get]
func (c *DeviceVerificationController) RejectDevice() {
	token := c.GetString("token")
	if token == "" {
		c.TplName = "device_verification.html"
		c.Data["Error"] = "Token tidak valid"
		c.Data["IsReject"] = true
		return
	}

	o := orm.NewOrm()
	verification := models.DeviceVerification{Token: token}
	err := o.Read(&verification, "Token")
	if err != nil {
		c.TplName = "device_verification.html"
		c.Data["Error"] = "Token tidak ditemukan atau sudah tidak valid"
		c.Data["IsReject"] = true
		return
	}

	// Mark as rejected
	verification.IsRejected = true
	o.Update(&verification, "IsRejected")

	// Block the device
	device := models.UserDevice{DeviceId: verification.DeviceId}
	if o.Read(&device, "DeviceId") == nil {
		device.IsBlocked = true
		o.Update(&device, "IsBlocked")
	}

	// Invalidate all sessions for this device (would need session store implementation)
	// For now, we just mark the device as blocked

	c.TplName = "device_verification.html"
	c.Data["Success"] = true
	c.Data["Message"] = "Akses perangkat telah dibatalkan. Perangkat ini telah diblokir."
	c.Data["IsReject"] = true
}
