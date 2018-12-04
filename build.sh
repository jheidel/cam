#!/bin/bash

GOROOT=$HOME/go/src

set -x
set -e

# Compile polymer frontend
(cd web && polymer build --js-minify --css-minify --html-minify)

# Generate bindata.go file from polymer output
go-bindata web/build/default/... models/...

# Build standalone binary with resources embedded
go build
