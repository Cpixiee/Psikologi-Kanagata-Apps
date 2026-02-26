package controllers

import (
	"strings"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

type AdminUserController struct {
	beego.Controller
}

type AdminUserResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// @router /api/admin/users/search [get]
func (c *AdminUserController) Search() {
	// Hanya admin
	roleVal := c.GetSession("user_role")
	roleStr, _ := roleVal.(string)
	if roleStr != string(models.RoleAdmin) {
		c.Ctx.Output.SetStatus(403)
		c.Data["json"] = AdminUserResponse{
			Success: false,
			Message: "Akses ditolak, hanya admin yang boleh mengakses",
		}
		c.ServeJSON()
		return
	}

	query := strings.TrimSpace(c.GetString("q"))
	if query == "" {
		c.Data["json"] = AdminUserResponse{
			Success: true,
			Data:    []interface{}{},
		}
		c.ServeJSON()
		return
	}

	o := orm.NewOrm()
	var users []models.User

	qs := o.QueryTable(new(models.User))
	cond := orm.NewCondition().
		Or("Email__icontains", query).
		Or("NamaLengkap__icontains", query)

	_, err := qs.SetCond(cond).Limit(10).All(&users, "Id", "NamaLengkap", "Email")
	if err != nil {
		c.Ctx.Output.SetStatus(500)
		c.Data["json"] = AdminUserResponse{
			Success: false,
			Message: "Gagal mencari user",
		}
		c.ServeJSON()
		return
	}

	// Hanya kirim field ringan
	type UserLite struct {
		Id          int    `json:"id"`
		NamaLengkap string `json:"nama_lengkap"`
		Email       string `json:"email"`
	}
	result := make([]UserLite, 0, len(users))
	for _, u := range users {
		result = append(result, UserLite{
			Id:          u.Id,
			NamaLengkap: u.NamaLengkap,
			Email:       u.Email,
		})
	}

	c.Data["json"] = AdminUserResponse{
		Success: true,
		Data:    result,
	}
	c.ServeJSON()
}

