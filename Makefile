COMPOSE_FILE=docker-compose.prod.yml

.PHONY: setup up down restart logs ps build pull migrate status seed

setup:
	cp -n .env.docker.example .env.docker || true

up:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) up -d --build

down:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) down

restart:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) restart

logs:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) logs -f app

ps:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) ps

build:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) build --no-cache

pull:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) pull

migrate:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) run --rm app /app/migrate -command=up

status:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) run --rm app /app/migrate -command=status

seed:
	docker compose --env-file .env.docker -f $(COMPOSE_FILE) run --rm app /app/seed
