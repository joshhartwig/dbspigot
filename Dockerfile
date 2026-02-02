# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install templ
RUN go install github.com/a-h/templ/cmd/templ@latest

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Generate templ files
RUN templ generate

# Build binary
RUN CGO_ENABLED=0 GOOS=linux go build -o /dbspigot ./cmd/server

# Runtime stage
FROM alpine:3.19

RUN apk --no-cache add ca-certificates

WORKDIR /

COPY --from=builder /dbspigot /dbspigot

EXPOSE 8080

ENTRYPOINT ["/dbspigot"]
