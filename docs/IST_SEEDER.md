# IST Full Seeder Documentation

## Overview
Seeder lengkap untuk soal-soal IST (Intelligence Structure Test) yang terdiri dari 9 subtest dengan total 176 soal.

## Subtest yang Tersedia

1. **SE (Sentence Completion)** - Soal 01-20: Melengkapi kalimat
2. **WA (Word Analogies)** - Soal 21-40: Mencari kata yang berbeda
3. **AN (Analogies)** - Soal 41-60: Analogi kata
4. **GE (General Comprehension)** - Soal 61-76: Mencari kata yang meliputi pengertian dua kata
5. **RA (Arithmetic)** - Soal 77-96: Soal hitungan matematika
6. **ZR (Number Series)** - Soal 97-116: Deret angka
7. **FA (Shape Assembly)** - Soal 117-136: Menyusun bentuk (dengan gambar)
8. **WU (Cube Rotation)** - Soal 137-156: Rotasi kubus (dengan gambar)
9. **ME (Memory)** - Soal 157-176: Soal hafalan kata

## Cara Menggunakan

### 1. Menjalankan Seeder

Uncomment baris berikut di `main.go`:

```go
if err := seeds.SeedISTFull(); err != nil {
    log.Printf("IST full seed warning: %v", err)
}
```

Atau jalankan melalui command line:

```bash
go run main.go
```

Seeder bersifat idempotent - jika soal sudah ada, akan di-skip.

### 2. Update Kunci Jawaban untuk FA dan WU

File `seeds/ist_answer_keys.json` berisi placeholder untuk kunci jawaban FA (117-136) dan WU (137-156).

**Langkah-langkah:**
1. Buka file Excel `Alat Test Psikologi/1. IST/Kunci IST.xlsx`
2. Cari jawaban untuk soal FA (117-136) dan WU (137-156)
3. Update file `seeds/ist_answer_keys.json` dengan jawaban yang benar
4. Atau langsung update di `seeds/ist_full_seeder.go` pada fungsi `seedFA()` dan `seedWU()`

### 3. Setup Gambar

**Langkah-langkah:**

1. Buat folder jika belum ada:
   ```bash
   mkdir -p static/gambar_alat_test/IST
   ```

2. Copy gambar dari folder `gambar alat test/IST/` ke `static/gambar_alat_test/IST/`:
   - `Soal Gambar Kelompok 7 hal 117 - 128.png`
   - `Soal Gmabar Kelompok 7 Hal 129 - 136.png`
   - `Soal gambar kelompok 8 hal 137 - 141.png`
   - `Soal gambar kelompok 8 142 - 156.png`

3. Pastikan file gambar bisa diakses via URL:
   - `/static/gambar_alat_test/IST/Soal Gambar Kelompok 7 hal 117 - 128.png`
   - dll.

**Catatan:** Jika nama file berbeda atau path berbeda, update di fungsi `seedFA()` dan `seedWU()` pada variabel `imageBasePath` dan `imageFile`.

## Struktur Data

### ISTQuestion
- `subtest_id`: ID subtest (SE, WA, AN, dll)
- `number`: Nomor soal (1-176)
- `prompt`: Pertanyaan/soal
- `option_a` sampai `option_e`: Pilihan jawaban
- `correct_option`: Jawaban yang benar (A-E)
- `image_url`: URL gambar (opsional, untuk FA dan WU)

### Catatan Penting

1. **Soal GE (61-76)**: Jawaban sebenarnya adalah teks bebas, tapi disimpan sebagai pilihan ganda dengan opsi yang sesuai.

2. **Soal RA dan ZR**: Jawaban numerik disimpan sebagai pilihan ganda dengan opsi angka.

3. **Soal FA dan WU**: Memerlukan gambar. Pastikan gambar sudah di-upload ke folder static.

4. **Soal ME**: Berdasarkan hafalan kata-kata:
   - BUNGA: SOKA, LARAT, FLAMBOYAN, YASMIN, DAHLIA
   - PERKAKAS: WAJAN, JARUM, KIKIR, CANGKUL, PALU
   - BURUNG: ITIK, ELANG, WALET, TERUKUR, NURI
   - KESENIAN: QUATET, ARCA, OPERA, UKIRAN, GAMELAN
   - BINATANG: RUSA, MUSANG, BERUANG, HARIMAU, ZEBRA

## Verifikasi

Setelah menjalankan seeder, verifikasi dengan query database:

```sql
SELECT s.code, COUNT(q.id) as jumlah_soal
FROM ist_subtests s
LEFT JOIN ist_questions q ON q.subtest_id = s.id
GROUP BY s.code
ORDER BY s.order_index;
```

Harusnya setiap subtest memiliki jumlah soal yang sesuai:
- SE: 20 soal
- WA: 20 soal
- AN: 20 soal
- GE: 16 soal
- RA: 20 soal
- ZR: 20 soal
- FA: 20 soal
- WU: 20 soal
- ME: 20 soal

**Total: 176 soal**

## Troubleshooting

1. **Gambar tidak muncul**: Pastikan path gambar benar dan file sudah di-upload.

2. **Jawaban salah**: Update kunci jawaban di `ist_full_seeder.go` atau `ist_answer_keys.json`.

3. **Soal duplikat**: Seeder sudah idempotent, tapi jika perlu reset:
   ```sql
   DELETE FROM ist_answers;
   DELETE FROM ist_questions;
   ```

4. **Error saat insert**: Pastikan subtests sudah dibuat terlebih dahulu (jalankan `SeedIST()` dulu).
