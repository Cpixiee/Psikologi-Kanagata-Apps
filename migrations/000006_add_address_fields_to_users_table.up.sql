-- Add address fields to users table
ALTER TABLE users ADD COLUMN kota VARCHAR(100) NULL;
ALTER TABLE users ADD COLUMN provinsi VARCHAR(100) NULL;
ALTER TABLE users ADD COLUMN kodepos VARCHAR(10) NULL;
