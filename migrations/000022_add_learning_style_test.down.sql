DROP TABLE IF EXISTS learning_style_results;
DROP TABLE IF EXISTS learning_style_answers;
DROP TABLE IF EXISTS learning_style_questions;

ALTER TABLE test_batches
DROP COLUMN IF EXISTS enable_learning_style;
