-- Create table for test batches (sesi tes per institusi)
CREATE TABLE IF NOT EXISTS test_batches (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    institution VARCHAR(255) NOT NULL,
    enable_ist BOOLEAN NOT NULL DEFAULT TRUE,
    enable_holland BOOLEAN NOT NULL DEFAULT FALSE,
    purpose_category VARCHAR(50) NOT NULL, -- education, career, other
    purpose_detail VARCHAR(100) NOT NULL,  -- e.g. sekolah, identifikasi_kecerdasan, pengembangan_potensi
    send_via_email BOOLEAN NOT NULL DEFAULT TRUE,
    send_via_browser BOOLEAN NOT NULL DEFAULT FALSE,
    created_by INT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Invitations for each participant (linked by email and token)
CREATE TABLE IF NOT EXISTS test_invitations (
    id SERIAL PRIMARY KEY,
    batch_id INT NOT NULL REFERENCES test_batches(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    user_id INT REFERENCES users(id) ON DELETE SET NULL,
    token VARCHAR(64) NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending', -- pending, used, expired, canceled
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_test_invitations_batch_id ON test_invitations(batch_id);
CREATE INDEX IF NOT EXISTS idx_test_invitations_email ON test_invitations(email);

-- Master IST subtests (SE, WA, AN, ME, RA, ZA, FA, WU, GE)
CREATE TABLE IF NOT EXISTS ist_subtests (
    id SERIAL PRIMARY KEY,
    code VARCHAR(10) NOT NULL UNIQUE,
    name VARCHAR(100) NOT NULL,
    order_index INT NOT NULL
);

-- Master IST questions (text-based, optional image)
CREATE TABLE IF NOT EXISTS ist_questions (
    id SERIAL PRIMARY KEY,
    subtest_id INT NOT NULL REFERENCES ist_subtests(id) ON DELETE CASCADE,
    number INT NOT NULL, -- 1-20 per subtest
    prompt TEXT NOT NULL,
    option_a TEXT NOT NULL,
    option_b TEXT NOT NULL,
    option_c TEXT NOT NULL,
    option_d TEXT NOT NULL,
    option_e TEXT NOT NULL,
    correct_option CHAR(1) NOT NULL, -- A-E
    image_url TEXT NULL,
    UNIQUE(subtest_id, number)
);

CREATE INDEX IF NOT EXISTS idx_ist_questions_subtest_id ON ist_questions(subtest_id);

-- Per-question answers for IST (needed for detailed review & export)
CREATE TABLE IF NOT EXISTS ist_answers (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    subtest_id INT NOT NULL REFERENCES ist_subtests(id) ON DELETE CASCADE,
    question_id INT NOT NULL REFERENCES ist_questions(id) ON DELETE CASCADE,
    answer_option CHAR(1) NOT NULL, -- A-E
    is_correct BOOLEAN NOT NULL,
    answered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(invitation_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_ist_answers_invitation_id ON ist_answers(invitation_id);
CREATE INDEX IF NOT EXISTS idx_ist_answers_user_id ON ist_answers(user_id);

-- Raw & standard scores + IQ per invitation
CREATE TABLE IF NOT EXISTS ist_results (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- raw scores
    raw_se INT NOT NULL DEFAULT 0,
    raw_wa INT NOT NULL DEFAULT 0,
    raw_an INT NOT NULL DEFAULT 0,
    raw_me INT NOT NULL DEFAULT 0,
    raw_ra INT NOT NULL DEFAULT 0,
    raw_za INT NOT NULL DEFAULT 0,
    raw_fa INT NOT NULL DEFAULT 0,
    raw_wu INT NOT NULL DEFAULT 0,
    raw_ge INT NOT NULL DEFAULT 0,
    -- standard scores (1-7)
    std_se INT NOT NULL DEFAULT 0,
    std_wa INT NOT NULL DEFAULT 0,
    std_an INT NOT NULL DEFAULT 0,
    std_me INT NOT NULL DEFAULT 0,
    std_ra INT NOT NULL DEFAULT 0,
    std_za INT NOT NULL DEFAULT 0,
    std_fa INT NOT NULL DEFAULT 0,
    std_wu INT NOT NULL DEFAULT 0,
    std_ge INT NOT NULL DEFAULT 0,
    total_standard_score INT NOT NULL DEFAULT 0,
    iq INT NOT NULL DEFAULT 0,
    iq_category VARCHAR(100) NOT NULL DEFAULT '',
    strengths TEXT,
    weaknesses TEXT,
    summary TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_ist_results_user_id ON ist_results(user_id);

-- Norm tables for IST (raw->standard) and total_standard->IQ
CREATE TABLE IF NOT EXISTS ist_norms (
    id SERIAL PRIMARY KEY,
    subtest_code VARCHAR(10) NOT NULL,
    age_min INT NOT NULL,
    age_max INT NOT NULL,
    raw_score INT NOT NULL,
    standard_score INT NOT NULL,
    UNIQUE(subtest_code, age_min, age_max, raw_score)
);

CREATE TABLE IF NOT EXISTS ist_iq_norms (
    id SERIAL PRIMARY KEY,
    total_standard_score INT NOT NULL UNIQUE,
    iq INT NOT NULL,
    category VARCHAR(100) NOT NULL
);

-- Holland master questions and descriptions
CREATE TABLE IF NOT EXISTS holland_questions (
    id SERIAL PRIMARY KEY,
    code CHAR(1) NOT NULL, -- R, I, A, S, E, C
    number INT NOT NULL,
    prompt TEXT NOT NULL,
    answer_type VARCHAR(20) NOT NULL, -- yes_no, scale
    UNIQUE(code, number)
);

CREATE TABLE IF NOT EXISTS holland_descriptions (
    id SERIAL PRIMARY KEY,
    code CHAR(1) NOT NULL UNIQUE,
    title VARCHAR(100) NOT NULL,
    description TEXT NOT NULL,
    recommended_majors TEXT,
    recommended_jobs TEXT
);

-- Answers & results for Holland
CREATE TABLE IF NOT EXISTS holland_answers (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id INT NOT NULL REFERENCES holland_questions(id) ON DELETE CASCADE,
    value INT NOT NULL, -- 0/1 for yes_no, 1-5 for scale
    answered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(invitation_id, question_id)
);

CREATE INDEX IF NOT EXISTS idx_holland_answers_invitation_id ON holland_answers(invitation_id);

CREATE TABLE IF NOT EXISTS holland_results (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    score_r INT NOT NULL DEFAULT 0,
    score_i INT NOT NULL DEFAULT 0,
    score_a INT NOT NULL DEFAULT 0,
    score_s INT NOT NULL DEFAULT 0,
    score_e INT NOT NULL DEFAULT 0,
    score_c INT NOT NULL DEFAULT 0,
    top1 CHAR(1),
    top2 CHAR(1),
    top3 CHAR(1),
    code VARCHAR(3),
    interpretation TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_holland_results_user_id ON holland_results(user_id);

