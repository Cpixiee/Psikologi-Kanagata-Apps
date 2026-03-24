package utils

import (
	"fmt"
	"psikologi_apps/models"
	"time"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

// NotificationType represents the type of notification
type NotificationType string

const (
	NotifTypeNewForYou      NotificationType = "new_for_you"
	NotifTypeActivity       NotificationType = "activity"
	NotifTypeBrowserLogin   NotificationType = "browser_login"
	NotifTypeDeviceLink     NotificationType = "device_link"
)

// SendNotification sends notification based on user settings
func SendNotification(userID int, notifType NotificationType, title, message string) error {
	o := orm.NewOrm()
	
	// Get user settings
	settings := models.UserSettings{UserId: userID}
	err := o.Read(&settings, "UserId")
	if err != nil {
		// If no settings, create default
		settings.UserId = userID
		settings.NotifNewForYouEmail = true
		settings.NotifNewForYouBrowser = true
		settings.NotifActivityEmail = true
		settings.NotifActivityBrowser = true
		settings.NotifBrowserLoginEmail = true
		settings.NotifBrowserLoginBrowser = true
		settings.NotifDeviceLinkEmail = true
		settings.NotifDeviceLinkBrowser = false
		settings.NotificationTiming = "online"
		o.Insert(&settings)
	}

	// Get user email
	user := models.User{Id: userID}
	if err := o.Read(&user); err != nil {
		return fmt.Errorf("user not found: %v", err)
	}

	// Determine which channels to send based on notification type and settings
	var sendEmail, sendBrowser bool

	switch notifType {
	case NotifTypeNewForYou:
		sendEmail = settings.NotifNewForYouEmail
		sendBrowser = settings.NotifNewForYouBrowser
	case NotifTypeActivity:
		sendEmail = settings.NotifActivityEmail
		sendBrowser = settings.NotifActivityBrowser
	case NotifTypeBrowserLogin:
		sendEmail = settings.NotifBrowserLoginEmail
		sendBrowser = settings.NotifBrowserLoginBrowser
	case NotifTypeDeviceLink:
		sendEmail = settings.NotifDeviceLinkEmail
		sendBrowser = settings.NotifDeviceLinkBrowser
	}

	// Apply global notification timing rule
	// "never"  -> jangan kirim lewat channel apa pun
	// "always" -> kirim sesuai checkbox di atas
	// "online" -> untuk saat ini diperlakukan sama seperti "always"
	if settings.NotificationTiming == "never" {
		sendEmail = false
		sendBrowser = false
	}

	// Send email notification
	if sendEmail {
		emailConfig := GetEmailConfig()
		emailBody := fmt.Sprintf(`
			<h2>%s</h2>
			<p>%s</p>
			<p>Terima kasih,<br>Psychee Wellness</p>
		`, title, message)
		
		emailData := EmailData{
			To:      user.Email,
			Subject: title,
			Body:    emailBody,
		}
		
		err := SendEmail(emailConfig, emailData)
		if err != nil {
			// Log error but don't fail the whole process
			fmt.Printf("Error sending email notification: %v\n", err)
		}
	}

	// Store browser notification in database
	if sendBrowser {
		notification := models.Notification{
			UserId:  userID,
			Type:    string(notifType),
			Title:   title,
			Message: message,
			IsRead:  false,
		}
		o.Insert(&notification)
	}

	return nil
}

// SendNewForYouNotification sends "New for you" notification
func SendNewForYouNotification(userID int, title, message string) error {
	return SendNotification(userID, NotifTypeNewForYou, title, message)
}

// SendActivityNotification sends "Account activity" notification
func SendActivityNotification(userID int, title, message string) error {
	return SendNotification(userID, NotifTypeActivity, title, message)
}

// SendBrowserLoginNotification sends "Browser login" notification
func SendBrowserLoginNotification(userID int, browserInfo string) error {
	title := "Browser Baru Digunakan untuk Masuk"
	message := fmt.Sprintf("Akun Anda baru saja digunakan untuk masuk dari browser: %s. Jika ini bukan Anda, segera ubah password Anda.", browserInfo)
	return SendNotification(userID, NotifTypeBrowserLogin, title, message)
}

// SendDeviceLinkNotification sends "Device link" notification with security validation
func SendDeviceLinkNotification(userID int, deviceInfo string) error {
	o := orm.NewOrm()
	
	// Generate verification token
	token := generateVerificationToken()
	
	// Get device ID from user's last device
	var device models.UserDevice
	err := o.QueryTable("user_devices").
		Filter("user_id", userID).
		OrderBy("-created_at").
		Limit(1).
		One(&device)
	
	if err == nil {
		// Create verification record
		verification := models.DeviceVerification{
			UserId:     userID,
			DeviceId:   device.DeviceId,
			Token:      token,
			IsVerified: false,
			IsRejected: false,
			ExpiresAt:  time.Now().Add(24 * time.Hour), // 24 hours expiry
		}
		o.Insert(&verification)
		
		// Get user email for verification link
		user := models.User{Id: userID}
		if o.Read(&user) == nil {
			emailConfig := GetEmailConfig()
			baseURL := getBaseURL()
			rejectLink := fmt.Sprintf("%s/reject-device?token=%s", baseURL, token)
			
			emailBody := fmt.Sprintf(`
				<h2 style="color: #dc3545;">⚠️ Perangkat Baru Terhubung</h2>
				<p>Akun Anda baru saja terhubung dari perangkat baru:</p>
				<p style="background-color: #f8f9fa; padding: 15px; border-radius: 5px; font-weight: bold;">%s</p>
				<p>Jika ini adalah Anda, tidak perlu melakukan apa-apa. Perangkat akan otomatis terverifikasi setelah 24 jam.</p>
				<p style="color: #dc3545; font-weight: bold; font-size: 16px;">Jika ini <strong>BUKAN</strong> Anda, segera klik tombol berikut untuk membatalkan akses:</p>
				<div style="text-align: center; margin: 30px 0;">
					<a href="%s" style="background-color: #dc3545; color: white; padding: 15px 30px; text-decoration: none; border-radius: 5px; display: inline-block; font-weight: bold; font-size: 16px;">🚫 Bukan Saya - Batalkan Akses</a>
				</div>
				<p style="color: #dc3545; font-weight: bold; border-left: 4px solid #dc3545; padding-left: 15px;">PENTING: Jika Anda tidak mengenali perangkat ini, segera ubah password Anda!</p>
				<p>Terima kasih,<br>Tim Psychee Wellness</p>
			`, deviceInfo, rejectLink)
			
			emailData := EmailData{
				To:      user.Email,
				Subject: "⚠️ Perangkat Baru Terhubung - Tindakan Diperlukan",
				Body:    emailBody,
			}
			SendEmail(emailConfig, emailData)
		}
	}
	
	title := "Perangkat Baru Terhubung"
	message := fmt.Sprintf("Akun Anda baru saja terhubung dari perangkat baru: %s. Cek email Anda untuk verifikasi keamanan.", deviceInfo)
	return SendNotification(userID, NotifTypeDeviceLink, title, message)
}

func generateVerificationToken() string {
	// Simple token generation (in production, use crypto/rand)
	return fmt.Sprintf("%d-%d", time.Now().Unix(), time.Now().UnixNano()%1000000)
}

func getBaseURL() string {
	// Get from app.conf atau gunakan default ke port Beego (8080)
	// Disarankan set BASE_URL di app.conf, misal:
	// BASE_URL = http://localhost:8080
	baseURL := beego.AppConfig.DefaultString("BASE_URL", "http://localhost:8080")
	return baseURL
}
