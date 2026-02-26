package seeds

import (
	"fmt"

	"psikologi_apps/models"

	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
)

// SeedAdmin membuat satu user admin jika belum ada.
// Email dan password bisa diatur lewat app.conf:
// admin_email, admin_password
func SeedAdmin() error {
	// Pastikan model sudah ter-register (import _ "psikologi_apps/models" di main caller)
	o := orm.NewOrm()

	adminEmail := beego.AppConfig.DefaultString("admin_email", "admin@psikologi.local")
	adminPassword := beego.AppConfig.DefaultString("admin_password", "admin123")

	// Cek apakah admin sudah ada
	existing := models.User{Email: adminEmail}
	err := o.Read(&existing, "Email")
	if err == nil {
		fmt.Println("Admin user already exists with email:", adminEmail)
		return nil
	}

	admin := models.User{
		NamaLengkap:  "Administrator",
		Email:        adminEmail,
		Password:     adminPassword,
		Role:         models.RoleAdmin,
		JenisKelamin: models.GenderLakiLaki, // default agar lolos constraint CHECK
	}

	if err := admin.HashPassword(); err != nil {
		return fmt.Errorf("failed to hash admin password: %w", err)
	}

	if _, err := o.Insert(&admin); err != nil {
		return fmt.Errorf("failed to insert admin user: %w", err)
	}

	fmt.Println("Admin user created with email:", adminEmail)
	return nil
}

