# Testing Guide Lengkap - Profil & Pengaturan

## Persiapan

### 1. Jalankan Migration Database
Pastikan semua kolom dan tabel sudah dibuat:

```bash
# Jalankan semua migration
go run migrations/migrate.go up

# Atau manual via psql
psql -U your_user -d your_database -f migrations/000005_add_foto_profil_to_users_table.up.sql
psql -U your_user -d your_database -f migrations/000006_add_address_fields_to_users_table.up.sql
psql -U your_user -d your_database -f migrations/000007_create_user_settings_table.up.sql
```

### 2. Pastikan Folder Uploads Ada
```bash
# Folder harus ada di:
static/uploads/profiles/
```

### 3. Install Dependencies
```bash
go get golang.org/x/image/draw
go mod tidy
```

## Testing - Halaman Profil

### Test 1: Akses Halaman Profil
1. **Login** terlebih dahulu via `/login`
2. Buka browser dan akses: `http://localhost:112/profile`
3. **Expected Result:**
   - Halaman profil muncul dengan form
   - Sidebar konsisten dengan dashboard
   - Tidak ada button "Upgrade to Pro"
   - Tidak ada bagian currency
   - Semua teks dalam bahasa Indonesia
   - Nomor handphone menggunakan format +62
   - Field baru: Kota, Provinsi, Kode Pos

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
   - Data user muncul (nama, email, no_handphone, kota, provinsi, kodepos, dll)
   - Jika belum ada foto_profil, field `foto_profil` null atau kosong

### Test 3: Update Profil dengan Field Baru
1. Login terlebih dahulu
2. Buka halaman `/profile`
3. Isi form:
   - Nama Lengkap: "John Doe"
   - Email: "john@example.com"
   - Nomor Handphone: "81234567890"
   - Jenis Kelamin: "Laki-laki"
   - Alamat: "Jl. Contoh No. 123"
   - Kota: "Jakarta"
   - Provinsi: "DKI Jakarta"
   - Kode Pos: "12345"
4. Klik "Simpan Perubahan"
5. **Expected Result:**
   - SweetAlert muncul dengan pesan "Profil berhasil diperbarui"
   - Data tersimpan di database (termasuk kota, provinsi, kodepos)
   - Form ter-update dengan data baru

### Test 4: Upload Foto dengan Crop
1. Login terlebih dahulu
2. Buka halaman `/profile`
3. Klik "Unggah Foto Baru"
4. Pilih file gambar (JPG/PNG, maksimal 5MB)
5. **Expected Result:**
   - Modal crop muncul dengan gambar yang dipilih
   - Bisa crop gambar (drag, resize)
   - Klik "Oke, Simpan" → foto ter-upload
   - Klik "Batal" → modal tutup tanpa upload
   - Foto yang di-upload otomatis di-resize menjadi maksimal 400x400px
   - Foto muncul di preview dengan ukuran sesuai (tidak terlalu besar)
   - File tersimpan di `static/uploads/profiles/` dengan nama format: `{user_id}_{timestamp}.jpg`
   - Database kolom `foto_profil` ter-update

### Test 5: Validasi Upload Foto
1. Login terlebih dahulu
2. Buka halaman `/profile`
3. **Test 5a - File terlalu besar (>5MB):**
   - Coba upload file > 5MB
   - **Expected:** SweetAlert error "Ukuran file maksimal 5MB"
4. **Test 5b - Format tidak valid:**
   - Coba upload file selain JPG/PNG (misal PDF)
   - **Expected:** SweetAlert error "Format file tidak valid"
5. **Test 5c - File valid dengan crop:**
   - Upload file JPG/PNG < 5MB
   - Crop gambar di modal
   - Klik "Oke, Simpan"
   - **Expected:** Upload berhasil, foto ter-resize

### Test 6: Foto Profil Ukuran Sesuai
1. Upload foto profil
2. **Expected Result:**
   - Foto di preview avatar (100x100px) tidak terlalu besar
   - Foto di navbar (40x40px) tidak terlalu besar
   - Foto di dropdown (40x40px) tidak terlalu besar
   - Semua foto menggunakan `object-fit: cover` untuk proporsi yang benar

