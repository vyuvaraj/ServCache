FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download || true
COPY . .
RUN go build -o servcache main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/servcache .
EXPOSE 8086
ENTRYPOINT ["./servcache"]
