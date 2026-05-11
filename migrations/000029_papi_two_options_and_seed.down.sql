-- Rollback: kembalikan kolom-kolom lama (kosong) dan CHECK lama
ALTER TABLE papi_questions ADD COLUMN IF NOT EXISTS question_text TEXT NOT NULL DEFAULT '';
ALTER TABLE papi_questions ADD COLUMN IF NOT EXISTS option_c TEXT NOT NULL DEFAULT '';
ALTER TABLE papi_questions ADD COLUMN IF NOT EXISTS option_d TEXT NOT NULL DEFAULT '';
ALTER TABLE papi_questions ADD COLUMN IF NOT EXISTS category_c VARCHAR(8) NOT NULL DEFAULT '';
ALTER TABLE papi_questions ADD COLUMN IF NOT EXISTS category_d VARCHAR(8) NOT NULL DEFAULT '';

DO $$
DECLARE
    cname text;
BEGIN
    SELECT conname INTO cname
    FROM pg_constraint
    WHERE conrelid = 'papi_answers'::regclass
      AND contype = 'c'
      AND pg_get_constraintdef(oid) ILIKE '%selected_option%';
    IF cname IS NOT NULL THEN
        EXECUTE format('ALTER TABLE papi_answers DROP CONSTRAINT %I', cname);
    END IF;
END$$;

ALTER TABLE papi_answers
    ADD CONSTRAINT papi_answers_selected_option_check
    CHECK (selected_option IN ('A', 'B', 'C', 'D'));
