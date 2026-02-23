# syntax=docker/dockerfile:1.7

ARG NODE_VERSION=24
ARG GO_VERSION=1.26

FROM node:${NODE_VERSION}-bookworm-slim AS web-builder
WORKDIR /workspace

# 先复制 workspace 依赖描述，利用 Docker layer cache 加速 npm ci。
COPY package.json package-lock.json ./
COPY apps/web/package.json apps/web/package.json
RUN npm ci

# 构建前端产物：客户端 dist + SSR worker dist-ssr。
COPY apps/web apps/web
RUN npm run build -w @plaindoc/web

FROM golang:${GO_VERSION}-bookworm AS go-builder
WORKDIR /workspace/apps/server

# 先下载 Go 依赖，命中缓存后仅在源码变化时重新编译。
COPY apps/server/go.mod apps/server/go.sum ./
RUN go mod download

COPY apps/server ./

ARG TARGETOS=linux
ARG TARGETARCH=amd64
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /out/plaindoc-server ./cmd/server

FROM node:${NODE_VERSION}-bookworm-slim AS runtime
WORKDIR /app

ENV APP_ENV=production \
    APP_ADDR=:8080 \
    WEB_ORIGIN=http://localhost:8080 \
    WEB_DIST_DIR=/app/apps/web/dist \
    DB_DRIVER=sqlite \
    DB_DSN=file:/app/data/plaindoc.db?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL) \
    SSR_WORKER_ENABLED=true \
    SSR_WORKER_EXEC=node \
    SSR_WORKER_ENTRY=/app/apps/web/dist-ssr/worker-entry.js

COPY --from=go-builder /out/plaindoc-server /app/plaindoc-server
COPY --from=web-builder /workspace/apps/web/dist /app/apps/web/dist
COPY --from=web-builder /workspace/apps/web/dist-ssr /app/apps/web/dist-ssr

RUN mkdir -p /app/data /app/uploads \
    && chown -R node:node /app

USER node
EXPOSE 8080

CMD ["/app/plaindoc-server"]
