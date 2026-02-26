DROP TABLE IF EXISTS holland_results;
DROP INDEX IF EXISTS idx_holland_results_user_id;

DROP TABLE IF EXISTS holland_answers;
DROP INDEX IF EXISTS idx_holland_answers_invitation_id;

DROP TABLE IF EXISTS holland_descriptions;
DROP TABLE IF EXISTS holland_questions;

DROP TABLE IF EXISTS ist_iq_norms;
DROP TABLE IF EXISTS ist_norms;

DROP TABLE IF EXISTS ist_results;
DROP INDEX IF EXISTS idx_ist_results_user_id;

DROP TABLE IF EXISTS ist_answers;
DROP INDEX IF EXISTS idx_ist_answers_invitation_id;
DROP INDEX IF EXISTS idx_ist_answers_user_id;

DROP TABLE IF EXISTS ist_questions;
DROP INDEX IF EXISTS idx_ist_questions_subtest_id;

DROP TABLE IF EXISTS ist_subtests;

DROP TABLE IF EXISTS test_invitations;
DROP INDEX IF EXISTS idx_test_invitations_batch_id;
DROP INDEX IF EXISTS idx_test_invitations_email;

DROP TABLE IF EXISTS test_batches;

