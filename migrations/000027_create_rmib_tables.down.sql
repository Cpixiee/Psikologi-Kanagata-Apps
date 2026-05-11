DROP TABLE IF EXISTS rmib_results;
DROP TABLE IF EXISTS rmib_answers;
DROP TABLE IF EXISTS rmib_sessions;
DROP TABLE IF EXISTS rmib_questions;

ALTER TABLE test_batches DROP COLUMN IF EXISTS enable_rmib;
