package utils

import (
	"strings"

	beego "github.com/beego/beego/v2/server/web"
)

// GetAppBaseURL returns the public domain base URL for invitation emails, WhatsApp messages, and OAuth callbacks.
func GetAppBaseURL() string {
	baseURL := strings.TrimSpace(beego.AppConfig.DefaultString("BASE_URL", ""))
	if baseURL == "" {
		baseURL = strings.TrimSpace(beego.AppConfig.DefaultString("app_url", ""))
	}

	// Replace outdated hardcoded localhost:112 port or empty config with official production domain
	if baseURL == "" || strings.Contains(baseURL, "localhost:112") {
		runmode := strings.ToLower(beego.AppConfig.DefaultString("runmode", "prod"))
		if runmode == "dev" {
			baseURL = "http://localhost:8086"
		} else {
			baseURL = "https://psikotes.kanagata.co.id"
		}
	}

	return strings.TrimRight(baseURL, "/")
}
