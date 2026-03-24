-- Update users role constraint to include new roles: mahasiswa and umum
-- Drop existing constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;

-- Add new constraint with all valid roles
ALTER TABLE users 
ADD CONSTRAINT users_role_check 
CHECK (role IN ('siswa', 'mahasiswa', 'guru', 'pekerja', 'umum', 'admin'));
