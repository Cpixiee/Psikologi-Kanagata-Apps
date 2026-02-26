package controllers

import (
	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

type NotificationController struct {
	beego.Controller
}

type NotificationResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// @router /api/notifications [get]
func (c *NotificationController) GetNotifications() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = NotificationResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	var notifications []models.Notification
	_, err := o.QueryTable("notifications").
		Filter("user_id", userID.(int)).
		OrderBy("-created_at").
		Limit(50).
		All(&notifications)

	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = NotificationResponse{
			Success: false,
			Message: "Gagal memuat notifikasi",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = NotificationResponse{
		Success: true,
		Data:    notifications,
	}
	c.ServeJSON()
}

// @router /api/notifications/:id/read [put]
func (c *NotificationController) MarkAsRead() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = NotificationResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	notificationID, err := c.GetInt(":id")
	if err != nil {
		c.Ctx.Output.SetStatus(400)
		c.Data["json"] = NotificationResponse{
			Success: false,
			Message: "ID notifikasi tidak valid",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	notification := models.Notification{Id: notificationID}
	if err := o.Read(&notification); err != nil {
		c.Ctx.Output.SetStatus(404)
		c.Data["json"] = NotificationResponse{
			Success: false,
			Message: "Notifikasi tidak ditemukan",
		}
		c.ServeJSON()
		return
	}

	// Verify ownership
	if notification.UserId != userID.(int) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = NotificationResponse{
			Success: false,
			Message: "Akses ditolak",
		}
		c.ServeJSON()
		return
	}

	notification.IsRead = true
	o.Update(&notification, "IsRead")

	c.Data["json"] = NotificationResponse{
		Success: true,
		Message: "Notifikasi ditandai sebagai sudah dibaca",
	}
	c.ServeJSON()
}

// @router /api/notifications/read-all [put]
func (c *NotificationController) MarkAllAsRead() {
	userID := c.GetSession("user_id")
	if userID == nil {
		c.Ctx.Output.SetStatus(401)
		c.Data["json"] = NotificationResponse{
			Success: false,
			Message: "Silakan login terlebih dahulu",
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	_, err := o.QueryTable("notifications").
		Filter("user_id", userID.(int)).
		Filter("is_read", false).
		Update(orm.Params{
			"is_read": true,
		})

	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = NotificationResponse{
			Success: false,
			Message: "Gagal memperbarui notifikasi",
		}
		c.ServeJSON()
		return
	}

	c.Data["json"] = NotificationResponse{
		Success: true,
		Message: "Semua notifikasi ditandai sebagai sudah dibaca",
	}
	c.ServeJSON()
}
