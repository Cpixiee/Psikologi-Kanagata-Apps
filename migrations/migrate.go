package migrations

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	beego "github.com/beego/beego/v2/server/web"
)

type Migration struct {
	Version int64
	Up      string
	Down    string
}

type Migrator struct {
	db *sql.DB
}

func NewMigrator() (*Migrator, error) {
	// Get database connection string from config
	dbHost := beego.AppConfig.DefaultString("db_host", "localhost")
	dbPort := beego.AppConfig.DefaultString("db_port", "5432")
	dbUser := beego.AppConfig.DefaultString("db_user", "postgres")
	dbPassword := beego.AppConfig.DefaultString("db_password", "postgres")
	dbName := beego.AppConfig.DefaultString("db_name", "psikologi_db")
	dbSslMode := beego.AppConfig.DefaultString("db_sslmode", "disable")

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
		dbUser, dbPassword, dbHost, dbPort, dbName, dbSslMode)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %v", err)
	}

	return &Migrator{db: db}, nil
}

func (m *Migrator) Close() error {
	return m.db.Close()
}

func (m *Migrator) ensureSchemaMigrationsTable() error {
	query := `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version BIGINT PRIMARY KEY,
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`
	_, err := m.db.Exec(query)
	return err
}

func (m *Migrator) GetAppliedMigrations() (map[int64]bool, error) {
	if err := m.ensureSchemaMigrationsTable(); err != nil {
		return nil, err
	}

	rows, err := m.db.Query("SELECT version FROM schema_migrations ORDER BY version")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := make(map[int64]bool)
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, err
		}
		applied[version] = true
	}
	return applied, rows.Err()
}

func (m *Migrator) recordMigration(version int64) error {
	_, err := m.db.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)",
		version, time.Now())
	return err
}

func (m *Migrator) removeMigration(version int64) error {
	_, err := m.db.Exec("DELETE FROM schema_migrations WHERE version = $1", version)
	return err
}

func LoadMigrations(migrationsPath string) ([]Migration, error) {
	files, err := ioutil.ReadDir(migrationsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read migrations directory: %v", err)
	}

	migrationMap := make(map[int64]*Migration)

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filename := file.Name()
		parts := strings.Split(filename, "_")
		if len(parts) < 2 {
			continue
		}

		versionStr := parts[0]
		version, err := strconv.ParseInt(versionStr, 10, 64)
		if err != nil {
			continue
		}

		if migrationMap[version] == nil {
			migrationMap[version] = &Migration{Version: version}
		}

		fullPath := filepath.Join(migrationsPath, filename)
		content, err := ioutil.ReadFile(fullPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read migration file %s: %v", filename, err)
		}

		if strings.HasSuffix(filename, ".up.sql") {
			migrationMap[version].Up = string(content)
		} else if strings.HasSuffix(filename, ".down.sql") {
			migrationMap[version].Down = string(content)
		}
	}

	var migrations []Migration
	for _, migration := range migrationMap {
		if migration.Up == "" {
			return nil, fmt.Errorf("migration %d is missing .up.sql file", migration.Version)
		}
		migrations = append(migrations, *migration)
	}

	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

func (m *Migrator) Up() error {
	migrationsPath := filepath.Join(".", "migrations")
	migrations, err := LoadMigrations(migrationsPath)
	if err != nil {
		return err
	}

	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	for _, migration := range migrations {
		if applied[migration.Version] {
			fmt.Printf("Migration %d already applied, skipping...\n", migration.Version)
			continue
		}

		fmt.Printf("Applying migration %d...\n", migration.Version)

		tx, err := m.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %v", err)
		}

		if _, err := tx.Exec(migration.Up); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to apply migration %d: %v", migration.Version, err)
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)",
			migration.Version, time.Now()); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to record migration %d: %v", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration %d: %v", migration.Version, err)
		}

		fmt.Printf("Migration %d applied successfully\n", migration.Version)
	}

	return nil
}

func (m *Migrator) Down(steps int) error {
	migrationsPath := filepath.Join(".", "migrations")
	migrations, err := LoadMigrations(migrationsPath)
	if err != nil {
		return err
	}

	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	// Reverse order for down migrations
	for i := len(migrations) - 1; i >= 0 && steps > 0; i-- {
		migration := migrations[i]
		if !applied[migration.Version] {
			continue
		}

		if migration.Down == "" {
			fmt.Printf("Migration %d has no down migration, skipping...\n", migration.Version)
			continue
		}

		fmt.Printf("Rolling back migration %d...\n", migration.Version)

		tx, err := m.db.Begin()
		if err != nil {
			return fmt.Errorf("failed to begin transaction: %v", err)
		}

		if _, err := tx.Exec(migration.Down); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to rollback migration %d: %v", migration.Version, err)
		}

		if _, err := tx.Exec("DELETE FROM schema_migrations WHERE version = $1", migration.Version); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to remove migration record %d: %v", migration.Version, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit rollback %d: %v", migration.Version, err)
		}

		fmt.Printf("Migration %d rolled back successfully\n", migration.Version)
		steps--
	}

	return nil
}

func (m *Migrator) Status() error {
	migrationsPath := filepath.Join(".", "migrations")
	migrations, err := LoadMigrations(migrationsPath)
	if err != nil {
		return err
	}

	applied, err := m.GetAppliedMigrations()
	if err != nil {
		return err
	}

	fmt.Println("\nMigration Status:")
	fmt.Println("==================")
	for _, migration := range migrations {
		status := "PENDING"
		if applied[migration.Version] {
			status = "APPLIED"
		}
		fmt.Printf("Migration %d: %s\n", migration.Version, status)
	}
	fmt.Println()

	return nil
}
