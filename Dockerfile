FROM node:20-bookworm-slim AS assets
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY static ./static
COPY tailwind.config.js postcss.config.js ./
RUN npm run build:css

FROM golang:1.24-bookworm AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=assets /app/static/tailwind/output.css /src/static/tailwind/output.css
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app main.go
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/seed ./cmd/seed

FROM debian:bookworm-slim AS runtime
WORKDIR /app
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/app /app/app
COPY --from=builder /out/migrate /app/migrate
COPY --from=builder /out/seed /app/seed
COPY conf ./conf
COPY views ./views
COPY static ./static
COPY migrations ./migrations
COPY seeds ./seeds
COPY data ./data
COPY scripts/entrypoint.sh /app/scripts/entrypoint.sh

RUN chmod +x /app/scripts/entrypoint.sh /app/app /app/migrate /app/seed

EXPOSE 8081
ENTRYPOINT ["/app/scripts/entrypoint.sh"]
