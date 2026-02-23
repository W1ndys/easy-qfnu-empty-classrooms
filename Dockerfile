FROM node:20-alpine AS frontend-builder
WORKDIR /app

COPY frontend/package.json frontend/package-lock.json ./frontend/
RUN npm --prefix frontend ci

COPY frontend ./frontend
RUN npm --prefix frontend run build

FROM golang:1.25-alpine AS go-builder
WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /app/web ./web

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o /easy-qfnu-empty-classrooms .

FROM alpine:3.20
WORKDIR /app

RUN adduser -D app \
    && mkdir -p /app/logs \
    && chown -R app:app /app

COPY --from=go-builder /easy-qfnu-empty-classrooms /app/easy-qfnu-empty-classrooms

USER app

ENV GIN_MODE=release
ENV PORT=8080

EXPOSE 8080

ENTRYPOINT ["/app/easy-qfnu-empty-classrooms"]
