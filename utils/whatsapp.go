package utils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	beego "github.com/beego/beego/v2/server/web"
)

// WhatsAppConfig membaca konfigurasi WA dari app.conf / env.
type WhatsAppConfig struct {
	Enabled     bool
	Provider    string // fonnte | wablas | whacenter
	APIURL      string
	APIToken    string
	CountryCode string // misal: 62 (Indonesia)
}

// GetWhatsAppConfig membaca konfigurasi WA dengan default Fonnte & support env var.
func GetWhatsAppConfig() WhatsAppConfig {
	enabledStr := getEnv("WA_ENABLED", "")
	enabled := false
	if enabledStr != "" {
		enabled = strings.ToLower(enabledStr) == "true" || enabledStr == "1"
	} else {
		enabled = beego.AppConfig.DefaultBool("wa_enabled", false)
	}

	provider := getEnv("WA_PROVIDER", "")
	if provider == "" {
		provider = beego.AppConfig.DefaultString("wa_provider", "fonnte")
	}

	apiURL := getEnv("WA_API_URL", "")
	if apiURL == "" {
		apiURL = beego.AppConfig.DefaultString("wa_api_url", "https://api.fonnte.com/send")
	}

	apiToken := getEnv("WA_API_TOKEN", "")
	if apiToken == "" {
		apiToken = beego.AppConfig.DefaultString("wa_api_token", "")
	}

	countryCode := getEnv("WA_COUNTRY_CODE", "")
	if countryCode == "" {
		countryCode = beego.AppConfig.DefaultString("wa_country_code", "62")
	}

	return WhatsAppConfig{
		Enabled:     enabled,
		Provider:    strings.ToLower(provider),
		APIURL:      apiURL,
		APIToken:    apiToken,
		CountryCode: countryCode,
	}
}

// NormalizePhoneID mengubah nomor HP Indonesia ke format internasional tanpa "+".
//   "08123..."  -> "628123..."
//   "+628..."   -> "628..."
//   "8123..."   -> "628123..."
// Kosong / tidak valid -> "".
func NormalizePhoneID(raw, countryCode string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Hapus karakter non-digit kecuali leading +
	var b strings.Builder
	for i, ch := range s {
		if ch >= '0' && ch <= '9' {
			b.WriteRune(ch)
		} else if ch == '+' && i == 0 {
			// skip, hanya dibuang
		} else if ch == ' ' || ch == '-' || ch == '(' || ch == ')' {
			// skip
		}
	}
	digits := b.String()
	if digits == "" {
		return ""
	}
	if countryCode == "" {
		countryCode = "62"
	}
	if strings.HasPrefix(digits, countryCode) {
		return digits
	}
	if strings.HasPrefix(digits, "0") {
		return countryCode + digits[1:]
	}
	if strings.HasPrefix(digits, "8") {
		return countryCode + digits
	}
	return digits
}

// SendWhatsApp mengirim pesan WhatsApp via gateway.
// Untuk saat ini diimplementasikan untuk Fonnte (POST x-www-form-urlencoded
// header "Authorization: <token>" + body target=<phone>&message=<text>).
// Provider lain bisa ditambah di switch di bawah.
func SendWhatsApp(cfg WhatsAppConfig, toPhone, message string) error {
	if !cfg.Enabled {
		return fmt.Errorf("whatsapp gateway disabled (set wa_enabled=true di app.conf)")
	}
	if cfg.APIToken == "" {
		return fmt.Errorf("whatsapp api token kosong (isi wa_api_token di app.conf)")
	}
	target := NormalizePhoneID(toPhone, cfg.CountryCode)
	if target == "" {
		return fmt.Errorf("nomor whatsapp tidak valid: %q", toPhone)
	}

	switch cfg.Provider {
	case "fonnte", "":
		return sendFonnte(cfg, target, message)
	case "wablas":
		return sendWablas(cfg, target, message)
	default:
		// Generic fallback: POST form-urlencoded ke APIURL dengan Authorization
		return sendFonnte(cfg, target, message)
	}
}

func sendFonnte(cfg WhatsAppConfig, target, message string) error {
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = "https://api.fonnte.com/send"
	}
	form := url.Values{}
	form.Set("target", target)
	form.Set("message", message)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", cfg.APIToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fonnte request error: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("fonnte http %d: %s", resp.StatusCode, string(body))
	}
	// Fonnte mengembalikan JSON {status:true/false, reason, ...}
	var parsed struct {
		Status bool        `json:"status"`
		Reason interface{} `json:"reason"`
	}
	if jerr := json.Unmarshal(body, &parsed); jerr == nil && !parsed.Status {
		return fmt.Errorf("fonnte gagal: %v", parsed.Reason)
	}
	return nil
}

func sendWablas(cfg WhatsAppConfig, target, message string) error {
	apiURL := cfg.APIURL
	if apiURL == "" {
		apiURL = "https://console.wablas.com/api/send-message"
	}
	form := url.Values{}
	form.Set("phone", target)
	form.Set("message", message)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", cfg.APIToken)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("wablas request error: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("wablas http %d: %s", resp.StatusCode, string(body))
	}
	return nil
}
