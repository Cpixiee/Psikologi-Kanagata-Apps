package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

type Notification struct {
	Id        int       `orm:"auto;pk" json:"id"`
	UserId    int       `orm:"index" json:"user_id"`
	Type      string    `orm:"size(50)" json:"type"`      // new_for_you, activity, browser_login, device_link
	Title     string    `orm:"size(255)" json:"title"`
	Message   string    `orm:"type(text)" json:"message"`
	IsRead    bool      `orm:"default(false)" json:"is_read"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (n *Notification) TableName() string {
	return "notifications"
}

func init() {
	orm.RegisterModel(new(Notification))
}