### Test 7: Update Email yang Sudah Digunakan
1. Login dengan user A
2. Update email menjadi email yang sudah digunakan user B
3. **Expected Result:**
   - SweetAlert error "Email sudah digunakan"
   - Data tidak ter-update

## Testing - Halaman Dashboard (Dynamic)

### Test 8: Dashboard Load User Data Dynamic
1. Login terlebih dahulu
2. Buka halaman `/dashboard`
3. Klik dropdown user di navbar kanan atas
4. **Expected Result:**
   - Nama user muncul (bukan "Pengguna" static)
   - Email user muncul (bukan "Member" static)
   - Foto profil user muncul (jika sudah upload)
   - Data di-load dari API `/api/profile`

### Test 9: Dashboard Avatar Update Setelah Upload Foto
1. Login terlebih dahulu
2. Upload foto profil di halaman `/profile`
3. Kembali ke `/dashboard`
4. **Expected Result:**
   - Avatar di navbar ter-update dengan foto baru
   - Avatar di dropdown ter-update dengan foto baru
   - Tidak perlu refresh manual

## Testing - Halaman Pengaturan (Settings)

### Test 10: Akses Halaman Settings
1. **Login** terlebih dahulu
2. Buka browser dan akses: `http://localhost:112/settings`
3. **Expected Result:**
   - Halaman settings muncul dengan notifications table
   - Sidebar konsisten dengan dashboard dan profile
   - Tidak ada button "Upgrade to Pro"
   - Semua teks dalam bahasa Indonesia
   - Table notifications dengan kolom: Tipe, Email, Browser, App

### Test 11: Load Settings Data (GET /api/settings)
1. Login terlebih dahulu
2. Buka browser console (F12)
3. Jalankan:
   ```javascript
   fetch('/api/settings')
     .then(r => r.json())
     .then(console.log)
   ```
4. **Expected Result:**
   - Response JSON dengan `success: true`
   - Data settings muncul dengan default values:
     - "New for you": Email ✓, Browser ✓, App ✓
     - "Account activity": Email ✓, Browser ✓, App ✓
     - "Browser login": Email ✓, Browser ✓, App ✗
     - "Device link": Email ✓, Browser ✗, App ✗
   - Notification timing: "online"

### Test 12: Update Settings
1. Login terlebih dahulu
2. Buka halaman `/settings`
3. Ubah beberapa checkbox:
   - Uncheck "New for you" → Browser
   - Uncheck "Account activity" → App
   - Check "Browser login" → App
   - Ubah dropdown "Kapan kirim notifikasi" → "Kapan saja"
4. Klik "Simpan Perubahan"
5. **Expected Result:**
   - SweetAlert muncul dengan pesan "Pengaturan berhasil disimpan"
   - Data tersimpan di database
   - Form ter-update dengan nilai baru
   - Setelah refresh, checkbox tetap sesuai yang disimpan

### Test 13: Request Browser Notification Permission
1. Login terlebih dahulu
2. Buka halaman `/settings`
3. Klik link "Minta Izin" di bagian "Perangkat Terkini"
4. **Expected Result:**
   - Browser meminta izin notifikasi
   - Jika granted → SweetAlert "Izin Diberikan"
   - Jika denied → SweetAlert "Izin Ditolak"

### Test 14: Settings Default Values
1. Register user baru
2. Login dengan user baru
3. Akses `/settings`
4. **Expected Result:**
   - Checkbox terisi dengan default values (sesuai migration)
   - Notification timing: "Hanya saat saya online"

## Testing - Register (Field Minimal)

### Test 15: Register dengan Field Minimal
1. Buka halaman `/register`
2. Isi form register:
   - Nama Lengkap: "Test User"
   - Email: "test@example.com"
   - Password: "password123"
   - Konfirmasi Password: "password123"
   - CAPTCHA: isi sesuai gambar
3. Submit form
4. **Expected Result:**
   - Register berhasil
   - User bisa login
   - Field lain (alamat, kota, provinsi, kodepos, no_handphone, jenis_kelamin, foto_profil) bisa NULL/kosong
   - User bisa lengkapi profil nanti di halaman `/profile`

