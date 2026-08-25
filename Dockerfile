FROM golang:1.25-alpine AS builder
WORKDIR /app

ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /easy-qfnu-kjs .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" \
    -o /migrate-sqlite-to-postgres ./cmd/migrate-sqlite-to-postgres

FROM alpine:3.20
WORKDIR /app

RUN apk add --no-cache su-exec tzdata \
    && adduser -D app \
    && mkdir -p /app/data /app/logs \
    && chown -R app:app /app

COPY --from=builder /easy-qfnu-kjs /app/easy-qfnu-kjs
COPY --from=builder /migrate-sqlite-to-postgres /app/migrate-sqlite-to-postgres
COPY docker-entrypoint.sh /app/docker-entrypoint.sh

RUN chmod +x /app/docker-entrypoint.sh

ENV GIN_MODE=release
ENV PORT=8080
ENV TZ=Asia/Shanghai

EXPOSE 8080

ENTRYPOINT ["/app/docker-entrypoint.sh"]
