-- Tambah field-field onboarding profil pengguna (NISN/NIP, kelas, jurusan,
-- tempat lahir, kecamatan, dan flag apakah profil sudah dilengkapi).
ALTER TABLE users
  ADD COLUMN IF NOT EXISTS nisn VARCHAR(20) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS nip VARCHAR(30) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS kelas VARCHAR(50) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS jurusan VARCHAR(100) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS tempat_lahir VARCHAR(100) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS kecamatan VARCHAR(100) NOT NULL DEFAULT '',
  ADD COLUMN IF NOT EXISTS profile_completed BOOLEAN NOT NULL DEFAULT FALSE;
