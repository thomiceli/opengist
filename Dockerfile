FROM alpine:3.17

# Install required dependencies
RUN apk update && \
    apk add --no-cache \
    openssl \
    openssh \
    curl \
    wget \
    git \
    gnupg \
    make \
    xz \
    gcc \
    musl-dev \
    libstdc++

# Install Golang
COPY --from=golang:1.19-alpine /usr/local/go/ /usr/local/go/
ENV PATH="/usr/local/go/bin:${PATH}"

# Install Node.js
COPY --from=node:18-alpine /usr/local/ /usr/local/
ENV NODE_PATH="/usr/local/lib/node_modules"
ENV PATH="/usr/local/bin:${PATH}"

# Set the working directory
WORKDIR /opengist

# Copy all source files
COPY . .

# Build the application
RUN make

# Expose the ports
EXPOSE 6157 2222

# Mount the .opengist volume
VOLUME /root/.opengist

# Run the webserver
CMD ["./opengist"]
