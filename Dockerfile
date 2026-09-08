FROM golang:1.27-bookworm AS builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o igocloud .

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates tzdata && rm -rf /var/lib/apt/lists/*
ENV TZ=Asia/Jakarta

COPY --from=builder /app/igocloud .

COPY --from=builder /app/public ./public

EXPOSE 3000

CMD ["./igocloud"]