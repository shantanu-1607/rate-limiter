FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/limiter ./cmd/limiter
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/upstream ./cmd/upstream

FROM alpine:3.19

WORKDIR /root/
COPY --from=builder /bin/limiter /bin/limiter
COPY --from=builder /bin/upstream /bin/upstream

EXPOSE 8080 9000
CMD ["/bin/limiter"]