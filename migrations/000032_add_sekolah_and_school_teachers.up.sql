-- Tambah role 'sekolah' ke constraint role.
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;
ALTER TABLE users
  ADD CONSTRAINT users_role_check
  CHECK (role IN ('siswa', 'mahasiswa', 'guru', 'pekerja', 'umum', 'admin', 'sekolah'));

-- Kolom 'sekolah' (enum string) untuk siswa & akun sekolah.
-- Daftar saat ini (dummy): SMKN 22 Jakarta, SMKN 46 Jakarta, SMKN 43 Jakarta,
-- SMKN 20 Jakarta, SMKN 70 Jakarta. Disimpan sebagai VARCHAR + CHECK supaya
-- mudah diperluas tanpa membuat tipe ENUM Postgres.
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS sekolah VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_sekolah_check;
ALTER TABLE users
  ADD CONSTRAINT users_sekolah_check
  CHECK (sekolah IN (
    '',
    'SMKN 22 Jakarta',
    'SMKN 46 Jakarta',
    'SMKN 43 Jakarta',
    'SMKN 20 Jakarta',
    'SMKN 70 Jakarta'
  ));

-- Tabel daftar guru per akun sekolah. Guru login dengan email mereka
-- sendiri, password yang diverifikasi adalah password akun sekolah induk.
CREATE TABLE IF NOT EXISTS school_teachers (
  id          SERIAL PRIMARY KEY,
  school_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  nama        VARCHAR(255) NOT NULL DEFAULT '',
  kelas       VARCHAR(100) NOT NULL DEFAULT '',
  email       VARCHAR(255) NOT NULL UNIQUE,
  created_at  TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_school_teachers_school_id ON school_teachers(school_id);
CREATE INDEX IF NOT EXISTS idx_school_teachers_email ON school_teachers(email);
CREATE INDEX IF NOT EXISTS idx_users_sekolah ON users(sekolah);
