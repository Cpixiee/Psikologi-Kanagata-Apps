-- Rollback: restore original constraint
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_role_check;

-- Restore original constraint (only siswa, guru, pekerja, admin)
ALTER TABLE users 
ADD CONSTRAINT users_role_check 
CHECK (role IN ('siswa', 'guru', 'pekerja', 'admin'));
