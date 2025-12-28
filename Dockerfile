FROM alpine:3.22 AS base

RUN apk update && \
        apk add --no-cache \
        make \
        gcc \
        git \
        musl-dev \
        libstdc++

COPY --from=golang:1.25-alpine3.22 /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"
ENV CGO_ENABLED=0

COPY --from=node:24.9.0-alpine3.22 /usr/local/ /usr/local/
ENV NODE_PATH="/usr/local/lib/node_modules"
ENV PATH="/usr/local/bin:${PATH}"

WORKDIR /opengist

COPY . .


FROM base AS dev
RUN apk add --no-cache \
    openssl \
    openssh-server \
    curl \
    wget \
    git \
    gnupg \
    xz

EXPOSE 6157 2222 16157

RUN git config --global --add safe.directory /opengist
RUN make install

VOLUME /opengist

CMD ["make", "watch"]


FROM base AS build

RUN make


FROM alpine:3.22 as prod

RUN apk update && \
    apk add --no-cache \
    shadow \
    openssh-server \
    curl \
    git

RUN addgroup -S opengist && \
    adduser -S -G opengist -s /bin/ash -g 'Opengist User' opengist

WORKDIR /app/opengist

COPY --from=build --chown=opengist:opengist /opengist/config.yml /config.yml
COPY --from=build --chown=opengist:opengist /opengist/opengist .
COPY --from=build --chown=opengist:opengist /opengist/docker ./docker

EXPOSE 6157 2222
VOLUME /opengist
HEALTHCHECK --interval=60s --timeout=30s --start-period=15s --retries=3 CMD curl -f http://localhost:6157/healthcheck || exit 1
ENTRYPOINT ["./docker/entrypoint.sh"]
