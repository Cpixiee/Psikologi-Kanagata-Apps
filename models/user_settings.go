package models

import (
	"time"

	"github.com/beego/beego/v2/client/orm"
)

type UserSettings struct {
	Id                          int       `orm:"auto;pk" json:"id"`
	UserId                      int       `orm:"unique" json:"user_id"`
	// New for you notifications
	NotifNewForYouEmail         bool      `orm:"default(true)" json:"notif_new_for_you_email"`
	NotifNewForYouBrowser       bool      `orm:"default(true)" json:"notif_new_for_you_browser"`
	NotifNewForYouApp           bool      `orm:"default(true)" json:"notif_new_for_you_app"`
	// Account activity notifications
	NotifActivityEmail          bool      `orm:"default(true)" json:"notif_activity_email"`
	NotifActivityBrowser        bool      `orm:"default(true)" json:"notif_activity_browser"`
	NotifActivityApp            bool      `orm:"default(true)" json:"notif_activity_app"`
	// Browser login notifications
	NotifBrowserLoginEmail      bool      `orm:"default(true)" json:"notif_browser_login_email"`
	NotifBrowserLoginBrowser    bool      `orm:"default(true)" json:"notif_browser_login_browser"`
	NotifBrowserLoginApp        bool      `orm:"default(false)" json:"notif_browser_login_app"`
	// Device link notifications
	NotifDeviceLinkEmail        bool      `orm:"default(true)" json:"notif_device_link_email"`
	NotifDeviceLinkBrowser      bool      `orm:"default(false)" json:"notif_device_link_browser"`
	NotifDeviceLinkApp          bool      `orm:"default(false)" json:"notif_device_link_app"`
	NotificationTiming          string    `orm:"size(50);default('online')" json:"notification_timing"` // online, always, never
	CreatedAt                   time.Time `orm:"auto_now_add;type(datetime)" json:"created_at"`
	UpdatedAt                   time.Time `orm:"auto_now;type(datetime)" json:"updated_at"`
}

func (u *UserSettings) TableName() string {
	return "user_settings"
}

func init() {
	orm.RegisterModel(new(UserSettings))
}
