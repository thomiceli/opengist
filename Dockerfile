# syntax=docker/dockerfile:1
#
# Opengist image
#
# Versions of the base toolchain are parametrized so they can be overridden at
# build time (--build-arg) and kept in sync in a single place.
#
#   docker build \
#     --build-arg ALPINE_VERSION=3.24 \
#     --build-arg GOLANG_VERSION=1.26.4 \
#     --build-arg NODE_VERSION=26.3.0 \
#     --build-arg OPENGIST_VERSION=1.13.1 \
#     -t opengist .
#
# Build targets:
#   dev   – live-reloading development image (source mounted as a volume)
#   build – compiles the production binary
#   prod  – minimal runtime image (default)

ARG ALPINE_VERSION=3.24
ARG GOLANG_VERSION=1.26.4
ARG NODE_VERSION=26.3.0

########################################
# base: shared toolchain (go + node)   #
########################################
FROM alpine:${ALPINE_VERSION} AS base

ARG GOLANG_VERSION
ARG NODE_VERSION
ARG ALPINE_VERSION

ENV CGO_ENABLED=0 \
    NODE_PATH=/usr/local/lib/node_modules \
    PATH=/usr/local/go/bin:${PATH}

RUN apk add --no-cache \
        gcc \
        git \
        make \
        libstdc++ \
        musl-dev

COPY --from=golang:${GOLANG_VERSION}-alpine${ALPINE_VERSION} /usr/local/go/ /usr/local/go/
COPY --from=node:${NODE_VERSION}-alpine${ALPINE_VERSION} /usr/local/ /usr/local/

WORKDIR /opengist

COPY . .

########################################
# dev: development image               #
########################################
FROM base AS dev

ARG OPENGIST_VERSION=unknown

RUN apk add --no-cache \
        curl \
        gnupg \
        openssh-server \
        openssl \
        wget \
        xz \
    && git config --global --add safe.directory /opengist \
    && make install

EXPOSE 6157 6158 2222 16157

VOLUME ["/opengist"]

CMD ["make", "watch"]

########################################
# build: compile the production binary #
########################################
FROM base AS build

ARG OPENGIST_VERSION=unknown
RUN make

########################################
# prod: minimal runtime image          #
########################################
FROM alpine:${ALPINE_VERSION} AS prod

LABEL org.opencontainers.image.title="Opengist" \
      org.opencontainers.image.description="Self-hosted pastebin powered by Git" \
      org.opencontainers.image.source="https://github.com/thomiceli/opengist" \
      org.opencontainers.image.licenses="AGPL-3.0"

# git is required at runtime (Opengist shells out to it); the opengist account
# is a fixed non-root user (UID/GID 1000). Run as a different user with
# `docker run --user` / Compose `user:` and chown the data volume on the host.
RUN apk add --no-cache \
        git \
    && addgroup -S -g 1000 opengist \
    && adduser -S -G opengist -u 1000 -h /opengist -s /sbin/nologin -g 'Opengist' opengist

WORKDIR /app/opengist

COPY --from=build --chown=opengist:opengist /opengist/opengist ./opengist
COPY --from=build --chown=opengist:opengist /opengist/config.yml /config.yml

ENV HOME=/opengist \
    OG_OPENGIST_HOME=/opengist

USER opengist:opengist

EXPOSE 6157 6158 2222
VOLUME ["/opengist"]

HEALTHCHECK --interval=60s --timeout=30s --start-period=15s --retries=3 \
    CMD wget -qO /dev/null http://127.0.0.1:6157/healthcheck || exit 1

ENTRYPOINT ["./opengist"]
CMD ["--config", "/config.yml"]
