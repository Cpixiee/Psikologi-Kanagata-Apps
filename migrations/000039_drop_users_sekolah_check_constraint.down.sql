-- Down migration 39
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_sekolah_check;
