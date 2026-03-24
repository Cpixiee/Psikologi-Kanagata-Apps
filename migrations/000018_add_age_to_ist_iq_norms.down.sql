-- Remove age columns from ist_iq_norms table
ALTER TABLE ist_iq_norms DROP COLUMN IF EXISTS age_min;
ALTER TABLE ist_iq_norms DROP COLUMN IF EXISTS age_max;

-- Drop the unique index
DROP INDEX IF EXISTS idx_ist_iq_norms_unique;

-- Restore the original unique constraint on total_standard_score
ALTER TABLE ist_iq_norms ADD CONSTRAINT ist_iq_norms_total_standard_score_key UNIQUE (total_standard_score);
