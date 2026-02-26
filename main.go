package main

import (
	"log"

	_ "psikologi_apps/models"
	_ "psikologi_apps/routers"
	"psikologi_apps/seeds"

	beego "github.com/beego/beego/v2/server/web"
)

func main() {
	// Seed IST data (9 subtests, sample questions, norms) - idempotent
	if err := seeds.SeedIST(); err != nil {
		log.Printf("IST seed warning: %v", err)
	}
	beego.Run()
}
