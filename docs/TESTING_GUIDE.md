# Panduan Testing Aplikasi Psikologi

## Cara Test Aplikasi

### 1. Setup Database
Pastikan database sudah di-setup dan migration sudah dijalankan:
```bash
# Pastikan PostgreSQL berjalan
# Database: psikologi_apps
# User: postgres (atau sesuai konfigurasi)
```

### 2. Jalankan Seeder IST
Seeder akan otomatis berjalan saat aplikasi start. Pastikan:
- File `seeds/ist_full_seeder.go` sudah ada
- File `seeds/ist_answer_keys.json` sudah ada (opsional, untuk referensi)
- Gambar soal FA dan WU sudah di-copy ke `static/gambar_alat_test/IST/`

### 3. Jalankan Aplikasi
```bash
go run main.go
```
Aplikasi akan berjalan di `http://localhost:8081` (atau sesuai konfigurasi)

### 4. Login sebagai Admin
1. Buka `http://localhost:8081/login`
2. Login dengan akun admin
3. Akses halaman admin: `http://localhost:8081/admin/psychotest`

### 5. Membuat Batch Tes
1. Di halaman admin, klik "Buat Batch Baru"
2. Isi form:
   - Nama Batch: Contoh "Tes Intake 2024"
   - Institusi: Contoh "SMA Negeri 1"
   - Tujuan: Pilih dari dropdown
   - Aktifkan IST dan/atau Holland
   - Pilih metode pengiriman undangan
3. Klik "Simpan Batch"

### 6. Membuat Undangan
1. Setelah batch dibuat, klik "Lihat Hasil" pada batch tersebut
2. Di bagian "Undang Peserta":
   - Cari peserta dengan nama/email, atau
   - Tempel email manual di textarea
3. Klik "Buat Undangan"
4. Token akan otomatis dibuat untuk setiap peserta

### 7. Melihat Token Undangan
1. Setelah undangan dibuat, di tabel hasil akan muncul kolom "Token"
2. Klik tombol "Lihat Token" pada peserta yang diinginkan
3. Token akan ditampilkan dalam kotak yang menarik dengan:
   - Desain gradient yang menarik
   - Font besar dan mudah dibaca
   - Tombol "Salin Token" untuk copy ke clipboard
4. Token bisa diberikan kepada peserta untuk mengakses tes

### 8. Test sebagai Peserta
1. Login dengan akun peserta yang diundang
2. Buka `http://localhost:8081/test`
3. Masukkan token yang diberikan admin
4. Klik "Mulai Tes"
5. Ikuti alur pengerjaan tes IST/Holland

### 9. Export Hasil ke Excel
1. Di halaman admin, buka batch yang sudah ada hasilnya
2. Klik tombol "Export Jawaban"
3. File CSV akan terdownload dengan format:
   - Header informasi peserta
   - Tabel dengan kolom No. dan Jawaban untuk setiap subtest
   - Baris RW (Raw Score) per subtest
4. Buka file CSV di Excel untuk melihat hasil

## Fitur Token Undangan

### Tampilan Token
Token ditampilkan dalam modal dengan desain yang menarik:
- **Kotak Gradient**: Background gradient biru-ungu yang menarik
- **Font Besar**: Font 32px dengan letter-spacing untuk mudah dibaca
- **Copy Button**: Tombol untuk menyalin token ke clipboard
- **Informasi**: Menampilkan email peserta yang terkait dengan token

### Cara Menggunakan Token
1. Admin memberikan token kepada peserta
2. Peserta login dengan akun yang sesuai dengan email undangan
3. Peserta membuka halaman `/test`
4. Peserta memasukkan token (case-insensitive)
5. Sistem akan memvalidasi token dan mengarahkan ke halaman tes

## Troubleshooting

### Token tidak dikenali
- Pastikan token diketik dengan benar (case-insensitive)
- Pastikan peserta login dengan email yang sesuai dengan undangan
- Cek apakah token sudah expired (masa berlaku 1 hari)
- Cek status undangan di admin panel

### Seeder tidak berjalan
- Pastikan `main.go` memanggil `seeds.SeedISTFull()`
- Cek log error di console
- Pastikan database connection sudah benar

### Gambar tidak muncul
- Pastikan gambar sudah di-copy ke `static/gambar_alat_test/IST/`
- Cek path gambar di seeder sesuai dengan struktur folder
- Pastikan file gambar ada dan bisa diakses via browser

### Export Excel tidak berfungsi
- Pastikan ada data jawaban di database
- Cek format CSV yang dihasilkan
- Pastikan Excel bisa membuka file CSV dengan encoding UTF-8
