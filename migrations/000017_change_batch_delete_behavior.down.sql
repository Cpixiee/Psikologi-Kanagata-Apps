-- Rollback: kembalikan ke ON DELETE CASCADE

-- Hapus constraint baru
ALTER TABLE test_invitations 
  DROP CONSTRAINT IF EXISTS test_invitations_batch_id_fkey;

-- Ubah batch_id kembali menjadi NOT NULL (set NULL dulu untuk data yang NULL)
UPDATE test_invitations SET batch_id = 0 WHERE batch_id IS NULL;

-- Ubah batch_id menjadi NOT NULL
ALTER TABLE test_invitations 
  ALTER COLUMN batch_id SET NOT NULL;

-- Tambah constraint lama dengan ON DELETE CASCADE
ALTER TABLE test_invitations 
  ADD CONSTRAINT test_invitations_batch_id_fkey 
  FOREIGN KEY (batch_id) 
  REFERENCES test_batches(id) 
  ON DELETE CASCADE;
