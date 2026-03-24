# Verifikasi Seeder IST

## Status Seeder
✅ Seeder sudah berjalan dengan sukses berdasarkan log:
```
IST norms already exist, skip
IST full seeder completed successfully
```

## Cara Verifikasi Soal Sudah Terisi

### 1. Verifikasi via Database (SQL)

Jalankan query berikut di database PostgreSQL:

```sql
-- Cek jumlah soal per subtest
SELECT 
    s.code AS subtest_code,
    s.name AS subtest_name,
    COUNT(q.id) AS jumlah_soal
FROM ist_subtests s
LEFT JOIN ist_questions q ON q.subtest_id = s.id
GROUP BY s.code, s.name
ORDER BY s.order_index;
```

**Hasil yang diharapkan:**
- SE: 20 soal
- WA: 20 soal
- AN: 20 soal
- GE: 16 soal
- RA: 20 soal
- ZR: 20 soal
- FA: 20 soal
- WU: 20 soal
- ME: 20 soal
- **Total: 176 soal**

### 2. Verifikasi via Aplikasi

1. **Login sebagai Admin**
   - Buka `http://localhost:8081/login`
   - Login dengan akun admin

2. **Buat Batch Tes**
   - Buka `http://localhost:8081/admin/psychotest`
   - Klik "Buat Batch Baru"
   - Isi form dan aktifkan IST
   - Simpan batch

3. **Buat Undangan**
   - Klik "Lihat Hasil" pada batch yang baru dibuat
   - Buat undangan untuk satu peserta (gunakan email yang sudah terdaftar)

4. **Test sebagai Peserta**
   - Login dengan akun peserta yang diundang
   - Buka `http://localhost:8081/test`
   - Masukkan token dari email
   - Klik "Mulai Test IST"
   - **Cek apakah soal sudah muncul dengan benar (bukan "Soal contoh")**

### 3. Cek Soal Langsung via Browser

Setelah login sebagai peserta dan mulai tes, cek:
- Soal SE (Sentence Completion) - harus ada 20 soal dengan pertanyaan lengkap
- Soal WA (Word Analogies) - harus ada 20 soal dengan pertanyaan lengkap
- Soal AN (Analogies) - harus ada 20 soal dengan pertanyaan lengkap
- dll.

**Tanda soal sudah benar:**
- ✅ Pertanyaan lengkap dan jelas (bukan "Soal contoh 1 untuk WA")
- ✅ Ada 5 pilihan jawaban (A, B, C, D, E)
- ✅ Jumlah soal sesuai (20 untuk sebagian besar, 16 untuk GE)

**Tanda soal masih placeholder:**
- ❌ Teks "Soal contoh 1 untuk [CODE]"
- ❌ Teks "Soal contoh 2 untuk [CODE]"
- ❌ Hanya ada 2 soal per subtest

## Troubleshooting

### Jika soal masih placeholder:

1. **Cek apakah seeder benar-benar berjalan:**
   ```bash
   # Restart aplikasi dan lihat log
   go run .
   ```
   Harus muncul: `IST full seeder completed successfully`

2. **Cek database langsung:**
   ```sql
   SELECT COUNT(*) FROM ist_questions;
   ```
   Harusnya ada 176 baris

3. **Jika masih ada soal placeholder:**
   - Hapus soal placeholder dulu:
     ```sql
     DELETE FROM ist_questions WHERE prompt LIKE 'Soal contoh%';
     ```
   - Restart aplikasi untuk menjalankan seeder lagi

4. **Jika seeder tidak berjalan:**
   - Pastikan `main.go` memanggil `seeds.SeedISTFull()`
   - Cek error di log aplikasi
   - Pastikan database connection benar

## Catatan Penting

- Seeder bersifat **idempotent** - aman dijalankan berkali-kali
- Soal yang sudah ada tidak akan di-duplicate
- Jika ada soal baru, akan ditambahkan
- Norms hanya dibuat sekali (jika belum ada)
