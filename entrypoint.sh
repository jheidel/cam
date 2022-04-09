#!/bin/bash

# Docker entrypoint

/app/cam --port 80 --port_ssl 443 --root /data/ --config /config/config.json
