FROM alpine:3.17 AS build

RUN apk update && \
    apk add --no-cache \
    make \
    gcc \
    musl-dev \
    libstdc++

COPY --from=golang:1.20-alpine /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"

COPY --from=node:18-alpine /usr/local/ /usr/local/
ENV NODE_PATH="/usr/local/lib/node_modules"
ENV PATH="/usr/local/bin:${PATH}"

WORKDIR /opengist

COPY . .

RUN make


FROM alpine:3.17 as run

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
    adduser -S -G opengist -H -s /bin/ash -g 'Opengist User' opengist

COPY --from=build --chown=opengist:opengist /opengist/config.yml config.yml

WORKDIR /app/opengist

COPY --from=build --chown=opengist:opengist /opengist/opengist .
COPY --from=build --chown=opengist:opengist /opengist/docker ./docker

EXPOSE 6157 2222
VOLUME /opengist
ENTRYPOINT ["./docker/entrypoint.sh"]
