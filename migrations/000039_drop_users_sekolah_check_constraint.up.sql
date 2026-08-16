-- Drop restrictive users_sekolah_check constraint and extend sekolah column size
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_sekolah_check;
ALTER TABLE users ALTER COLUMN sekolah TYPE VARCHAR(255);
