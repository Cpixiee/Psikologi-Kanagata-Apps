# Panduan Testing di Local

## 1. Setup Database & Migration

### Jalankan Migration
```bash
cd c:\laragon\www\Psikologi_Apps
go run cmd/migrate/main.go -command=up
```

Ini akan membuat tabel `password_resets` di database.

### Cek Status Migration
```bash
go run cmd/migrate/main.go -command=status
```

## 2. Konfigurasi Email (Sudah di app.conf)

Pastikan di `conf/app.conf` sudah ada:
```ini
SMTP_HOST = smtp.gmail.com
SMTP_PORT = 587
SMTP_USER = kanagatapsikologi@gmail.com
SMTP_PASSWORD = zqcavhgtsrgmwcku
FROM_EMAIL = kanagatapsikologi@gmail.com
FROM_NAME = Psychee Wellness
```

## 3. Jalankan Aplikasi

```bash
go run main.go
```

Aplikasi akan berjalan di: `http://localhost:112`

## 4. Testing Contact Form

1. Buka browser: `http://localhost:112/contact`
2. Isi form:
   - Nama Lengkap: Test User
   - Email: test@example.com
   - Nomor Telepon: 081234567890
   - Pesan: Ini adalah test pesan dari local
3. Klik "Kirim Pesan"
4. **Cek email**: Buka inbox `kanagatapsikologi@gmail.com`
   - Email akan masuk ke inbox atau spam folder
   - Subject: "Pesan Baru dari Contact Form - Psychee Wellness"

## 5. Testing Reset Password

### Step 1: Request OTP
1. Buka: `http://localhost:112/reset-password`
2. Masukkan email user yang sudah terdaftar (misal: email yang digunakan saat register)
3. Klik "Kirim Kode OTP"
4. **Cek email**: Buka inbox email user tersebut
   - Subject: "Kode OTP Reset Password - Psychee Wellness"
   - Akan ada kode OTP 6 digit (contoh: 123456)

### Step 2: Verify OTP & Reset Password
1. Masukkan kode OTP yang diterima di email
2. Masukkan password baru (minimal 6 karakter)
3. Konfirmasi password baru
4. Klik "Reset Password"
5. Jika berhasil, akan redirect ke halaman login
6. Test login dengan password baru

## 6. Testing dengan User Baru

Jika belum ada user, daftar dulu:
1. Buka: `http://localhost:112/register`
2. Daftar user baru
3. Setelah berhasil, gunakan email tersebut untuk test reset password

## Troubleshooting

### Email tidak terkirim
1. **Cek log aplikasi** - Lihat error di terminal/console
2. **Pastikan App Password benar** - Untuk Gmail, harus pakai App Password (16 karakter)
3. **Cek firewall** - Pastikan port 587 tidak diblokir
4. **Cek spam folder** - Email mungkin masuk ke spam

### OTP tidak diterima
1. Cek spam folder
2. Pastikan email user sudah terdaftar di database
3. Cek log aplikasi untuk error detail
4. Pastikan SMTP_PASSWORD sudah benar di app.conf

### Database error
1. Pastikan database PostgreSQL sudah running
2. Pastikan koneksi database di app.conf benar
3. Pastikan migration sudah dijalankan

## Checklist Testing

- [ ] Migration berhasil dijalankan
- [ ] Contact form bisa kirim email ke admin
- [ ] Email contact masuk ke inbox kanagatapsikologi@gmail.com
- [ ] Request OTP berhasil (email terkirim ke user)
- [ ] OTP code valid dan bisa digunakan
- [ ] Reset password berhasil
- [ ] Login dengan password baru berhasil
