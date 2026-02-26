# Database Migrations Guide

Sistem migrasi database ini memungkinkan Anda untuk mengelola perubahan skema database dengan cara yang terstruktur dan dapat di-rollback.

## Struktur Migrasi

Semua file migrasi berada di folder `migrations/` dengan format:
- `{version}_{description}.up.sql` - Migrasi ke atas (membuat/mengubah tabel)
- `{version}_{description}.down.sql` - Migrasi ke bawah (rollback)

Contoh:
- `000001_create_schema_migrations.up.sql`
- `000001_create_schema_migrations.down.sql`

## Menjalankan Migrasi

### 1. Apply semua migrasi yang belum dijalankan
```bash
go run cmd/migrate/main.go -command=up
```

### 2. Cek status migrasi
```bash
go run cmd/migrate/main.go -command=status
```

### 3. Rollback migrasi (1 langkah ke belakang)
```bash
go run cmd/migrate/main.go -command=down -steps=1
```

### 4. Rollback beberapa migrasi
```bash
go run cmd/migrate/main.go -command=down -steps=2
```

## Membuat Migrasi Baru

1. Buat file migrasi baru di folder `migrations/` dengan nomor versi yang lebih tinggi
2. Format nama file: `{version}_{deskripsi}.up.sql` dan `{version}_{deskripsi}.down.sql`
3. Contoh: `000003_add_user_status_column.up.sql` dan `000003_add_user_status_column.down.sql`

### Contoh Migrasi Baru

**000003_add_user_status_column.up.sql:**
```sql
ALTER TABLE users ADD COLUMN status VARCHAR(20) DEFAULT 'active';
CREATE INDEX idx_users_status ON users(status);
```

**000003_add_user_status_column.down.sql:**
```sql
DROP INDEX IF EXISTS idx_users_status;
ALTER TABLE users DROP COLUMN IF EXISTS status;
```

## Best Practices

1. **Selalu buat file `.down.sql`** untuk setiap migrasi agar bisa di-rollback
2. **Gunakan transaksi** - Migrator sudah menggunakan transaksi secara otomatis
3. **Test migrasi** di environment development sebelum production
4. **Backup database** sebelum menjalankan migrasi di production
5. **Version numbering** - Gunakan format 6 digit (000001, 000002, dst) untuk konsistensi

## Production Deployment

Sebelum deploy ke production:

1. Backup database:
```bash
pg_dump -U postgres psikologi_db > backup_$(date +%Y%m%d_%H%M%S).sql
```

2. Cek status migrasi:
```bash
go run cmd/migrate/main.go -command=status
```

3. Apply migrasi:
```bash
go run cmd/migrate/main.go -command=up
```

4. Verifikasi:
```bash
go run cmd/migrate/main.go -command=status
```

## Troubleshooting

### Error: "relation already exists"
- Pastikan migrasi sudah dijalankan sebelumnya
- Cek status dengan `-command=status`

### Error: "migration X already applied"
- Ini normal, migrator akan skip migrasi yang sudah dijalankan

### Rollback gagal
- Pastikan file `.down.sql` ada dan benar
- Cek apakah ada data yang menghalangi rollback (foreign key, dll)
