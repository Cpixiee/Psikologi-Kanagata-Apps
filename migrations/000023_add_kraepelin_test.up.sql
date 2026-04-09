-- Enable Kraepelin test per batch dan buat tabel attempt

ALTER TABLE test_batches
ADD COLUMN IF NOT EXISTS enable_kraepelin BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS kraepelin_attempts (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,

    test_name VARCHAR(255) NOT NULL DEFAULT '',
    test_gender VARCHAR(20) NOT NULL DEFAULT '',
    test_birth_place VARCHAR(255) NOT NULL DEFAULT '',
    -- Teks YYYY-MM-DD: menghindari format time.Time dari ORM yang ditolak PostgreSQL
    test_birth_date VARCHAR(10) NULL,
    test_age INT NOT NULL DEFAULT 0,
    test_address TEXT NOT NULL DEFAULT '',
    test_education VARCHAR(255) NOT NULL DEFAULT '',
    test_major VARCHAR(255) NOT NULL DEFAULT '',
    test_job VARCHAR(255) NULL,
    tester VARCHAR(255) NOT NULL DEFAULT '',
    test_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    column_count INT NOT NULL DEFAULT 40,
    digits_per_column INT NOT NULL DEFAULT 27,
    seconds_per_column INT NOT NULL DEFAULT 30,
    grace_seconds_on_switch INT NOT NULL DEFAULT 0,

    digits_json TEXT NOT NULL,
    answers_json TEXT NULL,
    correct_counts_json TEXT NULL,

    total_correct INT NOT NULL DEFAULT 0,
    total_errors INT NOT NULL DEFAULT 0,
    total_skipped INT NOT NULL DEFAULT 0,

    status VARCHAR(20) NOT NULL DEFAULT 'in_progress',
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at TIMESTAMP NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_kraepelin_attempts_user_id ON kraepelin_attempts(user_id);
CREATE INDEX IF NOT EXISTS idx_kraepelin_attempts_invitation_id ON kraepelin_attempts(invitation_id);

