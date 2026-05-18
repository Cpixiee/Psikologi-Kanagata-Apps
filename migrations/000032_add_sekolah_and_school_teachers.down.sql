DROP INDEX IF EXISTS idx_users_sekolah;
DROP INDEX IF EXISTS idx_school_teachers_email;
DROP INDEX IF EXISTS idx_school_teachers_school_id;
DROP TABLE IF EXISTS school_teachers;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_sekolah_check;
ALTER TABLE users DROP COLUMN IF EXISTS sekolah;

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users
  ADD CONSTRAINT users_role_check
  CHECK (role IN ('siswa', 'mahasiswa', 'guru', 'pekerja', 'umum', 'admin'));
