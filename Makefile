.PHONY: build test demo

build:
	go build -o writ ./cmd/writ

test:
	go test ./...

demo:
	scripts/record-demo/record.sh
