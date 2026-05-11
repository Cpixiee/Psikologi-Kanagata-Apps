-- Drop tabel PAPI
DROP TABLE IF EXISTS papi_results;
DROP TABLE IF EXISTS papi_answers;
DROP TABLE IF EXISTS papi_sessions;
DROP TABLE IF EXISTS papi_questions;

-- Hapus toggle PAPI dari test_batches
ALTER TABLE test_batches
    DROP COLUMN IF EXISTS enable_papi;
