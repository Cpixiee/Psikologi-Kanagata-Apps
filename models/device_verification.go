package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

type DeviceVerification struct {
	Id        int       `orm:"auto;pk" json:"id"`
	UserId    int       `orm:"index" json:"user_id"`
	DeviceId  string    `orm:"size(255);index" json:"device_id"`
	Token     string    `orm:"size(255);unique" json:"token"` // Unique verification token
	IsVerified bool     `orm:"default(false)" json:"is_verified"`
	IsRejected bool     `orm:"default(false)" json:"is_rejected"` // User clicked "Bukan Saya"
	ExpiresAt time.Time `orm:"type(datetime)" json:"expires_at"`
	CreatedAt time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
}

func (d *DeviceVerification) TableName() string {
	return "device_verifications"
}

func init() {
	orm.RegisterModel(new(DeviceVerification))
}
