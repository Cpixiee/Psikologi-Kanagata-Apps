-- Add role column to users table (for existing databases where users table was created without role)
ALTER TABLE users
ADD COLUMN IF NOT EXISTS role VARCHAR(20) NOT NULL DEFAULT 'siswa'
    CHECK (role IN ('siswa', 'guru', 'pekerja', 'admin'));

