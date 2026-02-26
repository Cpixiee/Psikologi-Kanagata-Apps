package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"psikologi_apps/models"
	"strings"
	"time"

	"github.com/beego/beego/v2/client/orm"
)

// GenerateDeviceId generates a unique device ID from user agent and IP
func GenerateDeviceId(userAgent, ipAddress string) string {
	data := fmt.Sprintf("%s|%s", userAgent, ipAddress)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for shorter ID
}

// ParseUserAgent extracts browser and OS info from user agent string
func ParseUserAgent(userAgent string) string {
	ua := strings.ToLower(userAgent)
	
	// Detect browser
	browser := "Unknown Browser"
	if strings.Contains(ua, "chrome") && !strings.Contains(ua, "edg") {
		browser = "Chrome"
	} else if strings.Contains(ua, "firefox") {
		browser = "Firefox"
	} else if strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome") {
		browser = "Safari"
	} else if strings.Contains(ua, "edg") {
		browser = "Edge"
	} else if strings.Contains(ua, "opera") {
		browser = "Opera"
	}
	
	// Detect OS
	os := "Unknown OS"
	if strings.Contains(ua, "windows") {
		os = "Windows"
	} else if strings.Contains(ua, "mac") {
		os = "macOS"
	} else if strings.Contains(ua, "linux") {
		os = "Linux"
	} else if strings.Contains(ua, "android") {
		os = "Android"
	} else if strings.Contains(ua, "ios") || strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") {
		os = "iOS"
	}
	
	return fmt.Sprintf("%s on %s", browser, os)
}

// CheckAndRegisterDevice checks if device is new and registers it
func CheckAndRegisterDevice(userID int, userAgent, ipAddress string) (isNew bool, device *models.UserDevice, err error) {
	o := orm.NewOrm()
	
	deviceId := GenerateDeviceId(userAgent, ipAddress)
	deviceName := ParseUserAgent(userAgent)
	
	// Check if device exists
	device = &models.UserDevice{DeviceId: deviceId}
	err = o.Read(device, "DeviceId")
	
	if err != nil {
		// Device not found, create new
		device = &models.UserDevice{
			UserId:      userID,
			DeviceId:    deviceId,
			DeviceName:  deviceName,
			BrowserInfo: userAgent,
			IpAddress:   ipAddress,
			IsVerified:  false,
			IsBlocked:   false,
		}
		_, err = o.Insert(device)
		if err != nil {
			return false, nil, err
		}
		return true, device, nil
	}
	
	// Device exists, update last used
	device.LastUsedAt = time.Now()
	o.Update(device, "LastUsedAt")
	
	return false, device, nil
}
