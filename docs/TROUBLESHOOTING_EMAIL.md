# Troubleshooting Email Error 535

## Error yang Terjadi
```
535 5.7.8 Username and Password not accepted
```

## Penyebab & Solusi

### 1. App Password Tidak Valid atau Salah

**Cek App Password:**
- App Password Gmail harus 16 karakter (tanpa spasi)
- Format: `xxxxxxxxxxxxxxxx` (16 karakter)
- Jangan ada spasi di awal atau akhir

**Cara Membuat App Password Baru:**
1. Buka: https://myaccount.google.com/apppasswords
2. Login dengan akun `kanagatapsikologi@gmail.com`
3. Pastikan **2-Step Verification sudah aktif**
4. Pilih "Mail" dan "Other (Custom name)"
5. Masukkan nama: "Psychee Wellness"
6. Klik "Generate"
7. **Copy App Password yang muncul** (16 karakter, contoh: `abcd efgh ijkl mnop`)
8. **Hapus semua spasi** saat paste ke app.conf (jadi: `abcdefghijklmnop`)

### 2. Format di app.conf Salah

**Pastikan format seperti ini (TANPA spasi setelah `=`):**
```ini
SMTP_PASSWORD =abcdefghijklmnop
```

**ATAU dengan spasi (keduanya OK):**
```ini
SMTP_PASSWORD = abcdefghijklmnop
```

**JANGAN seperti ini:**
```ini
SMTP_PASSWORD = abcdefghijklmnop   # Ada spasi di akhir
SMTP_PASSWORD=abcdefghijklmnop     # OK juga
```

### 3. 2-Step Verification Belum Aktif

**Cara Aktifkan:**
1. Buka: https://myaccount.google.com/security
2. Cari "2-Step Verification"
3. Klik "Get Started"
4. Ikuti langkah-langkah untuk setup
5. Setelah aktif, baru bisa buat App Password

### 4. App Password Sudah Dihapus atau Expired

- Jika App Password sudah dihapus, buat yang baru
- App Password tidak pernah expire, tapi bisa dihapus manual
- Jika ragu, buat App Password baru

### 5. Email Belum Diverifikasi

- Pastikan email `kanagatapsikologi@gmail.com` sudah diverifikasi
- Login ke Gmail dan pastikan tidak ada notifikasi verifikasi

## Langkah Perbaikan

### Step 1: Buat App Password Baru
1. Buka: https://myaccount.google.com/apppasswords
2. Buat App Password baru
3. Copy password (16 karakter)

### Step 2: Update app.conf
```ini
SMTP_USER = kanagatapsikologi@gmail.com
SMTP_PASSWORD =[PASTE_APP_PASSWORD_DI_SINI_TANPA_SPASI]
FROM_EMAIL = kanagatapsikologi@gmail.com
```

**Contoh:**
```ini
SMTP_PASSWORD =zqcavhgtsrgmwcku
```

### Step 3: Restart Aplikasi
```bash
# Stop aplikasi (Ctrl+C)
# Jalankan lagi
go run main.go
```

### Step 4: Test Lagi
1. Buka: http://localhost:112/contact
2. Isi form dan submit
3. Cek apakah email terkirim

## Verifikasi App Password

Untuk memastikan App Password benar, cek:
- Panjang: **16 karakter** (tanpa spasi)
- Format: Hanya huruf dan angka
- Tidak ada karakter khusus: `!@#$%^&*()`

## Test dengan Telnet (Advanced)

Jika masih error, test koneksi SMTP manual:

```bash
# Windows PowerShell
$client = New-Object System.Net.Sockets.TcpClient("smtp.gmail.com", 587)
$stream = $client.GetStream()
$sslStream = New-Object System.Net.Security.SslStream($stream)
$sslStream.AuthenticateAsClient("smtp.gmail.com")
$writer = New-Object System.IO.StreamWriter($sslStream)
$reader = New-Object System.IO.StreamReader($sslStream)

$writer.WriteLine("EHLO localhost")
$writer.Flush()
$reader.ReadLine()

$writer.WriteLine("AUTH LOGIN")
$writer.Flush()
$reader.ReadLine()

# Base64 encode email
$emailBytes = [System.Text.Encoding]::UTF8.GetBytes("kanagatapsikologi@gmail.com")
$emailBase64 = [Convert]::ToBase64String($emailBytes)
$writer.WriteLine($emailBase64)
$writer.Flush()
$reader.ReadLine()

# Base64 encode password
$passBytes = [System.Text.Encoding]::UTF8.GetBytes("zqcavhgtsrgmwcku")
$passBase64 = [Convert]::ToBase64String($passBytes)
$writer.WriteLine($passBase64)
$writer.Flush()
$response = $reader.ReadLine()
Write-Host $response

# Jika muncul "235 2.7.0 Accepted" = BERHASIL
# Jika muncul "535" = Password salah
```

## Checklist

- [ ] 2-Step Verification sudah aktif
- [ ] App Password sudah dibuat (16 karakter)
- [ ] App Password di app.conf tanpa spasi di awal/akhir
- [ ] Email `kanagatapsikologi@gmail.com` sudah diverifikasi
- [ ] Aplikasi sudah di-restart setelah update app.conf
- [ ] Port 587 tidak diblokir firewall

## Jika Masih Error

1. **Coba App Password yang berbeda** - Buat App Password baru
2. **Cek log aplikasi** - Lihat error detail di console
3. **Test dengan email lain** - Coba dengan Gmail account lain
4. **Cek firewall** - Pastikan port 587 tidak diblokir
