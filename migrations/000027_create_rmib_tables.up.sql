-- Tambahkan toggle RMIB ke test_batches
ALTER TABLE test_batches
    ADD COLUMN IF NOT EXISTS enable_rmib BOOLEAN NOT NULL DEFAULT FALSE;

-- Master soal RMIB (pria & wanita); item disimpan per (gender_version, group_number, item_order).
CREATE TABLE IF NOT EXISTS rmib_questions (
    id SERIAL PRIMARY KEY,
    gender_version VARCHAR(10) NOT NULL,           -- 'pria' atau 'wanita'
    group_number INT NOT NULL,                     -- 1..8
    group_title VARCHAR(255) NOT NULL,             -- mis. 'Persiapan Perjalanan'
    group_description TEXT,                        -- narasi singkat di halaman
    item_order INT NOT NULL,                       -- 1..12
    question_text TEXT NOT NULL,
    category_code VARCHAR(8) NOT NULL,             -- OUT, MEC, COMP, SCI, PERS, AEST, MUS, LIT, SOC, CLER, PRAC, MED
    UNIQUE (gender_version, group_number, item_order)
);

CREATE INDEX IF NOT EXISTS idx_rmib_questions_gv_group ON rmib_questions(gender_version, group_number);

-- Sesi pengerjaan RMIB per invitation.
CREATE TABLE IF NOT EXISTS rmib_sessions (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    batch_id INT REFERENCES test_batches(id) ON DELETE SET NULL,
    gender_version VARCHAR(10) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'in_progress', -- in_progress, finished
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL
);

CREATE INDEX IF NOT EXISTS idx_rmib_sessions_user ON rmib_sessions(user_id);

-- Jawaban ranking (auto-save). UPSERT by (session_id, question_id).
CREATE TABLE IF NOT EXISTS rmib_answers (
    id SERIAL PRIMARY KEY,
    session_id INT NOT NULL REFERENCES rmib_sessions(id) ON DELETE CASCADE,
    group_number INT NOT NULL,
    question_id INT NOT NULL REFERENCES rmib_questions(id) ON DELETE CASCADE,
    selected_rank INT NOT NULL CHECK (selected_rank BETWEEN 1 AND 12),
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (session_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_rmib_answers_session_group ON rmib_answers(session_id, group_number);

-- Hasil akhir per invitation (relasi 1-1, auto-cascade).
CREATE TABLE IF NOT EXISTS rmib_results (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    gender_version VARCHAR(10) NOT NULL,
    result_json TEXT NOT NULL,                     -- map kategori -> {score, rank}
    dominant_category VARCHAR(8) NOT NULL,
    top1 VARCHAR(8) NOT NULL,
    top2 VARCHAR(8) NOT NULL,
    top3 VARCHAR(8) NOT NULL,
    interpretation TEXT,
    completed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_rmib_results_user ON rmib_results(user_id);
