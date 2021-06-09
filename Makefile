ts := $(shell /bin/date "+%s")
hash := $(shell git log -1 --pretty=format:"%H")

all: build

web/build:
	cd web && $(MAKE)

assetfs: web/build
	go-bindata web/build/default/...  models/...

debugfs: web/build
	go-bindata -debug web/build/default/... models/...

go:
	go mod download
	go get -d -v  # Attempt to upgrade
	go build -tags matprofile -ldflags "-X main.BuildTimestamp=$(ts) -X main.BuildGitHash=$(hash)"

.PHONY: debug build

debug: debugfs go
build: assetfs go

clean:
	cd web && $(MAKE) clean
	rm -rf bindata.go cam libs

# Assemble all shared library dependencies (used for docker image building)
libs:
	mkdir libs
	ldd ./cam | grep '=> /' | awk '{print $$3}' | xargs -I{} readlink -f {} | xargs -I{} cp {} libs/
	du -sh libs
