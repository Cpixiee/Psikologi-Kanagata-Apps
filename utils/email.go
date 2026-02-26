package utils

import (
	"bytes"
	"fmt"
	"html/template"
	"net/smtp"
	"os"

	beego "github.com/beego/beego/v2/server/web"
)

type EmailConfig struct {
	SMTPHost     string
	SMTPPort   string
	SMTPUser     string
	SMTPPassword string
	FromEmail    string
	FromName     string
}

func GetEmailConfig() EmailConfig {
	// Try to get from environment variables first, then from app.conf
	smtpHost := getEnv("SMTP_HOST", "")
	if smtpHost == "" {
		smtpHost = beego.AppConfig.DefaultString("SMTP_HOST", "smtp.gmail.com")
	}
	
	smtpPort := getEnv("SMTP_PORT", "")
	if smtpPort == "" {
		smtpPort = beego.AppConfig.DefaultString("SMTP_PORT", "587")
	}
	
	smtpUser := getEnv("SMTP_USER", "")
	if smtpUser == "" {
		smtpUser = beego.AppConfig.DefaultString("SMTP_USER", "")
	}
	// Trim whitespace
	if len(smtpUser) > 0 {
		smtpUser = trimSpace(smtpUser)
	}
	
	smtpPassword := getEnv("SMTP_PASSWORD", "")
	if smtpPassword == "" {
		smtpPassword = beego.AppConfig.DefaultString("SMTP_PASSWORD", "")
	}
	// Trim whitespace from password
	if len(smtpPassword) > 0 {
		smtpPassword = trimSpace(smtpPassword)
	}
	
	fromEmail := getEnv("FROM_EMAIL", "")
	if fromEmail == "" {
		fromEmail = beego.AppConfig.DefaultString("FROM_EMAIL", "kanagatapsikologi@gmail.com")
	}
	// Trim whitespace
	if len(fromEmail) > 0 {
		fromEmail = trimSpace(fromEmail)
	}
	
	fromName := getEnv("FROM_NAME", "")
	if fromName == "" {
		fromName = beego.AppConfig.DefaultString("FROM_NAME", "Psychee Wellness")
	}
	
	return EmailConfig{
		SMTPHost:     smtpHost,
		SMTPPort:     smtpPort,
		SMTPUser:     smtpUser,
		SMTPPassword: smtpPassword,
		FromEmail:    fromEmail,
		FromName:     fromName,
	}
}

