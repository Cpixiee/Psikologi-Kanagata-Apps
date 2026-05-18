package controllers

import (
	"encoding/json"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

type SettingsController struct {
	beego.Controller
}

type SettingsResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// @router /api/settings [get]
func (c *SettingsController) GetSettings() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = SettingsResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	settings := models.UserSettings{UserId: userID.(int)}
	err := o.Read(&settings, "UserId")
	if err != nil {
		// Create default settings if not exists
		settings.UserId = userID.(int)
		settings.NotifNewForYouEmail = true
		settings.NotifNewForYouBrowser = true
		settings.NotifNewForYouApp = true
		settings.NotifActivityEmail = true
		settings.NotifActivityBrowser = true
		settings.NotifActivityApp = true
		settings.NotifBrowserLoginEmail = true
		settings.NotifBrowserLoginBrowser = true
		settings.NotifBrowserLoginApp = false
		settings.NotifDeviceLinkEmail = true
		settings.NotifDeviceLinkBrowser = false
		settings.NotifDeviceLinkApp = false
		settings.NotificationTiming = "online"
		
		_, err = o.Insert(&settings)
		if err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = SettingsResponse{
				Success: false,
				Message: "Gagal membuat pengaturan default",
			}
			c.ServeJSON()
			return
		}
	}

	c.Data["json"] = SettingsResponse{
		Success: true,
		Data:    settings,
	}
	c.ServeJSON()
}

// @router /api/settings [put]
func (c *SettingsController) UpdateSettings() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = SettingsResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	var updateData struct {
		NotifNewForYouEmail      bool   `json:"notif_new_for_you_email"`
		NotifNewForYouBrowser    bool   `json:"notif_new_for_you_browser"`
		NotifNewForYouApp        bool   `json:"notif_new_for_you_app"`
		NotifActivityEmail       bool   `json:"notif_activity_email"`
		NotifActivityBrowser     bool   `json:"notif_activity_browser"`
		NotifActivityApp         bool   `json:"notif_activity_app"`
		NotifBrowserLoginEmail   bool   `json:"notif_browser_login_email"`
		NotifBrowserLoginBrowser bool   `json:"notif_browser_login_browser"`
		NotifBrowserLoginApp     bool   `json:"notif_browser_login_app"`
		NotifDeviceLinkEmail     bool   `json:"notif_device_link_email"`
		NotifDeviceLinkBrowser   bool   `json:"notif_device_link_browser"`
		NotifDeviceLinkApp       bool   `json:"notif_device_link_app"`
		NotificationTiming       string `json:"notification_timing"`
	}

	if err := json.Unmarshal(c.Ctx.Input.RequestBody, &updateData); err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = SettingsResponse{
			Success: false,
			Message: "Format data tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	settings := models.UserSettings{UserId: userID.(int)}
	err := o.Read(&settings, "UserId")
	if err != nil {
		// Create new settings if not exists
		settings.UserId = userID.(int)
		settings.NotifNewForYouEmail = updateData.NotifNewForYouEmail
		settings.NotifNewForYouBrowser = updateData.NotifNewForYouBrowser
		settings.NotifNewForYouApp = updateData.NotifNewForYouApp
		settings.NotifActivityEmail = updateData.NotifActivityEmail
		settings.NotifActivityBrowser = updateData.NotifActivityBrowser
		settings.NotifActivityApp = updateData.NotifActivityApp
		settings.NotifBrowserLoginEmail = updateData.NotifBrowserLoginEmail
		settings.NotifBrowserLoginBrowser = updateData.NotifBrowserLoginBrowser
		settings.NotifBrowserLoginApp = updateData.NotifBrowserLoginApp
		settings.NotifDeviceLinkEmail = updateData.NotifDeviceLinkEmail
		settings.NotifDeviceLinkBrowser = updateData.NotifDeviceLinkBrowser
		settings.NotifDeviceLinkApp = updateData.NotifDeviceLinkApp
		settings.NotificationTiming = updateData.NotificationTiming
		
		_, err = o.Insert(&settings)
		if err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = SettingsResponse{
				Success: false,
				Message: "Gagal menyimpan pengaturan",
			}
			c.ServeJSON()
			return
		}
	} else {
		// Update existing settings
		settings.NotifNewForYouEmail = updateData.NotifNewForYouEmail
		settings.NotifNewForYouBrowser = updateData.NotifNewForYouBrowser
		settings.NotifNewForYouApp = updateData.NotifNewForYouApp
		settings.NotifActivityEmail = updateData.NotifActivityEmail
		settings.NotifActivityBrowser = updateData.NotifActivityBrowser
		settings.NotifActivityApp = updateData.NotifActivityApp
		settings.NotifBrowserLoginEmail = updateData.NotifBrowserLoginEmail
		settings.NotifBrowserLoginBrowser = updateData.NotifBrowserLoginBrowser
		settings.NotifBrowserLoginApp = updateData.NotifBrowserLoginApp
		settings.NotifDeviceLinkEmail = updateData.NotifDeviceLinkEmail
		settings.NotifDeviceLinkBrowser = updateData.NotifDeviceLinkBrowser
		settings.NotifDeviceLinkApp = updateData.NotifDeviceLinkApp
		settings.NotificationTiming = updateData.NotificationTiming
		
		_, err = o.Update(&settings)
		if err != nil {
			c.Ctx.Output.SetStatus(500)
			c.Data["json"] = SettingsResponse{
				Success: false,
				Message: "Gagal memperbarui pengaturan",
			}
			c.ServeJSON()
			return
		}
	}

	c.Data["json"] = SettingsResponse{
		Success: true,
		Message: "Pengaturan berhasil disimpan",
		Data:    settings,
	}
	c.ServeJSON()
}
