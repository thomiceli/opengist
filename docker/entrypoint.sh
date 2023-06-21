#!/bin/sh

export USER=opengist
UID=${UID:-1000}
GID=${GID:-1000}
groupmod -o -g "$GID" $USER
usermod -o -u "$UID" $USER

chown -R "$USER:$USER" /opengist

export OG_OPENGIST_HOME=/opengist

su -m $USER -c "/app/opengist/opengist"
