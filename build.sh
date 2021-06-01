#!/bin/sh

# Builds the docker image and pushes it to docker hub.

set -e
set -x

docker build -t jheidel/cam .
docker push jheidel/cam:latest
