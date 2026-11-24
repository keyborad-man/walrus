# Stage 1: Build React SPA
FROM node:22-alpine AS frontend
ARG NPM_REGISTRY=
WORKDIR /src/web
COPY web/package.json web/package-lock.json* ./
RUN if [ -n "$NPM_REGISTRY" ]; then npm config set registry "$NPM_REGISTRY"; fi && \
    (npm ci || npm install)
COPY web/ ./
RUN npm run build

# Stage 2: Build Go single static binary with embedded frontend
FROM golang:1.24-bookworm AS backend
ARG GOPROXY=https://proxy.golang.org,direct
ARG DEFAULT_STEAM_API_KEY=
ARG WALRUS_VERSION=dev
ARG WALRUS_COMMIT=none
ARG WALRUS_BUILD_TIME=unknown
ENV GOPROXY=${GOPROXY}
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . ./
COPY --from=frontend /src/web/dist ./web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
    -ldflags="-s -w -X walrus/internal/config.DefaultSteamAPIKey=${DEFAULT_STEAM_API_KEY} -X walrus/internal/version.Version=${WALRUS_VERSION} -X walrus/internal/version.Commit=${WALRUS_COMMIT} -X walrus/internal/version.BuildTime=${WALRUS_BUILD_TIME}" \
    -o /out/walrus ./cmd/walrus

# Stage 2b: Build 64-bit & 32-bit proxychains-ng so 32-bit SteamCMD can tunnel through HTTP/SOCKS5 proxies
# (Ubuntu 24.04 official repos do not ship libproxychains4:i386)
FROM ubuntu:24.04 AS proxychains-builder
ARG PROXYCHAINS_VERSION=4.17
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
      build-essential \
      gcc-multilib \
      curl \
      ca-certificates && \
    rm -rf /var/lib/apt/lists/*
WORKDIR /tmp/proxychains-ng
RUN curl -fsSL "https://github.com/rofl0r/proxychains-ng/archive/refs/tags/v${PROXYCHAINS_VERSION}.tar.gz" | tar -xz --strip-components=1 && \
    ./configure --prefix=/usr --libdir=/usr/lib/x86_64-linux-gnu --sysconfdir=/etc && \
    make -j"$(nproc)" && \
    make DESTDIR=/out install && \
    make clean && \
    CFLAGS="-m32" LDFLAGS="-m32" ./configure --prefix=/usr --libdir=/usr/lib/i386-linux-gnu --sysconfdir=/etc && \
    make -j"$(nproc)" libproxychains4.so && \
    install -D -m 644 libproxychains4.so /out/usr/lib/i386-linux-gnu/libproxychains4.so

# Stage 3: Ubuntu runtime with SteamCMD & DST dedicated server libs
FROM ubuntu:24.04

ENV DEBIAN_FRONTEND=noninteractive \
    WALRUS_ADDR=:8080 \
    WALRUS_DATA_DIR=/data \
    WALRUS_RUNTIME=linux \
    TZ=Asia/Shanghai

RUN dpkg --add-architecture i386 && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
      ca-certificates \
      curl \
      tar \
      gzip \
      unzip \
      lib32gcc-s1 \
      lib32stdc++6 \
      libcurl3t64-gnutls \
      libcurl3t64-gnutls:i386 \
      tzdata && \
    rm -rf /var/lib/apt/lists/*

# Pre-stage SteamCMD tarball extraction directory
RUN mkdir -p /opt/steamcmd /data && \
    curl -sqL "https://steamcdn-a.akamaihd.net/client/installer/steamcmd_linux.tar.gz" | tar -xz -C /opt/steamcmd

COPY --from=proxychains-builder /out/usr/bin/proxychains4 /usr/bin/proxychains4
COPY --from=proxychains-builder /out/usr/lib/x86_64-linux-gnu/libproxychains4.so /usr/lib/x86_64-linux-gnu/libproxychains4.so
COPY --from=proxychains-builder /out/usr/lib/i386-linux-gnu/libproxychains4.so /usr/lib/i386-linux-gnu/libproxychains4.so
RUN ldconfig

COPY --from=backend /out/walrus /usr/local/bin/walrus
COPY docker/entrypoint.sh /usr/local/bin/entrypoint.sh
RUN sed -i 's/\r$//' /usr/local/bin/entrypoint.sh && \
    chmod +x /usr/local/bin/walrus /usr/local/bin/entrypoint.sh

EXPOSE 8080/tcp
EXPOSE 10990-11060/udp

VOLUME ["/data"]

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
