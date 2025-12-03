PKG_ROOT := github.com/socheatsok78/sentrytunnel
PKG_VERSION := dev

it:
	go mod tidy

run:
	go run cmd/sentrytunnel/sentrytunnel.go --log-level=debug

debug:
	go run cmd/sentrytunnel/sentrytunnel.go --log-level=debug

build: bin/sentrytunnel

bin/sentrytunnel:
	go build -ldflags="-s -X $(PKG_ROOT).Version=$(PKG_VERSION)" -o bin/sentrytunnel cmd/sentrytunnel/sentrytunnel.go

clean:
	rm -rf bin
