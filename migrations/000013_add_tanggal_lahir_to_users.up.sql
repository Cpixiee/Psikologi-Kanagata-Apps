-- Add optional tanggal lahir for IST (usia dibutuhkan untuk skoring)
ALTER TABLE users ADD COLUMN IF NOT EXISTS tanggal_lahir DATE NULL;
