FROM golang:1.23-alpine AS builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -o worker ./cmd/worker

FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata postgresql16-client wget
WORKDIR /app
COPY --from=builder /app/server .
COPY --from=builder /app/worker .
COPY --from=builder /app/entrypoint.sh .
RUN chmod +x ./entrypoint.sh
COPY --from=builder /app/internal/database/migrations ./migrations
EXPOSE 8080
ENTRYPOINT ["./entrypoint.sh"]
CMD ["server"]
