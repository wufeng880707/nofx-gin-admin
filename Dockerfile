# syntax=docker/dockerfile:1.7

FROM node:26.12.1-alpine3.24 AS web-builder

WORKDIR /src/web
COPY web/package.json web/package-lock.json ./
RUN --mount=type=cache,target=/root/.npm npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26.5-alpine3.24 AS go-builder

ARG VERSION=v10.1.0
ARG RELEASE_TAG=${VERSION}
ARG GOPROXY=https://proxy.golang.org,direct
ARG APK_MIRROR=https://dl-cdn.alpinelinux.org/alpine

ENV CGO_ENABLED=1 \
    CGO_CFLAGS=-D_LARGEFILE64_SOURCE \
    GOPROXY=${GOPROXY}

RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${APK_MIRROR}#g" /etc/apk/repositories \
    && apk add --no-cache build-base sqlite-dev
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -ldflags "-w -s -X main.VERSION=${RELEASE_TAG}" -o /out/xtrading .

FROM alpine:3.24 AS runtime

ARG APK_MIRROR=https://dl-cdn.alpinelinux.org/alpine

RUN sed -i "s#https://dl-cdn.alpinelinux.org/alpine#${APK_MIRROR}#g" /etc/apk/repositories \
    && apk add --no-cache ca-certificates gettext-envsubst libgcc sqlite-libs tzdata \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app
COPY --from=go-builder /out/xtrading /usr/local/bin/xtrading
COPY --from=web-builder /src/web/dist /app/web/dist
COPY configs/docker /app/config-templates
COPY configs/menu_cn.json configs/rbac_model.conf /app/config-templates/
COPY internal/swagger/swagger.json /app/config-templates/openapi.json
COPY docker/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN mkdir -p /app/data /app/runtime-config \
    && chmod +x /usr/local/bin/docker-entrypoint.sh \
    && chown -R app:app /app

USER app

EXPOSE 8040
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
CMD ["start", "-d", "/app/runtime-config", "-c", "server.toml,middleware.toml,logging.toml", "-s", "/app/web/dist"]
