package main

import (
	"log"

	"psikologi_apps/seeds"

	_ "psikologi_apps/models"

	beego "github.com/beego/beego/v2/server/web"
)

func main() {
	// Load config (DB, admin email/password, etc.)
	if err := beego.LoadAppConfig("ini", "conf/app.conf"); err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	if err := seeds.SeedAdmin(); err != nil {
		log.Fatalf("failed to seed admin user: %v", err)
	}
}

