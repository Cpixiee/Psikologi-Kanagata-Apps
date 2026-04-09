ALTER TABLE test_batches
ADD COLUMN IF NOT EXISTS enable_learning_style BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS learning_style_questions (
    id SERIAL PRIMARY KEY,
    number INT NOT NULL UNIQUE,
    statement TEXT NOT NULL,
    dimension CHAR(1) NOT NULL
);

CREATE TABLE IF NOT EXISTS learning_style_answers (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    question_id INT NOT NULL REFERENCES learning_style_questions(id) ON DELETE CASCADE,
    answer_yes INT NOT NULL DEFAULT 0,
    answer_no INT NOT NULL DEFAULT 0,
    answered_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT learning_style_answers_unique UNIQUE(invitation_id, question_id),
    CONSTRAINT learning_style_answer_binary_check CHECK (
        (answer_yes = 1 AND answer_no = 0) OR
        (answer_yes = 0 AND answer_no = 1)
    )
);

CREATE INDEX IF NOT EXISTS idx_learning_style_answers_invitation ON learning_style_answers(invitation_id);
CREATE INDEX IF NOT EXISTS idx_learning_style_answers_user ON learning_style_answers(user_id);

CREATE TABLE IF NOT EXISTS learning_style_results (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL UNIQUE REFERENCES test_invitations(id) ON DELETE CASCADE,
    user_id INT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    test_name VARCHAR(255) NOT NULL DEFAULT '',
    test_age INT NOT NULL DEFAULT 0,
    test_institution VARCHAR(255) NOT NULL DEFAULT '',
    test_gender VARCHAR(20) NOT NULL DEFAULT '',
    test_date TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    score_visual INT NOT NULL DEFAULT 0,
    score_auditory INT NOT NULL DEFAULT 0,
    score_kinesthetic INT NOT NULL DEFAULT 0,
    dominant_type VARCHAR(20) NOT NULL DEFAULT '',
    interpretation_visual TEXT,
    interpretation_auditory TEXT,
    interpretation_kinesthetic TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_learning_style_results_user ON learning_style_results(user_id);
