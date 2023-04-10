FROM alpine:3.17 AS build

RUN apk update && \
    apk add --no-cache \
    make \
    gcc \
    musl-dev \
    libstdc++

COPY --from=golang:1.19-alpine /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"

COPY --from=node:18-alpine /usr/local/ /usr/local/
ENV NODE_PATH="/usr/local/lib/node_modules"
ENV PATH="/usr/local/bin:${PATH}"

WORKDIR /opengist

COPY . .

RUN make


FROM alpine:3.17

RUN apk update && \
    apk add --no-cache \
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

WORKDIR /opengist

COPY --from=build /opengist/opengist .

EXPOSE 6157 2222
VOLUME /root/.opengist
CMD ["./opengist"]
