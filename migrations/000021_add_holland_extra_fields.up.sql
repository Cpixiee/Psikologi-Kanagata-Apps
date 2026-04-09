-- Add extra profile fields for Holland test (page 3 answers)
ALTER TABLE IF EXISTS holland_results
ADD COLUMN IF NOT EXISTS dream_job_1 TEXT,
ADD COLUMN IF NOT EXISTS dream_job_2 TEXT,
ADD COLUMN IF NOT EXISTS dream_job_3 TEXT,
ADD COLUMN IF NOT EXISTS favorite_subject TEXT,
ADD COLUMN IF NOT EXISTS disliked_subject TEXT;

