-- Tambah kolom jenis_kelamin ke tabel school_teachers.
-- Guru memiliki jenis kelamin masing-masing yang dipilih saat pendaftaran.
ALTER TABLE school_teachers
  ADD COLUMN IF NOT EXISTS jenis_kelamin VARCHAR(50) NOT NULL DEFAULT 'laki_laki';
