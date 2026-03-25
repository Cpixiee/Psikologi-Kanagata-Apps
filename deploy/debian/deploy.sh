#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="${1:-/opt/psikologi_apps}"

if [[ ! -d "${PROJECT_DIR}" ]]; then
  echo "Project directory not found: ${PROJECT_DIR}"
  echo "Clone first: git clone <repo_url> ${PROJECT_DIR}"
  exit 1
fi

cd "${PROJECT_DIR}"

if [[ ! -f ".env.docker" ]]; then
  cp .env.docker.example .env.docker
  echo "Created .env.docker from example. Please edit secrets before rerun."
  exit 1
fi

git pull --rebase
docker compose --env-file .env.docker -f docker-compose.prod.yml up -d --build
docker compose --env-file .env.docker -f docker-compose.prod.yml ps

echo "Deployment finished."
