-- Rollback untuk skema tes Kraepelin

DROP TABLE IF EXISTS kraepelin_attempts;

ALTER TABLE test_batches
DROP COLUMN IF EXISTS enable_kraepelin;

