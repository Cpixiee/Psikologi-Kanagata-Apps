-- Tambahkan toggle PAPI ke test_batches
ALTER TABLE test_batches
    ADD COLUMN IF NOT EXISTS enable_papi BOOLEAN NOT NULL DEFAULT FALSE;

-- Master soal PAPI (90 item dengan format paired comparison)
CREATE TABLE IF NOT EXISTS papi_questions (
    id SERIAL PRIMARY KEY,
    item_number INT NOT NULL,                       -- 1..90
    question_text TEXT NOT NULL,                    -- teks pertanyaan
    option_a TEXT NOT NULL,                         -- pilihan A
    option_b TEXT NOT NULL,                         -- pilihan B
    option_c TEXT NOT NULL,                         -- pilihan C
    option_d TEXT NOT NULL,                         -- pilihan D
    category_a VARCHAR(8) NOT NULL,                 -- kategori untuk pilihan A
    category_b VARCHAR(8) NOT NULL,                 -- kategori untuk pilihan B
    category_c VARCHAR(8) NOT NULL,                 -- kategori untuk pilihan C
    category_d VARCHAR(8) NOT NULL,                 -- kategori untuk pilihan D
    UNIQUE (item_number)
);

CREATE INDEX IF NOT EXISTS idx_papi_questions_item ON papi_questions(item_number);

-- Sesi pengerjaan PAPI per invitation
CREATE TABLE IF NOT EXISTS papi_sessions (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    batch_id INT REFERENCES test_batches(id) ON DELETE SET NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'in_progress', -- in_progress, finished, expired
    started_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP NULL,
    time_limit_minutes INT NOT NULL DEFAULT 60,     -- 60 menit
    time_remaining_seconds INT NOT NULL DEFAULT 3600 -- 3600 detik = 60 menit
);

CREATE INDEX IF NOT EXISTS idx_papi_sessions_user ON papi_sessions(user_id);

-- Jawaban PAPI (auto-save). UPSERT by (session_id, question_id)
CREATE TABLE IF NOT EXISTS papi_answers (
    id SERIAL PRIMARY KEY,
    session_id INT NOT NULL REFERENCES papi_sessions(id) ON DELETE CASCADE,
    question_id INT NOT NULL REFERENCES papi_questions(id) ON DELETE CASCADE,
    selected_option CHAR(1) NOT NULL CHECK (selected_option IN ('A', 'B', 'C', 'D')),
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (session_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_papi_answers_session ON papi_answers(session_id);

-- Hasil akhir PAPI per invitation (relasi 1-1, auto-cascade)
CREATE TABLE IF NOT EXISTS papi_results (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    result_json TEXT NOT NULL,                      -- JSON dengan skor per kategori
    dominant_category VARCHAR(8) NOT NULL,          -- kategori dominan
    top_categories TEXT,                            -- JSON array top 3 kategori
    interpretation TEXT,                            -- interpretasi hasil
    completed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    time_taken_minutes INT NOT NULL                 -- waktu yang diperlukan (menit)
);

CREATE INDEX IF NOT EXISTS idx_papi_results_user ON papi_results(user_id);
