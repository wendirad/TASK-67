FROM golang:1.23-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY internal/ ./internal/
COPY cmd/ ./cmd/
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
COPY --from=builder /app/internal/templates ./templates
EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
CMD ["server"]
