#!/bin/bash
# Automatic Database Backup Script for Psikologi Kanagata Apps
# Saves compressed full PostgreSQL backups with retention

set -e

BACKUP_DIR="/home/Infinity/backups/psikologi_db"
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="${BACKUP_DIR}/backup_${DATE}.sql.gz"

mkdir -p "$BACKUP_DIR"

echo "[$(date)] Starting PostgreSQL database backup..."
docker exec psikologi_db pg_dumpall -U postgres | gzip > "$BACKUP_FILE"

SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
echo "[$(date)] Backup completed successfully: ${BACKUP_FILE} (${SIZE})"

# Retention policy: Keep backups for 30 days
find "$BACKUP_DIR" -name "backup_*.sql.gz" -mtime +30 -delete 2>/dev/null || true
echo "[$(date)] Retention check completed (keeping 30 days of backups)."
