-- Add age_min and age_max columns to ist_iq_norms table
-- IQ norms are age-dependent, so we need to store age ranges for each norm entry
ALTER TABLE ist_iq_norms 
ADD COLUMN IF NOT EXISTS age_min INT DEFAULT 0,
ADD COLUMN IF NOT EXISTS age_max INT DEFAULT 99;

-- Update existing records to have default age range (0-99)
UPDATE ist_iq_norms SET age_min = 0, age_max = 99 WHERE age_min IS NULL OR age_max IS NULL;

-- Make age columns NOT NULL after setting defaults
ALTER TABLE ist_iq_norms 
ALTER COLUMN age_min SET NOT NULL,
ALTER COLUMN age_max SET NOT NULL;

-- Drop the unique constraint on total_standard_score since we now have multiple entries per score (one per age range)
-- First check if constraint exists, then drop it
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint 
        WHERE conname = 'ist_iq_norms_total_standard_score_key'
    ) THEN
        ALTER TABLE ist_iq_norms DROP CONSTRAINT ist_iq_norms_total_standard_score_key;
    END IF;
END $$;

-- Create new unique constraint that includes age range
CREATE UNIQUE INDEX IF NOT EXISTS idx_ist_iq_norms_unique ON ist_iq_norms(total_standard_score, age_min, age_max);
