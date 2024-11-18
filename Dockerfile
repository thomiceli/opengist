FROM alpine:3.19 AS base

RUN apk update && \
        apk add --no-cache \
        make \
        shadow \
        openssl \
        openssh \
        curl \
        wget \
        git \
        gnupg \
        xz \
        gcc \
        musl-dev \
        libstdc++

COPY --from=golang:1.23-alpine /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=0

COPY --from=node:20-alpine /usr/local/ /usr/local/
ENV NODE_PATH="/usr/local/lib/node_modules"
ENV PATH="/usr/local/bin:${PATH}"

WORKDIR /opengist

COPY . .


FROM base AS dev

EXPOSE 6157 2222 16157
VOLUME /opengist

CMD ["make", "watch"]


FROM base AS build

RUN make


FROM alpine:3.19 as prod

RUN apk update && \
    apk add --no-cache \
    shadow \
    openssl \
    openssh \
    curl \
    wget \
    git \
    gnupg \
    xz \
    gcc \
    musl-dev \
    libstdc++

RUN addgroup -S opengist && \
    adduser -S -G opengist -s /bin/ash -g 'Opengist User' opengist

COPY --from=build --chown=opengist:opengist /opengist/config.yml config.yml

WORKDIR /app/opengist

COPY --from=build --chown=opengist:opengist /opengist/opengist .
COPY --from=build --chown=opengist:opengist /opengist/docker ./docker

EXPOSE 6157 2222
VOLUME /opengist
HEALTHCHECK --interval=60s --timeout=30s --start-period=15s --retries=3 CMD curl -f http://localhost:6157/healthcheck || exit 1
ENTRYPOINT ["./docker/entrypoint.sh"]
