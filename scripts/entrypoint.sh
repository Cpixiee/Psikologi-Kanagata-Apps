#!/bin/sh
set -eu

CONFIG_FILE="/app/conf/app.conf"

update_ini() {
  key="$1"
  value="$2"

  awk -v k="$key" -v v="$value" '
    BEGIN { updated = 0 }
    $0 ~ "^[[:space:]]*" k "[[:space:]]*=" {
      print k " = " v
      updated = 1
      next
    }
    { print }
    END {
      if (updated == 0) {
        print k " = " v
      }
    }
  ' "$CONFIG_FILE" > "${CONFIG_FILE}.tmp"

  mv "${CONFIG_FILE}.tmp" "$CONFIG_FILE"
}

mkdir -p /app/logs /app/static/uploads/profiles

APP_HTTP_PORT="${APP_HTTP_PORT:-8081}"
APP_RUNMODE="${APP_RUNMODE:-prod}"
DB_HOST="${DB_HOST:-db}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_NAME="${DB_NAME:-psikologi_db}"
DB_SSLMODE="${DB_SSLMODE:-disable}"
SMTP_HOST="${SMTP_HOST:-}"
SMTP_PORT="${SMTP_PORT:-587}"
SMTP_USER="${SMTP_USER:-}"
SMTP_PASSWORD="${SMTP_PASSWORD:-}"
FROM_EMAIL="${FROM_EMAIL:-}"
FROM_NAME="${FROM_NAME:-Psychee Wellness}"
ADMIN_EMAIL="${ADMIN_EMAIL:-admin@example.com}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-changeme123}"
BASE_URL="${BASE_URL:-http://localhost:8081}"

update_ini "httpport" "$APP_HTTP_PORT"
update_ini "runmode" "$APP_RUNMODE"
update_ini "db_host" "$DB_HOST"
update_ini "db_port" "$DB_PORT"
update_ini "db_user" "$DB_USER"
update_ini "db_password" "$DB_PASSWORD"
update_ini "db_name" "$DB_NAME"
update_ini "db_sslmode" "$DB_SSLMODE"
update_ini "SMTP_HOST" "$SMTP_HOST"
update_ini "SMTP_PORT" "$SMTP_PORT"
update_ini "SMTP_USER" "$SMTP_USER"
update_ini "SMTP_PASSWORD" "$SMTP_PASSWORD"
update_ini "FROM_EMAIL" "$FROM_EMAIL"
update_ini "FROM_NAME" "$FROM_NAME"
update_ini "admin_email" "$ADMIN_EMAIL"
update_ini "admin_password" "$ADMIN_PASSWORD"
update_ini "BASE_URL" "$BASE_URL"

if [ "${AUTO_MIGRATE:-true}" = "true" ]; then
  echo "Running migration up..."
  /app/migrate -command=up
fi

if [ "${AUTO_SEED_ADMIN:-false}" = "true" ]; then
  echo "Running admin seed..."
  /app/seed
fi

echo "Starting application..."
exec /app/app
