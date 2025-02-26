it:
	go mod tidy

run:
	go run cmd/sentrytunnel/sentrytunnel.go

debug:
	go run cmd/sentrytunnel/sentrytunnel.go --log-level=debug

build: bin/sentrytunnel

bin/sentrytunnel:
	go build -o bin/sentrytunnel cmd/sentrytunnel/sentrytunnel.go

clean:
	rm -rf bin
