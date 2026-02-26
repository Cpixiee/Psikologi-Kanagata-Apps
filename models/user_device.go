package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

type UserDevice struct {
	Id          int       `orm:"auto;pk" json:"id"`
	UserId      int       `orm:"index" json:"user_id"`
	DeviceId    string    `orm:"size(255);unique" json:"device_id"` // Unique device identifier
	DeviceName  string    `orm:"size(255)" json:"device_name"`     // e.g., "Chrome on Windows"
	BrowserInfo string    `orm:"size(500)" json:"browser_info"`    // User agent
	IpAddress   string    `orm:"size(50)" json:"ip_address"`
	IsVerified  bool      `orm:"default(false)" json:"is_verified"` // User verified this device
	IsBlocked   bool      `orm:"default(false)" json:"is_blocked"` // Device blocked by user
	LastUsedAt  time.Time `orm:"auto_now;type(datetime)" json:"last_used_at"`
	CreatedAt   time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (u *UserDevice) TableName() string {
	return "user_devices"
}

func init() {
	orm.RegisterModel(new(UserDevice))
}
