-- Tanggal lahir disimpan sebagai VARCHAR(10) YYYY-MM-DD agar insert/update dari Beego stabil.

ALTER TABLE kraepelin_attempts
  ALTER COLUMN test_birth_date TYPE VARCHAR(10)
  USING (
    CASE
      WHEN test_birth_date IS NULL THEN NULL
      WHEN trim(test_birth_date::text) = '' THEN NULL
      ELSE to_char(test_birth_date::timestamptz, 'YYYY-MM-DD')
    END
  );
