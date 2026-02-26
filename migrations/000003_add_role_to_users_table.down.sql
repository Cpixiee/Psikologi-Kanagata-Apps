-- Drop role column from users table (rollback for version 3)
ALTER TABLE users
DROP COLUMN IF EXISTS role;

