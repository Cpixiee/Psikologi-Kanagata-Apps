-- Ubah behavior delete batch: history tetap ada meskipun batch dihapus
-- Ubah batch_id menjadi nullable dan ON DELETE SET NULL

-- Hapus constraint lama
ALTER TABLE test_invitations 
  DROP CONSTRAINT IF EXISTS test_invitations_batch_id_fkey;

-- Ubah batch_id menjadi nullable
ALTER TABLE test_invitations 
  ALTER COLUMN batch_id DROP NOT NULL;

-- Tambah constraint baru dengan ON DELETE SET NULL
ALTER TABLE test_invitations 
  ADD CONSTRAINT test_invitations_batch_id_fkey 
  FOREIGN KEY (batch_id) 
  REFERENCES test_batches(id) 
  ON DELETE SET NULL;
