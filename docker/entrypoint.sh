#!/bin/sh

export USER=opengist
PID=${PID:-1000}
GID=${GID:-1000}
groupmod -o -g "$GID" $USER
usermod -o -u "$PID" $USER

chown -R "$USER:$USER" /opengist

export OG_OPENGIST_HOME=/opengist

su -m $USER -c "/app/opengist/opengist"
