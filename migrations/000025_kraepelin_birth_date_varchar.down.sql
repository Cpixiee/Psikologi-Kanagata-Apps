ALTER TABLE kraepelin_attempts
  ALTER COLUMN test_birth_date TYPE TIMESTAMPTZ
  USING NULLIF(trim(test_birth_date), '')::timestamptz;
