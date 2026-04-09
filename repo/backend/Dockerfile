FROM golang:1.24-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
RUN go install github.com/a-h/templ/cmd/templ@v0.2.793
COPY internal/ ./internal/
COPY cmd/ ./cmd/
RUN templ generate ./internal/templates/
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata postgresql16-client wget
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/worker .
COPY backend/entrypoint.sh ./entrypoint.sh
RUN chmod +x ./entrypoint.sh
COPY --from=builder /app/internal/database/migrations ./migrations
EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
CMD ["server"]
