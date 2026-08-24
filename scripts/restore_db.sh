#!/bin/bash
# Database Restore Script for Psikologi Kanagata Apps
# Usage: ./scripts/restore_db.sh /home/Infinity/backups/psikologi_db/backup_20260824_205619.sql.gz

set -e

BACKUP_FILE="$1"
if [ -z "$BACKUP_FILE" ]; then
  echo "Usage: $0 <path_to_backup_file.sql.gz>"
  echo "Example: $0 /home/Infinity/backups/psikologi_db/backup_20260824_205619.sql.gz"
  exit 1
fi

if [ ! -f "$BACKUP_FILE" ]; then
  echo "Error: Backup file '$BACKUP_FILE' not found!"
  exit 1
fi

echo "[$(date)] Restoring database from: ${BACKUP_FILE}..."
gunzip -c "$BACKUP_FILE" | docker exec -i psikologi_db psql -U postgres -d postgres
echo "[$(date)] Database restore completed successfully!"
