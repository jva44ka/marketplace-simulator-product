# ---------- BUILD STAGE ----------
FROM golang:latest AS builder

WORKDIR /app

# Копируем зависимости отдельно — кэш
COPY go.mod go.sum ./
RUN go mod download

# Копируем исходники
COPY . .

# Собираем бинарь
RUN CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    go build -o app ./cmd/server


# ---------- MIGRATOR STAGE ----------
FROM golang:latest AS migrator

WORKDIR /app

COPY migrations ./migrations
RUN go install github.com/pressly/goose/v3/cmd/goose@v3.24.1

ENTRYPOINT ["goose", "-dir", "/app/migrations", "postgres"]


# ---------- RUNTIME STAGE ----------
FROM gcr.io/distroless/base-debian12

WORKDIR /app

# Копируем бинарник и swagger-файлы
COPY --from=builder /app/app /app/app
COPY --from=builder /app/swagger /app/swagger

EXPOSE 5000

ENTRYPOINT ["/app/app"]
