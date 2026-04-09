-- Add asal instansi field to users profile
ALTER TABLE users ADD COLUMN IF NOT EXISTS asal_instansi VARCHAR(255) NULL;
