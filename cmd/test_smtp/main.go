package main

import (
	"fmt"
	"net/smtp"
	"psikologi_apps/utils"
	beego "github.com/beego/beego/v2/server/web"
)

func main() {
	// Load config
	beego.LoadAppConfig("ini", "../../conf/app.conf")
	
	// Get email config
	config := utils.GetEmailConfig()
	
	fmt.Println("=== SMTP Configuration Test ===")
	fmt.Printf("SMTP Host: %s\n", config.SMTPHost)
	fmt.Printf("SMTP Port: %s\n", config.SMTPPort)
	fmt.Printf("SMTP User: %s\n", config.SMTPUser)
	fmt.Printf("Password Length: %d\n", len(config.SMTPPassword))
	if len(config.SMTPPassword) > 0 {
		showLen := 4
		if len(config.SMTPPassword) < 4 {
			showLen = len(config.SMTPPassword)
		}
		fmt.Printf("Password (first %d chars): %s****\n", showLen, config.SMTPPassword[:showLen])
	}
	fmt.Printf("From Email: %s\n", config.FromEmail)
	fmt.Println()
	
	// Test SMTP connection
	fmt.Println("Testing SMTP connection...")
	addr := fmt.Sprintf("%s:%s", config.SMTPHost, config.SMTPPort)
	auth := smtp.PlainAuth("", config.SMTPUser, config.SMTPPassword, config.SMTPHost)
	
	// Try to connect and authenticate
	client, err := smtp.Dial(addr)
	if err != nil {
		fmt.Printf("❌ Failed to connect to SMTP server: %v\n", err)
		return
	}
	defer client.Close()
	
	// Start TLS
	if err := client.StartTLS(nil); err != nil {
		fmt.Printf("❌ Failed to start TLS: %v\n", err)
		return
	}
	
	// Authenticate
	if err := client.Auth(auth); err != nil {
		fmt.Printf("❌ Authentication failed: %v\n", err)
		fmt.Println()
		fmt.Println("=== SOLUSI ===")
		fmt.Println("1. Buka: https://myaccount.google.com/apppasswords")
		fmt.Println("2. Login dengan: kanagatapsikologi@gmail.com")
		fmt.Println("3. Pastikan 2-Step Verification SUDAH AKTIF")
		fmt.Println("4. Hapus App Password lama (jika ada)")
		fmt.Println("5. Buat App Password BARU:")
		fmt.Println("   - Pilih 'Mail'")
		fmt.Println("   - Pilih 'Other (Custom name)'")
		fmt.Println("   - Nama: 'Psychee Wellness'")
		fmt.Println("   - Generate")
		fmt.Println("6. Copy App Password (16 karakter, contoh: abcd efgh ijkl mnop)")
		fmt.Println("7. HAPUS SEMUA SPASI (jadi: abcdefghijklmnop)")
		fmt.Println("8. Update di conf/app.conf:")
		fmt.Println("   SMTP_PASSWORD =[PASTE_DI_SINI_TANPA_SPASI]")
		fmt.Println("9. Restart aplikasi")
		return
	}
	
	fmt.Println("✅ SMTP connection and authentication successful!")
	fmt.Println("✅ Email configuration is CORRECT!")
	fmt.Println()
	fmt.Println("Jika masih error saat kirim email, cek:")
	fmt.Println("- Firewall tidak memblokir port 587")
	fmt.Println("- Internet connection stabil")
}
