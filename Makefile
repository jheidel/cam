ts := $(shell /bin/date "+%s")

all: build

web/build:
	cd web && $(MAKE)

assetfs: web/build
	go-bindata web/build/default/...  models/...

debugfs: web/build
	go-bindata -debug web/build/default/... models/...

go:
	go get -d -v
	go build -ldflags "-X main.BuildTimestamp=$(ts)"

.PHONY: debug build

debug: debugfs go
build: assetfs go

clean:
	cd web && $(MAKE) clean
	rm bindata.go cam
