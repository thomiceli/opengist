#!/bin/sh

export USER=opengist
PUID=${PUID:-1000}
PGID=${PGID:-1000}
groupmod -o -g "$PGID" $USER
usermod -o -u "$PUID" $USER

chown -R "$USER:$USER" /opengist

export OG_OPENGIST_HOME=/opengist

su -m $USER -c "/app/opengist/opengist"
