package models

import (
	"fmt"
	"github.com/beego/beego/v2/client/orm"
	beego "github.com/beego/beego/v2/server/web"
	_ "github.com/lib/pq"
)

func init() {
	orm.RegisterDriver("postgres", orm.DRPostgres)
	
	// Load database configuration from app.conf
	dbHost := beego.AppConfig.DefaultString("db_host", "localhost")
	dbPort := beego.AppConfig.DefaultString("db_port", "5432")
	dbUser := beego.AppConfig.DefaultString("db_user", "postgres")
	dbPassword := beego.AppConfig.DefaultString("db_password", "postgres")
	dbName := beego.AppConfig.DefaultString("db_name", "psikologi_db")
	dbSslMode := beego.AppConfig.DefaultString("db_sslmode", "disable")
	
	// Build connection string
	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPassword, dbHost, dbPort, dbName, dbSslMode)
	
	orm.RegisterDataBase("default", "postgres", connStr)
	
	// Note: Tables are created via migrations, not auto-sync
	// Run migrations using: go run cmd/migrate/main.go -command=up
}
