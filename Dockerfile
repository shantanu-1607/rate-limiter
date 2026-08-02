FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /rate-limiter ./cmd/limiter/main.go
RUN CGO_ENABLED=0 GOOS=linux go build -tags musl -o /rate-limiter ./cmd/limiter/main.go

FROM alpine:3.19

WORKDIR /root/
COPY --from=builder /rate-limiter /bin/limiter
COPY --from=builder /bin/upstream
/bin/upstream
COPY --from=builder /app/internal/limiter/scripts /root/internal/limiter/scripts
EXPOSE 8080 9000
CMD ["/bin/limiter"]