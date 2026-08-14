# Production Deployment (Debian + Docker)

## 1) Package yang perlu di-install di server Debian

Minimal:
- Docker Engine (`docker-ce`, `docker-ce-cli`, `containerd.io`)
- Docker Compose plugin (`docker-compose-plugin`)
- Buildx plugin (`docker-buildx-plugin`)
- `git`, `curl`, `gnupg`, `ca-certificates`

Auto install semua package:

```bash
sudo bash deploy/debian/setup-server.sh
```

## 2) Persiapan project di server

```bash
sudo mkdir -p /opt
cd /opt
sudo git clone <REPO_URL> psikologi_apps
sudo chown -R $USER:$USER /opt/psikologi_apps
cd /opt/psikologi_apps
cp .env.docker.example .env.docker
```

Edit `.env.docker` lalu isi minimal:
- `DB_PASSWORD`
- `ADMIN_EMAIL`
- `ADMIN_PASSWORD`
- `SMTP_*` (jika fitur email dipakai)

## 3) Jalankan aplikasi (otomatis migrate)

```bash
docker compose --env-file .env.docker -f docker-compose.prod.yml up -d --build
```

Yang otomatis dijalankan:
- Build image app
- Start PostgreSQL
- Inject config app dari `.env.docker`
- Auto migration (`AUTO_MIGRATE=true`)
- Start aplikasi pada port `APP_HTTP_PORT` (default `8086`)

## 4) Command operasional penting

```bash
# lihat status service
docker compose --env-file .env.docker -f docker-compose.prod.yml ps

# lihat log aplikasi
docker compose --env-file .env.docker -f docker-compose.prod.yml logs -f app

# stop service
docker compose --env-file .env.docker -f docker-compose.prod.yml down

# migration manual
docker compose --env-file .env.docker -f docker-compose.prod.yml run --rm app /app/migrate -command=up

# cek status migration
docker compose --env-file .env.docker -f docker-compose.prod.yml run --rm app /app/migrate -command=status

# seed admin manual
docker compose --env-file .env.docker -f docker-compose.prod.yml run --rm app /app/seed
```

## 5) One-command deploy update

Setelah setup awal, update aplikasi:

```bash
bash deploy/debian/deploy.sh /opt/psikologi_apps
```

Script akan:
- `git pull --rebase`
- rebuild image
- restart service via compose

## 6) Port & error `Bind for 0.0.0.0:8081 failed: port is already allocated`

- `docker-compose.prod.yml` memetakan **`HOST_PORT` (default 8086) → 8086** di dalam container, dan memaksa **`APP_HTTP_PORT=8086`** di service `app` (supaya nilai lama `APP_HTTP_PORT=8081` di `.env.docker` tidak membuat bind host masih ke 8081).
- Kalau **8086** bentrok di server, set di `.env.docker`: `HOST_PORT=port_lain` (mis. `9000`) lalu akses `http://IP:9000`.
- Setelah ubah env, jalankan: `docker compose --env-file .env.docker -f docker-compose.prod.yml up -d`.

## 8) Domain & SSL Setup (Reverse Proxy: Apache / Nginx)

Agar aplikasi dapat diakses langsung menggunakan domain `https://psikotes.kanagata.co.id` tanpa port `8086` atau IP `103.236.140.19`:

### Opsi A: Menggunakan APACHE Web Server

1. **Install Apache & Certbot**:
   ```bash
   sudo apt update
   sudo apt install -y apache2 certbot python3-certbot-apache
   ```

2. **Aktifkan Modul Proxy Apache**:
   ```bash
   sudo a2enmod proxy proxy_http proxy_wstunnel rewrite ssl headers
   sudo systemctl restart apache2
   ```

3. **Buka Port Firewall**:
   ```bash
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   sudo ufw reload
   ```

4. **Copy Konfigurasi VirtualHost Apache**:
   ```bash
   sudo cp deploy/apache/psikotes.kanagata.co.id.conf /etc/apache2/sites-available/psikotes.kanagata.co.id.conf
   sudo a2ensite psikotes.kanagata.co.id.conf
   sudo apache2ctl configtest
   sudo systemctl reload apache2
   ```

5. **Aktifkan SSL GRATIS (HTTPS) via Certbot**:
   ```bash
   sudo certbot --apache -d psikotes.kanagata.co.id
   ```

---

### Opsi B: Menggunakan NGINX Web Server

1. **Install Nginx & Certbot**:
   ```bash
   sudo apt update
   sudo apt install -y nginx certbot python3-certbot-nginx
   ```

2. **Buka Port Firewall**:
   ```bash
   sudo ufw allow 80/tcp
   sudo ufw allow 443/tcp
   sudo ufw reload
   ```

3. **Copy & Aktifkan Konfigurasi Nginx**:
   ```bash
   sudo cp deploy/nginx/psikotes.kanagata.co.id.conf /etc/nginx/sites-available/psikotes.kanagata.co.id
   sudo ln -s /etc/nginx/sites-available/psikotes.kanagata.co.id /etc/nginx/sites-enabled/
   sudo nginx -t
   sudo systemctl reload nginx
   ```

4. **Aktifkan SSL GRATIS (HTTPS) via Certbot**:
   ```bash
   sudo certbot --nginx -d psikotes.kanagata.co.id
   ```

---

### Langkah Akhir (Wajib untuk Apache maupun Nginx):

Update `BASE_URL` di file `.env.docker`:
```env
BASE_URL=https://psikotes.kanagata.co.id
```

Lalu restart aplikasi Docker:
```bash
docker compose --env-file .env.docker -f docker-compose.prod.yml up -d
```