## Checklist Testing Lengkap

### Profil
- [ ] Halaman profil bisa diakses setelah login
- [ ] Data profil bisa di-load dari database
- [ ] Update profil berhasil menyimpan ke database (termasuk kota, provinsi, kodepos)
- [ ] Upload foto profil dengan crop berhasil
- [ ] Modal crop muncul saat upload foto
- [ ] Validasi ukuran file (maksimal 5MB sebelum crop) bekerja
- [ ] Validasi format file (hanya JPG/PNG) bekerja
- [ ] Foto otomatis di-resize menjadi maksimal 400x400px
- [ ] Foto yang di-upload ukurannya sesuai (tidak terlalu besar di preview)
- [ ] Format nomor handphone otomatis +62
- [ ] Semua teks dalam bahasa Indonesia
- [ ] Tidak ada button "Upgrade to Pro"
- [ ] Tidak ada bagian currency
- [ ] Sidebar konsisten dengan dashboard
- [ ] Security: tidak bisa akses tanpa login
- [ ] Email tidak bisa duplikat
- [ ] Foto lama terhapus saat upload baru

### Dashboard
- [ ] Dashboard load user data dynamic (nama, email, foto)
- [ ] Avatar di navbar ter-update setelah upload foto
- [ ] Avatar di dropdown ter-update setelah upload foto
- [ ] Logout function bekerja

### Settings
- [ ] Halaman settings bisa diakses setelah login
- [ ] Data settings bisa di-load dari database
- [ ] Update settings berhasil menyimpan ke database
- [ ] Checkbox notifications bekerja dengan benar
- [ ] Dropdown notification timing bekerja
- [ ] Request browser notification permission bekerja
- [ ] Default values sesuai dengan migration
- [ ] Sidebar konsisten dengan dashboard dan profile
- [ ] Semua teks dalam bahasa Indonesia
- [ ] Tidak ada button "Upgrade to Pro"

### Register
- [ ] Register bisa dilakukan dengan field minimal
- [ ] Field lain bisa NULL/kosong saat register
- [ ] User bisa lengkapi profil nanti

## Troubleshooting

### Error: "Gagal membuat direktori upload"
- Pastikan folder `static/uploads/profiles/` sudah dibuat
- Pastikan aplikasi punya permission untuk write di folder tersebut

### Error: "Gagal memproses gambar"
- Pastikan file yang di-upload benar-benar file gambar valid
- Cek apakah library `golang.org/x/image/draw` sudah terinstall

### Error: "Foto terlalu besar"
- Foto sudah di-resize otomatis menjadi maksimal 400x400px
- Jika masih terlalu besar, cek CSS `object-fit: cover` sudah diterapkan

### Error: "Settings tidak tersimpan"
- Cek apakah migration `000007_create_user_settings_table.up.sql` sudah dijalankan
- Cek apakah model `UserSettings` sudah ter-register di `init()`

### Foto tidak muncul setelah upload
- Cek console browser untuk error JavaScript
- Cek network tab untuk melihat response dari API
- Pastikan path foto benar di database (`foto_profil` field)
- Pastikan Beego bisa serve static files dari folder `static/uploads/`

## Catatan Penting

1. **Folder Uploads**: Pastikan folder `static/uploads/profiles/` ada dan aplikasi punya permission untuk write
2. **Static Files**: Beego secara default serve static files dari folder `static/`, jadi file di `static/uploads/profiles/` bisa diakses via `/static/uploads/profiles/{filename}`
3. **Security**: Semua endpoint API profile dan settings memerlukan login (dicek via session)
4. **File Naming**: File foto disimpan dengan format `{user_id}_{timestamp}.jpg` (selalu JPG setelah resize)
5. **Image Resize**: Foto otomatis di-resize menjadi maksimal 400x400px dengan maintain aspect ratio
6. **Crop**: Foto bisa di-crop sebelum upload menggunakan Cropper.js (aspect ratio 1:1)
7. **Register**: Field minimal saja, field lain bisa diisi nanti di halaman profil
