it:
	go mod tidy

test:
	go test -v ./...

run:
	go run cmd/sentry-tunnel/main.go
debug:
	go run cmd/sentry-tunnel/main.go --log-level=debug
build: \
	bin/sentry-stub-server \
	bin/sentry-tunnel
clean:
	rm -rf bin

bin/sentry-stub-server:
	go build -o bin/sentry-stub-server cmd/sentry-stub-server/main.go
bin/sentry-tunnel:
	go build -o bin/sentry-tunnel cmd/sentry-tunnel/main.go

docker/build:
	docker buildx bake --load dev
docker/run:
	docker run --rm -it -p 8080:8080 socheatsok78/sentry-tunnel:dev

.PHONY: benchmark
benchmark:
	wrk -t12 -c400 -d30s -s benchmarks/envelope.lua http://localhost:8080/tunnel

.PHONY: benchmarks/self-hosted.lua
benchmarks/self-hosted.lua:
	wrk -t12 -c400 -d30s -s $@ http://localhost:8080/tunnel
