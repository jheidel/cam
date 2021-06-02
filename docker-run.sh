#!/bin/sh

# This file contains one possible configuration for running as a persistent
# docker daemon against a mysql server running on the host machine.

DB_DIR="/data/gatecam/gate/"
CONFIG_DIR="/root/go/src/cam/config.heidel/"
DATABASE="cam:cam@(172.17.0.1)/cam"

docker run \
  -d \
  --restart always \
  -p 8889:8443 \
  --mount type=bind,source=${DB_DIR?},target=/mnt/db \
  --mount type=bind,source=${CONFIG_DIR?},target=/mnt/config,readonly \
  --mount type=bind,source=/etc/letsencrypt,target=/etc/letsencrypt,readonly \
  --mount type=bind,source=/etc/ssl,target=/etc/ssl,readonly \
  -e "DATABASE=${DATABASE?}" \
  --cpus=4 --memory=10g --memory-swap=10g \
  --name cam \
  jheidel/cam
