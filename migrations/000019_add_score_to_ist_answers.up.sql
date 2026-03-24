-- Add per-item score column for IST answers (needed for GE scoring 2/1/0)
ALTER TABLE ist_answers
  ADD COLUMN IF NOT EXISTS score INT NOT NULL DEFAULT 0;

-- Backfill existing data: is_correct => score 1, else 0
UPDATE ist_answers
SET score = CASE WHEN is_correct THEN 1 ELSE 0 END
WHERE score = 0;

