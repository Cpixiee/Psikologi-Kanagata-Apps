-- Tambah kolom phone di test_invitations untuk fitur kirim WA
ALTER TABLE test_invitations
  ADD COLUMN IF NOT EXISTS phone VARCHAR(20) NOT NULL DEFAULT '';

-- Tambah kolom send_via_whatsapp di test_batches
ALTER TABLE test_batches
  ADD COLUMN IF NOT EXISTS send_via_whatsapp BOOLEAN NOT NULL DEFAULT FALSE;
