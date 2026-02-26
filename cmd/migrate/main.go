package main

import (
	"flag"
	"fmt"
	"log"

	"psikologi_apps/migrations"
	beego "github.com/beego/beego/v2/server/web"
)

func main() {
	// Initialize Beego config
	beego.LoadAppConfig("ini", "conf/app.conf")

	var (
		command = flag.String("command", "up", "Migration command: up, down, status")
		steps   = flag.Int("steps", 1, "Number of migrations to rollback (for down command)")
	)
	flag.Parse()

	migrator, err := migrations.NewMigrator()
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	defer migrator.Close()

	switch *command {
	case "up":
		if err := migrator.Up(); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		fmt.Println("\nAll migrations applied successfully!")
	case "down":
		if err := migrator.Down(*steps); err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		fmt.Printf("\nRolled back %d migration(s) successfully!\n", *steps)
	case "status":
		if err := migrator.Status(); err != nil {
			log.Fatalf("Status check failed: %v", err)
		}
	default:
		log.Printf("Unknown command: %s\n", *command)
		fmt.Println("Usage: go run cmd/migrate/main.go -command=[up|down|status] [-steps=N]")
	}
}
