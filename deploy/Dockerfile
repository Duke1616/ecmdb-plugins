FROM node:22.18.0-bookworm as frontend-build
ARG PLUGIN_NAME
WORKDIR /app
COPY . .
# 动态检测是否存在前端项目，若存在则使用 pnpm 构建，若无则建立空 dist 以防 deploy 阶段 COPY 报错
RUN if [ -d "plugins/${PLUGIN_NAME}/frontend" ]; then \
      cd plugins/${PLUGIN_NAME}/frontend && \
      npm install -g pnpm@10 && \
      pnpm set registry https://registry.npmmirror.com/ && \
      pnpm install && \
      pnpm run build; \
    else \
      mkdir -p plugins/${PLUGIN_NAME}/frontend/dist; \
    fi

FROM golang:1.25.0-alpine as build
MAINTAINER luankz
ARG PLUGIN_NAME
ARG VERSION=unknown
WORKDIR /app
COPY . .
RUN go mod tidy
RUN go build -ldflags="-X main.version=${VERSION} -w -s" -a -o dist/ecmdb-plugin plugins/${PLUGIN_NAME}/cmd/main.go

FROM alpine:latest as deploy
ARG PLUGIN_NAME
ENV EGO_DEBUG=true
WORKDIR /app
COPY --from=build /app/dist/ecmdb-plugin .
# 复制前端 UMD 资源
COPY --from=frontend-build /app/plugins/${PLUGIN_NAME}/frontend/dist ./plugins/${PLUGIN_NAME}/frontend/dist