ALTER TABLE users
  DROP COLUMN IF EXISTS no_nik,
  DROP COLUMN IF EXISTS no_kk,
  DROP COLUMN IF EXISTS nomor_akta_lahir,
  DROP COLUMN IF EXISTS agama,
  DROP COLUMN IF EXISTS tempat_tinggal,
  DROP COLUMN IF EXISTS mode_transportasi,
  DROP COLUMN IF EXISTS anak_ke,
  DROP COLUMN IF EXISTS jumlah_bersaudara,
  DROP COLUMN IF EXISTS riwayat_penyakit;
