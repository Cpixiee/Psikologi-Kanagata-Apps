package main

import (
	"log"
	"os"

	"psikologi_apps/models"
	_ "psikologi_apps/routers"
	"psikologi_apps/seeds"

	beego "github.com/beego/beego/v2/server/web"
	"github.com/beego/beego/v2/core/logs"
)

func main() {
	// Setup logging ke file dan console
	// Log file akan disimpan di logs/app.log
	logDir := "logs"
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		os.Mkdir(logDir, 0755)
	}
	
	// Konfigurasi logging: console + file
	logs.SetLogger(logs.AdapterMultiFile, `{"filename":"logs/app.log","separate":["error","info","warning"],"level":7,"daily":true,"maxdays":7}`)
	logs.EnableFuncCallDepth(true)
	logs.SetLogFuncCallDepth(3)
	
	// Ensure ist_progress table exists (auto-create if not exists)
	if err := models.EnsureISTProgressTable(); err != nil {
		log.Printf("IST progress table creation warning: %v", err)
		logs.Warning("IST progress table creation warning: %v", err)
	}
	
	// Ensure IST norms exist (RW->SW and WS->IQ). Data can be provided via data/ist_norms.csv & data/ist_iq_norms.csv
	if err := seeds.EnsureISTNorms(); err != nil {
		log.Printf("IST norms warning: %v", err)
		logs.Warning("IST norms warning: %v", err)
	}

	// Seed IST full questions (176 soal lengkap)
	if err := seeds.SeedISTFull(); err != nil {
		log.Printf("IST full seed warning: %v", err)
		logs.Warning("IST full seed warning: %v", err)
	}
	
	logs.Info("Application starting on port 8081...")
	beego.Run()
}
