############################
# 1) builder
############################
FROM golang:1.26-alpine AS builder

WORKDIR /src

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

# 构建信息，由 CI 注入，本地直接构建时默认为开发版
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_DATE=unknown

# 先拷依赖清单利用层缓存，再拷源码
COPY server/go.mod server/go.sum ./
RUN go mod download

COPY server/ ./

RUN CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" -o wecom-auth-center ./cmd/server

############################
# 2) runtime
############################
FROM alpine:3.20

ENV TZ=Asia/Shanghai

ARG ALPINE_MIRROR=
RUN if [ -n "$ALPINE_MIRROR" ]; then \
      sed -i "s/dl-cdn.alpinelinux.org/${ALPINE_MIRROR}/g" /etc/apk/repositories; \
    fi \
  && apk add --no-cache ca-certificates tzdata \
  && addgroup -g 10001 auth \
  && adduser -D -u 10001 -G auth auth

WORKDIR /app

COPY --from=builder /src/wecom-auth-center /app/wecom-auth-center
COPY --from=builder /src/web/static /app/web/static

# 配置文件不打进镜像，运行时挂载：
#   -v /opt/wecom-auth-center/config.yaml:/app/config.yaml:ro
USER auth

EXPOSE 8700

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO- http://127.0.0.1:8700/healthz || exit 1

ENTRYPOINT ["/app/wecom-auth-center", "-config", "/app/config.yaml"]
