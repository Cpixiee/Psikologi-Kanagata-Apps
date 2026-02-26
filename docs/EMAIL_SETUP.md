# Setup Email Configuration

## Konfigurasi SMTP untuk Email

Untuk mengaktifkan fitur email (Contact Form dan Reset Password), Anda perlu mengkonfigurasi SMTP di file `conf/app.conf`.

### Untuk Gmail

1. **Aktifkan 2-Step Verification** di akun Google Anda
   - Buka: https://myaccount.google.com/security
   - Aktifkan 2-Step Verification

2. **Buat App Password**
   - Buka: https://myaccount.google.com/apppasswords
   - Pilih "Mail" dan "Other (Custom name)"
   - Masukkan nama: "Psychee Wellness"
   - Copy App Password yang dihasilkan

3. **Update `conf/app.conf`**
   ```ini
   SMTP_HOST = smtp.gmail.com
   SMTP_PORT = 587
   SMTP_USER = rhaarhell@gmail.com
   SMTP_PASSWORD = [APP_PASSWORD_DARI_STEP_2]
   FROM_EMAIL = rhaarhell@gmail.com
   FROM_NAME = Psychee Wellness
   ```

### Untuk Email Provider Lain

#### Outlook/Hotmail
```ini
SMTP_HOST = smtp-mail.outlook.com
SMTP_PORT = 587
SMTP_USER = your-email@outlook.com
SMTP_PASSWORD = your-password
```

#### Yahoo
```ini
SMTP_HOST = smtp.mail.yahoo.com
SMTP_PORT = 587
SMTP_USER = your-email@yahoo.com
SMTP_PASSWORD = your-app-password
```

#### Custom SMTP
```ini
SMTP_HOST = smtp.yourdomain.com
SMTP_PORT = 587
SMTP_USER = noreply@yourdomain.com
SMTP_PASSWORD = your-password
FROM_EMAIL = noreply@yourdomain.com
FROM_NAME = Psychee Wellness
```

## Testing

Setelah konfigurasi, test dengan:
1. Mengirim pesan melalui Contact Form
2. Request reset password

Email akan dikirim ke:
- **Contact Form**: rhaarhell@gmail.com (admin)
- **Reset Password**: Email user yang request reset

## Troubleshooting

### Email tidak terkirim
1. Pastikan SMTP_PASSWORD sudah benar (untuk Gmail, gunakan App Password)
2. Pastikan firewall tidak memblokir port 587
3. Cek log aplikasi untuk error detail
4. Pastikan email FROM_EMAIL sudah diverifikasi

### Gmail Error: "Less secure app access"
- Gmail tidak lagi mendukung "Less secure app access"
- **Wajib** menggunakan App Password (lihat langkah di atas)
