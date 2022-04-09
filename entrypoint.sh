#!/bin/bash

# Docker entrypoint

UID=${RUN_AS_UID:-1000}
GID=$UID
NAME="cam"

groupadd -g $GID $NAME
useradd --shell /bin/bash -u $UID -g $NAME -o -c "" -m $NAME

su $(id -un $UID) -c "/app/cam --port 80 --port_ssl 443 --root /data/ --config /config/config.json"
