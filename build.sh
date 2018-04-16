#!/bin/bash

GOROOT=$HOME/go/src

set -x
set -e

# Compile polymer frontend
pushd web
polymer build --js-minify --css-minify --html-minify
popd

# Generate bindata.go file from polymer output
go-bindata web/build/default/...

# Configure environment for building with OpenCV
source $GOROOT/gocv.io/x/gocv/env.sh

# Build standalone binary with resources embedded
go build
