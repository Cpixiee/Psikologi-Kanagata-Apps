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
- Start aplikasi pada port `APP_HTTP_PORT` (default `8081`)

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

## 6) Catatan keamanan production

- Jangan commit `.env.docker`.
- Gunakan password DB/admin yang kuat.
- SMTP credential lama di `conf/app.conf` sebaiknya di-rotate.
- Buka port publik seperlunya (80/443 via reverse proxy, atau 8081 jika direct).
