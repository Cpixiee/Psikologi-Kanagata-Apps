-- Tabel untuk tracking progress peserta mengerjakan subtest IST
-- Setiap kali submit subtest, akan tercatat di sini
CREATE TABLE IF NOT EXISTS ist_progress (
    id SERIAL PRIMARY KEY,
    invitation_id INT NOT NULL REFERENCES test_invitations(id) ON DELETE CASCADE,
    subtest_code VARCHAR(10) NOT NULL, -- SE, WA, AN, GE, RA, ZR, FA, WU, ME
    status VARCHAR(20) NOT NULL DEFAULT 'completed', -- completed, in_progress
    completed_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(invitation_id, subtest_code)
);

CREATE INDEX IF NOT EXISTS idx_ist_progress_invitation_id ON ist_progress(invitation_id);
CREATE INDEX IF NOT EXISTS idx_ist_progress_subtest_code ON ist_progress(subtest_code);
