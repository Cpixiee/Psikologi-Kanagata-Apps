-- Rollback: hapus kolom jenis_kelamin dari school_teachers.
ALTER TABLE school_teachers DROP COLUMN IF EXISTS jenis_kelamin;
