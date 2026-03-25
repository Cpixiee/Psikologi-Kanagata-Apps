# Testing Guide - Fitur Profil Pengguna

## Persiapan

### 1. Jalankan Migration Database
Pastikan kolom `foto_profil` sudah ditambahkan ke tabel `users`:

```bash
# Jika menggunakan migrate.go
go run migrations/migrate.go up

# Atau manual via psql
psql -U your_user -d your_database -f migrations/000005_add_foto_profil_to_users_table.up.sql
```

### 2. Pastikan Folder Uploads Ada
Folder `static/uploads/profiles/` harus sudah dibuat. Jika belum, buat manual:

```bash
mkdir -p static/uploads/profiles
```

### 3. Pastikan Static Files Bisa Diakses
Pastikan Beego bisa serve static files dari folder `static/uploads/`. Biasanya sudah otomatis jika menggunakan konfigurasi default Beego.

## Testing Steps

### Test 1: Akses Halaman Profil (Harus Login)
1. **Login terlebih dahulu** via `/login`
2. Buka browser dan akses: `http://localhost:112/profile`
3. **Expected Result:**
   - Halaman profil muncul dengan form kosong (jika belum ada data)
   - Sidebar menampilkan menu yang sama dengan dashboard
   - Tidak ada button "Upgrade to Pro"
   - Tidak ada bagian currency
   - Semua teks dalam bahasa Indonesia
   - Nomor handphone menggunakan format +62

### Test 2: Load Data Profil (GET /api/profile)
1. Login terlebih dahulu
2. Buka browser console (F12)
3. Jalankan:
   ```javascript
   fetch('/api/profile')
     .then(r => r.json())
     .then(console.log)
   ```
4. **Expected Result:**
   - Response JSON dengan `success: true`
   - Data user muncul (nama, email, no_handphone, dll)
   - Jika belum ada foto_profil, field `foto_profil` null atau kosong

### Test 3: Update Profil (PUT /api/profile)
1. Login terlebih dahulu
2. Buka halaman `/profile`
3. Isi form:
   - Nama Lengkap: "John Doe"
   - Email: "john@example.com"
   - Nomor Handphone: "81234567890" (akan otomatis jadi +6281234567890)
   - Jenis Kelamin: "Laki-laki"
   - Alamat: "Jl. Contoh No. 123"
4. Klik "Simpan Perubahan"
5. **Expected Result:**
   - SweetAlert muncul dengan pesan "Profil berhasil diperbarui"
   - Data tersimpan di database
   - Form ter-update dengan data baru

### Test 4: Upload Foto Profil (POST /api/profile/upload)
1. Login terlebih dahulu
2. Buka halaman `/profile`
3. Klik "Unggah Foto Baru"
4. Pilih file gambar (JPG/PNG, maksimal 2MB)
5. **Expected Result:**
   - SweetAlert muncul dengan pesan "Foto profil berhasil diunggah"
   - Foto muncul di preview (avatar besar dan dropdown navbar)
   - File tersimpan di `static/uploads/profiles/` dengan nama format: `{user_id}_{timestamp}.{ext}`
   - Database kolom `foto_profil` ter-update dengan nama file

### Test 5: Validasi Upload Foto
1. Login terlebih dahulu
2. Buka halaman `/profile`
3. **Test 5a - File terlalu besar (>2MB):**
   - Coba upload file > 2MB
   - **Expected:** SweetAlert error "Ukuran file maksimal 2MB"
4. **Test 5b - Format tidak valid:**
   - Coba upload file selain JPG/PNG (misal PDF)
   - **Expected:** SweetAlert error "Format file tidak valid"
5. **Test 5c - File valid:**
   - Upload file JPG/PNG < 2MB
   - **Expected:** Upload berhasil

### Test 6: Update Email yang Sudah Digunakan
1. Login dengan user A
2. Update email menjadi email yang sudah digunakan user B
3. **Expected Result:**
   - SweetAlert error "Email sudah digunakan"
   - Data tidak ter-update

### Test 7: Reset Avatar
1. Login terlebih dahulu
2. Upload foto profil
3. Klik button "Reset"
4. **Expected Result:**
   - Avatar kembali ke default (`/static/icons/psikologi.png`)
   - File input di-reset

### Test 8: Akses Tanpa Login (Security Test)
1. **Logout** terlebih dahulu atau buka incognito window
2. Akses `http://localhost:112/profile`
3. **Expected Result:**
   - Redirect ke `/login?next=/profile`
4. Akses `http://localhost:112/api/profile` via browser console
5. **Expected Result:**
   - Response JSON: `{"success": false, "message": "Silakan login terlebih dahulu"}`
   - Status code: 401

### Test 9: Format Nomor Handphone
1. Login terlebih dahulu
2. Buka halaman `/profile`
3. Isi nomor handphone: "81234567890" (tanpa +62)
4. Simpan
5. **Expected Result:**
   - Data tersimpan dengan format "+6281234567890"
   - Input field menampilkan "81234567890" (tanpa +62 saat edit)

### Test 10: Hapus Foto Lama Saat Upload Baru
1. Login terlebih dahulu
2. Upload foto profil pertama (misal: `1_1234567890.jpg`)
3. Upload foto profil kedua (misal: `1_1234567891.jpg`)
4. **Expected Result:**
   - Foto pertama terhapus dari folder `static/uploads/profiles/`
   - Hanya foto kedua yang tersimpan
   - Database hanya menyimpan nama file foto kedua

## Checklist Testing

- [ ] Halaman profil bisa diakses setelah login
- [ ] Data profil bisa di-load dari database
- [ ] Update profil berhasil menyimpan ke database
- [ ] Upload foto profil berhasil
- [ ] Validasi ukuran file (maksimal 2MB) bekerja
- [ ] Validasi format file (hanya JPG/PNG) bekerja
- [ ] Format nomor handphone otomatis +62
- [ ] Semua teks dalam bahasa Indonesia
- [ ] Tidak ada button "Upgrade to Pro"
- [ ] Tidak ada bagian currency
- [ ] Tidak ada footer ThemeSelection
- [ ] Sidebar konsisten dengan dashboard
- [ ] Security: tidak bisa akses tanpa login
- [ ] Email tidak bisa duplikat
- [ ] Foto lama terhapus saat upload baru

## Troubleshooting

### Error: "Gagal membuat direktori upload"
- Pastikan folder `static/uploads/profiles/` sudah dibuat
- Pastikan aplikasi punya permission untuk write di folder tersebut

### Error: "File tidak ditemukan" saat akses foto
- Pastikan file benar-benar tersimpan di `static/uploads/profiles/`
- Pastikan Beego bisa serve static files dari folder `static/`
- Cek path di browser: `http://localhost:112/static/uploads/profiles/{filename}`

### Error: "User tidak ditemukan"
- Pastikan session `user_id` ada
- Pastikan user dengan ID tersebut ada di database

### Foto tidak muncul setelah upload
- Cek console browser untuk error JavaScript
- Cek network tab untuk melihat response dari API
- Pastikan path foto benar di database (`foto_profil` field)

## Catatan Penting

1. **Folder Uploads**: Pastikan folder `static/uploads/profiles/` ada dan aplikasi punya permission untuk write
2. **Static Files**: Beego secara default serve static files dari folder `static/`, jadi file di `static/uploads/profiles/` bisa diakses via `/static/uploads/profiles/{filename}`
3. **Security**: Semua endpoint API profile memerlukan login (dicek via session)
4. **File Naming**: File foto disimpan dengan format `{user_id}_{timestamp}.{ext}` untuk menghindari konflik nama file