func trimSpace(s string) string {
	// Remove ALL whitespace (spaces, tabs, newlines) from the string
	// This is important for App Passwords which might have spaces in the middle
	result := ""
	for _, char := range s {
		if char != ' ' && char != '\t' && char != '\n' && char != '\r' {
			result += string(char)
		}
	}
	return result
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

type EmailData struct {
	To      string
	Subject string
	Body    string
}

func SendEmail(config EmailConfig, data EmailData) error {
	// Remove ALL whitespace from password (App Passwords should have no spaces)
	password := trimSpace(config.SMTPPassword)
	
	// Setup authentication
	auth := smtp.PlainAuth("", config.SMTPUser, password, config.SMTPHost)

	// Create email message
	msg := []byte(fmt.Sprintf("From: %s <%s>\r\n", config.FromName, config.FromEmail) +
		fmt.Sprintf("To: %s\r\n", data.To) +
		fmt.Sprintf("Subject: %s\r\n", data.Subject) +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/html; charset=UTF-8\r\n" +
		"\r\n" +
		data.Body)

	// Send email
	addr := fmt.Sprintf("%s:%s", config.SMTPHost, config.SMTPPort)
	err := smtp.SendMail(addr, auth, config.FromEmail, []string{data.To}, msg)
	
	// Log error details for debugging
	if err != nil {
		return fmt.Errorf("SMTP error: %v (Host: %s, Port: %s, User: %s, Password length: %d)", 
			err, config.SMTPHost, config.SMTPPort, config.SMTPUser, len(password))
	}
	return err
}

func SendContactEmail(config EmailConfig, name, email, phone, message string) error {
	// Email to admin - use FROM_EMAIL as admin email
	adminEmail := config.FromEmail
	
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
	<head>
		<style>
			body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
			.container { max-width: 600px; margin: 0 auto; padding: 20px; }
			.header { background-color: #0d6efd; color: white; padding: 20px; text-align: center; }
			.content { background-color: #f8f9fa; padding: 20px; margin-top: 20px; }
			.field { margin-bottom: 15px; }
			.label { font-weight: bold; color: #0d6efd; }
			.value { margin-top: 5px; padding: 10px; background-color: white; border-radius: 5px; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h2>Pesan Baru dari Contact Form</h2>
			</div>
			<div class="content">
				<div class="field">
					<div class="label">Nama Lengkap:</div>
					<div class="value">{{.Name}}</div>
				</div>
				<div class="field">
					<div class="label">Email:</div>
					<div class="value">{{.Email}}</div>
				</div>
				<div class="field">
					<div class="label">Nomor Telepon:</div>
					<div class="value">{{.Phone}}</div>
				</div>
				<div class="field">
					<div class="label">Pesan:</div>
					<div class="value">{{.Message}}</div>
				</div>
			</div>
		</div>
	</body>
	</html>
	`

	tmpl, err := template.New("contact").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	err = tmpl.Execute(&body, map[string]string{
		"Name":    name,
		"Email":   email,
		"Phone":   phone,
		"Message": message,
	})
	if err != nil {
		return err
	}

	emailData := EmailData{
		To:      adminEmail,
		Subject: "Pesan Baru dari Contact Form - Psychee Wellness",
		Body:    body.String(),
	}

	return SendEmail(config, emailData)
}

func SendOTPEmail(config EmailConfig, email, otpCode string) error {
	htmlTemplate := `
	<!DOCTYPE html>
	<html>
	<head>
		<style>
			body { font-family: Arial, sans-serif; line-height: 1.6; color: #333; }
			.container { max-width: 600px; margin: 0 auto; padding: 20px; }
			.header { background-color: #0d6efd; color: white; padding: 20px; text-align: center; }
			.content { background-color: #f8f9fa; padding: 20px; margin-top: 20px; }
			.otp-box { background-color: white; padding: 20px; text-align: center; margin: 20px 0; border-radius: 10px; border: 2px solid #0d6efd; }
			.otp-code { font-size: 32px; font-weight: bold; color: #0d6efd; letter-spacing: 5px; }
			.warning { color: #dc3545; font-size: 14px; margin-top: 20px; }
		</style>
	</head>
	<body>
		<div class="container">
			<div class="header">
				<h2>Reset Password - Psychee Wellness</h2>
			</div>
			<div class="content">
				<p>Halo,</p>
				<p>Anda telah meminta untuk mereset password akun Anda. Gunakan kode OTP berikut untuk melanjutkan:</p>
				<div class="otp-box">
					<div class="otp-code">{{.OTP}}</div>
				</div>
				<p>Kode ini akan berlaku selama 15 menit.</p>
				<p class="warning">Jika Anda tidak meminta reset password, abaikan email ini.</p>
				<p>Terima kasih,<br>Tim Psychee Wellness</p>
			</div>
		</div>
	</body>
	</html>
	`

	tmpl, err := template.New("otp").Parse(htmlTemplate)
	if err != nil {
		return err
	}

	var body bytes.Buffer
	err = tmpl.Execute(&body, map[string]string{
		"OTP": otpCode,
	})
	if err != nil {
		return err
	}

	emailData := EmailData{
		To:      email,
		Subject: "Kode OTP Reset Password - Psychee Wellness",
		Body:    body.String(),
	}

	return SendEmail(config, emailData)
}
