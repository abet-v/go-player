# Multi-stage build to produce a small image

FROM golang:1.22-alpine AS builder
RUN apk add --no-cache ca-certificates tzdata build-base
WORKDIR /src

# Pre-cache deps
COPY go.mod go.sum* ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server

# Runtime image
FROM alpine:3.20
RUN adduser -D -H -u 10001 app && apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /out/server /app/server
COPY web /app/web
USER app
EXPOSE 8080
CMD ["/app/server"]

