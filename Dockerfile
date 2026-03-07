FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o scheduler ./cmd/api

FROM alpine:latest

RUN apk add --no-cache tzdata ca-certificates

WORKDIR /app

# Copy binary
COPY --from=builder /app/scheduler .
# Copy HTML templates
COPY --from=builder /app/views ./views

EXPOSE 8080

CMD ["./scheduler"]