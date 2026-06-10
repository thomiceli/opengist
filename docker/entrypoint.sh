#!/bin/sh

load_secrets() {
    if [ -f "/run/secrets/opengist_secrets" ]; then
        set -a
        . /run/secrets/opengist_secrets
        set +a
    fi
}

if [ "$(id -u)" -ne 0 ]; then
    load_secrets
    exec env HOME=/opengist OG_OPENGIST_HOME=/opengist /app/opengist/opengist --config /config.yml
fi

export USER=opengist
UID=${UID:-1000}
GID=${GID:-1000}
groupmod -o -g "$GID" $USER
usermod -o -u "$UID" $USER

chown -R "$USER:$USER" /opengist
chown -R "$USER:$USER" /config.yml

load_secrets

exec su $USER -c "OG_OPENGIST_HOME=/opengist /app/opengist/opengist --config /config.yml"
